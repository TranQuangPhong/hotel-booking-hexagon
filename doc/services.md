Hotel booking system
Details: doc/uc1-create-booking/

0. Communication
- Client -> Orchestrator: REST
- Orchestrator -> Room / Booking / Payment: gRPC (before payment, user is waiting)
- Orchestrator <-> Room / Booking / Payment: Kafka cmd -> reply event (after payment)
- Every Kafka msg goes through an outbox table (same DB tx as the state change)

1. AWS API gateway (later)
- Route client requests to services

2. User service
- Store user profile only
- AWS Cognito handles sign up, login & ROLE (later)
- AWS Lambda to sync Cognito -> User service DB (later)
- REST
    + GET /users
    + GET /users/{id}
    + POST /users
    + PUT /users/{id}
- Techstack: Golang + PostgreSQL

3. Room service
- Room info (type, status)
- Inventory: price per day, open/closed day
- Reservations: hold a room for a date range (no double booking)
- REST
    + GET /rooms
    + GET /rooms/{id}
    + POST /rooms
    + PUT /rooms/{id}
- gRPC
    + ReserveRoom
- Kafka
    + cmd:   ConfirmReservation, ReleaseRoom
    + event: ReservationConfirmed, RoomReleased
- Techstack: Golang + PostgreSQL (+ Redis lock later)

4. Booking service
- Booking order: user & room snapshot, dates, price per night
- REST
    + GET /bookings
    + GET /bookings/{id}       - client polls booking status
- gRPC
    + CreateBooking
- Kafka
    + cmd:   ConfirmBooking, ExpireBooking
    + event: BookingConfirmed, BookingExpired
- Techstack: Golang + PostgreSQL

5. Payment service
- Payment record, integrate PSP (Stripe)
- Create payment intent, receive webhook, capture payment
- REST
    + POST /payments/webhooks/stripe    - Stripe calls it
- gRPC
    + CreatePaymentIntent
- Kafka
    + cmd:   CapturePayment
    + event: PaymentAuthorized, PaymentCaptured, PaymentCaptureFailed
- Techstack: Golang + PostgreSQL + Stripe (sandbox)

6. Notification service
- Send notification email
- Kafka
    + event (subscribe): BookingConfirmed
- Techstack: Golang + MongoDB (log every notification) + AWS SES (later)

7. Orchestrator
- Saga coordinator: calls services, sends commands, reacts to events
- Deadline worker: release the room if the user doesn't pay in time
- REST
    + POST /orchestrator/bookings                  - create booking (hold room)
    + POST /orchestrator/bookings/{id}/payment     - start payment
- Kafka
    + cmd (send):      ConfirmReservation, ReleaseRoom, CapturePayment, ConfirmBooking, ExpireBooking
    + event (consume): PaymentAuthorized, ReservationConfirmed, RoomReleased, PaymentCaptured, PaymentCaptureFailed, BookingConfirmed, BookingExpired
- Techstack: Golang + PostgreSQL

TODO: Cancel booking (UC2), Payment refund (UC3)
