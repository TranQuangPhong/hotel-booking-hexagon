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
- UC1 detailed design: doc/uc1-create-booking/ (v1 lean, decisions Q1-Q4 confirmed)
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

UC1 build order (details: doc/uc1-create-booking/README.md section 6)
- M1: Room ReserveRoom gRPC (done 2026-10-26)
- M2: Booking CreateBooking gRPC (idempotent by saga_id)   <- NOW, steps below
- M3: Orchestrator phase A: POST /bookings -> check headers + body (Notes) -> ReserveRoom -> CreateBooking (no Kafka yet)
- M4: Kafka + outbox poller + phase D (deadline worker -> ReleaseRoom -> ExpireBooking)
- M5: Payment: CreateCheckoutSession (Stripe hosted page) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer

Next step: M2 step 1
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask

M2: Booking CreateBooking gRPC
booking-service predates the UC1 schema: create_tables.sql has the right columns, but the Go code still reads a nightly_rates
table, has old statuses, and the adapter doesn't implement its port. Same shape as room's M1

1. Wiring
    + go.mod: logger v1.1.0 (grpclogger) + replace booking/contracts (hexagonal-structure.md section 1), then go mod tidy.
      booking/platform comes with the outbox in M4. The require comes in step 5, because tidy removes a require nothing
      imports: this line + go mod tidy (PowerShell needs the quotes): go mod edit "-require=booking/contracts@v0.0.0"
    + config: GRPC{Port} from GRPC_PORT, same shape as room; .env.example; your .env needs a port room doesn't use
    + create_tables.sql: remove the 3 indexes (idx_bookings_user_id, _room_id, _dates): no UC1 query uses them (Why M2)
    + db/migration/drop_all.sql like room's: old nightly_rates first (FK to bookings), bookings, then both enum types.
      Then rebuild the DB from create_tables.sql
2. Core model (internal/booking)
    + booking.go: one entity, Detail goes away (nightly rates are a column now, read with the row)
      type Booking struct {
          ID, SagaID, ReservationID                    string
          UserID, UserName, UserEmail, UserPhoneNumber string
          RoomID, RoomNumber, RoomType                 string
          CheckInDate, CheckOutDate                    time.Time // midnight UTC
          NumberOfGuests                               int
          TotalAmount                                  int64
          Currency                                     string
          NightlyRates                                 []NightlyRate
          Status                                       Status
          PaymentStatus                                PaymentStatus
          ExpiresAt, CreatedAt, UpdatedAt              time.Time
      }
      In the code: one field per line with a db tag = column name, like room (a tag on `ID, SagaID string` applies to both
      names and the scan fails at runtime). RowToAddrOfStructByName needs one field per selected column (21)
      Status constants = the DB enum: remove RESERVED, RESERVATION_FAILED, PAYMENT_FAILED; add StatusExpired (EXPIRED)
      Remove PaymentStatus.IsTerminal: no caller, same reason as the old port methods (step 3)
    + nightly_rate.go
      type NightlyRate struct { Date time.Time; Price int64 } // a JSONB element: no id, no booking_id
      func (n NightlyRate) MarshalJSON() ([]byte, error)      // same as room: date as YYYY-MM-DD, like the proto
      func (n *NightlyRate) UnmarshalJSON(b []byte) error
    + errors.go: ErrInvalidInput, ErrNotFound
    + booking.go
      // validate rejects what proto3 lets through: a forgotten field arrives as 0 or "", not as an error. No I/O
      func (b *Booking) validate() error
          ErrInvalidInput: check-in not before check-out, guests <= 0, empty user name / email or room number / type
          (phone may be empty), currency not 3 uppercase letters, not one rate per night (len = nights, rates[i].Date =
          check-in + i days: compare with Equal, == also compares the Location), a rate price <= 0, prices don't sum to
          total. The sum also catches a forgotten total (Why M2)
          The error names the field, never its value: grpclogger writes the gRPC message to the access log, and name,
          email and phone are personal data (room's %q echoes only IDs and dates)
      booking_test.go: table test like setRates; nightly_rate_test.go: JSON round trip. They compile only after step 3
      (repository.go, service.go still name Detail); go build ./... stays broken until step 6. Until then build what is done:
      go build ./internal/booking (step 3), add ./internal/adapter/postgres (step 4), add ./internal/adapter/grpc (step 5)
3. Port + service
    + repository.go: replaces the old methods (no UC1 caller; M4 adds Expire, M6 Confirm, by sagaID)
      // GetByID returns ErrNotFound when no booking has this id
      GetByID(ctx context.Context, id string) (*Booking, error)
      // Create inserts the booking and returns the stored row. Idempotent by saga_id: a repeated call returns the
      // existing row as stored, even if the rest of the request differs
      Create(ctx context.Context, b *Booking) (*Booking, error)
    + service.go: field bookingRepository -> repository, like room
      func (s *Service) GetByID(ctx context.Context, id string) (*Booking, error)
      // in: filled by the gRPC server, except ID, Status, PaymentStatus, CreatedAt, UpdatedAt (the service sets the
      // statuses, the DB the rest)
      func (s *Service) Create(ctx context.Context, b *Booking) (*Booking, error)
          validate -> Status = PENDING, PaymentStatus = PENDING -> repository.Create
4. Postgres adapter (booking_repository.go): rewrite
      func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository // no ctx, like room
      var _ booking.Repository = (*BookingRepository)(nil)
      Param b, not booking (the old Create): booking *booking.Booking hides the package inside the function
    + GetByID: select the 21 columns, CollectOneRow + RowToAddrOfStructByName; pgx.ErrNoRows -> ErrNotFound
    + Create: insert ... on conflict (saga_id) do nothing returning id, created_at, updated_at
        pgx.ErrNoRows = the insert was skipped: a call with this saga_id committed first, so return the row by saga_id
        (unexported getBySagaID). The insert is the check: no check-then-insert race
        Target (saga_id), unlike room's no target: room must also skip overlaps, here any other conflict is a real error
5. gRPC server (adapter/grpc/server.go), same shape as room's (require booking/contracts first, step 1)
      type Server struct { bookingv1.UnimplementedBookingServiceServer; svc *booking.Service }
      func New(s *booking.Service) *Server
      func (s *Server) CreateBooking(ctx context.Context, req *bookingv1.CreateBookingRequest) (*bookingv1.CreateBookingResponse, error)
          ErrInvalidInput -> INVALID_ARGUMENT with the field; anything else is logged (saga_id, error) -> INTERNAL
          response: booking_id, status (the stored status on a repeated call)
      func toBooking(req *bookingv1.CreateBookingRequest) (*booking.Booking, error)
          uuid.Parse saga_id, reservation_id, user.id, room.id (canonical form); time.DateOnly for the stay and every rate;
          expires_at CheckValid. A missing user or room is safe: generated getters on a nil message return "", so
          uuid.Parse fails -> INVALID_ARGUMENT
6. REST: GET by id only
    + dto.go
      // dates as YYYY-MM-DD (contracts.md); no saga_id / reservation_id: internal
      type BookingResponse struct { ... } // the other fields under their column names
      func toBookingResponse(b *booking.Booking) BookingResponse
    + handler.go: GetBookingByID with parseBookingID + writeError like room (400, 404, else a logged generic 500).
      Pass c.Request.Context(), not c: gin.Context doesn't carry the request's trace_id or its cancel
    + router.go: /health + GET /:id. GetBookings goes away (Why M2), and so do the commented-out routes (CreateBooking is
      gRPC now, the others have no design yet)
7. cmd/main.go, same as room's: time.Local = time.UTC first, pgxpool.New + Ping, repo -> service -> handler,
   gRPC server (grpclogger first, recovery, reflection), graceful stop of both. No unknown-fields flag: no JSON body here.
   Then go mod tidy: the recovery interceptor (go-grpc-middleware/v2) is a new import
8. Done when (grpcurl + Postman)
    + CreateBooking -> booking_id + PENDING; the same saga_id again -> same booking_id and no new row, even when the
      second request has other valid data
    + DB: nightly_rates has "date":"YYYY-MM-DD"; status and payment_status are PENDING
    + INVALID_ARGUMENT: malformed saga_id, no user, no expires_at, check-out = check-in, guests 0, no rates, one rate
      missing, prices that don't sum to the total
    + GET by id: stay and rate dates are YYYY-MM-DD, expires_at / created_at end in Z, no saga_id; unknown id -> 404,
      malformed -> 400
    + go test ./... passes
    + Docs (same pass): CLAUDE.md status row, naming example (GetDetailsByUserID, UpdateStatus), go test line (+ booking-service
      internal/booking); contracts.md GET by id errors (+ 400 malformed id) and user.phone_number (may be empty); flows.md
      phase A (CreateBooking also takes reservationId + currency; rates live in the row); services.md and
      hexagonal-structure.md (no list route, errors.go); Postman (booking_url + booking get by id)

Why M2 (accepted 2026-11-04)
- One Booking struct: Detail only existed to join the old nightly_rates table. With JSONB there is nothing to join
- No GetBySagaID port, unlike room: room must see the stored row first (RELEASED -> error), booking has no rule on a
  repeated call
- validate in the core: proto3 sends a forgotten field as 0 or "", so a forgotten total would be stored as 0
  (data-model.md: "a forgotten total is an error, not 0"). The sum check catches it: at least one night, every price > 0,
  sum = total, so there is no separate total check
- Phone may be empty: users has no phone column and the X-User-* headers don't require one; "" is stored, no NULL
- No GET list route: not in contracts.md and no UC1 caller. The old handler read a path param its route doesn't have.
  It comes back with UC2 / the FE
- No indexes on bookings yet: every UC1 lookup is by id or saga_id (PK, UNIQUE), the same reasoning as reservations in
  data-model.md. idx_bookings_user_id comes back with the list route

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
- M3: ReserveRoom error -> saga
    + definite rejection (NOT_FOUND, INVALID_ARGUMENT, FAILED_PRECONDITION) -> saga FAILED, nothing to release, reply 404 / 400 / 409
    + unknown outcome (timeout, UNAVAILABLE, INTERNAL) -> phase D, reply 503
- M3: saga_instances migration is behind data-model.md: add deadline_at (= ReserveRoom expires_at) + user_id + idx_saga_deadline,
  reservation_id TEXT -> UUID
- M3: check the request before the saga starts: user id is a UUID, name + email not empty, guests > 0, else 400. These come
  from the client and only CreateBooking checks them, so a bad header would hold the room, fail with INVALID_ARGUMENT, go
  to phase D and answer 503, not contracts.md's 400
- M3: deadline_at made in Go: time.Now().UTC().Add(HOLD_TTL), so the 201's expires_at ends in Z like later reads
- M4: main.go -> errgroup (hexagonal-structure.md section 5) once the Kafka consumer + outbox poller join HTTP + gRPC:
  one failing goroutine then stops the others and the defers run (pool.Close), instead of os.Exit in each goroutine
- M4: envelope.OccurredAt is a plain string: time.Now().UTC().Format(time.RFC3339)
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
    + M4 message handlers (flows.md "Shape of every message handler": FOR UPDATE -> status check -> UPDATE + outbox)
      may need the same pattern, so RunInTx can come back in M4: decide there
    + test by hand with two sessions (a mocked unit test has no tx or lock, so it can't show this). psql session A plays
      the admin: BEGIN; SELECT id FROM rooms WHERE id = '<room>' FOR UPDATE; -> grpcurl ReserveRoom hangs (its FOR SHARE
      waits) -> A: UPDATE rooms SET status = 'INACTIVE' ...; COMMIT; -> ReserveRoom returns ROOM_UNAVAILABLE (READ
      COMMITTED: a FOR SHARE that waited reads the row as A committed it). Two admin edits: Postman PATCH instead of
      grpcurl, it waits for A's COMMIT. Before the fix the same script shows the race (Reserve doesn't wait). A Go
      integration test (real DB, two txs + timeout) is later: phase 1 has no test DB

Cleanup (any time)
- contracts/envelope: round-trip test (skipped in step 1, do it once the real gRPC call works)
- user-service handlers + repository, same as room's step 9 fixes: code done (2026-11-03), left: run by hand in Postman:
  malformed id, unknown JSON field, bad role, malformed email -> 400; unknown id on GET / PUT -> 404; taken email on
  POST / PUT -> 409, also when only the case differs (the service stores email lowercase); PUT with its own email -> 200; timestamps on every response are real and end in Z (room's too),
  updated_at moves on PUT; empty table -> []. The saved "user update" id (356e528f...) isn't in the seed: use an id that
  exists, or it's a correct 404

Docs to sync
- flows.md, state-machines.md, README timers: room status + date validation in ReserveRoom; ReserveRoom is idempotent by saga_id
  (a repeated call returns the stored row)
- use-cases.md line 35: "Update inventory" is stale (occupancy lives only in reservations)
- state-machines.md line 89: "Proposed enum changes (confirm)" is stale (README Q1 confirmed them)

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
