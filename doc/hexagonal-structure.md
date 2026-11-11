# Hexagonal structure

Target folder layout for the repo and each service. The trees show files that exist today plus the ones UC1 v1 adds (marked `# UC1`).
The UC1 design is in [uc1-create-booking/](uc1-create-booking/README.md).

## 1. Repo layout

```
hotel-booking-hexagon/
├── contracts/                    # module booking/contracts: DEFINITIONS ONLY (no pgx, kafka, gin)
│   ├── go.mod
│   ├── buf.yaml, buf.gen.yaml    # protobuf generation config
│   ├── Makefile                  # `make`: buf format + lint + generate, then go mod tidy
│   ├── envelope/
│   │   └── envelope.go           # Kafka envelope: message_id, type, saga_id, producer, occurred_at, data
│   ├── room/v1/                  # owned by room-service
│   │   ├── doc.go                # package comment
│   │   ├── room.proto            # gRPC: ReserveRoom
│   │   ├── room.pb.go, room_grpc.pb.go   # generated, never edit by hand
│   │   └── messages.go           # commands room accepts + events room emits (data structs, type names, topics)
│   ├── booking/v1/               # owned by booking-service (CreateBooking, ConfirmBooking, BookingConfirmed...)
│   └── payment/v1/               # owned by payment-service (CreateCheckoutSession, PaymentAuthorized...)
│
├── platform/                     # module booking/platform: shared INFRA code, imported by adapters and cmd/main.go
│   ├── go.mod
│   ├── outbox/
│   │   ├── outbox.go             # Insert(ctx, tx, Message{ID, Topic, Key, Payload}): one outbox row inside the caller's tx
│   │   └── poller.go             # Poller: unpublished rows -> Kafka -> published_at
│   └── kafka/
│       └── consumer.go           # Run(ctx, client, handler): poll -> call the handler until it succeeds -> commit (franz-go)
│
├── user-service/  room-service/  booking-service/  orchestrator-service/  payment-service/  notification-service/
├── infra/                        # docker-compose: postgres, kafka (UC1 M4a), mongodb (later)
└── doc/
```

Each service imports the shared modules through its `go.mod`. `go mod tidy` ignores `go.work`, and `booking/...` isn't a downloadable path, so a `replace` directive is required:

```
require (
    booking/contracts v0.0.0
    booking/platform  v0.0.0
)
replace (
    booking/contracts => ../contracts
    booking/platform  => ../platform
)
```

Later (Docker): the build context must be the repo root so `../contracts` and `../platform` are reachable.

## 2. Dependency rules

```
                 ┌──────────────────────────────┐
  cmd/main.go ──►│ adapter/*  (driving + driven)│──► booking/platform
     (wires)     │   maps DTO / proto / message │──► booking/contracts
                 │        ⇅ domain types        │──► gin, pgx, grpc, kafka, stripe
                 └──────────────┬───────────────┘
                                ▼
                 ┌──────────────────────────────┐
                 │ internal/<domain>  (CORE)    │   imports: std lib only
                 │ entities, service, ports     │
                 └──────────────────────────────┘

  cmd/main.go ──► booking/platform/outbox (starts the poller), booking/platform/kafka (runs the consumer loop)
```

1. **The core imports nothing external**: not gin, pgx, grpc or kafka, and **not `booking/contracts` either**. Proto types and Kafka message structs are the service's *external* API, exactly like Gin DTOs. Adapters map them to and from domain types. The cost is a little mapping code; the benefit is that a contract change never ripples into business logic.
2. **Ports live in the core.** Driven ports are interfaces the core calls (repository, gateway, client). Driving adapters call core services (HTTP handler, gRPC server, Kafka consumer, worker).
3. **Only adapters and `cmd/main.go` import `booking/platform`,** never the core.
4. **A service imports only the `contracts` packages it needs:** its own `<svc>/v1`, plus `envelope`. Exceptions: the orchestrator imports all of them (it's the coordinator), and notification imports `booking/v1` (to consume `BookingConfirmed`).
5. **State change + outgoing message = one transaction, inside one port method.** The core reads the row, decides, and tells the repository both what changes and which domain event happened, e.g. `repo.UpdateReservationStatus(ctx, sagaID, from, to, event)`. The postgres adapter runs `UPDATE … WHERE status = from` (a conditional update, no row lock), maps the event to a contracts message and calls `outbox.Insert(ctx, tx, …)` in the same tx. It returns `false` when 0 rows changed. So the core never "publishes" anything, and no `EventPublisher` port, Kafka producer adapter or `RunInTx` port is needed; the platform poller does the publishing.
6. **Outgoing messages are mapped in `adapter/postgres`** (it writes the outbox row); `adapter/kafka` maps incoming messages only. Otherwise one adapter would import another. Domain events are core types (`room.Event`), because the core can't import `contracts`.

## 3. Adapter vocabulary (same names in every service)

| Folder | Direction | What it does |
|---|---|---|
| `adapter/handler/` | driving | Gin REST: `router.go`, `handler.go`, `dto.go` (DTO ↔ domain) |
| `adapter/grpc/` | driving | gRPC server: implements `contracts/<svc>/v1` server, maps proto ↔ domain |
| `adapter/kafka/` | driving | consumer: envelope → domain call, run by `platform/kafka`; `mapper.go` maps incoming messages to domain inputs |
| `adapter/worker/` | driving | time-triggered jobs (deadline worker) |
| `adapter/postgres/` | driven | repositories; maps outgoing domain events to envelope + contracts message and writes the outbox row in the same tx |
| `adapter/grpcclient/` | driven | calls other services' gRPC (orchestrator only) |
| `adapter/<vendor>/` | driven | third-party SDKs: `stripe/`, `cognito/`, `ses/` |

## 4. Services

### orchestrator-service

```
orchestrator-service/
├── cmd/main.go                       # wire + start: REST, Kafka consumer, outbox poller, deadline worker
├── config/config.go                  # UC1: + ROOM / BOOKING / PAYMENT_GRPC_ADDR, GRPC_TIMEOUT, PAYMENT_GRPC_TIMEOUT, HOLD_TTL, DEADLINE_POLL
├── internal/
│   ├── saga/                         ===== CORE =====
│   │   ├── saga.go                   # Saga entity, Step + Status enums, context      (was state.go)
│   │   ├── transitions.go            # UC1: the transition table as a pure function: transition(status, step, input) -> Outcome
│   │   ├── service.go                # UC1: StartBooking, StartPayment, OnEvent, ExpireOverdue   (was orchestrator.go)
│   │   ├── repository.go             # UC1 port: saga.Repository (Create, GetByID, GetByBookingID, Save(saga, commands), GetOverdue)
│   │   └── participants.go           # UC1 ports: RoomClient, BookingClient, PaymentClient (sync calls)
│   │
│   └── adapter/
│       ├── handler/                  # REST /orchestrator/api/v1/bookings…
│       ├── grpcclient/               # UC1: implements RoomClient / BookingClient / PaymentClient
│       ├── kafka/
│       │   ├── consumer.go           # *.events -> service.OnEvent
│       │   └── mapper.go             # UC1: envelope -> domain input
│       ├── worker/
│       │   └── deadline.go           # UC1: every DEADLINE_POLL -> service.ExpireOverdue
│       └── postgres/
│           └── saga_repository.go    # UC1: saga_instances + outbox. Save = version guard + commands in one tx
│                                     #      (producer.go goes away: the platform poller publishes)
├── db/migration/
└── go.mod                            # + replace contracts, platform
```

### room-service

```
room-service/
├── cmd/main.go                       # wire + start: REST, gRPC, Kafka consumer, outbox poller
├── config/config.go                  # UC1: + GRPC_PORT
├── internal/
│   ├── room/                         ===== CORE: rooms (CRUD, done) + prices + holds, one package =====
│   │   ├── room.go                   # Room + status / type enums
│   │   ├── inventory.go              # Inventory (currency per row) + Day {status AVAILABLE|MAINTENANCE, price}; NewInventories builder
│   │   ├── reservation.go            # UC1: Reservation entity + status enum
│   │   ├── errors.go                 # ErrInvalidInput (+ UC1: ErrRoomUnavailable, ...)
│   │   ├── repository.go             # port: rooms + inventories (+ UC1: reservations by sagaID)
│   │   └── service.go                # room CRUD (+ UC1: Reserve, ConfirmReservation, ReleaseRoom)
│   │
│   └── adapter/
│       ├── handler/                  # REST /rooms/api/v1 (CRUD)
│       ├── grpc/
│       │   └── server.go             # UC1: ReserveRoom
│       ├── kafka/
│       │   ├── consumer.go           # UC1: room.commands -> room service
│       │   └── mapper.go
│       └── postgres/
│           └── room_repository.go    # rooms + inventories (+ UC1: reservations (overlap -> ErrRoomUnavailable) + outbox)
├── db/migration/
└── go.mod
```

Rooms, inventories and reservations share one package because they depend on each other both ways: creating a room writes its inventories in one transaction, and `ReserveRoom` reads the room and its prices and writes the hold in one transaction. Go forbids import cycles, so two packages would not compile. Split again when a part gets a life of its own.
Redis lock / `cached_room_repository.go` decorator: roadmap phase 5, not needed for correctness.

### booking-service

```
booking-service/
├── cmd/main.go                       # wire + start: REST (GET), gRPC, Kafka consumer, outbox poller
├── config/config.go                  # UC1: + GRPC_PORT
├── internal/
│   ├── booking/                      ===== CORE =====
│   │   ├── booking.go                # Booking, statuses (PENDING, BOOKED, EXPIRED, …)
│   │   ├── nightly_rate.go
│   │   ├── repository.go             # port (+ UC1: UpdateStatus, AddEvent by sagaID)
│   │   └── service.go                # Create (idempotent by sagaID), UC1: Confirm, Expire
│   │
│   └── adapter/
│       ├── handler/                  # REST /bookings/api/v1 (GET list, GET by id)
│       ├── grpc/server.go            # UC1: CreateBooking
│       ├── kafka/                    # UC1: booking.commands consumer + mapper
│       └── postgres/booking_repository.go
├── db/migration/
└── go.mod
```

### payment-service (new)

```
payment-service/
├── cmd/main.go                       # wire + start: gRPC, webhook REST, Kafka consumer, outbox poller
├── config/config.go                  # + GRPC_PORT, STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET, STRIPE_SUCCESS_URL
├── internal/
│   ├── payment/                      ===== CORE =====
│   │   ├── payment.go                # Payment, status enum (CREATED, AUTHORIZED, CAPTURED, CAPTURE_FAILED; M7: CANCELLED)
│   │   ├── gateway.go                # port: Gateway (CreateCheckout, GetCheckout, Capture; M7: Cancel); Stripe is one implementation
│   │   ├── repository.go             # port
│   │   └── service.go                # CreateCheckout, MarkAuthorized (from webhook), Capture; M7: Cancel
│   │
│   └── adapter/
│       ├── grpc/server.go            # CreateCheckoutSession
│       ├── handler/                  # POST /payments/api/v1/webhooks/stripe (verify signature -> MarkAuthorized)
│       ├── kafka/                    # payment.commands consumer + mapper
│       ├── stripe/gateway.go         # implements payment.Gateway (stripe-go)
│       └── postgres/payment_repository.go
├── db/migration/
└── go.mod
```

The `payment.Gateway` port is where hexagonal pays off: the core never sees a Stripe type, and a fake gateway makes the service testable without the network.

### notification-service (new, v1 = log only)

```
notification-service/
├── cmd/main.go                       # wire + start: Kafka consumer
├── config/config.go
├── internal/
│   ├── notification/                 ===== CORE =====
│   │   ├── notification.go           # BookingConfirmedNotice (what to tell whom)
│   │   ├── sender.go                 # port: Sender
│   │   └── service.go                # NotifyBookingConfirmed
│   │
│   └── adapter/
│       ├── kafka/consumer.go         # booking.events (BookingConfirmed) -> service
│       └── logsender/sender.go       # v1: implements Sender by logging
│                                     # later: ses/ (Phase 3), mongodb/ log (G9)
└── go.mod
```

### user-service (done, unchanged by UC1)

```
user-service/
├── cmd/main.go
├── config/config.go
├── internal/
│   ├── user/                         ===== CORE =====
│   │   ├── user.go
│   │   ├── errors.go                 # ErrInvalidInput, ErrNotFound, ErrEmailTaken
│   │   ├── service.go
│   │   ├── identity.go               # port: IdentityService (Cognito, Phase 3)
│   │   └── repository.go             # port
│   └── adapter/
│       ├── handler/                  # REST /users/api/v1
│       ├── postgres/user_repository.go
│       └── cognito/                  # later (Phase 3): implements IdentityService
├── db/migration/
└── go.mod
```

## 5. `cmd/main.go` shape (services with several entry points)

```
time.Local = time.UTC → load config → pgxpool → repos → gateways/clients → core services
→ adapters (handler, grpc server, kafka consumer, worker, outbox poller)
→ run each in its own goroutine under one context (errgroup)
→ on SIGINT/SIGTERM: cancel context, graceful stop HTTP + gRPC, wait for all goroutines, close pool
```
