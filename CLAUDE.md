# CLAUDE.md

## How to work with me (read first)

This is a **learning project**. I'm studying Go, microservices/Saga, CI/CD, and AWS by building it myself.

- **I write most of the code.** Default to explaining, reviewing, designing, and planning. Don't generate large amounts of implementation code unless I ask for it.
- When I ask "how do I do X", give me the approach, the trade-offs, and a small snippet if needed. Don't hand me a finished file.
- When reviewing my code, point out bugs and non-idiomatic Go, and explain *why*. Let me make the fix unless I say "fix it".
- For design questions, give a recommendation with reasoning. Mention alternatives briefly so I learn the trade-off, then commit to one.
- Keep suggestions within the current phase (see Roadmap). Flag later-phase ideas like Redis, CDC, or AWS as "later" instead of pulling them in now.
- Docs in `doc/` are my working notes and may be rough, partly in Vietnamese, or behind the code. Check them against the code, and tell me when they disagree.

## Project overview

A simple hotel booking system, built as Go microservices with **hexagonal architecture** inside each service and a **Saga orchestrator** coordinating the booking flow.

| Service | Status | Store | Notes |
|---|---|---|---|
| `user-service` | Done (CRUD APIs tested) | PostgreSQL | Profile only; Cognito handles auth (phase 2) |
| `room-service` | Room CRUD done; inventory WIP | PostgreSQL (+ Redis lock later) | Rooms, monthly inventory (JSONB days), reservations |
| `booking-service` | WIP: model/repo/service, handler partial, `main.go` stub | PostgreSQL | Booking stores user and room **snapshots** plus nightly rates |
| `orchestrator-service` | Skeleton only | PostgreSQL | `saga_instances` + `saga_message_log` schemas designed |
| `payment-service` | Empty | PostgreSQL + Stripe sandbox | |
| `notification-service` | Empty | MongoDB + AWS SES | |

Key docs:
- `doc/services.md`: service responsibilities, endpoints, tech stack
- `doc/use-cases.md`: UC1 create booking, which includes the payment request. UC2 is cancel booking, UC3 is payment refund.
- `doc/uc1-create-booking/`: **detailed UC1 design, the source of truth** (lean v1: flows, saga/entity state machines, REST/gRPC/Kafka contracts, schema deltas, build order M1–M6; cut items live in `good-to-have.md`, so don't pull them into v1 work)
- `doc/architecture_uc_create_order.excalidraw`: one-page UC1 picture (keep in sync with `uc1-create-booking/`). `architecture_saga_deprecated.excalidraw` is outdated, so don't use it.
- `doc/hexagonal-structure.md`: target folder layout per service
- `doc/Message-definition.json`: example Kafka message envelope, v1 (messageId, type, sagaId, producer, occurredAt, data); fields explained in `uc1-create-booking/contracts.md`
- `doc/plan.md`: roadmap and current next step
- `doc/Booking_hexagon.postman_collection.json`: manual API tests

## Architecture

**Communication (UC1, create booking; details in `doc/uc1-create-booking/`):**
- Client → (API Gateway later) → **Orchestrator** over REST
- Before payment (user waiting): Orchestrator → Room `ReserveRoom`, Booking `CreateBooking`, Payment `CreatePaymentIntent` over **gRPC**, sync
- Stripe webhook → **Payment svc** → `PaymentAuthorized` event → orchestrator
- After payment: **sequential** Kafka command → reply event: ConfirmReservation → CapturePayment (Stripe manual capture) → ConfirmBooking → `BookingConfirmed` (notification subscribes)
- One compensation path (deadline or phase-A error): ReleaseRoom → ExpireBooking, looked up by `sagaId`
- Every Kafka message goes through a transactional **outbox**; handlers are idempotent by checking the current status

**Repo layout:** one module per service, plus two shared modules in this repo, wired with `replace` directives:
- `contracts/` (`booking/contracts`): proto + Kafka message definitions, one package per owning service, no infra deps
- `platform/` (`booking/platform`): shared infra (outbox insert + poller), imported by adapters only

**Hexagonal layout per service** (`<svc>/`, full trees in `doc/hexagonal-structure.md`):
```
cmd/main.go            # manual DI: load config → pgxpool → repo → service → adapters; run servers/consumers/workers; graceful shutdown
config/config.go       # env vars via godotenv (.env optional)
internal/<domain>/     # CORE: entities, service (business logic), port interfaces (repository, gateway, clients)
internal/adapter/      # handler (Gin), grpc, kafka, worker, postgres, stripe… implement or drive the core ports
db/migration/*.sql     # plain SQL, applied manually
```
Rules:
- The core (`internal/<domain>`) must not import adapters, infra libraries like gin or pgx, **or `booking/contracts`**. Adapters map proto / Kafka messages to domain types, the same way handlers map DTOs. Ports are interfaces defined in the core.
- No publisher port: the repository writes the state change + outbox row in one tx, and the platform poller publishes.
- Handlers map DTOs to domain models (`dto.go`, `ToXxx()` methods). They call services, never repositories.
- A service may have more than one domain package, e.g. room-service has `room/` and `inventory/`.

## Conventions (follow existing code)

- Go 1.25; each service is its **own Go module** (`module booking/<svc>-service`). Shared code: the logger (separate git repo) plus `contracts/` and `platform/` (modules in this repo, `replace` directives, not `go.work` alone, because `go mod tidy` ignores it).
- Shared logger: `github.com/TranQuangPhong/hotel-booking-logger`, which is my own library. `slog.SetDefault(logger.NewLogger())` plus `logger.LoggingMiddleware()` in Gin.
- HTTP: Gin, `gin.New()` + `Recovery` + logging middleware. Routes are `/<prefix>/health` and `/<prefix>/api/v1/...`, with a unique prefix per service (`/users`, `/rooms`, `/bookings`, `/orchestrator`, `/payments`). Versioning lives in code; the gateway (later) only routes and rewrites.
- DB: `pgx/v5` + `pgxpool`, raw SQL, no ORM. Multi-table writes run in a transaction with `defer tx.Rollback(ctx)`, and bulk inserts use `pgx.Batch`.
- IDs are UUIDs (`gen_random_uuid()`), handled as `string` in Go.
- **Money is stored in minor units as `int64`** (cents), with a separate `currency CHAR(3)`.
- Status enums are Go `type X string` constants with `IsValid()`, mirrored by Postgres `ENUM` types.
- Errors are wrapped with context: `fmt.Errorf("failed to ...: %w", err)`.
- Config comes from env: `SERVER_PORT`, `DB_HOST/PORT/USER/PASSWORD/NAME`, `KAFKA_BROKERS` (comma-separated). `.env.example` is only a template, so identical values across services (like the port) are intentional. The real `.env` is gitignored, local only, and I set it up myself for each service.
- `main.go` exits with `slog.Error` + `os.Exit(1)` when `config.Load()` or DB init fails.
- Commit messages look like `[scope] message`, e.g. `[booking svc] http handler and router` or `[UC: create booking] init`.

## Commands

Run from inside a service directory, since each one is a separate module:
```
go run ./cmd            # start service (needs .env or env vars)
go build ./...
go vet ./...
go test ./...           # no tests yet
go mod tidy
```
Infra: `infra/postgresql/docker-compose.yml` currently defines only `postgres-booking` (host port 5440). `infra/kafka` and `infra/mongodb` are empty placeholders. Migrations in `db/migration/` are applied by hand.

## Roadmap

1. **Phase 1, local:** finish service APIs, then implement **UC1 create booking end to end** (orchestrator → room → booking → payment → notify). ← *current focus*: message/event contracts and the orchestrator.
2. Local deployment and test of the full flow (no AWS Gateway, Cognito, or Lambda yet).
3. **Phase 2, AWS:** API Gateway, Cognito (plus a Lambda to sync users), SES, deployment.
4. CI/CD, monitoring, and logging.
5. Later optimizations: Redis reservation lock or cache decorator, CDC.

Decided for UC1 (see `doc/uc1-create-booking/`): shared modules (`contracts/` with protos inside, `platform/`), Kafka topic names, v1 event contracts, booking statuses, `HOLD_TTL` 15 min.
Undecided or not yet defined: Cancel booking (UC2) and Payment refund (UC3).
