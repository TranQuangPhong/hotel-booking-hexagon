# UC1 contracts

Field lists only, not full proto or JSON schemas. Enough to write handlers and messages consistently.
Conventions: IDs are UUID strings, money is `int64` minor units + `currency` (ISO 4217, 3 letters), dates are `YYYY-MM-DD` (`checkOut` is exclusive), timestamps are RFC 3339 UTC.

## 1. Client REST

Paths are shown logically. The real prefix follows the project rule `/<resource>/api/v1/...`. Note: the orchestrator router currently uses `/booking/` without `/api/v1`, so align it when implementing.

| # | Service | Endpoint | Request | Success | Errors |
|---|---|---|---|---|---|
| 1 | Orchestrator | `POST /bookings` | header `Idempotency-Key` (UUID from client), JWT. Body: `roomId, checkIn, checkOut, numberOfGuests` | `201` `bookingId, status: "PENDING", totalAmount, currency, expiresAt` | `400` validation · `404 ROOM_NOT_FOUND` · `409 ROOM_UNAVAILABLE` · `409 REQUEST_IN_PROGRESS` (same key, still running) · `503 BOOKING_FAILED` |
| 2 | Orchestrator | `POST /bookings/{id}/payment` | JWT | `200` `clientSecret, amount, currency, expiresAt` | `404` not found / not owner · `409 BOOKING_NOT_PAYABLE` (expired, already paid) · `503` |
| 3 | Booking | `GET /bookings/{id}` | JWT | `200` booking incl. `status`, `paymentStatus`, `expiresAt` | `404` |
| 4 | Payment | `POST /payments/webhooks/stripe` | raw body + `Stripe-Signature` header (**no JWT**, public) | `200` fast | `400` bad signature |

The client never sends a price. It gets `totalAmount` back from #1 to display it.

## 2. gRPC (sync, orchestrator → participant)

Every request carries `saga_id` (the idempotency key). The same `saga_id` returns the same result.

**`room.v1.RoomService/ReserveRoom`**
- req: `saga_id, room_id, check_in, check_out`
- resp: `reservation_id, expires_at, room_number, room_type, currency, total_amount, nightly_rates[] {date, price}`
- errors: `NOT_FOUND` (room) · `FAILED_PRECONDITION` + reason `ROOM_UNAVAILABLE` (overlap, maintenance day, or missing inventory) · `INVALID_ARGUMENT`

**`booking.v1.BookingService/CreateBooking`**
- req: `saga_id, reservation_id, user {id, name, email, phone}, room {id, number, type}, check_in, check_out, number_of_guests, nightly_rates[], total_amount, currency, expires_at`
- resp: `booking_id, status`
- errors: `INVALID_ARGUMENT`

**`payment.v1.PaymentService/CreatePaymentIntent`**
- req: `saga_id, booking_id, amount, currency`
- resp: `payment_id, client_secret`
- errors: `FAILED_PRECONDITION` + reason `PAYMENT_CANCELLED` (the saga already cancelled payment, E4) · `UNAVAILABLE` (Stripe down)

Retry rule for the orchestrator: `UNAVAILABLE` / `DEADLINE_EXCEEDED` → retry (same `saga_id`). Anything else is final.

## 3. Kafka topics

| Topic | Producer | Consumers (group) | Messages |
|---|---|---|---|
| `room.commands` | orchestrator | room-service | `ConfirmReservation`, `ReleaseRoom` |
| `room.events` | room | orchestrator-service | `ReservationConfirmed`, `ReservationConfirmFailed`, `RoomReleased` |
| `payment.commands` | orchestrator | payment-service | `CapturePayment`, `CancelPayment` |
| `payment.events` | payment | orchestrator-service | `PaymentAuthorized`, `PaymentCaptured`, `PaymentCaptureFailed`, `PaymentCancelled` |
| `booking.commands` | orchestrator | booking-service | `ConfirmBooking`, `ExpireBooking` |
| `booking.events` | booking | orchestrator-service, notification-service | `BookingConfirmed`, `BookingExpired` |

- **Key = `sagaId`** on every message, so ordering is per saga.
- A consumer ignores (acks) any `type` it doesn't handle. This lets topics gain new message types (UC2, UC3) without breaking old consumers.
- Commands go to exactly one owner, while events can have many subscribers. That's why notification reads `booking.events` and receives no commands.

## 4. Message catalog (`data` field of the envelope)

| Type | Kind | Topic | `data` fields |
|---|---|---|---|
| `PaymentAuthorized` | event | payment.events | `paymentId, bookingId, amount, currency` |
| `ConfirmReservation` | command | room.commands | `reservationId` |
| `ReservationConfirmed` | event | room.events | `reservationId` |
| `ReservationConfirmFailed` | event | room.events | `reservationId, reason` (`RELEASED`, `EXPIRED`, `NOT_FOUND`) |
| `CapturePayment` | command | payment.commands | `paymentId, amount, currency` |
| `PaymentCaptured` | event | payment.events | `paymentId, amount, currency` |
| `PaymentCaptureFailed` | event | payment.events | `paymentId, reason` |
| `ConfirmBooking` | command | booking.commands | `bookingId` |
| `BookingConfirmed` | event | booking.events | `bookingId, userId, userName, userEmail, roomNumber, roomType, checkIn, checkOut, numberOfGuests, totalAmount, currency` (**fat event**, notification needs it all) |
| `CancelPayment` | command | payment.commands | `reason` (`DEADLINE`, `ROOM_CONFIRM_FAILED`). Keyed by `sagaId`, not paymentId (E4) |
| `PaymentCancelled` | event | payment.events | `paymentId` (null if none existed) |
| `ReleaseRoom` | command | room.commands | `reservationId` (null if unknown, then room svc looks it up by `sagaId`, E1), `reason` (`DEADLINE`, `BOOKING_CREATE_FAILED`, `ROOM_CONFIRM_FAILED`) |
| `RoomReleased` | event | room.events | `reservationId` |
| `ExpireBooking` | command | booking.commands | `bookingId, reason` |
| `BookingExpired` | event | booking.events | `bookingId, userId, userEmail, reason` |

Naming: commands are **imperative** (`DoThing`) and go to one owner. Events are **past tense** (`ThingDone`) and anyone may listen.

## 5. Envelope

Example: [`../Message-definition.json`](../Message-definition.json).

| Field | Type | Meaning |
|---|---|---|
| `messageId` | UUID | unique per message; the **inbox dedupe key**. A re-sent command gets a new one |
| `kind` | `command` \| `event` | |
| `type` | string | a name from the catalog above |
| `schemaVersion` | int | version of `data` for this `type`; bump on breaking change |
| `producer` | string | e.g. `payment-service` |
| `sagaId` | UUID | saga instance; also the Kafka key |
| `correlationId` | UUID | one ID for the whole user journey. Created at `POST /bookings`, it goes into logs (slog attr) and HTTP/gRPC metadata. In UC1 it's often the same value as sagaId, but it's kept separate because one journey can span several sagas later (book → cancel → refund) |
| `causationId` | UUID \| null | `messageId` of the message that caused this one; rebuilds the chain in `saga_message_log` |
| `occurredAt` | timestamp | when the state change happened (not the publish time) |
| `data` | object | payload from the catalog |

No `user` blob and no `saga` block. Personal data only goes into the `data` of messages that really need it (`BookingConfirmed`, `BookingExpired`).

## Proto location

**Recommendation (open question Q2):** put a root `proto/` folder with `room/v1`, `booking/v1` and `payment/v1`, generated with `buf` into **one** Go module, `booking/contracts`, which works the same way as your logger library. It also holds the envelope struct and the `data` structs for Kafka messages, so producer and consumer share one definition. Each service imports it through `replace booking/contracts => ../contracts` (or a `go.work` at the root).

The alternative is to copy the generated code into each service. It avoids a shared module, but the copies drift and it's easy to forget one. Not worth it with only one developer.
