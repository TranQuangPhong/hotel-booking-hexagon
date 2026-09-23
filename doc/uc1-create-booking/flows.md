# UC1 flows

One diagram per phase. Legend:

- `->>` sync call (REST / gRPC / Stripe API), `-->>` its response
- `-)` async message via **Kafka**. It is always written to the sender's **outbox** in the same transaction as the state change, then published by the outbox poller. Kafka is not drawn as a box, to keep the diagrams readable.
- `Note` = what happens inside one DB transaction

Message fields are in [contracts.md](contracts.md), and the step and status names in [state-machines.md](state-machines.md).

---

## Phase A: hold room and create booking (sync)

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orchestrator
    participant R as Room svc
    participant B as Booking svc

    C->>O: POST /bookings (JWT, Idempotency-Key) roomId, checkIn, checkOut, guests
    Note over O: saga exists for (userId, Idempotency-Key)?<br/>yes: return its current result, stop here
    Note over O: INSERT saga: IN_PROGRESS / RESERVING_ROOM<br/>context = user snapshot (JWT) + request
    O->>R: ReserveRoom(sagaId, roomId, checkIn, checkOut)
    Note over R: tx: expire dead overlapping holds<br/>read inventory prices (all nights AVAILABLE?)<br/>INSERT reservation RESERVED, expires_at = now + HOLD_TTL<br/>(exclusion constraint blocks overlaps)
    alt room not available
        R-->>O: FAILED_PRECONDITION ROOM_UNAVAILABLE
        Note over O: saga FAILED
        O-->>C: 409 ROOM_UNAVAILABLE
    else reserved
        R-->>O: reservationId, expiresAt, nightlyRates, total, currency, room snapshot
        Note over O: save reservationId + prices to context<br/>step CREATING_BOOKING
        O->>B: CreateBooking(sagaId, user + room snapshot, dates, guests, nightlyRates, total, expiresAt)
        Note over B: tx: INSERT booking PENDING + nightly_rates
        B-->>O: bookingId
        Note over O: save bookingId<br/>step AWAITING_PAYMENT, deadline_at = expiresAt
        O-->>C: 201 bookingId, status PENDING, total, currency, expiresAt
    end
```

Notes
- Steps 3 and 5 are each a short transaction. Rule 5 applies: no transaction stays open across the gRPC call. If the orchestrator crashes in between, the saga is left in `RESERVING_ROOM` or `CREATING_BOOKING` and the **recovery worker** re-sends the call. That's safe because of `sagaId` idempotency.
- A client retry with the same `Idempotency-Key` never creates a second hold (step 2).

### Phase A, compensation: CreateBooking fails

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orchestrator
    participant R as Room svc

    Note over O: CreateBooking failed after retries (step CREATING_BOOKING)
    Note over O: tx: COMPENSATING / RELEASING_ROOM<br/>+ outbox ReleaseRoom
    O-->>C: 503 BOOKING_FAILED (try again)
    O-)R: ReleaseRoom(reservationId)
    Note over R: tx: reservation RESERVED to RELEASED<br/>+ outbox RoomReleased
    R-)O: RoomReleased
    Note over O: no bookingId, so saga FAILED
```

---

## Phase B: pay (sync start + user on Stripe + webhook)

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orchestrator
    participant P as Payment svc
    participant S as Stripe

    C->>O: POST /bookings/{id}/payment (JWT)
    Note over O: load saga by bookingId<br/>check owner = JWT user, step = AWAITING_PAYMENT, now < deadline_at
    O->>P: CreatePaymentIntent(sagaId, bookingId, amount, currency)
    Note over P: payment for sagaId exists? yes: return it
    P->>S: create PaymentIntent (capture_method=manual,<br/>Stripe Idempotency-Key = sagaId, metadata.sagaId)
    S-->>P: pi_id, client_secret
    Note over P: INSERT payment CREATED (saga_id UNIQUE)
    P-->>O: paymentId, clientSecret
    Note over O: save paymentId in saga
    O-->>C: 200 clientSecret, amount, currency, expiresAt

    C->>S: Stripe.js confirms card (card data never touches our services)
    alt card declined
        S-->>C: error, user may retry with another card (intent stays open)
    else authorized
        S-->>C: requires_capture, redirect to booking page (polls GET /bookings/{id})
        S->>P: webhook payment_intent.amount_capturable_updated
        Note over P: verify signature, dedupe by Stripe event.id<br/>tx: payment CREATED to AUTHORIZED<br/>+ outbox PaymentAuthorized
        P-->>S: 200
        P-)O: PaymentAuthorized
    end
```

Notes
- The Stripe call happens **before** the DB insert and is protected by Stripe's own `Idempotency-Key`. If the insert fails, a retry gets the same intent back from Stripe.
- The webhook handler only records the fact and returns 200 quickly. The saga logic runs in the orchestrator.
- The redirect and "success page" are UX only. The booking is confirmed only when Phase C finishes, which is why the client polls.

---

## Phase C: confirm → capture → book → notify (async)

```mermaid
sequenceDiagram
    autonumber
    participant O as Orchestrator
    participant R as Room svc
    participant P as Payment svc
    participant S as Stripe
    participant B as Booking svc
    participant N as Notification svc

    P-)O: PaymentAuthorized
    Note over O: step AWAITING_PAYMENT to CONFIRMING_ROOM<br/>+ outbox ConfirmReservation
    O-)R: ConfirmReservation(reservationId)
    Note over R: reservation RESERVED to CONFIRMED<br/>(accepted even if expires_at just passed)<br/>+ outbox ReservationConfirmed
    R-)O: ReservationConfirmed
    Note over O: step CAPTURING_PAYMENT<br/>+ outbox CapturePayment
    O-)P: CapturePayment(paymentId)
    P->>S: capture PaymentIntent (Idempotency-Key = sagaId + capture)
    S-->>P: succeeded
    Note over P: payment AUTHORIZED to CAPTURED<br/>+ outbox PaymentCaptured
    P-)O: PaymentCaptured
    Note over O: step CONFIRMING_BOOKING<br/>+ outbox ConfirmBooking
    O-)B: ConfirmBooking(bookingId)
    Note over B: booking PENDING to BOOKED, payment_status COMPLETED<br/>+ outbox BookingConfirmed
    B-)O: BookingConfirmed
    Note over O: saga COMPLETED
    B-)N: BookingConfirmed (same event, own consumer group)
    Note over N: dedupe by messageId, send email<br/>(Phase 1: log only, SES in Phase 3)
```

Notes
- Why this order: **room first** (the only step that can legitimately fail), **then take the money**, **then** tell the booking. Nobody is charged for a room we couldn't confirm.
- If `ReservationConfirmFailed` comes back (the hold was already released), go to the Phase D path from `CANCELLING_PAYMENT`: the authorization is voided and the user isn't charged.
- `BookingConfirmed` is a **fat event**: it carries email, room number, dates and amount, so Notification never calls another service.

---

## Phase D: expire (deadline reached, no authorization)

```mermaid
sequenceDiagram
    autonumber
    participant W as Orchestrator deadline worker
    participant O as Orchestrator
    participant P as Payment svc
    participant S as Stripe
    participant R as Room svc
    participant B as Booking svc
    participant N as Notification svc

    Note over W: every DEADLINE_POLL:<br/>SELECT sagas WHERE step = AWAITING_PAYMENT<br/>AND deadline_at < now() FOR UPDATE SKIP LOCKED
    Note over W: tx: COMPENSATING / CANCELLING_PAYMENT<br/>+ outbox CancelPayment
    W-)P: CancelPayment(sagaId)
    alt intent exists
        P->>S: cancel PaymentIntent (voids authorization if any)
        S-->>P: canceled
    end
    Note over P: payment to CANCELLED (or "cancelled" marker if none existed)<br/>+ outbox PaymentCancelled
    P-)O: PaymentCancelled
    Note over O: step RELEASING_ROOM + outbox ReleaseRoom
    O-)R: ReleaseRoom(reservationId)
    Note over R: reservation RESERVED to RELEASED + outbox RoomReleased
    R-)O: RoomReleased
    Note over O: step EXPIRING_BOOKING + outbox ExpireBooking
    O-)B: ExpireBooking(bookingId)
    Note over B: booking PENDING to EXPIRED, payment_status CANCELLED<br/>+ outbox BookingExpired
    B-)O: BookingExpired
    Note over O: saga FAILED
    B-)N: BookingExpired (optional "your hold expired" email)
```

Notes
- **Cancel the payment first, release the room second.** Released first, the room could be sold to someone else while this user's authorization is still alive.
- `CancelPayment` is keyed by `sagaId`, not `paymentId`. It also works when the user never clicked Pay, or when an intent is being created at this very moment (E4).
- A `PaymentAuthorized` arriving now is ignored by the orchestrator (step ≠ `AWAITING_PAYMENT`). The cancel already voided that authorization.

---

## Anatomy of one async step

Every Kafka consumer in this design follows the same shape. Learn it once and reuse it.

**Orchestrator consuming a reply event:**

```mermaid
flowchart TD
    M["Kafka message<br/>(reply event)"] --> T["BEGIN tx"]
    T --> I["INSERT saga_message_log IN<br/>ON CONFLICT (message_id) DO NOTHING"]
    I --> D{"inserted?"}
    D -- "no: duplicate" --> CM["COMMIT, ack offset"]
    D -- "yes" --> L["SELECT saga FOR UPDATE"]
    L --> V{"expected in<br/>current step?"}
    V -- "no: late / out of order" --> G["log ignored"] --> CM
    V -- "yes" --> U["UPDATE saga step / status / context"]
    U --> OB["INSERT saga_message_log OUT<br/>(next command = outbox row)"]
    OB --> CM
```

**Participant (Room / Booking / Payment) consuming a command:**

```mermaid
flowchart TD
    M["Kafka message<br/>(command)"] --> T["BEGIN tx"]
    T --> I["INSERT processed_messages<br/>ON CONFLICT DO NOTHING"]
    I --> D{"inserted?"}
    D -- "no: duplicate" --> CM["COMMIT, ack offset"]
    D -- "yes" --> U["guarded UPDATE<br/>WHERE status = ANY(allowedFrom)"]
    U --> R{"rows = 1?"}
    R -- "yes" --> OK["outbox: success reply"] --> CM
    R -- "no" --> S{"already in<br/>target status?"}
    S -- "yes: done before" --> OK
    S -- "no: invalid" --> F["outbox: failure reply<br/>(or log if no failure reply defined)"] --> CM
```

Why "already in target → reply success again": the **recovery worker** re-sends a stuck step's command with a **new `messageId`**, so inbox dedupe doesn't catch it. The participant must answer from the current state, otherwise the saga waits forever.

Steps that call Stripe (`CapturePayment`, `CancelPayment`) call Stripe **before** opening the transaction, using a Stripe idempotency key. A crash after the Stripe call just repeats the same Stripe call on redelivery.
