# UC1 flows (v1)

Legend:
- `->>` sync call (REST / gRPC / Stripe API), `-->>` its response
- `-)` async message via **Kafka**, always written to the sender's **outbox** in the same transaction as the state change (Kafka isn't drawn)
- `Note` = what happens inside one DB transaction

---

## Phase A: hold room and create booking (sync)

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orchestrator
    participant R as Room svc
    participant B as Booking svc

    C->>O: POST /bookings (user headers) roomId, checkIn, checkOut, guests
    Note over O: INSERT saga IN_PROGRESS / RESERVING_ROOM<br/>deadline_at = now + HOLD_TTL, context = user + request
    O->>R: ReserveRoom(sagaId, roomId, checkIn, checkOut, expiresAt = deadline_at)
    Note over R: read inventory prices (every night AVAILABLE?)<br/>INSERT reservation RESERVED<br/>(exclusion constraint blocks overlaps)
    alt room not available
        R-->>O: FAILED_PRECONDITION ROOM_UNAVAILABLE
        Note over O: saga FAILED (nothing to undo)
        O-->>C: 409 ROOM_UNAVAILABLE
    else reserved
        R-->>O: reservationId, nightlyRates, total, currency, room snapshot
        Note over O: save to context, step CREATING_BOOKING
        O->>B: CreateBooking(sagaId, user + room snapshot, dates, guests, nightlyRates, total, expiresAt)
        Note over B: INSERT booking PENDING + nightly_rates
        B-->>O: bookingId
        Note over O: save bookingId, step AWAITING_PAYMENT
        O-->>C: 201 bookingId, PENDING, total, currency, expiresAt
    end
    Note over O,B: any other gRPC error (timeout, 5xx) in steps 3 or 6:<br/>tx: COMPENSATING / RELEASING_ROOM + outbox ReleaseRoom, reply 503, continue in phase D
```

Each `Note` over O is its own short transaction, and no transaction stays open during a gRPC call. If the orchestrator crashes mid-phase, the saga keeps its `deadline_at`, so the **deadline worker** releases whatever was held (phase D).

---

## Phase B: pay

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orchestrator
    participant P as Payment svc
    participant S as Stripe

    C->>O: POST /bookings/{id}/payment
    Note over O: load saga by bookingId<br/>check owner, step = AWAITING_PAYMENT, now < deadline_at
    O->>P: CreatePaymentIntent(sagaId, bookingId, amount, currency)
    Note over P: payment for sagaId exists? yes: return it
    P->>S: create PaymentIntent (capture_method=manual, Idempotency-Key = sagaId)
    S-->>P: pi_id, client_secret
    Note over P: INSERT payment CREATED
    P-->>O: paymentId, clientSecret
    O-->>C: 200 clientSecret, amount, expiresAt

    C->>S: Stripe.js confirms card
    alt declined
        S-->>C: error, user may retry (intent stays open)
    else authorized
        S-->>C: requires_capture, client polls GET /bookings/{id}
        S->>P: webhook payment_intent.amount_capturable_updated
        Note over P: verify signature<br/>payment CREATED to AUTHORIZED + outbox PaymentAuthorized
        P-->>S: 200
        P-)O: PaymentAuthorized
    end
```

- Locally, `stripe listen --forward-to localhost:<port>/payments/api/v1/webhooks/stripe` delivers webhooks and prints the signing secret.
- A duplicate webhook finds the payment already `AUTHORIZED` → return 200 and do nothing (rule 3).

---

## Phase C: confirm → capture → book → notify

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
    Note over O: AWAITING_PAYMENT to CONFIRMING_ROOM<br/>+ outbox ConfirmReservation
    O-)R: ConfirmReservation
    Note over R: reservation RESERVED to CONFIRMED + outbox ReservationConfirmed
    R-)O: ReservationConfirmed
    Note over O: CAPTURING_PAYMENT + outbox CapturePayment
    O-)P: CapturePayment
    P->>S: capture PaymentIntent (Idempotency-Key = sagaId-capture)
    S-->>P: succeeded
    Note over P: payment AUTHORIZED to CAPTURED + outbox PaymentCaptured
    P-)O: PaymentCaptured
    Note over O: CONFIRMING_BOOKING + outbox ConfirmBooking
    O-)B: ConfirmBooking
    Note over B: booking PENDING to BOOKED + outbox BookingConfirmed
    B-)O: BookingConfirmed
    Note over O: saga COMPLETED
    B-)N: BookingConfirmed (own consumer group)
    Note over N: log "email sent to ..."
```

- Order: **room first** (make sure we have it), **then take the money**, **then** mark the booking.
- `ConfirmReservation` can't fail in v1: only the orchestrator releases holds, and it can't be releasing and confirming the same saga at once (row lock on the saga).
- If Stripe rejects the capture, Payment replies `PaymentCaptureFailed`, and the orchestrator marks the saga FAILED and logs it for a manual fix (automatic handling = G10).
- If `PaymentAuthorized` arrives after the deadline, the saga is no longer in `AWAITING_PAYMENT`, so it's ignored and never captured.

---

## Phase D: release (deadline or phase A error)

```mermaid
sequenceDiagram
    autonumber
    participant W as Deadline worker (orchestrator)
    participant O as Orchestrator
    participant R as Room svc
    participant B as Booking svc

    Note over W: every DEADLINE_POLL, SELECT … FOR UPDATE SKIP LOCKED:<br/>step IN (RESERVING_ROOM, CREATING_BOOKING, AWAITING_PAYMENT)<br/>AND deadline_at < now()
    Note over W: tx: COMPENSATING / RELEASING_ROOM + outbox ReleaseRoom
    W-)R: ReleaseRoom (by sagaId)
    Note over R: reservation RESERVED to RELEASED (none? nothing to do)<br/>+ outbox RoomReleased
    R-)O: RoomReleased
    Note over O: EXPIRING_BOOKING + outbox ExpireBooking
    O-)B: ExpireBooking (by sagaId)
    Note over B: booking PENDING to EXPIRED (none? nothing to do)<br/>+ outbox BookingExpired
    B-)O: BookingExpired
    Note over O: saga FAILED
```

Both commands are looked up **by `sagaId`**, and "no row" is a valid success. That's why one path covers every case: timeout, a crash in phase A, or a gRPC timeout where we don't know whether the reservation or booking was created.

---

## Shape of every message handler

```mermaid
flowchart TD
    M["Kafka message"] --> T["BEGIN tx"]
    T --> L["SELECT row by sagaId FOR UPDATE<br/>(orchestrator: the saga, participant: its entity)"]
    L --> X{"current status?"}
    X -- "already target" --> RE["outbox: success reply again<br/>(orchestrator: nothing)"]
    X -- "allowed from" --> U["UPDATE status (+ step, context)"] --> OB["outbox: reply / next command"]
    X -- "anything else" --> IG["log ignored"]
    RE --> CM["COMMIT, then ack Kafka offset"]
    OB --> CM
    IG --> CM
```

- **Ack after commit.** A crash between the two means the message comes again, and the status check turns the duplicate into a no-op.
- Handlers that call Stripe (`CapturePayment`) check the status first, call Stripe with an idempotency key **outside** the transaction, then run the transaction above.
