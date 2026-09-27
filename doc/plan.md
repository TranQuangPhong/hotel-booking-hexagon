Plan

Roadmap
1. Phase 1 - local: service APIs + UC1 create booking end to end   <- NOW
2. Local deployment & test of the full flow (no AWS Gateway, Cognito, Lambda)
3. Phase 2 - AWS: API Gateway, Cognito (+ Lambda sync user), SES, deployment
4. CI/CD, monitoring, logging
5. Later optimizations: Redis lock / cache, CDC

Done
- User service: model, CRUD APIs, SQL, logging, tested
- Room service: room model, CRUD APIs, SQL, tested
- UC1 detailed design: doc/uc1-create-booking/ (v1 lean, decisions Q1-Q3 confirmed)
- Refactor: folder structure, naming, contracts/ + platform/ modules scaffolded

UC1 build order (details: doc/uc1-create-booking/README.md section 6)
- M1: contracts/ (envelope, room.proto + buf gen) + Room ReserveRoom gRPC + exclusion constraint   <- NEXT
- M2: Booking CreateBooking gRPC (idempotent by saga_id)
- M3: Orchestrator phase A: POST /bookings -> ReserveRoom -> CreateBooking (no Kafka yet)
- M4: Kafka + outbox poller + phase D (deadline worker -> ReleaseRoom -> ExpireBooking)
- M5: Payment: CreatePaymentIntent + Stripe webhook -> PaymentAuthorized
- M6: Phase C (confirm room -> capture -> confirm booking) + notification consumer

Next step: M1
1. contracts/: envelope struct, room/v1/room.proto (ReserveRoom), generate code with buf
    1.1. Envelope design (Go), shape = doc/Message-definition.json
         + Envelope struct + json tags, Data as json.RawMessage (decode later by type)
         + constructor New(type, sagaId, producer, data) -> sets messageId (UUID) + occurredAt (UTC)
         + helper to decode Data into a typed struct
    1.2. Envelope impl: contracts/envelope/envelope.go + small test (marshal -> unmarshal round trip)
    1.3. Data structs design (per owning package)
         + commands live in the receiver's package, events in the emitter's (room/v1, booking/v1, payment/v1)
         + message type constants (ConfirmReservation, RoomReleased, ...) + topic constants (room.commands, room.events, ...)
         + field types: IDs string, money int64 + currency, dates "YYYY-MM-DD" string, nullable -> pointer (RoomReleased.reservationId)
    1.4. Data structs impl: <svc>/v1/messages.go for the 12 messages in contracts.md section 4
         + only room ones are needed for M1, the rest can wait until M4/M6
    1.5. ReserveRoom proto design: contracts/room/v1/room.proto
         + package room.v1, go_package = "booking/contracts/room/v1;roomv1"
           (generated code and messages.go share the folder -> same Go package name)
         + service RoomService { rpc ReserveRoom(ReserveRoomRequest) returns (ReserveRoomResponse) }
         + request:  saga_id, room_id, check_in, check_out, expires_at (google.protobuf.Timestamp)
         + response: reservation_id, room_number, room_type, currency, total_amount (int64), repeated NightlyRate{date, price}
         + errors = gRPC status, not proto fields: NOT_FOUND, FAILED_PRECONDITION + reason ROOM_UNAVAILABLE
    1.6. buf setup
         + install buf, protoc-gen-go, protoc-gen-go-grpc
         + buf.yaml (v2, module root = contracts/), buf.gen.yaml (plugins go + go-grpc, paths=source_relative)
    1.7. Generate: buf lint -> buf generate -> room.pb.go + room_grpc.pb.go
         + commit generated code (services build without buf)
         + go mod tidy in contracts/ (grpc + protobuf deps)
    1.8. Wire into room-service: require booking/contracts + replace => ../contracts, go build ./...
    Done when: buf lint clean, envelope test passes, contracts/ and room-service build
2. Room DB: reservations.saga_id UNIQUE + btree_gist exclusion constraint; inventory days = price + AVAILABLE/MAINTENANCE
3. Room core (internal/inventory): reservation entity, repository port, Reserve service (read prices, insert reservation)
4. Room adapters: postgres repo (map 23P01 -> ErrRoomUnavailable), gRPC server (-> FAILED_PRECONDITION)
5. Seed inventory SQL for a test room (no inventory admin API yet)
6. Done when: grpcurl reserves; a 2nd overlapping call fails with ROOM_UNAVAILABLE

After UC1
- UC2 cancel booking, UC3 payment refund: TODO, not designed
- Good-to-have items: doc/uc1-create-booking/good-to-have.md
- FE for all services (gen AI)
