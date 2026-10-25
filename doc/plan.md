Plan

Roadmap
1. Phase 1 - local: service APIs + UC1 create booking end to end   <- NOW
2. Local deployment & test of the full flow (no AWS Gateway, Cognito, Lambda)
3. Phase 2 - AWS: API Gateway, Cognito (+ Lambda sync user), SES, deployment
4. CI/CD, monitoring, logging
5. Later optimizations: Redis lock / cache, CDC

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

UC1 build order (details: doc/uc1-create-booking/README.md section 6)
- M1: Room ReserveRoom gRPC (done 2026-10-26)
- M2: Booking CreateBooking gRPC (idempotent by saga_id)   <- next, after step 9
- M3: Orchestrator phase A: POST /bookings -> ReserveRoom -> CreateBooking (no Kafka yet)
- M4: Kafka + outbox poller + phase D (deadline worker -> ReleaseRoom -> ExpireBooking)
- M5: Payment: CreateCheckoutSession (Stripe hosted page) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer

Next step: step 9 (room admin API, outside M1), then M2 (Booking CreateBooking gRPC)
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask

9. Room admin API (outside M1): inventory edit + hold checks. The races stay accepted (Notes: "Later")
    + core (internal/room)
        + inventory.go
          // DayPatch is an admin edit of inventory days: a nil field keeps the stored value
          type DayPatch struct { Status *DayStatus; Price *int64 }
          // apply sets the patch on every existing day from firstNight to lastNight (inclusive). No I/O
          func (p DayPatch) apply(inventories []*Inventory, firstNight, lastNight time.Time)
              loops the rows and their days, not the nights: a far lastNight costs nothing and no day is added.
              inventory_test.go: table test like setRates
        + errors.go: ErrReserved = a RESERVED / CONFIRMED hold covers the nights (REST 409)
        + service.go
          func (s *Service) UpdateInventories(ctx context.Context, roomID string, firstNight, lastNight time.Time,
              patch DayPatch) ([]*Inventory, error)
              ErrInvalidInput: no field set, invalid status, price <= 0, firstNight before today (UTC) or after lastNight
              ErrNotFound: GetByID first (an empty GetInventories can't tell a missing room from an empty range)
              ErrReserved: the patch sets MAINTENANCE and HasReservation(roomID, firstNight, lastNight) is true
              then GetInventories -> apply -> SaveInventories; returns the touched month rows ([] and no save when the
              range has no inventory)
          Update (same signature): INACTIVE / ARCHIVED and HasReservation(room.ID, today, 9999-12-31) -> ErrReserved.
              Only stays not over yet count: a reservation stays CONFIRMED after the stay
    + port (repository.go)
        // SaveInventories writes back each row's days by id (updated_at = now()) in one pgx.Batch: all rows or none,
        // since pgx runs a batch as one implicit transaction
        SaveInventories(ctx context.Context, inventories []*Inventory) error
        // HasReservation reports whether a RESERVED or CONFIRMED reservation holds any night from firstNight to lastNight
        HasReservation(ctx context.Context, roomID string, firstNight, lastNight time.Time) (bool, error)
            daterange(check_in_date, check_out_date, '[)') && daterange($2, $3, '[]'): the exclusion constraint's
            expression + WHERE, so its GiST index serves it
    + handler
        + PATCH /rooms/api/v1/:id/inventories -> h.UpdateInventories
          type UpdateInventoriesRequest struct {
              FirstNight string  `json:"first_night" binding:"required,datetime=2006-01-02"`
              LastNight  string  `json:"last_night" binding:"required,datetime=2006-01-02"`
              Status     *string `json:"status"`
              Price      *int64  `json:"price" binding:"omitempty,gt=0"`
          }
          both nights inclusive; status / price nil = keep; price in the room's currency; no currency field
          func (req UpdateInventoriesRequest) ToNights() (firstNight, lastNight time.Time, err error) // time.DateOnly, UTC
          func (req UpdateInventoriesRequest) ToDayPatch() room.DayPatch
          200 touched month rows, 400 ErrInvalidInput, 404 ErrNotFound, 409 ErrReserved
        + UpdateRoom: + 404 ErrNotFound (taken over from Cleanup), 409 ErrReserved
    + Docs (same pass): services.md room REST (+ PATCH), data-model.md Room (days change only through it: status / price,
      never currency or new days), good-to-have.md E4 (holds by construction: the edit takes no currency), Postman,
      CLAUDE.md Commands: drop "no tests yet" from go test (room-service has unit tests in internal/room)
    + Done when (Postman + grpcurl, test room from a new POST /rooms)
        + PATCH a price from today to 2099-12-31: every existing day from today changes; earlier days of this month
          don't, and no month is added
        + ReserveRoom, PATCH another price on those nights, the same saga_id again -> the old prices (snapshot)
          + DB: that new reservation's nightly_rates has "date":"YYYY-MM-DD" (first live insert since NightlyRate MarshalJSON)
        + PATCH MAINTENANCE on free nights -> ReserveRoom over them fails with ROOM_UNAVAILABLE
        + PATCH MAINTENANCE on a held night -> 409; PUT the room INACTIVE while it has a hold -> 409
        + PATCH a past first_night or no field -> 400; PUT / PATCH an unknown room id -> 404
        + go test ./... passes (apply table test)

Why (decided 2026-10-12)
- Price snapshot on the reservation: an admin will be able to change inventory prices, so a repeated call must return what was
  first quoted, not re-read inventory. JSONB because it is write-once and never queried by night
- Reserve reads the room: the response carries room_number + room_type, and an INACTIVE / ARCHIVED room still has AVAILABLE days
- Reserve without a tx in v1 (2026-10-15): its only write is one INSERT, and the constraints cover overlap, dates and a
  repeated saga_id. Accepted race: an admin change of room status / day status / price between the reads and the insert
  (reachable once step 9 exists; still accepted, fix in Notes: "Later")
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
  hold would still be confirmed and charged
- apply in the core (read -> apply -> one batch), not one jsonb UPDATE in SQL: the day rule stays in Go with a table test,
  and the batch saves all months or none
- Hold checks now, races later: the checks catch the case that happens locally (the hold already exists). Two races stay
  accepted (production-only, like the 2026-10-15 one): a Reserve in flight vs a status change (its insert lands after
  the admin's check), two admin edits on one month (the later batch erases the other's days). One fix: Notes, "Later"

Notes for later milestones
- M2: when booking moves nightly_rates to JSONB, give booking.NightlyRate the same date-only MarshalJSON/UnmarshalJSON as room
- M3: ReserveRoom error -> saga
    + definite rejection (NOT_FOUND, INVALID_ARGUMENT, FAILED_PRECONDITION) -> saga FAILED, nothing to release, reply 404 / 400 / 409
    + unknown outcome (timeout, UNAVAILABLE, INTERNAL) -> phase D, reply 503
- M3: saga_instances migration is behind data-model.md: add deadline_at (= ReserveRoom expires_at) + user_id + idx_saga_deadline,
  reservation_id TEXT -> UUID
- M4: main.go -> errgroup (hexagonal-structure.md section 5) once the Kafka consumer + outbox poller join HTTP + gRPC:
  one failing goroutine then stops the others and the defers run (pool.Close), instead of os.Exit in each goroutine
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
- handler.go: GetRoomByID returns 404 for every error -> map ErrNotFound only (the repository returns it now; UpdateRoom's
  mapping moved to step 9)

Docs to sync
- flows.md, state-machines.md, README timers: room status + date validation in ReserveRoom; ReserveRoom is idempotent by saga_id
  (a repeated call returns the stored row)
- use-cases.md line 35: "Update inventory" is stale (occupancy lives only in reservations)

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
