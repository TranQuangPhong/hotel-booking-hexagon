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

UC1 build order (details: doc/uc1-create-booking/README.md section 6)
- M1: Room ReserveRoom gRPC   <- NOW
- M2: Booking CreateBooking gRPC (idempotent by saga_id)
- M3: Orchestrator phase A: POST /bookings -> ReserveRoom -> CreateBooking (no Kafka yet)
- M4: Kafka + outbox poller + phase D (deadline worker -> ReleaseRoom -> ExpireBooking)
- M5: Payment: CreateCheckoutSession (Stripe hosted page) + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer

Next step: M1, Room ReserveRoom gRPC   <- resume at step 6 (steps 1-5 done)
Not in M1: the orchestrator gRPC client (that is M3). Test with grpcurl
Work split: I write the main gRPC logic by hand; Claude handles the subtle details + refactoring when I ask
Before step 8 (if not done yet): re-build the room DB (drop_all.sql, then create rooms / inventories / reservations) and re-run the Postman room CRUD

6. gRPC server (adapter/grpc): only maps, calls room.Service
    + type Server struct { roomv1.UnimplementedRoomServiceServer; svc *room.Service }, func New(svc *room.Service) *Server
    + func (s *Server) ReserveRoom(ctx context.Context, req *roomv1.ReserveRoomRequest) (*roomv1.ReserveRoomResponse, error)
    + validate here -> INVALID_ARGUMENT: saga_id + room_id are UUIDs, dates parse as YYYY-MM-DD, expires_at is set
      (uuid.Parse, time.Parse(time.DateOnly, s) gives midnight UTC, req.GetExpiresAt().CheckValid())
    + mapper.go: toReservation(req) *room.Reservation, toReserveRoomResponse(*room.Reservation, *room.Room)
      (expires_at.AsTime(); NightlyRate.Date -> "YYYY-MM-DD")
    + errors (errors.Is, then status.Error(codes.X, msg)):
              ErrInvalidInput        -> INVALID_ARGUMENT
              ErrNotFound            -> NOT_FOUND
              ErrRoomUnavailable     -> FAILED_PRECONDITION "ROOM_UNAVAILABLE"
              ErrReservationReleased -> FAILED_PRECONDITION "RESERVATION_RELEASED"
              anything else          -> INTERNAL (generic message, real error in the log)
7. config + cmd/main.go
    + GRPC_PORT (config.go, .env.example); net.Listen, grpc.NewServer,
      RegisterRoomServiceServer(srv, roomgrpc.New(roomService)), Serve in a goroutine
    + reflection.Register (dev only): grpcurl works without -proto flags
    + grpc.NewServer(grpc.ChainUnaryInterceptor(recovery.UnaryServerInterceptor()))
      (github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery): unlike gin.Recovery, grpc-go
      does not recover a handler panic, it kills the whole process. The interceptor turns it into INTERNAL
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
- Reserve without a tx in v1 (2026-10-15): its only write is one INSERT, and the constraints cover overlap, dates and a
  repeated saga_id. Accepted race: an admin change of room status / day status / price between the reads and the insert
- ON CONFLICT DO NOTHING with no target (2026-10-19): with (saga_id) only, two identical calls at the same moment
  could fail on the overlap constraint (23P01, or 40P01 deadlock) and return ROOM_UNAVAILABLE for their own hold
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
- Later (when the admin inventory-edit API exists): Reserve in one tx
    + port: RunInTx(ctx context.Context, fn func(txRepo Repository) error) error. The core passes the steps,
      the adapter runs Begin, fn(the struct bound to the tx), Commit (the struct holds a small query interface
      that both the pool and a tx satisfy). Inside fn use txRepo, never s.repository: that runs outside the tx
    + the room + inventory reads use FOR SHARE: an admin update waits for the hold to commit,
      and two Reserve calls never block each other
    + M4 message handlers (flows.md "Shape of every message handler": FOR UPDATE -> status check -> UPDATE + outbox)
      may need the same pattern, so RunInTx can come back in M4: decide there
- Later phases: extend the inventory horizon (a room gets 12 months at creation, nothing adds more)

Cleanup (any time)
- contracts/envelope: round-trip test (skipped in step 1, do it once the real gRPC call works)
- handler.go: GetRoomByID returns 404 for every error, UpdateRoom returns 500 for a missing room -> use ErrNotFound
  (the repository returns ErrNotFound for both now; only the handler mapping is left)

Docs to sync
- flows.md, state-machines.md, README timers: room status + date validation in ReserveRoom; ReserveRoom is idempotent by saga_id
  (a repeated call returns the stored row)
- good-to-have.md: add E6 and the inventory horizon; E4 becomes "not checked in v1"
- use-cases.md line 35: "Update inventory" is stale (occupancy lives only in reservations)

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
