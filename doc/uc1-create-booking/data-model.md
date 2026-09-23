# UC1 data model changes

These are **deltas** against the current migrations, written as sketches. You write the real migrations.

## Shared by every service that consumes or produces Kafka

Room, Booking and Payment each get these two tables. The orchestrator uses `saga_message_log` for both jobs instead (see below).

```sql
-- Outbox: written in the same tx as the state change; the poller publishes it.
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

-- Inbox: dedupe incoming messages; inserted in the same tx as the side effect.
CREATE TABLE processed_messages (
    message_id    UUID PRIMARY KEY,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Outbox poller** (one goroutine per service):
1. Every ~500 ms, in one transaction: `SELECT … WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`.
2. Produce each row (sync, `acks=all`), then `UPDATE outbox SET published_at = now()`.
3. A crash between produce and update means the message is re-published. That's fine, because consumers dedupe.
4. Order is kept per key because rows are sent in `id` order.

Cleanup of old published rows is for later.

## Orchestrator

`saga_instances`: add or change

| Column | Change | Why |
|---|---|---|
| `user_id TEXT NOT NULL` | add | owner check on `POST /bookings/{id}/payment`; scope of the idempotency key |
| `idempotency_key TEXT NOT NULL` | add, `UNIQUE (user_id, idempotency_key)` | client double-click → the same saga |
| `deadline_at TIMESTAMPTZ` | add | set when entering `AWAITING_PAYMENT` |
| `reservation_id` | `TEXT` → `UUID` | reservations use UUID |
| `payment_intent_id` | rename → `payment_id UUID` | the orchestrator knows our payment ID, never Stripe's |
| `version` | keep or drop | all writes use `SELECT … FOR UPDATE` in short transactions, so it's optional |

```sql
CREATE INDEX idx_saga_deadline ON saga_instances (deadline_at)
    WHERE current_step = 'AWAITING_PAYMENT';
```

`context` JSONB holds everything needed to **rebuild any call or command** (for the recovery worker): user snapshot, room snapshot, dates, guests, nightly rates, total, currency, `expiresAt`.

`saga_message_log` works as **both outbox (OUT) and inbox (IN)**:

| Column | Change |
|---|---|
| `message_id UUID NOT NULL UNIQUE` | add. Inbox dedupe for IN rows, and the envelope `messageId` for OUT rows |
| `topic TEXT` | add (OUT rows) |
| `published_at TIMESTAMPTZ` | add (OUT rows; the poller sets it) |
| `"type"` | ENUM `message_type` → `TEXT` + `CHECK` (the list grows with every use case; see plan.md) |

```sql
CREATE INDEX idx_saga_log_unpublished ON saga_message_log (id)
    WHERE direction = 'OUT' AND published_at IS NULL;
```

## Room

`reservations`:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE reservations
    ADD COLUMN saga_id UUID NOT NULL UNIQUE,
    ADD CONSTRAINT reservations_no_overlap
        EXCLUDE USING gist (room_id WITH =, daterange(check_in, check_out) WITH &&)
        WHERE ("status" IN ('RESERVED', 'CONFIRMED'));
```

- `daterange(check_in, check_out)` is `[check_in, check_out)` by default, which matches "check-out day is free for the next guest".
- Inserting an overlapping row fails with SQLSTATE `23P01` (exclusion_violation). The repository maps it to the domain error `ErrRoomUnavailable`, which the gRPC layer turns into `FAILED_PRECONDITION`.
- `ReserveRoom` transaction:
  1. `UPDATE reservations SET status='EXPIRED' WHERE room_id=$1 AND status='RESERVED' AND expires_at + HOLD_GRACE < now() AND daterange(...) && daterange($2,$3)`
  2. read inventory for the months covering the stay; every night must exist and be `AVAILABLE`
  3. `INSERT` the reservation
  4. commit

`inventory.days` JSONB: each day becomes `{ "status": "AVAILABLE" | "MAINTENANCE", "price": 12000, "currency": "USD" }`.
Remove `RESERVED` / `BOOKED` and `booking_id`. Occupancy lives only in `reservations`, so the Go `InventoryDay` struct and `InventoryDayStatus` constants change to match.

Plus `outbox` and `processed_messages`.

## Booking

| Change | Why |
|---|---|
| `saga_id UUID NOT NULL UNIQUE` | idempotent `CreateBooking` |
| `reservation_id UUID NOT NULL` | link to the hold (reference only, no FK across services) |
| `expires_at TIMESTAMPTZ` | countdown in the UI while `PENDING` |
| `booking_status` / `payment_status` enums | see [state-machines.md](state-machines.md#booking) |

Plus `outbox` and `processed_messages`.

## Payment (new service)

```sql
CREATE TYPE payment_state AS ENUM ('CREATED','AUTHORIZED','CAPTURED','CANCELLED','CAPTURE_FAILED');

CREATE TABLE payments (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_id                UUID NOT NULL UNIQUE,
    booking_id             UUID NOT NULL,
    amount                 BIGINT NOT NULL,           -- minor units
    currency               CHAR(3) NOT NULL,
    "status"               payment_state NOT NULL,
    psp                    TEXT NOT NULL DEFAULT 'stripe',
    psp_payment_intent_id  TEXT UNIQUE,               -- pi_...
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Webhook dedupe + audit (Stripe retries webhooks).
CREATE TABLE psp_webhook_events (
    psp_event_id  TEXT PRIMARY KEY,                   -- evt_...
    "type"        TEXT NOT NULL,
    payload       JSONB NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- Cancel before the intent exists (E4): `CancelPayment` with no payment row inserts a row `status = CANCELLED, psp_payment_intent_id = NULL`. A later `CreatePaymentIntent` for that `saga_id` then finds a `CANCELLED` row and returns `PAYMENT_CANCELLED`.
- Plus `outbox` and `processed_messages`.

## Notification (MongoDB)

Collection `notifications`: `{ messageId (unique index), bookingId, type, channel: "email", to, status: "SENT" | "FAILED", createdAt }`.
The unique `messageId` is its inbox. Phase 1 "sends" by logging, and Phase 3 plugs in SES behind a `Sender` port.
