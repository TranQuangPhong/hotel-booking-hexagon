# UC1 state machines

There are four state machines. Only the **saga** knows the whole flow. Each entity only knows its own legal transitions.

## Saga

Owned by the orchestrator (`saga_instances`).

```mermaid
stateDiagram-v2
    state has_booking <<choice>>

    [*] --> RESERVING_ROOM: POST /bookings
    RESERVING_ROOM --> CREATING_BOOKING: room reserved
    RESERVING_ROOM --> FAILED: ROOM_UNAVAILABLE
    CREATING_BOOKING --> AWAITING_PAYMENT: booking created
    CREATING_BOOKING --> RELEASING_ROOM: create failed

    AWAITING_PAYMENT --> CONFIRMING_ROOM: PaymentAuthorized
    AWAITING_PAYMENT --> CANCELLING_PAYMENT: deadline reached

    CONFIRMING_ROOM --> CAPTURING_PAYMENT: ReservationConfirmed
    CONFIRMING_ROOM --> CANCELLING_PAYMENT: ReservationConfirmFailed
    CAPTURING_PAYMENT --> CONFIRMING_BOOKING: PaymentCaptured
    CONFIRMING_BOOKING --> COMPLETED: BookingConfirmed

    CANCELLING_PAYMENT --> RELEASING_ROOM: PaymentCancelled
    RELEASING_ROOM --> has_booking: RoomReleased
    has_booking --> EXPIRING_BOOKING: bookingId set
    has_booking --> FAILED: no booking
    EXPIRING_BOOKING --> FAILED: BookingExpired

    COMPLETED --> [*]
    FAILED --> [*]
```

`current_step` holds the step name. `status` is derived from it:

| `status` | steps |
|---|---|
| `IN_PROGRESS` | `RESERVING_ROOM`, `CREATING_BOOKING`, `AWAITING_PAYMENT`, `CONFIRMING_ROOM`, `CAPTURING_PAYMENT`, `CONFIRMING_BOOKING` |
| `COMPENSATING` | `CANCELLING_PAYMENT`, `RELEASING_ROOM`, `EXPIRING_BOOKING` |
| `COMPLETED` / `FAILED` | terminal; `current_step` keeps the last step for debugging |

### Transition table (what the orchestrator does)

| Current step | Input | Action (one tx) | Next step |
|---|---|---|---|
| — | `POST /bookings` | insert saga | `RESERVING_ROOM` → call `ReserveRoom` |
| `RESERVING_ROOM` | gRPC ok | save reservation + prices | `CREATING_BOOKING` → call `CreateBooking` |
| `RESERVING_ROOM` | `ROOM_UNAVAILABLE` | — | **FAILED** |
| `CREATING_BOOKING` | gRPC ok | save bookingId, `deadline_at` | `AWAITING_PAYMENT` |
| `CREATING_BOOKING` | gRPC failed after retries | outbox `ReleaseRoom` | `RELEASING_ROOM` |
| `AWAITING_PAYMENT` | `POST /bookings/{id}/payment` | call `CreatePaymentIntent`, save paymentId | *(same step)* |
| `AWAITING_PAYMENT` | `PaymentAuthorized` | outbox `ConfirmReservation` | `CONFIRMING_ROOM` |
| `AWAITING_PAYMENT` | deadline worker | outbox `CancelPayment` | `CANCELLING_PAYMENT` |
| `CONFIRMING_ROOM` | `ReservationConfirmed` | outbox `CapturePayment` | `CAPTURING_PAYMENT` |
| `CONFIRMING_ROOM` | `ReservationConfirmFailed` | outbox `CancelPayment` | `CANCELLING_PAYMENT` |
| `CAPTURING_PAYMENT` | `PaymentCaptured` | outbox `ConfirmBooking` | `CONFIRMING_BOOKING` |
| `CAPTURING_PAYMENT` | `PaymentCaptureFailed` | **parked**, see E7 | — |
| `CONFIRMING_BOOKING` | `BookingConfirmed` | — | **COMPLETED** |
| `CANCELLING_PAYMENT` | `PaymentCancelled` | outbox `ReleaseRoom` | `RELEASING_ROOM` |
| `RELEASING_ROOM` | `RoomReleased` | bookingId set? outbox `ExpireBooking` | `EXPIRING_BOOKING`, else **FAILED** |
| `EXPIRING_BOOKING` | `BookingExpired` | — | **FAILED** |
| *any other combination* | any message | log "ignored" and ack | *(unchanged)* |

The last row is what makes late, duplicate and out-of-order messages harmless. For example, a `PaymentAuthorized` that arrives after the deadline fired is ignored, and the cancel already voided that authorization.

**Recovery worker:** a saga in a non-waiting, non-terminal step (anything except `AWAITING_PAYMENT`, `COMPLETED`, `FAILED`) with `updated_at < now() - STUCK_AFTER` gets its step's call or command **re-sent**, with a new `messageId` and the same `sagaId`.

---

## Reservation

Owned by room svc.

```mermaid
stateDiagram-v2
    [*] --> RESERVED: ReserveRoom
    RESERVED --> CONFIRMED: ConfirmReservation
    RESERVED --> RELEASED: ReleaseRoom
    RESERVED --> EXPIRED: sweeper / stale cleanup (after expires_at + HOLD_GRACE)
    CONFIRMED --> RELEASED: UC2 cancel (later)
```

- **Occupying statuses:** `RESERVED`, `CONFIRMED`. The exclusion constraint only looks at these.
- `ConfirmReservation` accepts `RESERVED` **even if `expires_at` has passed**. The orchestrator already decided within its deadline, so message latency must not break the booking. Only `RELEASED` or `EXPIRED` means failure.

## Booking

Owned by booking svc.

```mermaid
stateDiagram-v2
    [*] --> PENDING: CreateBooking (room already held)
    PENDING --> BOOKED: ConfirmBooking
    PENDING --> EXPIRED: ExpireBooking
    BOOKED --> CANCELLED: UC2 (later)
    BOOKED --> CHECKED_IN: later
    CHECKED_IN --> CHECKED_OUT: later
    BOOKED --> NO_SHOW: later
```

**Proposed enum changes (confirm):**

| Enum | Today | Proposed | Why |
|---|---|---|---|
| `booking_status` | PENDING, RESERVED, RESERVATION_FAILED, PAYMENT_FAILED, BOOKED, CANCELLED, CHECKED_IN, CHECKED_OUT, NO_SHOW | PENDING, BOOKED, **EXPIRED**, CANCELLED, CHECKED_IN, CHECKED_OUT, NO_SHOW | room-first, so a booking is born with the room held (`RESERVED` = `PENDING`); a reservation failure never creates a booking; a declined card doesn't end the saga, the deadline does → `EXPIRED` |
| `payment_status` (on booking, a read-only mirror for the UI) | PENDING, COMPLETED, FAILED, REFUNDED, PARTIALLY_REFUNDED | PENDING, COMPLETED, **CANCELLED**, REFUNDED, PARTIALLY_REFUNDED | expiry means "no money collected", not "failed"; `REFUNDED*` stays for UC3 |

`booking.payment_status` changes only together with `status`: `BOOKED` + `COMPLETED`, or `EXPIRED` + `CANCELLED`. Payment svc stays the source of truth for the payment itself.

## Payment

Owned by payment svc.

```mermaid
stateDiagram-v2
    [*] --> CREATED: CreatePaymentIntent
    CREATED --> AUTHORIZED: webhook amount_capturable_updated
    AUTHORIZED --> CAPTURED: CapturePayment
    CREATED --> CANCELLED: CancelPayment
    AUTHORIZED --> CANCELLED: CancelPayment (voids the hold)
    AUTHORIZED --> CAPTURE_FAILED: capture error (E7, later)
```

| Stripe PaymentIntent status | Our `payments.status` |
|---|---|
| `requires_payment_method`, `requires_confirmation`, `requires_action` | `CREATED` (declined attempts stay here; optionally log each attempt) |
| `requires_capture` | `AUTHORIZED` |
| `succeeded` | `CAPTURED` |
| `canceled` | `CANCELLED` |

---

## Guards

Every status write uses `WHERE status = ANY($allowedFrom)`. If 0 rows are updated and the row is already in the target status, reply success again. Otherwise reply failure or log it.

| Entity | Target | Allowed from | Failure reply |
|---|---|---|---|
| reservation | `CONFIRMED` | `RESERVED` | `ReservationConfirmFailed` |
| reservation | `RELEASED` | `RESERVED` | — (`RELEASED`/`EXPIRED` already → reply `RoomReleased`) |
| reservation | `EXPIRED` | `RESERVED` | — (sweeper, no reply) |
| booking | `BOOKED` | `PENDING` | log + alert (shouldn't happen) |
| booking | `EXPIRED` | `PENDING` | log + alert |
| payment | `AUTHORIZED` | `CREATED` | ignore (late webhook after cancel) |
| payment | `CAPTURED` | `AUTHORIZED` | `PaymentCaptureFailed` |
| payment | `CANCELLED` | `CREATED`, `AUTHORIZED` | — (already `CANCELLED` → reply `PaymentCancelled`) |

`RELEASED` and `EXPIRED` count as "already released" for `ReleaseRoom`: both mean the room is free again, so the reply is still `RoomReleased`.
