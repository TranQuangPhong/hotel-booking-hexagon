Plan

Roadmap
1. Phase 1 - local: service APIs + UC1 create booking end to end   <- NOW
2. Phase 2 - local deployment & test of the full flow (no AWS Gateway, Cognito, Lambda)
3. Phase 3 - AWS: API Gateway, Cognito (+ Lambda sync user), SES, deployment
4. Phase 4 - CI/CD, monitoring, logging
5. Phase 5 - later optimizations: Redis lock / cache, CDC

Done
- User service: model, CRUD APIs, SQL, logging, tested
- Room service: room model, CRUD APIs, SQL, tested. Creating a room also opens 12 months of inventory
- UC1 detailed design: doc/uc1-create-booking/ (v1 lean, decisions Q1-Q8 confirmed)
- Refactor: folder structure, naming, contracts/ + platform/ modules scaffolded
- contracts/: envelope, messages, room / booking / payment proto + buf generated code
- M1 steps 1-3: contracts wired into room-service, reservations migration (exclusion constraint, price snapshot) + drop_all.sql,
  internal/inventory merged into internal/room
- M1 step 4: core (internal/room): Reservation + setRates (unit tested), Repository ports, Service.Reserve
- M1 step 5: Postgres adapter for all of room.Repository: ErrNotFound on GetByID / Update, CreateReservation with
  ON CONFLICT DO NOTHING (no target) + lookup by saga_id. GetInventories keeps ORDER BY year, month (setRates takes
  Currency from inventories[0])
- M1 step 6: gRPC server (adapter/grpc/server.go): ReserveRoom + toReservation / toResponse in the same file (one RPC,
  no mapper.go). Domain errors -> codes as in room.proto; INVALID_ARGUMENT carries the field, INTERNAL is logged
- M1 step 7: GRPC_PORT as config.GRPC{Port} (same shape as payment-service); main.go runs the gRPC server next to
  HTTP: recovery handler logs panic + stack and returns INTERNAL (v2's default sends UNKNOWN + stack, logs nothing),
  reflection for grpcurl (dev only), GracefulStop bounded by the 5 s shutdown ctx (context.AfterFunc -> Stop)
- M1 step 8 (2026-10-26): ReserveRoom tested with grpcurl + DB check: reserve, overlap -> ROOM_UNAVAILABLE, same saga_id ->
  same reservation_id + prices, zero-night / past check-in -> INVALID_ARGUMENT, unknown room -> NOT_FOUND. Along the way:
  gRPC access log (logger v1.1.0, grpclogger.UnaryServerInterceptor first in the chain; the handler still logs the INTERNAL
  cause), nightly_rates JSON date as YYYY-MM-DD like the proto (NightlyRate MarshalJSON / UnmarshalJSON), log errors as
  the value ("error", err)
- Step 9 (room admin API, outside M1; 2026-11-03): PATCH /rooms/api/v1/:id/inventories (DayPatch.apply with a table test,
  SaveInventories in one batch), hold checks -> 409, malformed id -> 400, taken number -> 409, unknown JSON fields -> 400,
  writeError: any other error is a logged generic 500. The races stay accepted (Notes: "Later"). Not run end to end:
  inventory endpoints are side functions, integration tested when major use cases involve them (decided 2026-11-04)
- UTC timestamps (2026-11-04): user + room main.go start with time.Local = time.UTC; every later service with a DB does too
- M2 step 1 (2026-11-07): booking-service wiring: logger v1.1.0 + replace contracts/platform, GRPC_PORT config, bookings
  indexes removed, drop_all.sql (bookings + both enums; the old nightly_rates table is dropped by hand once)
- M2 step 2 (2026-11-07): core model: one Booking (21 db-tagged fields, Detail gone), statuses = DB enum (+ EXPIRED),
  NightlyRate JSON as YYYY-MM-DD, errors.go, validate (every field rule + one rate per night + sum = total) with a table
  test and a JSON round-trip test. go build ./... already passes (handler + repository kept compiling)
- M2 steps 3-6 (2026-11-11): port GetByID / GetByUserID / Create (UpdateStatus gone); Create returns the stored *Booking
  (insert: RETURNING id, created_at, updated_at; saga_id conflict: getBySagaID), and neither service nor repo modifies
  its argument. gRPC CreateBooking: toBooking maps every field, any non-input error -> logged INTERNAL (no NOT_FOUND).
  REST: GET /:id only, own BookingResponse + NightlyRateResponse DTO, parseBookingID -> 400, writeError 404 / logged 500.
  booking.Booking has db tags only. Builds + go test passes; not run yet (needs step 7)
- M2 step 7 (2026-11-13): main.go same as room's (HTTP + gRPC, grpclogger before recovery, graceful stop of both),
  go-grpc-middleware/v2 direct after tidy; both main.go have the same commented interceptor chain. Not run yet (step 8)

UC1 build order (main line of each flow first, decided 2026-11-11; done-when per milestone: uc1 README.md section 6)
- M1: Room ReserveRoom gRPC (done 2026-10-26)
- M2: Booking CreateBooking gRPC (idempotent by saga_id)   <- NOW, steps below
- M3: Orchestrator phase A: saga migration + core + Save; POST /bookings -> ReserveRoom -> CreateBooking -> 201.
  No outbox, no Kafka
- M4a: infra/kafka, platform outbox (Insert + Poller) + consumer loop, errgroup main.go; deadline worker -> ReleaseRoom
  -> RoomReleased
- M4b: ExpireBooking -> BookingExpired -> saga FAILED
- M5: Payment: CreateCheckoutSession (Stripe hosted page, session expires after 31 min) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer
- M7: hardening: phase A error -> phase D at once, request checks (400), capture rejected, void a late authorization;
  then good-to-have.md
- Rule for M3-M6: an edge case met on the way gets one line under M7 (Notes), not code

Next step: M2 step 8 (run booking-service, grpcurl + Postman checks, doc sync)
Open decision (review round 2, 2026-11-12; decide before M5): starting payment extends the hold to the Stripe session's
  end (StartPayment's Save sets deadline_at = the session's expires_at + 1 min; CreateCheckoutSessionResponse gets
  expires_at). Today a user who opens the page at minute 10 and pays at minute 16 sees Stripe's success page and then an
  EXPIRED booking (the void only protects the money). Cost: a user who opens the page and leaves holds the room up to
  ~46 min, and bookings.expires_at goes stale (the client uses the expires_at of the payment response). Other fix: G1
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask

M2: Booking CreateBooking gRPC
booking-service predates the UC1 schema: create_tables.sql has the right columns, but the Go code still reads a nightly_rates
table, has old statuses, and the adapter doesn't implement its port. Same shape as room's M1

8. Done when (grpcurl + Postman)
    + Each gRPC call logs "gRPC request completed" with trace_id and saga_id (grpclogger is wired)
    + CreateBooking -> booking_id + PENDING; the same saga_id again -> same booking_id and no new row, even when the
      second request has other valid data
    + DB: nightly_rates has "date":"YYYY-MM-DD"; status and payment_status are PENDING
    + INVALID_ARGUMENT: malformed saga_id, no user, no expires_at, check-out = check-in, guests 0, no rates, one rate
      missing, prices that don't sum to the total
    + GET by id: stay and rate dates are YYYY-MM-DD, expires_at / created_at end in Z, no saga_id; unknown id -> 404,
      malformed -> 400
    + go test ./... passes
    + Docs (same pass): CLAUDE.md status row, naming example (GetDetailsByUserID; keep UpdateStatus: the M4b booking port has it again), go test line (+ booking-service
      internal/booking); contracts.md GET by id errors (+ 400 malformed id) and user.phone_number (may be empty); flows.md
      phase A (CreateBooking also takes reservationId + currency; rates live in the row); services.md and
      hexagonal-structure.md (no list route, errors.go); Postman (booking_url + booking get by id)

Why M2 (accepted 2026-11-04)
- One Booking struct: Detail only existed to join the old nightly_rates table. With JSONB there is nothing to join
- No GetBySagaID port in M2, unlike room: room must see the stored row first (RELEASED -> error), booking has no rule on a
  repeated call. M4b adds it: the command handlers read the row first (Notes, Participant ports)
- validate in the core: proto3 sends a forgotten field as 0 or "", so a forgotten total would be stored as 0
  (data-model.md: "a forgotten total is an error, not 0"). The sum check catches it: at least one night, every price > 0,
  sum = total, so there is no separate total check
- Phone may be empty: users has no phone column and the X-User-* headers don't require one; "" is stored, no NULL
- No GET list route: not in contracts.md and no UC1 caller. The old handler read a path param its route doesn't have.
  It comes back with UC2 / the FE
- No indexes on bookings yet: every UC1 lookup is by id or saga_id (PK, UNIQUE), the same reasoning as reservations in
  data-model.md. idx_bookings_user_id comes back with the list route
- GetByUserID stays in port + service (2026-11-11), no route: the user's booking history comes back later
- Create returns the stored *Booking, not (id, status) (2026-11-11): a repeated call must answer the stored status
  (EXPIRED, BOOKED), and later callers can read more fields without a signature change
- No layer modifies its argument (2026-11-11): service and repo copy b (nb := *b). Otherwise the caller's b holds the
  request on one path and the stored row on the other; always use the return value
- REST has its own DTOs, Booking has no json tags (2026-11-11): NightlyRate's JSON is the JSONB column format, so the
  API stays the same if the storage changes

Why design review (decided 2026-11-11; signatures are in Notes for later milestones)
- Thin milestones (M3-M6 = the main line only, M7 = hardening): after 7 weeks room + booking were polished while the
  orchestrator, payment, outbox and Kafka (the learning goals) were still stubs. M4 was the biggest milestone, so it is split
- Handlers use a conditional update, not SELECT ... FOR UPDATE: read without a lock -> the core decides -> one repository
  method does "UPDATE ... WHERE <still as read> + outbox row" in one tx. 0 rows changed = another writer was first: return
  an error and the consumer loop calls the handler again. The tx stays inside one port method, so no RunInTx and no
  GetForUpdate port; RunInTx is only for the room lock (Later)
- The saga is guarded by version, participants by status: POST .../payment saves payment_id while the step stays
  AWAITING_PAYMENT, so with a step guard a deadline worker that read the saga earlier would write its old copy back and
  erase payment_id. Keep saga_instances.version (data-model.md wanted to drop it). A participant's guard is its status
  column; the same UPDATE may set more (booking: payment_status, payment: psp_payment_intent_id)
- Deadline worker: a plain SELECT of overdue sagas, each one through Save; a second instance gets ErrStale and skips.
  FOR UPDATE SKIP LOCKED stays in the outbox poller only
- Kafka client: franz-go (pure Go; confluent-kafka-go needs cgo + a C toolchain on Windows)
- The consumer loop calls the handler again with the same record until it succeeds. Not committing is not a retry: the
  client keeps its read position in memory, so the loop still gets the next record, and committing that later record
  also marks the failed one as done. An uncommitted record comes back only when its partition is read again from the
  committed offset: after a restart, or when a rebalance gives the partition to another member
- The consumer loop is shared in platform/kafka, against "repeat over share": 5 services need the same loop and a wrong
  copy loses messages silently
- One outbox table in every service, the orchestrator too; saga_message_log is dropped from v1: its columns don't fit a
  shared poller (no msg_key, a direction, an enum type) and its audit rows are G8 anyway
- Outgoing messages are mapped in adapter/postgres (it writes the outbox row); adapter/kafka maps incoming ones only.
  Otherwise adapter/postgres would import adapter/kafka: one adapter depending on another
- outbox.Insert takes topic / key / payload bytes, not an envelope: platform never imports contracts
- Capture: only a definite Stripe refusal becomes CAPTURE_FAILED; everything else is retried by the consumer loop.
  CAPTURE_FAILED leaves a CONFIRMED room for a manual fix: too costly for a network blip. Definite = by HTTP status
  (400 / 402 / 404) and the PaymentIntent's status, not by error type: a rate limit (429) and a wrong API key (401) are
  typed invalid_request_error too (checked against Stripe's docs, 2026-11-11)
- No fixed Idempotency-Key on Stripe calls (flows.md had sagaId and sagaId-capture): Stripe saves the first answer per
  key, a 5xx included, and returns it again for 24 h or more, so a retry with that key can never succeed; and the same
  key with other parameters (ExpiresAt = now + 31 min differs per call) is a 400. A repeated call is made safe by the
  state instead: one payment row per saga, and a PaymentIntent can be captured only once
- saga_instances.failure_reason: ExpireBooking.reason is sent after RoomReleased, so the saga must remember the reason
  between the two; it also tells apart the three meanings of FAILED (rejected, released, needs a human)
- Late payment: the Stripe session expires 31 min after creation (M5) and a late authorization is voided (M7,
  CancelPayment). Replaces G1's design: no saga step before ReleaseRoom, no "cancelled before session existed" marker
- bookings.payment_status stays (the review first said drop): dropping touches 6 files of finished M2 code + 2 docs, and
  UC3 adds it back. Rule: only booking-service commands change it, in the same UPDATE as status
- CreateCheckoutSession gets its own gRPC timeout (10 s): it wraps a Stripe call. ReserveRoom / CreateBooking keep 3 s

Why (decided 2026-10-12)
- Price snapshot on the reservation: an admin will be able to change inventory prices, so a repeated call must return what was
  first quoted, not re-read inventory. JSONB because it is write-once and never queried by night
- Reserve reads the room: the response carries room_number + room_type, and an INACTIVE / ARCHIVED room still has AVAILABLE days
- Reserve without a tx in v1 (2026-10-15): its only write is one INSERT, and the constraints cover overlap, dates and a
  repeated saga_id. Accepted race: an admin change of room status / day status / price between the reads and the insert
  (reachable since step 9; still accepted, fix in Notes: "Later")
- ON CONFLICT DO NOTHING with no target (2026-10-19): with (saga_id) only, two identical calls at the same moment
  could fail on the overlap constraint (23P01, or 40P01 deadlock) and return ROOM_UNAVAILABLE for their own hold
- The orchestrator makes one call (ReserveRoom) and checks nothing itself
- Idempotent by saga_id: the same call or message arriving a second time (caller retry, Kafka redelivery) gives the same
  result. Nobody triggers a repeat and there is no API for it; test it by sending the same request twice

Why step 9 (decided 2026-10-25)
- No price on PUT /rooms: the price lives per day in inventories (no rooms.price, so GET can't show it), and with one
  writer of days a room edit can't flatten per-day prices. "Today -> last existing day" = PATCH from today to 2099-12-31
- No currency in the edit: one currency per room, fixed at creation, so E4 (two currencies in one stay) can't happen
- 409 on MAINTENANCE / INACTIVE / ARCHIVED over a hold: ConfirmReservation never re-reads the room or inventory, so the
  hold would still be confirmed and charged. INACTIVE / ARCHIVED checks today -> 9999-12-31: only stays not over yet
  count, because a reservation stays CONFIRMED after the stay
- HasReservation uses the exclusion constraint's expression + WHERE, so its GiST index serves it
- apply in the core (read -> apply -> one batch), not one jsonb UPDATE in SQL: the day rule stays in Go with a table test,
  and the batch saves all months or none on a database error
- Hold checks now, races later: the checks catch the case that happens locally (the hold already exists). Two races stay
  accepted (production-only, like the 2026-10-15 one): a Reserve in flight vs a status change (its insert lands after
  the admin's check), two admin edits on one month (the later batch erases the other's days). One fix: Notes, "Later"

Why time.Local = time.UTC (decided 2026-11-04)
- contracts.md promises RFC 3339 UTC, but pgx scans TIMESTAMPTZ in time.Local (+07:00 here). Same instant, only the form
  differs, and containers normally run in UTC anyway. TZ=UTC does nothing on Windows
- One line in main.go, not a shared platform/pgpool package (tried 2026-11-03, removed): too much code and a new
  user-service dependency for this. Cost: local logs show UTC too, and each main.go needs the line
- user-service stays on logger v1.0.0 (no gRPC server, so no grpclogger): the root package is identical in v1.1.0, and the
  bump only moves 6 indirect dependencies

Why phase numbers (decided 2026-11-03)
- Phase number = roadmap number. "Phase 2, AWS" was item 3, so docs said both phase 2 and phase 3 for the same step.
  API Gateway, Cognito, SES = Phase 3

Notes for later milestones
- Every handler from M3 on: read without a lock -> the core decides -> one conditional write (Why design review)
- M3: saga core (internal/saga)
    + transition(status Status, step Step, in Input) (out Outcome, ok bool): the table of state-machines.md, no I/O
      type Outcome struct { Status Status; Step Step; FailureReason string; Commands []Command }
      ok = false: in doesn't apply here (duplicate, late, lost a race); the caller logs and ignores it
    + a row that ends the saga changes Status and keeps Step (BookingExpired at EXPIRING_BOOKING -> FAILED, step still
      EXPIRING_BOOKING): COMPLETED and FAILED are never steps. With status COMPLETED or FAILED every input gives
      ok = false (M7 adds one exception: PaymentAuthorized)
    + inputs, one per table row: RoomReserved, RoomRejected, BookingCreated, DeadlinePassed, PaymentStarted (M5: POST
      .../payment; ok only at IN_PROGRESS / AWAITING_PAYMENT, same step, no commands; ok = false -> 409
      BOOKING_NOT_PAYABLE), PaymentAuthorized, ReservationConfirmed, PaymentCaptured, PaymentCaptureFailed, BookingConfirmed, RoomReleased, BookingExpired. HTTP,
      consumer and worker all go through transition. M7 adds RoomUnknown and BookingFailed (the two phase A error rows);
      until then the HTTP handler doesn't call transition for those two results
    + outside transition, on purpose: owner and now < deadline_at (StartPayment, before the gRPC call; its step check is
      the PaymentStarted input); deadline_at < now
      and status IN_PROGRESS (GetOverdue's SQL); copying an input's data (reservation, booking_id, payment_id) to the saga
    + type Command string: only which message. Its data (ids, reason) is read from the saga when the outbox row is written
    + test: the doc's table row by row + a double loop: every other (step, input) pair gives ok = false + each finished
      status once: every input gives ok = false
    + port saga.Repository
        Create(ctx context.Context, s *Saga) (*Saga, error): returns the stored row (saga_id, version 1, created_at come
          from the DB); s is not modified
        GetByID(ctx context.Context, sagaID string) (*Saga, error): ErrNotFound when there is none
        GetByBookingID(ctx context.Context, bookingID string) (*Saga, error): ErrNotFound when there is none (M5: POST
          .../payment)
        Save(ctx context.Context, s *Saga, commands []Command) (*Saga, error): s + commands (outbox rows) in one tx, only
          if the stored version is still s.Version. Returns the stored row (version + 1); s is not modified, so the
          caller continues with the returned saga (phase A saves twice). ErrStale: another writer saved first, nothing
          was written. M3 passes no commands; the outbox insert joins in M4a
        GetOverdue(ctx context.Context, now time.Time, limit int) ([]*Saga, error): IN_PROGRESS sagas in RESERVING_ROOM /
          CREATING_BOOKING / AWAITING_PAYMENT with deadline_at < now (M4a)
    + ErrStale: consumer -> return it, the loop calls the handler again from the read; deadline worker -> skip (the next
      poll reads again); StartBooking -> 503 (the worker released the saga during the request); StartPayment -> read the
      saga once more: still AWAITING_PAYMENT with the same payment_id -> 200, else 409 BOOKING_NOT_PAYABLE. StartPayment
      doesn't Save when the saga already has this payment_id (a page refresh writes nothing)
- M3: ReserveRoom error -> saga
    + definite rejection (NOT_FOUND, INVALID_ARGUMENT, FAILED_PRECONDITION) -> saga FAILED (failure_reason ROOM_REJECTED),
      nothing to release, reply 404 / 400 / 409
    + unknown outcome (timeout, UNAVAILABLE, INTERNAL) or any CreateBooking error -> reply 503 and save nothing: the saga
      stays IN_PROGRESS at its step, the M4a deadline worker releases the room at deadline_at, and from M4b a PENDING
      booking (if one was created) is expired. Until M4a exists the hold stays: rebuild the DB. Phase D at once = M7
- M3: the user comes from headers, read in adapter/handler (no port: reading a request is the adapter's job):
  X-User-Id (a UUID, else 400: saga_instances.user_id is UUID, so a bad value would fail the INSERT as a 500),
  X-User-Name, X-User-Email, X-User-Phone-Number (may be empty). The handler fills a core struct
  saga.User{ID, Name, Email, PhoneNumber string} and passes it to StartBooking / StartPayment. Phase 3: a Cognito
  middleware fills the same struct. The other request checks stay in M7
- M3: go.mod wiring first, like M2 step 1: replace contracts + platform, pgx, grpc, logger v1.1.0 (now v1.0.0)
- M3: saga_instances migration is behind data-model.md: add deadline_at (= ReserveRoom expires_at) + user_id UUID +
  idx_saga_deadline + failure_reason TEXT, reservation_id TEXT -> UUID; keep version. New drop_all.sql (the CREATE file
  changes, so the DB is rebuilt): saga_instances, saga_message_log, function set_updated_at, enums saga_status,
  saga_type, direction, message_type; M4a adds outbox to it
- M3: failure_reason is NULL while IN_PROGRESS and set in the Save that leaves the main line, never cleared: ROOM_REJECTED
  (M3, FAILED at RESERVING_ROOM), DEADLINE (M4a, the worker's Save to RELEASING_ROOM), PHASE_A_ERROR (M7; until then a
  phase A error is released by the worker and recorded as DEADLINE), CAPTURE_FAILED (M7). ReleaseRoom.reason and
  ExpireBooking.reason are read from it, so both are DEADLINE or PHASE_A_ERROR
- M3: orchestrator config has none of these yet: ROOM_GRPC_ADDR, BOOKING_GRPC_ADDR, GRPC_TIMEOUT (3s), HOLD_TTL (15m); later
  PAYMENT_GRPC_ADDR + PAYMENT_GRPC_TIMEOUT (10s) in M5, DEADLINE_POLL (10s) in M4a. Durations: time.ParseDuration. Each
  client call gets its own context.WithTimeout
- M3: deadline_at made in Go: time.Now().UTC().Add(HOLD_TTL), so the 201's expires_at ends in Z like later reads
- M4a: platform/outbox
    + type Message struct { ID, Topic, Key string; Payload []byte }: ID = the envelope's message_id, Key = saga_id,
      Payload = the envelope as JSON
    + Insert(ctx context.Context, tx pgx.Tx, m Message) error: one outbox row inside the caller's tx
    + Poller: reads the outbox table (same name and DDL in every service, no table-name parameter), cl.ProduceSync, then
      sets published_at. acks=all and the idempotent producer are franz-go's defaults. ProduceSync retries without end
      while the broker is down and the tx holds the row locks, so give it a ctx with a timeout; with several rows in
      one call check each result's Err (set published_at only for the sent ones), not only FirstErr. One kgo.Client
      per service can both produce and consume
    + migrations: create_saga_message_log.sql goes away; create_outbox_table.sql with the data-model.md DDL in
      orchestrator + room (M4a), booking (M4b), payment (M5): 4 copies in the end. Add DROP TABLE IF EXISTS outbox to
      room's and booking's drop_all.sql
- M4a: platform/kafka (franz-go)
    + type Handler func(ctx context.Context, rec *kgo.Record) error: nil = done (processed, duplicate or ignored), an
      error = call me again with the same record
    + Run(ctx context.Context, cl *kgo.Client, h Handler) error: PollFetches; if fetches.IsClientClosed() or ctx.Err() !=
      nil, return nil (a cancelled ctx arrives as a fetch error, not as a return value), so errgroup sees a clean stop.
      Every other fetch error (fetches.EachError) is logged and Run polls again. For each record in order: call h; while
      it returns an error, log it, wait (1 s, doubling up to 30 s; when ctx is done return without committing) and call h
      again with the same record; after nil, CommitRecords(ctx, rec). A CommitRecords error is only logged: the record
      was handled, it may come again after a restart, and handlers are idempotent
    + client options: kgo.SeedBrokers, kgo.ConsumerGroup, kgo.ConsumeTopics, kgo.DisableAutoCommit. Nothing else: franz-go
      heartbeats in the background and has no max.poll.interval, so a handler that retries for minutes keeps its
      membership (no BlockRebalanceOnPoll: it would drop the member after 60 s)
    + franz-go defaults to know: a new consumer group starts at the earliest offset; a consumer on a missing topic stays
      idle without an error, and a producer to one fails after a few retries
    + an error no retry can fix: the handler logs it at error level and returns nil. These are bad JSON, a type that
      isn't in contracts for this topic, a saga_id that isn't a UUID, and saga not found (old messages after a DB
      rebuild). Any other error (DB down, Stripe down) is retried without end and stops every topic of that client, not
      one partition
    + a known type this consumer has no use for is normal, not an error: return nil, log at debug at most
      (notification gets every BookingExpired)
    + the 6 topics are created in infra/kafka, not by auto-create
- Participant ports: room in M4a (shown), booking in M4b, payment in M5 (webhook) and M6 (CapturePayment)
    + UpdateReservationStatus(ctx context.Context, sagaID string, from, to ReservationStatus, event Event) (bool, error):
      UPDATE ... WHERE saga_id = $1 AND status = $from + the event's outbox row, in one tx. false: the reservation was not
      in from (or there is none), nothing was written
    + AddEvent(ctx context.Context, event Event) error: an outbox row alone. The reply to a command that arrived a second
      time, and RoomReleased with a nil reservation_id when the saga has no reservation
    + room.Event: a small interface with one unexported method (isEvent()), implemented by the structs
      ReservationConfirmed{SagaID, ReservationID string} and RoomReleased{SagaID string, ReservationID *string};
      adapter/postgres does a type switch and builds the envelope + contracts message
    + core handler, ReleaseRoom: r := GetReservationBySagaID. No reservation -> AddEvent(RoomReleased with a nil
      ReservationID). RELEASED -> AddEvent(RoomReleased) again. RESERVED -> UpdateReservationStatus(RESERVED -> RELEASED,
      RoomReleased); on false return an error, so the loop calls the handler again. CONFIRMED -> error log, return nil
    + core handler, ConfirmReservation: the same without the first case: no reservation -> error log, return nil
    + booking and payment name the pair UpdateStatus + AddEvent (Booking / Payment is the package's main entity; room
      keeps UpdateReservationStatus). The guard is always the status; the UPDATE may set more: booking's UpdateStatus
      also takes the payment status (BOOKED + COMPLETED, decided in the core), and payment adds
      MarkAuthorized(ctx context.Context, sagaID, paymentIntentID string, event Event) (bool, error): CREATED ->
      AUTHORIZED + psp_payment_intent_id + the outbox row in one tx. CapturePayment keeps its own rule for another
      status (state-machines.md Guards)
    + booking port, M4b
        GetBySagaID(ctx context.Context, sagaID string) (*Booking, error): nil, nil when there is none, like room's
          GetReservationBySagaID. The adapter's unexported getBySagaID becomes this (today it returns ErrNotFound; Create's
          conflict path then treats nil as an error)
        UpdateStatus(ctx context.Context, sagaID string, from, to Status, paymentStatus PaymentStatus, event Event)
          (bool, error): UPDATE ... SET status, payment_status WHERE saga_id = $1 AND status = $from + the outbox row
        AddEvent(ctx context.Context, event Event) error
        booking.Event: BookingConfirmed{Booking *Booking} (the row as read: the adapter builds the 12-field message
          from it, the snapshot fields never change) and BookingExpired{SagaID string, BookingID *string}
    + done when, M4a: an unpaid hold -> reservation RELEASED, saga COMPENSATING / EXPIRING_BOOKING with failure_reason
      DEADLINE, and an ExpireBooking message is in booking.commands (nobody consumes it yet). M4b: booking-service
      consumes it -> booking EXPIRED, BookingExpired -> saga FAILED
- M4a: main.go -> errgroup (hexagonal-structure.md section 5) once the Kafka consumer + outbox poller join HTTP + gRPC:
  one failing goroutine then stops the others and the defers run (pool.Close), instead of os.Exit in each goroutine
- M4a: envelope.OccurredAt is a plain string: time.Now().UTC().Format(time.RFC3339)
- M5: Checkout Session ExpiresAt = now + 31 min (Stripe's minimum is 30 min; 1 min spare for clock difference), created
  without a fixed Idempotency-Key (Why design review). A session whose answer was lost was never shown to the user and
  expires by itself. payments.saga_id UNIQUE keeps one row per saga: when the INSERT conflicts (two calls at once),
  return the stored row's session
- M5: grpcclient payment uses PAYMENT_GRPC_TIMEOUT; payment svc passes the request ctx to stripe-go, so the Stripe call
  stops at the caller's deadline
- M5: webhook: stripe-go's ConstructEvent rejects an event whose API version differs from the SDK's. Locally run
  stripe listen --latest --forward-to ...; if it still reports a mismatch, use webhook.ConstructEventWithOptions with
  IgnoreAPIVersionMismatch: true
- M6: payment gateway port: Capture(ctx context.Context, paymentIntentID string) error, no idempotency key. The Kafka
  handler has no caller deadline, so it gives the call its own timeout. In M6 the Stripe adapter already treats
  "already captured" as done: on a *stripe.Error read the PaymentIntent's status (stripeErr.PaymentIntent, else
  Retrieve), succeeded = return nil. Without it a crash or DB error between Stripe's capture and our UPDATE loops
  forever, because the second capture always fails. Any other error is returned and the loop retries; the rejected
  case is M7. Until M7 a refused capture is retried forever and blocks the consumer (locally: stop the service and
  rebuild the DB). Check the exact error of a second capture once in the sandbox
- M5 / M6: go.mod wiring first, like M2 step 1: payment in M5 (replace contracts + platform, pgx, grpc, stripe-go,
  logger v1.1.0), notification in M6 (replace contracts + platform). M5: new payment-service drop_all.sql (payments,
  outbox, enum payment_state)
- M7: hardening list (add one line per edge case met in M3-M6)
    + phase A unknown outcome or CreateBooking error -> tx: COMPENSATING / RELEASING_ROOM + outbox ReleaseRoom
      (failure_reason PHASE_A_ERROR), reply 503
    + check the request before the saga starts: name + email not empty, guests > 0, else 400 (the user id check is
      already in M3). These come
      from the client and only CreateBooking checks them, so a bad header would hold the room, fail with INVALID_ARGUMENT
      and answer 503, not contracts.md's 400
    + capture rejected: var ErrCaptureRejected = errors.New("capture rejected by the PSP") in payment/gateway.go. Core
      Capture then does AUTHORIZED -> CAPTURE_FAILED + PaymentCaptureFailed, and the saga ends FAILED (failure_reason
      CAPTURE_FAILED). Test with a fake Gateway: capture ok / rejected / transient / already captured
    + the Stripe adapter decides by HTTP status (var stripeErr *stripe.Error; errors.As; stripeErr.HTTPStatusCode), not by
      error type. Returned as it is, so the loop retries: an error that isn't a *stripe.Error (network, timeout), or
      status 401, 403, 409, 429, >= 500. Definite: 400, 402, 404; there the M6 status read comes first
      (succeeded = already captured, return nil); anything else = ErrCaptureRejected
    + void a late authorization
        contracts payment/v1: TypeCancelPayment = "CancelPayment", type CancelPayment struct{} (topic payment.commands)
        transition: PaymentAuthorized while the step is RELEASING_ROOM or EXPIRING_BOOKING (status COMPENSATING or FAILED)
          -> command CancelPayment, status and step unchanged. At the steps CONFIRMING_ROOM, CAPTURING_PAYMENT and
          CONFIRMING_BOOKING (any status) a PaymentAuthorized is a duplicate: ok = false, never void a good payment
        payment: status CANCELLED; gateway Cancel(ctx context.Context, paymentIntentID string) error. Handler: AUTHORIZED
          -> Cancel -> CANCELLED, no event (the saga is already over); already CANCELLED -> do nothing, return nil
        the Stripe adapter's Cancel treats "already canceled" as done: on HTTP 400 read the PaymentIntent's status,
          canceled = return nil. Network errors, 409, 429 and 5xx are returned and the loop retries, as for capture
    + a repeated ConfirmBooking sends BookingConfirmed again with a new message_id, so notification logs twice. Later
      (SES): dedupe by booking_id
- Later, Phase 3: the DB URL built with fmt.Sprintf breaks on a password with @, / or # (generated DB passwords), and the
  parse error shows part of it. Fix: url.URL{Scheme: "postgres", User: url.UserPassword(user, password), ...}
- Later, Phase 3: GET /bookings/api/v1/{id} returns name, email and phone to anyone holding the id, while POST .../payment
  checks the owner. Add the same check with Cognito: X-User-* headers can be faked, so a check in v1 proves little
- Later, before production (2026-10-25; was "when the admin inventory-edit API exists"): room lock + Reserve in one tx.
  Closes the step 9 races: a Reserve in flight vs a status change, two admin edits on one month
    + port: RunInTx(ctx context.Context, fn func(txRepo Repository) error) error. The core passes the steps,
      the adapter runs Begin, fn(the struct bound to the tx), Commit (the struct holds a small query interface
      that both the pool and a tx satisfy). Inside fn use txRepo, never s.repository: that runs outside the tx
    + port: GetByIDForShare / GetByIDForUpdate(ctx context.Context, id string) (*Room, error): GetByID + FOR SHARE /
      FOR UPDATE, only inside RunInTx
    + one lock per room, the rooms row: Reserve reads it FOR SHARE (two Reserve calls never block each other), both
      admin endpoints (PUT /rooms, PATCH .../inventories) FOR UPDATE (two admin edits wait for each other). Rule:
      whatever changes room status or days takes it first; then the inventory reads need no lock of their own
    + admin order: lock -> HasReservation -> write. Checking before the lock brings the race back: the check misses a
      hold that commits while the admin waits for the lock
    + the FK on reservations.room_id doesn't do this: its FOR KEY SHARE doesn't conflict with an UPDATE that keeps the key
    + M4a / M4b message handlers don't use RunInTx (decided 2026-11-11: conditional update, Why design review)
    + test by hand with two sessions (a mocked unit test has no tx or lock, so it can't show this). psql session A plays
      the admin: BEGIN; SELECT id FROM rooms WHERE id = '<room>' FOR UPDATE; -> grpcurl ReserveRoom hangs (its FOR SHARE
      waits) -> A: UPDATE rooms SET status = 'INACTIVE' ...; COMMIT; -> ReserveRoom returns ROOM_UNAVAILABLE (READ
      COMMITTED: a FOR SHARE that waited reads the row as A committed it). Two admin edits: Postman PATCH instead of
      grpcurl, it waits for A's COMMIT. Before the fix the same script shows the race (Reserve doesn't wait). A Go
      integration test (real DB, two txs + timeout) is later: phase 1 has no test DB

- After UC1, before the FE: availability + capacity (the price is first seen in the 201 today, after the room is held)
    + GET /rooms/api/v1/availability?check_in_date=&check_out_date=&number_of_guests= -> rooms with total_amount, currency,
      nightly_rates. A quote, not a promise: ReserveRoom still locks the price
    + core: quote(checkIn, checkOut time.Time, inventories []*Inventory) ([]NightlyRate, int64, string, error): the pricing
      loop moved out of Reservation.setRates, which then calls it (reservation.go + its test)
    + port: GetAvailable(ctx context.Context, checkIn, checkOut time.Time) ([]*Room, error): ACTIVE rooms with NOT EXISTS an
      overlapping RESERVED / CONFIRMED hold, same daterange expression as the constraint so its GiST index serves it
    + rooms.capacity INT NOT NULL + number_of_guests in ReserveRoomRequest: room svc rejects more guests than capacity
    + later: the client sends the total it saw, 409 PRICE_CHANGED when ReserveRoom's total differs (compared, never used)
- Later (E6): tombstone for a hold that commits after its release
    + in reservations itself, not a separate table: under READ COMMITTED neither tx sees the other's uncommitted row, so
      both would commit. They must collide on one unique key (saga_id)
    + ReleaseRoom data gets room_id, check_in_date, check_out_date (from the saga context). Release, one tx with the
      outbox row: UPDATE RESERVED -> RELEASED; 0 rows -> INSERT a RELEASED row ON CONFLICT (saga_id) DO NOTHING (total 0,
      nightly_rates '[]', currency XXX, expires_at now()); 0 rows again -> a row with this saga_id just committed: run
      the UPDATE again (1 row = the hold, now released; 0 rows = already RELEASED by a repeated ReleaseRoom). Reply
      RoomReleased in every case
    + the INSERT fails on the room_id FK or check_dates when the saga has an unknown room or bad dates: treat 23503 /
      23514 as nothing to release (a hold with these values can't be inserted either), or the loop retries forever
    + existing code: CreateReservation returns the stored row on a conflict without looking at its status, so Reserve must
      check Released after CreateReservation too (room/service.go), not only before
    + E1 (booking) stays open: an orphan PENDING booking holds no room and no money

Cleanup (any time)
- plan.md: trim Done to one line per milestone (git has the detail)
- contracts/envelope: round-trip test (skipped in step 1, do it once the real gRPC call works)
- user-service handlers + repository, same as room's step 9 fixes: code done (2026-11-03), left: run by hand in Postman:
  malformed id, unknown JSON field, bad role, malformed email -> 400; unknown id on GET / PUT -> 404; taken email on
  POST / PUT -> 409, also when only the case differs (the service stores email lowercase); PUT with its own email -> 200; timestamps on every response are real and end in Z (room's too),
  updated_at moves on PUT; empty table -> []. The saved "user update" id (356e528f...) isn't in the seed: use an id that
  exists, or it's a correct 404

Code changes from the doc sync (2026-11-11: the docs now describe these, the code doesn't yet; for me to write)
- Comments only, any time
    + orchestrator-service/internal/saga/repository.go: says "Create, GetForUpdate, Save" and "saga_message_log, the
      outbox" -> Create, GetByID, GetByBookingID, Save, GetOverdue (Notes, M3); Save writes the outbox table
    + booking-service and payment-service internal/adapter/kafka/mapper.go: "to and from ... domain types" -> incoming
      messages only; outgoing events are mapped in adapter/postgres
    + contracts/booking/v1/messages.go: ExpireBooking.Reason gets the comment //DEADLINE, PHASE_A_ERROR (the same values
      as ReleaseRoom.Reason in room/v1/messages.go)
- With their milestone (signatures and rules: Notes for later milestones)
    + M3: orchestrator-service/db/migration/create_saga_instances.sql (deadline_at, user_id UUID, failure_reason,
      reservation_id UUID, idx_saga_deadline; keep version) + a new drop_all.sql; config/config.go + .env.example (the
      new variables); go.mod wiring
    + M4a: delete orchestrator-service/db/migration/create_saga_message_log.sql; create_outbox_table.sql in orchestrator +
      room; room's and orchestrator's drop_all.sql; platform/outbox + platform/kafka (pgx and franz-go go into
      platform/go.mod)
    + M4b: create_outbox_table.sql in booking; booking's drop_all.sql; booking port GetBySagaID / UpdateStatus / AddEvent
    + M5: payment-service/db/migration/create_payments_table.sql (the real DDL from data-model.md; today it is only a
      comment) + create_outbox_table.sql + drop_all.sql; go.mod wiring
    + M6: notification-service go.mod wiring
    + M7: contracts/payment/v1/messages.go (TypeCancelPayment, CancelPayment struct{}) and doc.go (payment.commands:
      + CancelPayment); payment-service internal/adapter/kafka/consumer.go comment (+ CancelPayment); payment_state
      + CANCELLED in create_payments_table.sql

Docs to sync
- Nothing open. Synced 2026-11-11: uc1 README, flows, state-machines, data-model, contracts, good-to-have,
  hexagonal-structure, services, use-cases, CLAUDE.md. architecture_uc_create_order.excalidraw was checked: it draws
  only the main line (no deadline, release or void arrows), so nothing in it became wrong
- The docs listed in M2 step 8 are not part of this sync: they stay with that step

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
