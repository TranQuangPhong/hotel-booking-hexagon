# CLAUDE.md

## How to work with me (read first)

This is a **learning project**. I'm studying Go, microservices/Saga, CI/CD, and AWS by building it myself.

**Who does what**
- **I write the main logic** (core services, the main path of each flow). Default to explaining, reviewing, designing, and planning. Don't generate large amounts of implementation code unless I ask for it.
- **I hand you the rest, one item at a time:** detailed design docs, workspace-wide refactors and renames, stubs, subtle edge-case fixes, doc sync. When I say "fix it", "you fix" or "you impl", finish the whole item.
- **Stay inside the scope I name.** "Discuss" and "review" mean change nothing. "Only note to plan.md" means edit only that file. "Fix the doc & code" does not include new functions. A plan or doc request never includes running commands.

**How to answer**
- **You are my consultant, so challenge me.** When you see an idea or solution better than mine, or better than what the code and docs do now, say so even if I didn't ask: what is better, why, and what it costs to switch. Don't just go along with my approach. Suggest only, I decide; a better idea that belongs to a later phase gets one line marked "later".
- When I ask "how do I do X", give me the approach, the trade-offs, and a small snippet if needed. Don't hand me a finished file.
- When reviewing my code, point out bugs and non-idiomatic Go, and explain *why*. Let me make the fix unless I say "fix it".
- Number review findings, must-fix first and nits last, and keep the numbers stable: I answer by number ("1 ok, 2 you fix, 3 why?"). On "review again", check what changed and give each earlier item a status.
- For design questions, give a recommendation with reasoning. Mention alternatives briefly so I learn the trade-off, then commit to one. When I push back or propose something simpler, judge it on its merits and say plainly whether you agree.
- I have general backend experience (SQL, transactions, REST), so skip those basics. Go idioms, gRPC/protobuf tooling, Postgres-specific features and saga/messaging patterns are what I'm learning: explain those with a small concrete example, in plain words and standard terms ("idempotent by saga_id" and "repeated call", not a label like "replay").
- If my notes, my message or an earlier decision disagree with what you are about to write, ask me. Don't pick silently (that is how Stripe.js once replaced the hosted page I wanted).
- After editing, list what you changed beyond what I asked and what you noticed but left alone.

**Scope and docs**
- Lean v1 first: the workload is heavy and this is for study. Keep suggestions within the current phase (see Roadmap). Flag later-phase ideas like Redis, CDC, or AWS as "later" instead of pulling them in now. If something only matters in production, say so in one line and let me skip it.
- Keep docs short: I want to read a file and get the overview without re-reading every detail. The details belong in `doc/uc1-create-booking/` only.
- `doc/plan.md` is our handoff between sessions: the next step, decisions with a one-line why, and "Docs to sync". "Note it down" means write it there. Remove done steps and keep the numbers of the remaining ones.
- When a change makes a doc wrong, fix the doc in the same pass, or add it to "Docs to sync" if I limited the scope.
- Docs in `doc/` are my working notes and may be rough, partly in Vietnamese, or behind the code. Check them against the code, and tell me when they disagree.

## Project overview

A simple hotel booking system, built as Go microservices with **hexagonal architecture** inside each service and a **Saga orchestrator** coordinating the booking flow.

| Service | Status | Store | Notes |
|---|---|---|---|
| `user-service` | Done (CRUD APIs tested) | PostgreSQL | Profile only; Cognito handles auth (phase 2) |
| `room-service` | Room CRUD done; inventory WIP | PostgreSQL (+ Redis lock later) | Rooms, monthly inventory (JSONB days), reservations |
| `booking-service` | WIP: model/repo/service, handler partial, `main.go` stub | PostgreSQL | Booking stores user and room **snapshots** plus nightly rates |
| `orchestrator-service` | Skeleton only | PostgreSQL | `saga_instances` + `saga_message_log` schemas designed |
| `payment-service` | Skeleton only (config done) | PostgreSQL + Stripe sandbox | |
| `notification-service` | Skeleton only | MongoDB + AWS SES | |

Key docs:
- `doc/services.md`: service responsibilities, endpoints, tech stack
- `doc/use-cases.md`: UC1 create booking, which includes the payment request. UC2 is cancel booking, UC3 is payment refund.
- `doc/uc1-create-booking/`: **detailed UC1 design, the source of truth** (lean v1: flows, saga/entity state machines, REST/gRPC/Kafka contracts, schema deltas, build order M1–M6; cut items live in `good-to-have.md`, so don't pull them into v1 work)
- `doc/architecture_uc_create_order.excalidraw`: one-page UC1 picture (keep in sync with `uc1-create-booking/`). `architecture_saga_deprecated.excalidraw` is outdated, so don't use it.
- `doc/hexagonal-structure.md`: target folder layout per service
- `doc/Message-definition.json`: example Kafka message envelope, v1 (message_id, type, saga_id, producer, occurred_at, data); fields explained in `uc1-create-booking/contracts.md`
- `doc/plan.md`: roadmap and current next step
- `doc/Booking_hexagon.postman_collection.json`: manual API tests

## Architecture

**Communication (UC1, create booking; details in `doc/uc1-create-booking/`):**
- Client → (API Gateway later) → **Orchestrator** over REST
- Before payment (user waiting): Orchestrator → Room `ReserveRoom`, Booking `CreateBooking`, Payment `CreateCheckoutSession` over **gRPC**, sync
- The user pays on Stripe's **hosted page** (Checkout Session, client is redirected to its URL), not an embedded Stripe.js form with a `client_secret`
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
- A service may have more than one domain package, but only when they don't need each other: room-service keeps rooms, inventories and reservations in one `room/` package, because they depend on each other both ways and Go forbids import cycles.

## Conventions (follow existing code)

- Go 1.25; each service is its **own Go module** (`module booking/<svc>-service`). Shared code: the logger (separate git repo) plus `contracts/` and `platform/` (modules in this repo, `replace` directives, not `go.work` alone, because `go mod tidy` ignores it).
- Shared logger: `github.com/TranQuangPhong/hotel-booking-logger`, which is my own library. `slog.SetDefault(logger.NewLogger())` plus `logger.LoggingMiddleware()` in Gin; services with a gRPC server also put `grpclogger.UnaryServerInterceptor()` first in `grpc.ChainUnaryInterceptor` (subpackage `/grpclogger`, v1.1.0+).
- HTTP: Gin, `gin.New()` + `Recovery` + logging middleware. Routes are `/<prefix>/health` and `/<prefix>/api/v1/...`, with a unique prefix per service (`/users`, `/rooms`, `/bookings`, `/orchestrator`, `/payments`). Versioning lives in code; the gateway (later) only routes and rewrites.
- DB: `pgx/v5` + `pgxpool`, raw SQL, no ORM. Multi-table writes run in a transaction with `defer tx.Rollback(ctx)`, and bulk inserts use `pgx.Batch`.
- IDs are UUIDs (`gen_random_uuid()`), handled as `string` in Go.
- **Money is stored in minor units as `int64`** (cents), with a separate `currency CHAR(3)`.
- Names don't repeat their package: `user.Service`, `user.Repository`, `user.NewService`, `handler.Handler` + `handler.New`, `config.DB`. The package's main entity is the exception (`room.Room`, `config.Config`). Core port/service methods drop the entity too: `GetAll`, `GetByID`, `Create`, `UpdateStatus`, `GetDetailsByUserID` (plural for slices). gRPC RPC names (`CreateBooking`) and HTTP handler methods (`h.GetUsers`) keep it, because their package doesn't imply the entity. Core files are `repository.go` / `service.go`; adapter files keep the entity prefix (`postgres/room_repository.go`).
- One name per field everywhere (REST, gRPC, Kafka, DB), `snake_case` in JSON. Flat structures prefix the owner (`user_phone_number`, `check_in_date`); nested objects drop it (`user.phone_number`, not `user.phone` or `user.user_phone_number`).
- Status enums are Go `type X string` constants with `IsValid()`, mirrored by Postgres `ENUM` types.
- Errors are wrapped with context: `fmt.Errorf("failed to ...: %w", err)`. Log the error value, not its string: `"error", err` / `slog.Any("error", err)`.
- Config comes from env: `SERVER_PORT`, `GRPC_PORT` (services with a gRPC server), `DB_HOST/PORT/USER/PASSWORD/NAME`, `KAFKA_BROKERS` (comma-separated). `.env.example` is only a template, so identical values across services (like the port) are intentional. The real `.env` is gitignored, local only, and I set it up myself for each service.
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

My setup:
- Windows + PowerShell: commands you give me must run there.
- I run the services and test with Postman / grpcurl myself. Don't run anything against my database.
- Local DBs are throwaway in phase 1: change the `CREATE` file and rebuild (room-service has `drop_all.sql`). No `ALTER` migrations.
- `.gitattributes` forces LF, so `gofmt -l .` from the repo root should print nothing. Anything it lists is a real formatting problem.

## Roadmap

1. **Phase 1, local:** finish service APIs, then implement **UC1 create booking end to end** (orchestrator → room → booking → payment → notify). ← *current focus*: message/event contracts and the orchestrator.
2. Local deployment and test of the full flow (no AWS Gateway, Cognito, or Lambda yet).
3. **Phase 2, AWS:** API Gateway, Cognito (plus a Lambda to sync users), SES, deployment.
4. CI/CD, monitoring, and logging.
5. Later optimizations: Redis reservation lock or cache decorator, CDC.

Decided for UC1 (see `doc/uc1-create-booking/`): shared modules (`contracts/` with protos inside, `platform/`), Kafka topic names, v1 event contracts, booking statuses, `HOLD_TTL` 15 min.
Undecided or not yet defined: Cancel booking (UC2) and Payment refund (UC3).
