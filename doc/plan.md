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
  go-grpc-middleware/v2 direct after tidy; both main.go have the same commented interceptor chain
- M2 step 8 (2026-11-15): run + verified with grpcurl, curl and DB checks: every done-when item passes, plus 9 extra
  INVALID_ARGUMENT cases (rate date out of order / duplicate, price 0, currency, room, ids, email). Along the way: missing
  expires_at -> "expires_at: missing" (not the proto nil-Timestamp text), core errors name the field by its flat wire name
  ("number_of_guests: must be > 0", "nightly_rates[1].date: ..."). Docs synced (CLAUDE.md, contracts, flows, services,
  hexagonal-structure, Postman). M2 done
- M3 step 1 (2026-11-16): orchestrator wiring: logger v1.1.0 + replace contracts/platform; config Clients (room +
  booking addr, GRPC_TIMEOUT) + Saga{HoldTTL}, no GRPC_PORT (no gRPC server); durations via time.ParseDuration,
  Load fails on an empty address or a duration <= 0; orchestrator DB + pg-orchestrator MCP
- M3 step 2 (2026-11-16): create_saga_instances.sql + drop_all.sql, applied to the orchestrator DB and tested in a
  rolled-back tx (every constraint, the version guard, the trigger, COALESCE / NULLIF, EXPLAIN of GetOverdue). Beyond
  the plan: failure_reason is an enum with check_failure_reason, idx_saga_deadline also filters GetOverdue's 3 steps,
  no DEFAULT on status / context, saga_type = CREATE_BOOKING only, booking_id UUID UNIQUE, type / status unquoted
- M3 review (2026-11-16): user_id, reservation_id, payment_id columns dropped (ids live in context; Why M3), DB rebuilt.
  Removed before their callers: StartPaymentTxn + /payment route, commented UC2 / UC3 routes, KAFKA_BROKERS config,
  placeholder files grpcclient/payment.go, kafka/, worker/ (the target tree stays in hexagonal-structure.md)
- M3 step 3 (2026-11-17): saga.go (Saga with db tags, Context with snake_case json tags = the stored format, omitempty
  on the user / room fields and on what ReserveRoom fills; enums named <Type><Value>, values = the SQL ENUMs, no
  IsValid), newSaga, errors.go. saga_test.go: newSaga + Context JSON as an exact string (Go decodes keys
  case-insensitively, so a round trip alone misses a missing tag)
- M3 step 4 (2026-11-18): transitions.go (Input, outcome, row; transition = the 3 phase A rows in one flat switch; next =
  a copy with out applied). transitions_test.go: the rows as wantRows map[row]outcome, compared whole (DeepEqual: M4a
  adds commands); every other (status, step, input) -> ok = false, and the all* lists must reach every row; next: s
  unchanged, every other field copied, sets / keeps the reason. Each check was seen failing on a broken copy
- M3 steps 5-6 (2026-11-19): ports (Repository = Create + Save only, the rest with their callers; RoomClient,
  BookingClient, Reservation) and StartBooking + advance(ctx, sg *Saga, in, fill) (fill may be nil). Review fixes: fill
  writes its own parameter (the closures wrote the old saga, so the reservation and booking_id never reached Save),
  room number / type copied, only a definite rejection (isRoomRejection in errors.go) saves FAILED / ROOM_REJECTED and
  is returned unwrapped (it is the 400 body); an unknown outcome saves nothing; a failed rejection Save is logged with
  slog.ErrorContext (the logger adds trace_id from ctx); other errors wrapped with %w. service_test.go: the 4 planned
  cases + a Save error after RoomReserved + advance with ok = false; each check seen failing on a broken copy
- M3 step 7 (2026-11-27): adapter/postgres SagaRepository. Create = INSERT of the 5 columns; Save = UPDATE ... WHERE
  saga_id AND version, 0 rows -> ErrStale (no lock needed: a second UPDATE waits for the first, then Postgres re-checks
  the WHERE on the new row). One sagaColumns list for every RETURNING / SELECT: COALESCE(x::text, '') on read,
  NULLIF($n, '')::type on write. One error message per method (server errors arrive in CollectOneRow, not Query).
  pgx direct in go.mod. Builds; not run against the DB yet (step 12)

UC1 build order (main line of each flow first, decided 2026-11-11; done-when per milestone: uc1 README.md section 6)
- M1: Room ReserveRoom gRPC (done 2026-10-26)
- M2: Booking CreateBooking gRPC, idempotent by saga_id (done 2026-11-15)
- M3: Orchestrator phase A: saga migration + core + Save; POST /bookings -> ReserveRoom -> CreateBooking -> 201.
  No outbox, no Kafka   <- NOW, steps below
- M4a: infra/kafka, platform outbox (Insert + Poller) + consumer loop, errgroup main.go; deadline worker -> ReleaseRoom
  -> RoomReleased
- M4b: ExpireBooking -> BookingExpired -> saga FAILED
- M5: Payment: CreateCheckoutSession (Stripe hosted page, session expires after 31 min) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer
- M7: hardening: phase A error -> phase D at once, request checks (400), capture rejected, void a late authorization;
  then good-to-have.md
- Rule for M3-M6: an edge case met on the way gets one line under M7 (Notes), not code

Next step: M3 step 8 (gRPC clients)
Open decision (review round 2, 2026-11-12; decide before M5): starting payment extends the hold to the Stripe session's
  end (StartPayment's Save sets deadline_at = the session's expires_at + 1 min; CreateCheckoutSessionResponse gets
  expires_at). Today a user who opens the page at minute 10 and pays at minute 16 sees Stripe's success page and then an
  EXPIRED booking (the void only protects the money). Cost: a user who opens the page and leaves holds the room up to
  ~46 min, and bookings.expires_at goes stale (the client uses the expires_at of the payment response). Other fix: G1
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask

M3: Orchestrator phase A (POST /bookings -> ReserveRoom -> CreateBooking -> 201)
orchestrator-service has its config, saga_instances, core model, transition table, ports, StartBooking and the
Postgres adapter (steps 1-7, see Done; M5 adds Context.PaymentID); main.go is empty and the other adapters are stubs.
No outbox, Kafka or worker yet. Same shape as M1 / M2, plus two gRPC clients. The rules behind the choices: Why M3

8. gRPC clients (adapter/grpcclient; the package implies the client, so Room / Booking)
    + first: go mod edit "-require=booking/contracts@v0.0.0", then go mod tidy (tidy removes a require nothing imports,
      so it waits for this first import; step 1 added the replace)
      type Room struct { client roomv1.RoomServiceClient; timeout time.Duration }
      func NewRoom(conn *grpc.ClientConn, timeout time.Duration) *Room
      var _ saga.RoomClient = (*Room)(nil)
      func (r *Room) Reserve(ctx context.Context, s *saga.Saga) (*saga.Reservation, error)
          ctx, cancel := context.WithTimeout(ctx, r.timeout); defer cancel(); expires_at = timestamppb.New(s.DeadlineAt)
          error: st, ok := status.FromError(err); ok = false -> unknown. NotFound -> ErrRoomNotFound; InvalidArgument ->
          wraps ErrInvalidInput and keeps room's message (it names the field, for the 400); FailedPrecondition -> wraps
          ErrRoomUnavailable with the message (RESERVATION_RELEASED can't happen: a new saga_id per request); every other
          code (DeadlineExceeded, Unavailable, Internal...) -> returned wrapped = unknown outcome
      type Booking struct { client bookingv1.BookingServiceClient; timeout time.Duration }
      func NewBooking(conn *grpc.ClientConn, timeout time.Duration) *Booking
      var _ saga.BookingClient = (*Booking)(nil)
      func (b *Booking) Create(ctx context.Context, s *saga.Saga) (bookingID, status string, err error)
          maps every CreateBookingRequest field from s; expires_at = s.DeadlineAt, so the reservation, the booking and
          the 201 share one deadline
      doc.go: "to and from the contracts/<svc>/v1 protos" stays true
9. REST (adapter/handler)
    + dto.go
      type CreateBookingRequest struct { RoomID, CheckInDate, CheckOutDate string; NumberOfGuests int } // snake_case json
      func (r CreateBookingRequest) ToStartRequest(u saga.User) saga.StartRequest
      type CreateBookingResponse struct {
          BookingID   string    `json:"booking_id"`
          Status      string    `json:"status"`
          TotalAmount int64     `json:"total_amount"`
          Currency    string    `json:"currency"`
          ExpiresAt   time.Time `json:"expires_at"` // ends in Z: deadline_at is made with time.Now().UTC()
      }
      func toCreateBookingResponse(st *saga.Started) CreateBookingResponse
    + handler.go
      type Handler struct { svc *saga.Service }
      func New(s *saga.Service) *Handler
      func (h *Handler) CreateBooking(c *gin.Context)
          userFromHeaders -> ShouldBindJSON (400) -> ctx := context.WithoutCancel(c.Request.Context()) (Why M3) ->
          StartBooking -> 201, or writeError
      // userFromHeaders: X-User-Id must be a UUID (canonical form) else ErrInvalidInput; X-User-Name, X-User-Email,
      // X-User-Phone-Number as sent (their checks: M7). Phase 3: a Cognito middleware fills the same saga.User
      func userFromHeaders(c *gin.Context) (saga.User, error)
      // ErrInvalidInput -> 400 {"error": err.Error()}; ErrRoomNotFound -> 404 {"error": "ROOM_NOT_FOUND"};
      // ErrRoomUnavailable -> 409 {"error": "ROOM_UNAVAILABLE"}; else logged -> 503 {"error": "service unavailable"}
      func writeError(c *gin.Context, err error)
    + router.go: /health + POST /bookings (StartPaymentTxn and its route already removed; M5 adds them back)
10. cmd/main.go, same as booking's without the gRPC server: time.Local = time.UTC, slog.SetDefault, config.Load,
    pgxpool.New + Ping, gin.EnableJsonDecoderDisallowUnknownFields() (like room), one grpc.NewClient per participant
    with insecure credentials (defer conn.Close()), repo -> grpcclient.NewRoom / NewBooking -> saga.NewService ->
    handler.New -> HTTP server, graceful shutdown (5 s). grpc.NewClient doesn't connect at start: the orchestrator
    starts while room is down, and the first call gets UNAVAILABLE -> 503. Then go mod tidy
11. Optional (or later): one trace_id across the three services. Logger v1.2.0 adds grpclogger.UnaryClientInterceptor():
    puts logger.GetTraceID(ctx) into the outgoing metadata x-trace-id, the key the server interceptor already reads;
    main.go passes grpc.WithChainUnaryInterceptor(...) to both clients. Without it each service logs its own trace_id
    and the saga_id still links the three logs
12. Done when (Postman + DB; room + booking running, a room with inventory)
    + 201: booking_id, status PENDING, total_amount, currency, expires_at ending in Z at about now + 15 min
    + DB: saga IN_PROGRESS / AWAITING_PAYMENT, version 3, booking_id set, context has user, room, dates,
      reservation_id and rates, failure_reason NULL. reservation RESERVED and booking PENDING carry this saga_id and its
      deadline_at as expires_at
    + unknown room -> 404 ROOM_NOT_FOUND; the same room and dates again -> 409 ROOM_UNAVAILABLE; malformed check_in_date
      or check-in in the past -> 400 with room's message. Each: saga FAILED / RESERVING_ROOM, ROOM_REJECTED, version 2
    + X-User-Id missing or not a UUID, malformed JSON, an unknown JSON field -> 400 and no saga row
    + room-service stopped -> 503, saga IN_PROGRESS / RESERVING_ROOM. booking-service stopped -> 503, saga at
      CREATING_BOOKING with context.reservation_id, reservation RESERVED (the hold stays until M4a: rebuild the room DB)
    + number_of_guests 0 -> 503 (CreateBooking INVALID_ARGUMENT; M7 makes it 400 before the saga starts)
    + logs: "HTTP request completed" in the orchestrator, "gRPC request completed" with the saga_id in room and booking
    + go test ./... passes (transitions, service)
    + Docs (same pass): CLAUDE.md orchestrator row, go test line, Commands (the orchestrator needs room + booking
      running); contracts.md (error body {"error": "ROOM_NOT_FOUND"});
      hexagonal-structure.md orchestrator tree (errors.go; worker and kafka are M4a); Postman (orchestrator_url + create
      booking with the X-User-* headers)

Why M3 (proposed 2026-11-15)
- Dates and rate dates stay YYYY-MM-DD strings in the saga: the orchestrator never computes with them (room checks
  them, booking checks the rates), and the context JSONB gets the same format as reservations and bookings without a
  third MarshalJSON
- Columns = the saga's own state (status, step, failure_reason, version, deadline_at) + what SQL reads (booking_id:
  GetByBookingID, UNIQUE); everything else in context (2026-11-16, like Eventuate's saga_data_json). So no user_id,
  reservation_id or payment_id column: nothing looks a saga up by them, and each participant row carries saga_id.
  The owner check on POST .../payment compares X-User-Id with Context.User.ID in Go; the handler still rejects a
  non-UUID X-User-Id with 400. A lookup by one of them, if ever: an expression index on context (later)
- Context nests room like user (2026-11-16): the same shape as CreateBookingRequest's Room {id, number, type}, so the
  booking client maps it 1:1, and the JSONB reads "room": {...} next to "user": {...} (nested objects drop the owner)
- "" for booking_id / failure_reason not known yet, not *string: the core only compares with "", and the NULL mapping
  is COALESCE / NULLIF in one adapter file. room's RoomReleased keeps *string: there the JSON message must carry null
- saga_step and saga_failure_reason are Postgres ENUMs, like every status column (CLAUDE.md): with TEXT a typo would be
  stored (a step that never matches a row of the table again, a reason sent on in ReleaseRoom)
- check_failure_reason (2026-11-16): failure_reason IS NULL exactly while status is IN_PROGRESS or COMPLETED. One line
  of SQL stops a transition bug at the write, not in a later ReleaseRoom; UC2 may revisit it for its own saga type
- idx_saga_deadline also filters the 3 pre-payment steps (2026-11-16): the same WHERE as GetOverdue, so the index holds
  only the sagas the worker scans. GetOverdue writes that WHERE with literals: with $n, a generic plan (pgx caches
  prepared statements) can't prove the index predicate and may scan the table (EXPLAIN on the real table, step 2)
- No DEFAULT on status and context (2026-11-16): Create always sets them, so a forgotten one is an error, like the
  price snapshot of reservations. saga_type is CREATE_BOOKING only until UC2 / UC3 are designed (ALTER TYPE ADD VALUE)
- The port and the transition table grow with their callers: M3 = Create + Save(ctx, s) and the 3 phase A rows; M4a
  adds Command, outcome.commands and Save's commands parameter with the outbox, M5 the /payment route and PaymentClient.
  The other way, the whole table now: pure and cheap to test, but inputs without callers and nothing to run them
- context.WithoutCancel in the handler: a client that disconnects after ReserveRoom would cancel CreateBooking and the
  Saves, and the saga would stop halfway for no reason. Values survive (trace_id); each call keeps its own timeout, so
  the request stays bounded (2 x GRPC_TIMEOUT + DB)
- A rejection whose Save fails still answers the rejection: nothing is held, and the M4a worker's ReleaseRoom finds no
  reservation (RoomReleased with a nil reservation_id)
- No idempotency on POST /bookings in v1: a retry after a 503 starts a second saga, and a double click gets 409 from the
  exclusion constraint (G2, Idempotency-Key)
- Saga enums without IsValid() (2026-11-17): no value comes from outside, and the Postgres ENUM rejects a wrong one at
  the write. CLAUDE.md now asks for IsValid only on outside input (room, user). Names are <Type><Value>
  (StatusInProgress): four enums share one package, and ALL_CAPS would make InputRoomRejected clash with ROOM_REJECTED
- transition is one flat switch on row{status, step, in} (2026-11-17): one case = one line of the table, so a row
  valid in two statuses (M7's late PaymentAuthorized) is two cases, not an inner status check. Keyed outcome literals:
  M4a's commands field then only touches the rows that send one. outcome and its fields are unexported: only the core
  reads them

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
- Saga core (internal/saga), every milestone; M3 builds the phase A part (M3 steps 3-6)
    + transition(status Status, step Step, in Input) (out outcome, ok bool): the table of state-machines.md, no I/O,
      one flat switch on row{status, step, in}
      type outcome struct { status Status; step Step; failureReason FailureReason; commands []Command }
      ok = false: in doesn't apply here (duplicate, late, lost a race); a consumer logs and acks
    + a row that ends the saga changes Status and keeps Step (BookingExpired at EXPIRING_BOOKING -> FAILED, step still
      EXPIRING_BOOKING): COMPLETED and FAILED are never steps. With status COMPLETED or FAILED every input gives
      ok = false (M7 adds one exception: PaymentAuthorized)
    + inputs, one per table row: RoomReserved, RoomRejected, BookingCreated, DeadlinePassed, PaymentStarted (M5: POST
      .../payment; ok only at IN_PROGRESS / AWAITING_PAYMENT, same step, no commands; ok = false -> 409
      BOOKING_NOT_PAYABLE), PaymentAuthorized, ReservationConfirmed, PaymentCaptured, PaymentCaptureFailed, BookingConfirmed, RoomReleased, BookingExpired. HTTP,
      consumer and worker all go through transition. M7 adds RoomUnknown and BookingFailed (the two phase A error rows);
      until then the HTTP handler doesn't call transition for those two results
    + outside transition, on purpose: owner and now < deadline_at (StartPayment, before the gRPC call; its step check is
      the PaymentStarted input); deadline_at < now, status IN_PROGRESS and the 3 pre-payment steps (GetOverdue's SQL);
      copying an input's data (reservation, booking_id, payment_id) to the saga
    + type Command string: only which message. Its data (ids, reason) is read from the saga when the outbox row is written
    + test (transitions_test.go): the doc's table as wantRows map[row]outcome, compared whole + one loop over every
      status, step and input: every other combination gives ok = false (the finished statuses too). Each milestone
      adds its rows to wantRows and its new values to the all* lists
    + port saga.Repository
        Create(ctx context.Context, s *Saga) (*Saga, error): returns the stored row (saga_id, version 1, created_at come
          from the DB); s is not modified
        GetByID(ctx context.Context, sagaID string) (*Saga, error): ErrNotFound when there is none
        GetByBookingID(ctx context.Context, bookingID string) (*Saga, error): ErrNotFound when there is none (M5: POST
          .../payment)
        Save(ctx context.Context, s *Saga, commands []Command) (*Saga, error): s + commands (outbox rows) in one tx, only
          if the stored version is still s.Version. Returns the stored row (version + 1); s is not modified, so the
          caller continues with the returned saga (phase A saves twice). ErrStale: another writer saved first, nothing
          was written. M3 has Save(ctx, s); M4a adds the commands parameter with the outbox insert
        GetOverdue(ctx context.Context, now time.Time, limit int) ([]*Saga, error): IN_PROGRESS sagas in RESERVING_ROOM /
          CREATING_BOOKING / AWAITING_PAYMENT with deadline_at < now (M4a). The WHERE repeats idx_saga_deadline's
          predicate with literals; only now and limit are $n (Why M3)
    + ErrStale: consumer -> return it, the loop calls the handler again from the read; deadline worker -> skip (the next
      poll reads again); StartBooking -> 503 (the worker released the saga during the request); StartPayment -> read the
      saga once more: still AWAITING_PAYMENT with the same payment_id -> 200, else 409 BOOKING_NOT_PAYABLE. StartPayment
      doesn't Save when the saga already has this payment_id (a page refresh writes nothing)
- failure_reason, every milestone: NULL while IN_PROGRESS and set in the Save that leaves the main line, never
  cleared: ROOM_REJECTED (M3, FAILED at RESERVING_ROOM), DEADLINE (M4a, the worker's Save to RELEASING_ROOM), PHASE_A_ERROR (M7; until then a
  phase A error is released by the worker and recorded as DEADLINE), CAPTURE_FAILED (M7). ReleaseRoom.reason and
  ExpireBooking.reason are read from it, so both are DEADLINE or PHASE_A_ERROR
- Config after M3: KAFKA_BROKERS (required: an empty value is an error, not [""]) + DEADLINE_POLL (10s) in M4a, PAYMENT_GRPC_ADDR + PAYMENT_GRPC_TIMEOUT (10s) in M5
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
      the orchestrator's, room's and booking's drop_all.sql (the orchestrator's also loses its saga_message_log,
      direction and message_type lines)
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
    + booking-service and payment-service internal/adapter/kafka/mapper.go: "to and from ... domain types" -> incoming
      messages only; outgoing events are mapped in adapter/postgres
    + contracts/booking/v1/messages.go: ExpireBooking.Reason gets the comment //DEADLINE, PHASE_A_ERROR (the same values
      as ReleaseRoom.Reason in room/v1/messages.go)
- With their milestone (signatures and rules: Notes for later milestones)
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
- Synced 2026-11-15 (M2 step 8): CLAUDE.md, contracts, flows, services, hexagonal-structure

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
