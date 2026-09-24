# UC1 contracts (v1)

Conventions: IDs are UUID strings · money is `int64` minor units + `currency` (3 letters) · dates are `YYYY-MM-DD` (`checkOut` exclusive) · timestamps are RFC 3339 UTC.

## 1. Client REST

Every service uses the same pattern: **`/<service-prefix>/health`** and **`/<service-prefix>/api/v1/...`**. The prefix is unique per service, so a gateway can route by prefix without ambiguity.

| Service | Prefix |
|---|---|
| user | `/users` |
| room | `/rooms` |
| booking | `/bookings` |
| orchestrator | `/orchestrator` |
| payment | `/payments` |

| Service | Endpoint (internal path) | Request | Success | Errors |
|---|---|---|---|---|
| Orchestrator | `POST /orchestrator/api/v1/bookings` | user headers; `roomId, checkIn, checkOut, numberOfGuests` | `201` `bookingId, status, totalAmount, currency, expiresAt` | `400` · `404 ROOM_NOT_FOUND` · `409 ROOM_UNAVAILABLE` · `503` |
| Orchestrator | `POST /orchestrator/api/v1/bookings/{id}/payment` | user headers | `200` `clientSecret, amount, currency, expiresAt` | `404` (not found / not owner) · `409 BOOKING_NOT_PAYABLE` · `503` |
| Booking | `GET /bookings/api/v1/{id}` | — | `200` booking | `404` |
| Payment | `POST /payments/api/v1/webhooks/stripe` | raw body + `Stripe-Signature` | `200` | `400` bad signature |

**Public vs internal (later, Phase 3):** the client will see one resource-oriented API, for example `POST /api/v1/bookings` routed to the orchestrator and `GET /api/v1/bookings/{id}` routed to booking svc. The API Gateway routes by method + path and rewrites to the internal paths above. In Phase 1 the client calls the internal paths directly.

**Versioning** lives in the code (`/api/v1`), and the gateway only passes it through. gRPC versions through the proto package (`room.v1`), Kafka messages through `schemaVersion` (G7), and `/health` is not versioned.

## 2. gRPC (orchestrator → participant)

`saga_id` in every request. An existing `saga_id` returns the existing result.

| Method | Request | Response | Errors |
|---|---|---|---|
| `room.v1.RoomService/ReserveRoom` | `saga_id, room_id, check_in, check_out, expires_at` | `reservation_id, room_number, room_type, currency, total_amount, nightly_rates[]{date, price}` | `NOT_FOUND` · `FAILED_PRECONDITION` `ROOM_UNAVAILABLE` |
| `booking.v1.BookingService/CreateBooking` | `saga_id, reservation_id, user{id, name, email, phone}, room{id, number, type}, check_in, check_out, number_of_guests, nightly_rates[], total_amount, currency, expires_at` | `booking_id, status` | `INVALID_ARGUMENT` |
| `payment.v1.PaymentService/CreatePaymentIntent` | `saga_id, booking_id, amount, currency` | `payment_id, client_secret` | `UNAVAILABLE` (Stripe) |

## 3. Kafka topics

| Topic | Producer | Consumer groups | Messages |
|---|---|---|---|
| `room.commands` | orchestrator | room-service | `ConfirmReservation`, `ReleaseRoom` |
| `room.events` | room | orchestrator-service | `ReservationConfirmed`, `RoomReleased` |
| `payment.commands` | orchestrator | payment-service | `CapturePayment` |
| `payment.events` | payment | orchestrator-service | `PaymentAuthorized`, `PaymentCaptured`, `PaymentCaptureFailed` |
| `booking.commands` | orchestrator | booking-service | `ConfirmBooking`, `ExpireBooking` |
| `booking.events` | booking | orchestrator-service, notification-service | `BookingConfirmed`, `BookingExpired` |

Key = `sagaId`. Consumers ack and skip any `type` they don't handle.

## 4. Messages (12)

Participants find their row **by `sagaId`** from the envelope, so most commands need no `data`.

| Type | Kind | `data` |
|---|---|---|
| `PaymentAuthorized` | event | `paymentId, bookingId, amount, currency` |
| `ConfirmReservation` | command | — |
| `ReservationConfirmed` | event | `reservationId` |
| `CapturePayment` | command | — |
| `PaymentCaptured` | event | `paymentId, amount, currency` |
| `PaymentCaptureFailed` | event | `paymentId, reason` |
| `ConfirmBooking` | command | — |
| `BookingConfirmed` | event | `bookingId, userId, userName, userEmail, roomNumber, roomType, checkIn, checkOut, numberOfGuests, totalAmount, currency` (fat event: notification needs no lookups) |
| `ReleaseRoom` | command | `reason` (`DEADLINE`, `PHASE_A_ERROR`) |
| `RoomReleased` | event | `reservationId` (null if there was nothing to release) |
| `ExpireBooking` | command | `reason` |
| `BookingExpired` | event | `bookingId` (null if there was no booking) |

Naming: commands are imperative and go to one owner; events are past tense and anyone may listen.

## 5. Envelope (v1)

Example: [`../Message-definition.json`](../Message-definition.json).

| Field | Meaning |
|---|---|
| `messageId` | UUID, unique per message (logs, outbox row ID) |
| `type` | name from the table above |
| `sagaId` | saga instance; also the Kafka key and the lookup key for participants |
| `producer` | e.g. `payment-service` |
| `occurredAt` | when the state change happened |
| `data` | payload |

`kind`, `schemaVersion`, `correlationId` and `causationId` are G7. Personal data only appears in the `data` of `BookingConfirmed`.

## Proto location

**Decided (Q2):** there are two shared Go modules **inside this repo**, rather than separate git repos or one module per service:

- **`contracts/`** (`booking/contracts`) holds **definitions only**, with no infra dependencies. It has one package per owning service, each containing the `.proto`, its generated code, and the Kafka `data` structs, plus a shared `envelope/`. Commands belong to the receiver's package and events to the emitter's. A service imports only the packages it needs.
- **`platform/`** (`booking/platform`) holds shared **infra** code, starting with `outbox/` (Insert + Poller). Only adapters import it.

Both are wired with `replace` directives. Full layout and import rules are in [hexagonal-structure.md](../hexagonal-structure.md).

Why not separate repos: contracts change almost daily during UC1, and a tag-and-bump cycle for each change is too slow. Revisit if they stabilize.
