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

UC1 build order (details: doc/uc1-create-booking/README.md section 6)
- M1: Room ReserveRoom gRPC   <- NOW
- M2: Booking CreateBooking gRPC (idempotent by saga_id)
- M3: Orchestrator phase A: POST /bookings -> ReserveRoom -> CreateBooking (no Kafka yet)
- M4: Kafka + outbox poller + phase D (deadline worker -> ReleaseRoom -> ExpireBooking)
- M5: Payment: CreateCheckoutSession (Stripe hosted page) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer

Next step: M1, Room ReserveRoom gRPC   <- resume at step 4 (steps 1-3 done)
Not in M1: the orchestrator gRPC client (that is M3). Test with grpcurl
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask
Before step 4: re-build the room DB (drop_all.sql, then create rooms / inventories / reservations) and re-run the Postman room CRUD

4. Core (internal/room)
    + reservation.go: Reservation (with the price snapshot) + status RESERVED / CONFIRMED / RELEASED with IsValid()
    + errors.go: ErrInvalidInput, ErrNotFound, ErrReservationReleased,
      ErrRoomUnavailable (overlap, MAINTENANCE night, no inventory, room not ACTIVE)
    + repository.go: InTx(ctx, fn), where fn gets a tx-bound Repository; get reservation by saga_id,
      read inventories for the months of the stay, insert reservation
    + service.go Reserve, every step inside one InTx:
        a. read the room -> ErrNotFound
        b. saga_id already has a reservation -> return it + the room's number / type (stored prices, nothing else is checked)
           RELEASED -> ErrReservationReleased
           "no reservation" is the normal first call: continue, never return it as ErrNotFound
        c. new hold only: check_out after check_in (no zero-night) and check_in >= today (UTC), else ErrInvalidInput
        d. new hold only: room ACTIVE -> every night exists and AVAILABLE -> nightly rates + total -> insert
           (currency = the inventory row's currency. No mixed-currency check: E4 is skipped)
    + nightly rates + total = a pure function with unit tests: stay crossing a month end, a year end,
      and check-out on the 1st (that month's inventory row must not be required)
5. Postgres adapter (one struct implements room.Repository; delete inventory_repository.go)
    + InTx: Begin, defer Rollback, fn(copy of the struct bound to the tx), Commit
      (the struct holds a small query interface that both the pool and a tx satisfy)
    + the room + inventory reads of Reserve use FOR SHARE: an admin update of status / price waits for the hold to commit,
      and two Reserve calls never block each other. The tx alone would not stop that race
    + INSERT ... ON CONFLICT (saga_id) DO NOTHING RETURNING; no row -> SELECT by saga_id (two identical calls racing)
    + 23P01 -> ErrRoomUnavailable, pgx.ErrNoRows -> ErrNotFound
      (optional: 40P01 deadlock, from two simultaneous overlapping inserts -> ErrRoomUnavailable too)
    + every query uses the request ctx, so the caller's 3 s deadline cancels a slow insert (see E6 below). No outbox row
6. gRPC server (adapter/grpc/server.go): only maps, calls room.Service
    + embed roomv1.UnimplementedRoomServiceServer
    + validate here -> INVALID_ARGUMENT: saga_id + room_id are UUIDs, dates parse as YYYY-MM-DD, expires_at is set
    + map: dates -> time.Time, expires_at.AsTime()
    + errors: ErrInvalidInput        -> INVALID_ARGUMENT
              ErrNotFound            -> NOT_FOUND
              ErrRoomUnavailable     -> FAILED_PRECONDITION "ROOM_UNAVAILABLE"
              ErrReservationReleased -> FAILED_PRECONDITION "RESERVATION_RELEASED"
              anything else          -> INTERNAL (generic message, real error in the log)
7. config + cmd/main.go
    + GRPC_PORT (config.go, .env.example); net.Listen + grpc.NewServer + RegisterRoomServiceServer in a goroutine
    + reflection.Register (dev only): grpcurl works without -proto flags
    + GracefulStop() next to the HTTP Shutdown; import alias for adapter/grpc
8. Done when (test room: POST /rooms, no seed SQL)
    + grpcurl reserves
    + a 2nd overlapping call fails with ROOM_UNAVAILABLE
    + the same saga_id again returns the same reservation_id and prices
    + a zero-night stay and a past check-in fail with INVALID_ARGUMENT

Why (decided 2026-10-12)
- Price snapshot on the reservation: an admin will be able to change inventory prices, so a repeated call must return what was
  first quoted, not re-read inventory. JSONB because it is write-once and never queried by night
- Reserve reads the room: the response carries room_number + room_type, and an INACTIVE / ARCHIVED room still has AVAILABLE days
- Reserve in one tx with FOR SHARE reads: what it read (room status, day status, price) cannot change before the hold commits
- The orchestrator makes one call (ReserveRoom) and checks nothing itself
- Idempotent by saga_id: the same call or message arriving a second time (caller retry, Kafka redelivery) gives the same
  result. Nobody triggers a repeat and there is no API for it; test it by sending the same request twice

Notes for later milestones
- M3: ReserveRoom error -> saga
    + definite rejection (NOT_FOUND, INVALID_ARGUMENT, FAILED_PRECONDITION) -> saga FAILED, nothing to release, reply 404 / 400 / 409
    + unknown outcome (timeout, UNAVAILABLE, INTERNAL) -> phase D, reply 503
- M3: saga_instances migration is behind data-model.md: add deadline_at (= ReserveRoom expires_at) + user_id + idx_saga_deadline,
  reservation_id TEXT -> UUID
- E6 (accepted in v1): ReserveRoom commits after ReleaseRoom already found "no row" -> a hold that nobody releases
  (the saga is FAILED, no sweeper). Mitigation: the request ctx on every query (step 5)
- Later phases: extend the inventory horizon (a room gets 12 months at creation, nothing adds more)

Cleanup (any time)
- contracts/envelope: round-trip test (skipped in step 1, do it once the real gRPC call works)
- handler.go: GetRoomByID returns 404 for every error, UpdateRoom returns 500 for a missing room -> use ErrNotFound

Docs to sync
- CLAUDE.md, hexagonal-structure.md: Reserve = one tx with FOR SHARE reads
- flows.md, state-machines.md, README timers: room status + date validation in ReserveRoom; ReserveRoom is idempotent by saga_id
  (a repeated call returns the stored row)
- good-to-have.md: add E6 and the inventory horizon; E4 becomes "not checked in v1"
- use-cases.md line 35: "Update inventory" is stale (occupancy lives only in reservations)

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
