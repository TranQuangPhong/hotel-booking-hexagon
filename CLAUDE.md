# CLAUDE.md

## How to work with me (read first)

This is a **learning project**. I'm studying Go, microservices/Saga, CI/CD, and AWS by building it myself.

**Who does what**
- **I write the main logic** (core services, the main path of each flow). Default to explaining, reviewing, designing, and planning. Don't generate large amounts of implementation code unless I ask for it.
- **I hand you the rest, one item at a time:** detailed design docs, workspace-wide refactors and renames, stubs, subtle edge-case fixes, doc sync. When I say "fix it", "you fix" or "you impl", finish the whole item.
- **Stay inside the scope I name.** "Discuss" and "review" mean change nothing. "Only note to plan.md" means edit only that file. "Fix the doc & code" does not include new functions. A plan or doc request never includes running commands.

**Execute or discuss**
- A question ("?", "should I", "how", "why", "discuss", "review") means discuss: answer, recommend, change nothing.
- An order ("fix", "impl", "add", "update", "you do") means execute: finish the whole item, don't ask about routine choices, pick and tell me after. My decision on a point you raised counts as an order to apply the fix you proposed.
- A numbered message is judged per item. If unclear, answer first and offer the action in one line.
- Be proactive in both: in a discussion, raise better ideas and risks I didn't ask about; in an execution, fix closely related issues, prefer best practice, verify (build, vet, test), then list the extras.

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

**Workflows and subagents**
- I opt in to multi-agent workflows in these cases. Start one without asking, but only when one of them holds:
  - a whole-design or cross-service review
  - facts about an external library or API that I will build on and you cannot confirm from the repo
  - a change touching 3+ services
  - an adversarial check of a saga or concurrency design before I build on it (races, repeated messages, a crash between two steps)
  - real-world practice research for an open decision that is costly to reverse
  - a milestone-end consistency audit: docs, contracts and code across services
  - a bug with several plausible causes across services (one agent per cause)
- Never for re-reviewing a fix, a single-service diff, doc sync, or reading one technical document: do those inline.
- No market plugins: I disabled them all. We write our own project skill (`.claude/skills/<name>/SKILL.md`) only for a procedure of 5+ steps that we have already run by hand twice; suggest it in one line when that happens. First candidate: closing a milestone.
- At most 4 agents per workflow. Lookup, doc-check and test-running agents use `model: 'sonnet'` at `effort: 'medium'`; only concurrency/saga reasoning and adversarial review inherit the session model.
- When you start one, say so in one line with the reason.
- If a workflow would clearly be better in a case not listed here, don't start it: suggest it in one line with a rough cost, and suggest the wording to add to this list. I decide.

## Project overview

A simple hotel booking system, built as Go microservices with **hexagonal architecture** inside each service and a **Saga orchestrator** coordinating the booking flow.

| Service | Status | Store | Notes |
|---|---|---|---|
| `user-service` | Done (CRUD APIs tested) | PostgreSQL | Profile only; Cognito handles auth (phase 3) |
| `room-service` | Room CRUD + inventory edit (`PATCH`) done (creating a room opens 12 months of inventory); `ReserveRoom` gRPC done (M1) | PostgreSQL (+ Redis lock later) | Rooms, monthly inventory (JSONB days), reservations |
| `booking-service` | WIP (M2): core, Postgres adapter, `CreateBooking` gRPC and `GET /:id` built, not run yet; current step in `doc/plan.md` | PostgreSQL | Booking stores user and room **snapshots** plus nightly rates |
| `orchestrator-service` | Skeleton only | PostgreSQL | `saga_instances` schema designed; it gets the same `outbox` table as the other services |
| `payment-service` | Skeleton only (config done) | PostgreSQL + Stripe sandbox | |
| `notification-service` | Skeleton only | None in v1 (logs the notification); MongoDB + AWS SES in phase 3 | v1 is a Kafka consumer only: no HTTP server, no DB |

Key docs:
- `doc/services.md`: service responsibilities, endpoints, tech stack
- `doc/use-cases.md`: UC1 create booking, which includes the payment request. UC2 is cancel booking, UC3 is payment refund.
- `doc/uc1-create-booking/`: **detailed UC1 design, the source of truth** (lean v1: flows, saga/entity state machines, REST/gRPC/Kafka contracts, schema deltas, build order M1–M7; cut items live in `good-to-have.md`, so don't pull them into v1 work)
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
- A payment authorized after the release is voided (`CancelPayment`, M7); the Stripe session expires 31 min after it is created
- Every Kafka message goes through a transactional **outbox**; handlers are idempotent by checking the current status
- Handlers write with a **conditional update**, not a row lock: participants `WHERE status = <as read>`, the saga `WHERE version = <as read>`; 0 rows changed means run the handler again

**Repo layout:** one module per service, plus two shared modules in this repo, wired with `replace` directives:
- `contracts/` (`booking/contracts`): proto + Kafka message definitions, one package per owning service, no infra deps
- `platform/` (`booking/platform`): shared infra (outbox insert + poller, Kafka consumer loop), imported by adapters and `cmd/main.go`, never the core

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
- Timestamps are RFC 3339 UTC on every API and message. Every `main.go` with a DB starts with `time.Local = time.UTC`, so `TIMESTAMPTZ` scans as UTC; a timestamp made in Go uses `time.Now().UTC()`.
- IDs are UUIDs (`gen_random_uuid()`), handled as `string` in Go.
- **Money is stored in minor units as `int64`** (cents), with a separate `currency CHAR(3)`.
- Names don't repeat their package: `user.Service`, `user.Repository`, `user.NewService`, `handler.Handler` + `handler.New`, `config.DB`. The package's main entity is the exception (`room.Room`, `config.Config`). Core port/service methods drop the entity too: `GetAll`, `GetByID`, `Create`, `UpdateStatus`, `GetDetailsByUserID` (plural for slices). gRPC RPC names (`CreateBooking`) and HTTP handler methods (`h.GetUsers`) keep it, because their package doesn't imply the entity. Core files are `repository.go` / `service.go`; adapter files keep the entity prefix (`postgres/room_repository.go`).
- One name per field everywhere (REST, gRPC, Kafka, DB), `snake_case` in JSON. Flat structures prefix the owner (`user_phone_number`, `check_in_date`); nested objects drop it (`user.phone_number`, not `user.phone` or `user.user_phone_number`).
- Status enums are Go `type X string` constants with `IsValid()`, mirrored by Postgres `ENUM` types.
- Errors are wrapped with context: `fmt.Errorf("failed to ...: %w", err)`. Log the error value, not its string: `"error", err` / `slog.Any("error", err)`.
- Config comes from env: `SERVER_PORT`, `GRPC_PORT` (services with a gRPC server), `DB_HOST/PORT/USER/PASSWORD/NAME`, `KAFKA_BROKERS` (comma-separated). The orchestrator adds `ROOM_GRPC_ADDR`, `BOOKING_GRPC_ADDR`, `PAYMENT_GRPC_ADDR`, `GRPC_TIMEOUT`, `PAYMENT_GRPC_TIMEOUT`, `HOLD_TTL`, `DEADLINE_POLL` (durations like `15m`, read with `time.ParseDuration`). `.env.example` is only a template, so identical values across services (like the port) are intentional. The real `.env` is gitignored, local only, and I set it up myself for each service.
- Kafka: `franz-go` (`kgo`). Consumers use the shared loop in `platform/kafka`: a handler returns nil when the message is done and an error to be called again with the same record; the offset is committed only after nil.
- Stripe calls carry no fixed `Idempotency-Key`: a repeated call is made safe by the state (one payment row per saga, a PaymentIntent is captured once).
- `main.go` exits with `slog.Error` + `os.Exit(1)` when `config.Load()` or DB init fails.
- Commit messages look like `[scope] message`, e.g. `[booking svc] http handler and router` or `[UC: create booking] init`. This holds for every commit, whatever a plugin or skill prescribes: a change to our own skills is `[Skills] ...`.

## Commands

Run from inside a service directory, since each one is a separate module:
```
go run ./cmd            # start service (needs .env or env vars)
go build ./...
go vet ./...
go test ./...           # unit tests so far: room-service internal/room, booking-service internal/booking
go mod tidy
```
Infra: `infra/postgresql/docker-compose.yml` currently defines only `postgres-booking` (host port 5440). `infra/kafka` and `infra/mongodb` are empty placeholders. Migrations in `db/migration/` are applied by hand.

My setup:
- Windows + PowerShell: commands you give me must run there.
- I run the services and test with Postman / grpcurl myself.
- The local DB is a test DB and you are its admin through the `pg-*` MCP servers: query and change it freely (start DDL with `SET ROLE booking`). Docker is usually off, so when a task needs the real DB, tell me and I'll start it.
- Local DBs are throwaway in phase 1: change the `CREATE` file and rebuild (room-service has `drop_all.sql`). No `ALTER` migrations.
- `.gitattributes` forces LF, so `gofmt -l .` from the repo root should print nothing. Anything it lists is a real formatting problem.

## Roadmap

1. **Phase 1, local:** finish service APIs, then implement **UC1 create booking end to end** (orchestrator → room → booking → payment → notify). ← *current focus*: the UC1 build order M1–M7 (`doc/uc1-create-booking/README.md` section 6; M3–M6 build the main line only, M7 hardens). M1 is done; M2 is in progress (current step in `doc/plan.md`).
2. **Phase 2, local deployment:** deploy and test the full flow locally (no AWS Gateway, Cognito, or Lambda yet).
3. **Phase 3, AWS:** API Gateway, Cognito (plus a Lambda to sync users), SES, deployment.
4. **Phase 4:** CI/CD, monitoring, and logging.
5. **Phase 5:** later optimizations: Redis reservation lock or cache decorator, CDC.

Decided for UC1 (see `doc/uc1-create-booking/`): shared modules (`contracts/` with protos inside, `platform/`), Kafka topic names, v1 event contracts, booking statuses, `HOLD_TTL` 15 min, conditional updates instead of row locks, franz-go, no fixed Stripe idempotency key.
Undecided or not yet defined: Cancel booking (UC2) and Payment refund (UC3).
