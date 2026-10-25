# UC1 data model changes (v1)

These are **deltas** against the current migrations, written as sketches. You write the real migrations.

## Outbox (Room, Booking, Payment)

The orchestrator uses its `saga_message_log` as the outbox instead (see below).

```sql
CREATE TABLE outbox (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id    UUID NOT NULL UNIQUE,
    topic         TEXT NOT NULL,
    msg_key       TEXT NOT NULL,             -- sagaId
    payload       JSONB NOT NULL,            -- full envelope
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);
CREATE INDEX idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;
```

**Poller** (write it once in `booking/platform/outbox`, parametrized by table name; imported only by adapters):
1. Every ~500 ms, in one tx: `SELECT … WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`.
2. Produce each row (sync, `acks=all`) → `UPDATE … SET published_at = now()` → commit.
3. A crash after produce means a re-publish, which consumers handle through the status check.

No inbox tables in v1: the status check makes handlers idempotent (G5).

## Orchestrator

`saga_instances`:

| Column | Change | Why |
|---|---|---|
| `user_id TEXT NOT NULL` | add | owner check on `POST …/payment` |
| `deadline_at TIMESTAMPTZ NOT NULL` | add | set at saga start (`now + HOLD_TTL`) |
| `reservation_id` | `TEXT` → `UUID` | |
| `payment_id UUID` | done (was `payment_intent_id TEXT`) | the orchestrator knows our payment ID, not Stripe's |
| `version` | drop (optional) | all writes use `SELECT … FOR UPDATE` |

```sql
CREATE INDEX idx_saga_deadline ON saga_instances (deadline_at)
    WHERE "status" = 'IN_PROGRESS';
```

`context` JSONB: user snapshot, room snapshot, dates, guests, nightly rates, total, currency. It holds everything needed to build the next call.

`saga_message_log` becomes the orchestrator's **outbox** (OUT rows only in v1):

| Column | Change |
|---|---|
| `message_id UUID NOT NULL UNIQUE` | add |
| `topic TEXT NOT NULL` | add |
| `published_at TIMESTAMPTZ` | add |
| `"type"` | ENUM → `TEXT` + `CHECK` (grows with every use case) |
| `correlation_id` / `causation_id` | make nullable or drop for now (G7) |

Logging IN rows for audit is G8.

## Room

Done in `room-service/db/migration/create_reservations_table.sql`. The migrations are full `CREATE`s, so the local DB is re-built with `drop_all.sql` + the `create_*.sql` files (rooms, inventories, reservations), not altered.

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- new in CREATE TABLE reservations
saga_id        UUID NOT NULL UNIQUE,
total_amount   BIGINT NOT NULL,     -- snapshot, minor units
currency       CHAR(3) NOT NULL,    -- snapshot
nightly_rates  JSONB NOT NULL,      -- snapshot, [{"date":"2026-10-25","price":12000}]: date YYYY-MM-DD, price minor units
CONSTRAINT reservations_no_overlap
    EXCLUDE USING gist (room_id WITH =, daterange(check_in_date, check_out_date, '[)') WITH &&)
    WHERE ("status" IN ('RESERVED', 'CONFIRMED')),
CONSTRAINT check_dates CHECK (check_in_date < check_out_date)
```

- `daterange` is `[check_in_date, check_out_date)`, so the check-out day is free for the next guest.
- `check_dates`: a zero-night stay is an empty range. It overlaps nothing, so it would pass the exclusion constraint.
- The insert uses `ON CONFLICT DO NOTHING` with no target, so a `saga_id` conflict and an overlap both skip the row (0 rows returned). The repository then looks up by `saga_id`: a row means an identical call committed first, so it returns that row; no row means an overlap, so `ErrRoomUnavailable`, which becomes gRPC `FAILED_PRECONDITION`. There is no "check, then insert": the insert is the check.
- Price snapshot (`total_amount`, `currency`, `nightly_rates`): written once at reserve time from the inventory prices, and returned as stored when ReserveRoom is called again with the same `saga_id`. No `DEFAULT`, so a forgotten value is an error.
- The primary key is `id`, like `rooms` / `bookings` / `users`. It is still called `reservation_id` in gRPC, Kafka and other tables.
- No extra indexes: v1 looks rows up by `saga_id` (`UNIQUE`), and the constraint's GiST index does the overlap check. An `expires_at` index belongs to the G6 sweeper.
- `reservation_status` enum: v1 uses `RESERVED`, `CONFIRMED`, `RELEASED` (`EXPIRED` is not in the enum; G6 adds it with `ALTER TYPE ... ADD VALUE`).
- `inventories.currency` is `CHAR(3)`, like every other currency column.
- `inventories.days` JSONB: each day becomes `{ "status": "AVAILABLE" | "MAINTENANCE", "price": 12000 }`. Remove `RESERVED` / `BOOKED` / `booking_id`, because occupancy lives only in `reservations`. `currency` stays a column on the row (one currency per room-month), not repeated on every day.
- Creating a room also creates `inventories` for the current month + next 11 (all days `AVAILABLE`), in the same transaction as the room insert.

## Booking

| Change | Why |
|---|---|
| `saga_id UUID NOT NULL UNIQUE` | idempotent CreateBooking + lookup for commands |
| `reservation_id UUID NOT NULL` | reference only |
| `expires_at TIMESTAMPTZ NOT NULL` | UI countdown while `PENDING`; CreateBooking always sends it |
| `user_id`, `room_id` UUID (were TEXT); `total_amount` without `DEFAULT` | IDs are UUIDs everywhere; a forgotten total is an error, not 0 |
| `nightly_rates JSONB NOT NULL` (`[{"date":"2026-10-25","price":12000}]`, date `YYYY-MM-DD` like the proto), no `nightly_rates` table | written once by CreateBooking and read with the booking, never queried by night (same as `reservations.nightly_rates`) |
| `booking_status` enum | see [state-machines.md](state-machines.md#booking) |

## Payment (new)

```sql
CREATE TYPE payment_state AS ENUM ('CREATED', 'AUTHORIZED', 'CAPTURED', 'CAPTURE_FAILED');

CREATE TABLE payments (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id                UUID NOT NULL UNIQUE,
    booking_id             UUID NOT NULL,
    amount                 BIGINT NOT NULL,
    currency               CHAR(3) NOT NULL,
    "status"               payment_state NOT NULL,
    psp_session_id         TEXT NOT NULL UNIQUE,     -- cs_...; retrieve the page URL again on a repeated call
    psp_payment_intent_id  TEXT UNIQUE,              -- pi_...; NULL until the webhook, then used by CapturePayment
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- Stripe creates the PaymentIntent when the user pays on the hosted page, so `psp_payment_intent_id` is unknown at insert time.
- The webhook finds the row by `saga_id` (sent to Stripe as PaymentIntent metadata), not by a Stripe ID.
- Neither the page URL nor any Stripe secret is stored.

## Notification

No storage in v1 (log only). MongoDB log + SES = G9.
