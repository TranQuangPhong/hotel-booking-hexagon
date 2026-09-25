# Hexagonal structure

Target folder layout for the repo and each service. The trees show files that exist today plus the ones UC1 v1 adds (marked `# UC1`).
The UC1 design is in [uc1-create-booking/](uc1-create-booking/README.md).

## 1. Repo layout

```
hotel-booking-hexagon/
├── contracts/                    # module booking/contracts: DEFINITIONS ONLY (no pgx, kafka, gin)
│   ├── go.mod
│   ├── buf.yaml, buf.gen.yaml    # protobuf generation config
│   ├── envelope/
│   │   └── envelope.go           # Kafka envelope: messageId, type, sagaId, producer, occurredAt, data
│   ├── room/v1/                  # owned by room-service
│   │   ├── room.proto            # gRPC: ReserveRoom
│   │   ├── room.pb.go, room_grpc.pb.go   # generated, never edit by hand
│   │   └── messages.go           # commands room accepts + events room emits (data structs, type names, topics)
│   ├── booking/v1/               # owned by booking-service (CreateBooking, ConfirmBooking, BookingConfirmed...)
│   └── payment/v1/               # owned by payment-service (CreatePaymentIntent, PaymentAuthorized...)
│
├── platform/                     # module booking/platform: shared INFRA code, imported by adapters only
│   ├── go.mod
│   └── outbox/
│       ├── outbox.go             # Insert(ctx, tx, envelope): write an outbox row inside the caller's tx
│       └── poller.go             # Poller: unpublished rows -> Kafka -> published_at
│
├── user-service/  room-service/  booking-service/  orchestrator-service/  payment-service/  notification-service/
├── infra/                        # docker-compose: postgres, kafka (UC1 M4), mongodb (later)
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
```

1. **The core imports nothing external**: not gin, pgx, grpc or kafka, and **not `booking/contracts` either**. Proto types and Kafka message structs are the service's *external* API, exactly like Gin DTOs. Adapters map them to and from domain types. The cost is a little mapping code; the benefit is that a contract change never ripples into business logic.
2. **Ports live in the core.** Driven ports are interfaces the core calls (repository, gateway, client). Driving adapters call core services (HTTP handler, gRPC server, Kafka consumer, worker).
3. **Only adapters import `booking/platform`.**
4. **A service imports only the `contracts` packages it needs:** its own `<svc>/v1`, plus `envelope`. Exceptions: the orchestrator imports all of them (it's the coordinator), and notification imports `booking/v1` (to consume `BookingConfirmed`).
5. **State change + outgoing message = one transaction.** The core tells the repository both what changed and which domain event happened, e.g. `repo.Confirm(ctx, sagaID, event)`. The postgres adapter maps the event to a contracts message and calls `outbox.Insert(ctx, tx, …)` in the same tx. So the core never "publishes" anything, and no `EventPublisher` port or Kafka producer adapter is needed; the platform poller does the publishing.

## 3. Adapter vocabulary (same names in every service)

| Folder | Direction | What it does |
|---|---|---|
| `adapter/handler/` | driving | Gin REST: `router.go`, `handler.go`, `dto.go` (DTO ↔ domain) |
| `adapter/grpc/` | driving | gRPC server: implements `contracts/<svc>/v1` server, maps proto ↔ domain |
| `adapter/kafka/` | driving | consumer: envelope → domain call; `mapper.go` for message ↔ domain event |
| `adapter/worker/` | driving | time-triggered jobs (deadline worker) |
| `adapter/postgres/` | driven | repositories; writes outbox rows in the same tx |
| `adapter/grpcclient/` | driven | calls other services' gRPC (orchestrator only) |
| `adapter/<vendor>/` | driven | third-party SDKs: `stripe/`, `cognito/`, `ses/` |

## 4. Services

### orchestrator-service

```
orchestrator-service/
├── cmd/main.go                       # wire + start: REST, Kafka consumer, outbox poller, deadline worker
├── config/config.go
├── internal/
│   ├── saga/                         ===== CORE =====
│   │   ├── saga.go                   # Saga entity, Step + Status enums, context      (was state.go)
│   │   ├── transitions.go            # UC1: the transition table: (step, input) -> action + next step
│   │   ├── service.go                # UC1: StartBooking, StartPayment, OnEvent, ExpireOverdue   (was orchestrator.go)
│   │   ├── repository.go             # UC1 port: saga.Repository (Create, GetForUpdate, Save(saga, commands))
│   │   └── participants.go           # UC1 ports: RoomClient, BookingClient, PaymentClient (sync calls)
│   │
│   └── adapter/
│       ├── handler/                  # REST /orchestrator/api/v1/bookings…
│       ├── grpcclient/               # UC1: implements RoomClient / BookingClient / PaymentClient
│       ├── kafka/
│       │   ├── consumer.go           # *.events -> service.OnEvent
│       │   └── mapper.go             # UC1: envelope <-> domain event / command
│       ├── worker/
│       │   └── deadline.go           # UC1: every DEADLINE_POLL -> service.ExpireOverdue
│       └── postgres/
│           └── saga_repository.go    # UC1: saga_instances + saga_message_log (= outbox)
│                                     #      (producer.go goes away: the platform poller publishes)
├── db/migration/
└── go.mod                            # + replace contracts, platform
```

### room-service

```
room-service/
├── cmd/main.go                       # wire + start: REST, gRPC, Kafka consumer, outbox poller
├── config/config.go
├── internal/
│   ├── room/                         ===== CORE: room catalog (CRUD, done) =====
│   │   ├── room.go
│   │   ├── repository.go             # port
│   │   └── service.go
│   │
│   ├── inventory/                    ===== CORE: prices + holds =====
│   │   ├── inventory.go              # inventory.Day {status AVAILABLE|MAINTENANCE, price, currency}
│   │   ├── reservation.go            # UC1: Reservation entity + status enum
│   │   ├── repository.go             # port (+ UC1: CreateReservation, Confirm, Release by sagaID)
│   │   ├── service.go                # UC1: ReserveRoom, ConfirmReservation, ReleaseRoom
│   │   └── errors.go                 # UC1: ErrRoomUnavailable, ...
│   │
│   └── adapter/
│       ├── handler/                  # REST /rooms/api/v1 (CRUD)
│       ├── grpc/
│       │   └── server.go             # UC1: ReserveRoom
│       ├── kafka/
│       │   ├── consumer.go           # UC1: room.commands -> inventory service
│       │   └── mapper.go
│       └── postgres/
│           ├── room_repository.go
│           └── inventory_repository.go   # UC1: reservations (23P01 -> ErrRoomUnavailable) + outbox
├── db/migration/
└── go.mod
```

Reservations live in `inventory/` rather than a third package, because `ReserveRoom` reads prices and writes the hold in one transaction.
Redis lock / `cached_room_repository.go` decorator: roadmap phase 5, not needed for correctness.

### booking-service

```
booking-service/
├── cmd/main.go                       # wire + start: REST (GET), gRPC, Kafka consumer, outbox poller
├── config/config.go
├── internal/
│   ├── booking/                      ===== CORE =====
│   │   ├── booking.go                # Booking, statuses (PENDING, BOOKED, EXPIRED, …)
│   │   ├── nightly_rate.go
│   │   ├── repository.go             # port (+ UC1: Confirm, Expire by sagaID)
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
├── config/config.go                  # + STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET
├── internal/
│   ├── payment/                      ===== CORE =====
│   │   ├── payment.go                # Payment, status enum (CREATED, AUTHORIZED, CAPTURED, CAPTURE_FAILED)
│   │   ├── gateway.go                # port: PaymentGateway (CreateIntent, Capture); Stripe is one implementation
│   │   ├── repository.go             # port
│   │   └── service.go                # CreateIntent, MarkAuthorized (from webhook), Capture
│   │
│   └── adapter/
│       ├── grpc/server.go            # CreatePaymentIntent
│       ├── handler/                  # POST /payments/api/v1/webhooks/stripe (verify signature -> MarkAuthorized)
│       ├── kafka/                    # payment.commands consumer + mapper
│       ├── stripe/gateway.go         # implements PaymentGateway (stripe-go)
│       └── postgres/payment_repository.go
├── db/migration/
└── go.mod
```

The `PaymentGateway` port is where hexagonal pays off: the core never sees a Stripe type, and a fake gateway makes the service testable without the network.

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
load config → pgxpool → repos → gateways/clients → core services
→ adapters (handler, grpc server, kafka consumer, worker, outbox poller)
→ run each in its own goroutine under one context (errgroup)
→ on SIGINT/SIGTERM: cancel context, graceful stop HTTP + gRPC, wait for all goroutines, close pool
```
