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
| `payment_intent_id` | rename → `payment_id UUID` | the orchestrator knows our payment ID, not Stripe's |
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

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE reservations
    ADD COLUMN saga_id UUID NOT NULL UNIQUE,
    ADD CONSTRAINT reservations_no_overlap
        EXCLUDE USING gist (room_id WITH =, daterange(check_in, check_out) WITH &&)
        WHERE ("status" IN ('RESERVED', 'CONFIRMED'));
```

- `daterange` is `[check_in, check_out)`, so the check-out day is free for the next guest.
- An overlap fails with SQLSTATE `23P01`, which the repository maps to `ErrRoomUnavailable`, which becomes gRPC `FAILED_PRECONDITION`.
- `reservation_status` enum: v1 uses `RESERVED`, `CONFIRMED`, `RELEASED` (`EXPIRED` stays unused until G6).
- `inventory.days` JSONB: each day becomes `{ "status": "AVAILABLE" | "MAINTENANCE", "price": 12000, "currency": "USD" }`. Remove `RESERVED` / `BOOKED` / `booking_id`, because occupancy lives only in `reservations`.

## Booking

| Change | Why |
|---|---|
| `saga_id UUID NOT NULL UNIQUE` | idempotent CreateBooking + lookup for commands |
| `reservation_id UUID NOT NULL` | reference only |
| `expires_at TIMESTAMPTZ` | UI countdown while `PENDING` |
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
    psp_payment_intent_id  TEXT NOT NULL UNIQUE,     -- pi_...; the webhook looks up by this
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Notification

No storage in v1 (log only). MongoDB log + SES = G9.
