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

    C->>O: POST /bookings (user headers) roomId, checkInDate, checkOutDate, guests
    Note over O: INSERT saga IN_PROGRESS / RESERVING_ROOM<br/>deadline_at = now + HOLD_TTL, context = user + request
    O->>R: ReserveRoom(sagaId, roomId, checkInDate, checkOutDate, expiresAt = deadline_at)
    Note over R: room ACTIVE? dates valid? every night AVAILABLE?<br/>read inventory prices, INSERT reservation RESERVED<br/>(exclusion constraint blocks overlaps)
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
    Note over O,B: unknown outcome in step 2 (timeout, UNAVAILABLE, INTERNAL) or any gRPC error in step 6:<br/>tx: COMPENSATING / RELEASING_ROOM + outbox ReleaseRoom, reply 503, continue in phase D
```

`ReserveRoom` errors come in two kinds:
- **Definite rejection** (`NOT_FOUND`, `INVALID_ARGUMENT`, `FAILED_PRECONDITION`): nothing was held, so the saga goes to `FAILED` and the client gets `404` / `400` / `409`. The diagram shows `ROOM_UNAVAILABLE` as the example.
- **Unknown outcome** (timeout, `UNAVAILABLE`, `INTERNAL`): a hold may exist, so the saga goes to phase D and the client gets `503`.

Any `CreateBooking` error goes to phase D, because the room is already held.

`ReserveRoom` itself checks the room (it exists and is `ACTIVE`), the dates (check-in before check-out, not in the past) and the inventory of every night. It is idempotent by `saga_id`: a repeated call returns the stored hold and its prices.

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
    O->>P: CreateCheckoutSession(sagaId, bookingId, amount, currency)
    Note over P: payment for sagaId exists? yes: retrieve its session, return the same URL
    P->>S: create Checkout Session (capture_method=manual, metadata.saga_id, expires_at = now + 31 min)
    S-->>P: cs_id, url
    Note over P: INSERT payment CREATED (psp_session_id = cs_id)
    P-->>O: paymentId, checkoutUrl
    O-->>C: 200 checkoutUrl, amount, currency, expiresAt

    C->>S: browser opens checkoutUrl, user enters the card on Stripe's page
    alt declined
        S-->>C: error on the page, user may retry (session stays open)
    else authorized
        S-->>C: redirect to success_url, client polls GET /bookings/{id}
        S->>P: webhook payment_intent.amount_capturable_updated
        Note over P: verify signature, find payment by metadata.saga_id<br/>payment CREATED to AUTHORIZED, save pi_id + outbox PaymentAuthorized
        P-->>S: 200
        P-)O: PaymentAuthorized
    end
```

- **Hosted page, not an embedded form.** The user pays on Stripe's own page (a Checkout Session). Our services only pass a URL, and no `client_secret` or card data goes through them.
- Session parameters: `mode=payment`, cards only (`allowed_payment_method_types=card`, or `payment_method_types=card` if your stripe-go version doesn't have it; without it the page offers every method enabled in the Stripe dashboard), one line item (`amount`, `currency`, and a required product name such as `Booking <bookingId>`), `payment_intent_data.capture_method=manual`, `payment_intent_data.metadata.saga_id`, `success_url` from config (`STRIPE_SUCCESS_URL`, with `{BOOKING_ID}` replaced), and `expires_at` = now + 31 min (Stripe's minimum is 30 min; 1 min spare for clock difference). v1 sets no `cancel_url`.
- **No fixed `Idempotency-Key`.** Stripe saves the first answer per key, a 5xx included, and returns it again for 24 h or more, so a retry with that key can never succeed; and the same key with another `expires_at` is rejected. A session whose answer was lost was never shown to the user and expires by itself. `payments.saga_id UNIQUE` keeps one row per saga: if two calls arrive at once, the second insert conflicts and returns the stored row's session.
- Stripe creates the PaymentIntent only when the user submits the card. So the payment row starts with a session ID and no intent ID; the webhook finds the row through `metadata.saga_id` and saves the `pi_…` ID that `CapturePayment` needs.
- A repeated call (page refresh, user comes back) retrieves the session from Stripe by `psp_session_id` and returns its URL again, so we store IDs only. Stripe only returns a URL while the session is open. If there is none (the user already paid, even when the webhook hasn't arrived yet), Payment answers `FAILED_PRECONDITION` and the orchestrator replies `409 BOOKING_NOT_PAYABLE`.
- A failed `CreateCheckoutSession` never moves the saga: the client gets `503` (Stripe down, timeout) or `409`, the saga stays in `AWAITING_PAYMENT`, and the client may call again until the deadline. Only phase A errors go to phase D.
- The session isn't tied to `deadline_at`: it expires 31 min after its own creation, so it can stay open after the hold expired. A payment made after the release is voided (see phase C and E5).
- Locally, `stripe listen --latest --forward-to localhost:<port>/payments/api/v1/webhooks/stripe` delivers webhooks and prints the signing secret. `--latest` matters: stripe-go rejects an event whose API version differs from the SDK's.
- A duplicate webhook finds the payment already `AUTHORIZED` → return 200 and do nothing (rule 3). An event without our `saga_id` (for example from `stripe trigger`) also gets 200 and is ignored.

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
    P->>S: capture PaymentIntent
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
- `ConfirmReservation` can't fail in v1: only the orchestrator releases holds, and it can't be releasing and confirming the same saga at once (every saga write is guarded by its `version`).
- If Stripe rejects the capture, Payment replies `PaymentCaptureFailed`, and the orchestrator marks the saga FAILED and logs it for a manual fix (automatic handling = G10). Only a definite refusal counts: HTTP 400 / 402 / 404 and the PaymentIntent is not `succeeded`. A timeout, 429 or 5xx is an error the consumer loop retries.
- A repeated capture needs no idempotency key: a PaymentIntent can be captured only once, and when Stripe refuses a second capture the PaymentIntent's status `succeeded` means "already captured".
- If `PaymentAuthorized` arrives after the release (the saga is in `RELEASING_ROOM` or `EXPIRING_BOOKING`, also when already `FAILED` there), it is never captured: the orchestrator sends `CancelPayment`, and Payment voids the authorization (`AUTHORIZED` → `CANCELLED`). There is no reply and no saga step.

---

## Phase D: release (deadline or phase A error)

```mermaid
sequenceDiagram
    autonumber
    participant W as Deadline worker (orchestrator)
    participant O as Orchestrator
    participant R as Room svc
    participant B as Booking svc

    Note over W: every DEADLINE_POLL, SELECT (no lock):<br/>status IN_PROGRESS, step IN (RESERVING_ROOM, CREATING_BOOKING, AWAITING_PAYMENT)<br/>AND deadline_at < now()
    Note over W: tx, only if the saga's version is unchanged:<br/>COMPENSATING / RELEASING_ROOM + outbox ReleaseRoom
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
    M["Kafka message"] --> L["read row by sagaId, no lock<br/>(orchestrator: the saga, participant: its entity)"]
    L --> X{"current status?"}
    X -- "already target" --> RE["tx: outbox success reply again<br/>(orchestrator: nothing)"]
    X -- "allowed from" --> U["tx: UPDATE ... WHERE still as read<br/>+ outbox reply / next command"]
    X -- "anything else" --> IG["log ignored"]
    U -- "0 rows: another writer was first" --> L
    U -- "1 row" --> CM["commit the Kafka offset"]
    RE --> CM
    IG --> CM
```

- **Conditional update, no row lock.** "Still as read" is `status = <from>` for a participant and `version = <the version read>` for the saga (then `version + 1`). The update and its outbox row are one transaction inside one repository method. If 0 rows change, the handler returns an error and the consumer loop calls it again from the read.
- **Commit the offset after the DB commit.** A crash between the two means the message comes again, and the status check turns the duplicate into a no-op.
- **A failed handler is called again with the same record** (wait 1 s, doubling up to 30 s). Not committing is not a retry: the client would simply hand over the next record. An error no retry can fix (bad JSON, unknown type, saga not found) is logged and skipped.
- **No row** is a normal case for the release commands: `ReleaseRoom` and `ExpireBooking` reply success with a null ID. For the confirm commands it is only logged.
- Handlers that call Stripe (`CapturePayment`, `CancelPayment`) check the status first, call Stripe **outside** any transaction, then do the write above.
