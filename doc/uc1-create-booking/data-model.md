# UC1 data model changes (v1)

These are **deltas** against the current migrations, written as sketches. You write the real migrations.

## Outbox (Orchestrator, Room, Booking, Payment)

The same table, with the same name, in every service that sends messages.

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

**Insert**: `outbox.Insert(ctx, tx, Message{ID, Topic, Key, Payload})` writes one row inside the caller's transaction. `Payload` is the envelope as JSON bytes, so `platform` never imports `contracts`.

**Poller** (written once in `booking/platform/outbox`; imported by adapters and `cmd/main.go`, never the core):
1. Every ~500 ms, in one tx: `SELECT … WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`.
2. Produce each row (franz-go `ProduceSync`; `acks=all` is its default) → `UPDATE … SET published_at = now()` for the rows that were sent → commit. The produce call gets a timeout, because the tx holds the row locks while the broker is down.
3. A crash after produce means a re-publish, which consumers handle through the status check.

This poller is the only place that uses a row lock; handlers use a conditional update ([flows.md](flows.md#shape-of-every-message-handler)).

No inbox tables in v1: the status check makes handlers idempotent (G5).

## Orchestrator

`saga_instances`:

| Column | Change | Why |
|---|---|---|
| `user_id UUID NOT NULL` | add | owner check on `POST …/payment`. The handler rejects an `X-User-Id` that isn't a UUID with `400` before the insert |
| `deadline_at TIMESTAMPTZ NOT NULL` | add | set at saga start (`now + HOLD_TTL`) |
| `reservation_id` | `TEXT` → `UUID` | |
| `payment_id UUID` | done (was `payment_intent_id TEXT`) | the orchestrator knows our payment ID, not Stripe's |
| `version` | keep | every write is `UPDATE … WHERE version = <the version read>`, then `version + 1`; 0 rows changed = another writer was first |
| `failure_reason TEXT` | add | NULL while `IN_PROGRESS`; then `ROOM_REJECTED`, `PHASE_A_ERROR`, `DEADLINE` or `CAPTURE_FAILED`. The `reason` of `ReleaseRoom` / `ExpireBooking` is read from it |

```sql
CREATE INDEX idx_saga_deadline ON saga_instances (deadline_at)
    WHERE "status" = 'IN_PROGRESS';
```

`context` JSONB: user snapshot, room snapshot, dates, guests, nightly rates, total, currency. It holds everything needed to build the next call.

`saga_message_log` is **not part of v1**: the orchestrator gets the same `outbox` table as the other services, so one poller fits all four. Its migration file goes away. An audit log of every message is G8.

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
- After that, days change only through `PATCH /rooms/api/v1/{id}/inventories`: the `status` / `price` of days that already exist. It never changes `currency` and never adds a day or a month.

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
CREATE TYPE payment_state AS ENUM ('CREATED', 'AUTHORIZED', 'CAPTURED', 'CAPTURE_FAILED', 'CANCELLED');  -- CANCELLED: M7

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
- `saga_id UNIQUE` is what makes a repeated `CreateCheckoutSession` safe (no Stripe idempotency key): the second insert conflicts and the stored row's session is returned.
- `CANCELLED`: an authorization that arrived after the hold was released, voided at Stripe.

## Notification

No storage in v1 (log only). MongoDB log + SES = G9.
