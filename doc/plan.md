1. User service
- Make it simple (id, name, role, email)
- APIs: Get/update/delete profile info
    + Note for delete api (or updating ROLE is similar):
        User service calls Cognito to delete user via Go AWS SDK
        User service deletes from DB
        User service sends msg via Kafka to Orchestrator (skip for now)
        Orchestrator commands other services to delete user-related data -> GDPR Compliance (skip for now)

2. Room
- Room & inventory
- Model
- APIs: rooms list, room details, create/update/delete room - admin
- Saga: Producers, consumers (phase 2: after done all APIs for all services)
- Redis reserve room (Phase 3: optimization)

3. Booking
- Model
- APIs: bookings list, booking details, cancel booking
- Saga (phase 2)

4. Payment
- Model
- APIs: request payment, get invoice & refund (skip for now)
- Saga (phase 2)

5. Notification
- Model
- APIs: Get notifications list
- Saga (phase 2)

6. FE for all services (gen AI)

Next step:
- Impl user-service: Design models -> folder structure -> Impl APIs (done)
    + Model + APIs (done)
    + SQL + Logging (done)
- Impl room-service: Design models -> folder structure -> Impl APIs (done)
    + Model + APIs (done)
    + SQL (done)
    + Temporarily Skip inventory SQL (impl in specific create-booking usecase)
    + Test APIs (done)

- Impl Usecase 1: Create booking order -> full flow: orchestrator -> booking -> room -> payment -> notify [NEXT]
    + Impl event (msg structure) module
    + Impl orchestrator
    + Detailed design: doc/uc1-create-booking/ (README -> flows -> state-machines -> contracts -> data-model -> edge-cases)
      -> supersedes "Suggested revised flow" below (change: declined card no longer ends the saga, only authorization or deadline does)

    UC1 design decisions (from architecture review)
    1. PSP webhook -> Payment svc (NOT orchestrator)
        + Payment svc: verify Stripe signature, dedupe by PSP event.id, persist payment record, return 2xx fast
        + Payment svc publishes domain events (PaymentAuthorized / PaymentFailed) -> orchestrator consumes
        + Orchestrator never sees Stripe payloads (no PSP coupling outside Payment svc)
    2. Late payment vs expired hold
        + Every reservation has a timeout (expires_at)
        + Orchestrator is the single owner of the timeout: saga deadline = reservation.expires_at
          -> on deadline: cancel PaymentIntent at PSP first, then ReleaseRoom
          -> if cancel fails because payment already authorized -> continue confirm path
        + Room svc sweeper = safety net only (longer TTL than the saga deadline)
        + Stripe capture_method=manual (authorize -> confirm room -> capture)
          -> confirm fails => cancel authorization (no refund, no fee) => UC1 does not depend on UC3 refund
          -> capture right after confirm (card auth holds expire after ~7 days)
    3. Post-payment steps (payment = pivot of the saga)
        + Sequential, NOT blind fan-out: ConfirmReservation (command + reply) -> CapturePayment -> update booking status
        + Notification svc subscribes to BookingConfirmed event (not a command from orchestrator)
        + Orchestrator marks saga COMPLETED only after replies arrive
    4. Transactional outbox for the WHOLE project (every service that writes DB + publishes Kafka)
        + Outgoing msgs inserted into an outbox table in the SAME DB tx as the state change
        + Polling publisher reads unpublished rows -> Kafka -> set published_at
        + CDC (Debezium) NOT applied, polling publisher only
        + Orchestrator: saga_message_log OUT rows can act as the outbox (add published_at)
        + Kafka is at-least-once -> every consumer is idempotent:
          processed_messages(message_id) table (or unique key), checked in the same tx as the side effect
        + Kafka partition key = saga_id (or booking_id) -> ordering per booking
    5. Idempotency on sync calls
        + saga_id is the idempotency key on every gRPC call (ReserveRoom, CreateBooking, CreatePaymentIntent)
        + Stored with UNIQUE constraint: reservations.saga_id, bookings.saga_id, payments.saga_id -> retry returns existing row
        + Client POST /bookings sends Idempotency-Key header -> orchestrator maps it to 1 saga (double-click safe)
    6. Order: room first, then booking
        + ReserveRoom returns reservation_id + nightly rates + room snapshot + expires_at -> input for CreateBooking
        + Remove RESERVATION_FAILED from booking_status (Go const + Postgres ENUM)
        + Inventory references reservation_id (not booking_id)
        + User snapshot (name, email, phone) comes from JWT claims
        + TODO decide: PENDING vs RESERVED overlap in booking status, status for "hold expired" (new EXPIRED?)
          and for "authorized but room confirm failed"
            + Do PENDING and RESERVED now mean the same thing?
            + What status does a booking get when its hold expires?
            + What status does it get when payment is authorized but the room confirm fails?
    7. Room svc: reservations = single source of truth for occupancy
        + Inventory JSONB keeps only price + closed/maintenance per day (no RESERVED/BOOKED duplicated there)
        + DB-level guard against double booking (btree_gist):
          EXCLUDE USING gist (room_id WITH =, daterange(check_in, check_out) WITH &&) WHERE status IN ('RESERVED','CONFIRMED')
        + Reserve tx first expires stale overlapping RESERVED rows (expires_at < now()) -> then insert
          (constraint can't use now(), so a not-yet-swept expired hold would otherwise block new bookings)
        + Redis lock not needed for correctness -> optional perf item later only
    8. Message envelope (Message-definition.json is a draft -> update to this shape)
        + Add: messageId (dedupe), sagaId, correlationId, causationId, schemaVersion
        + Remove "saga" block (step/status/compensation) -> saga knowledge stays inside orchestrator
        + Remove "user" blob (email = PII in every message/log)
        + Types: roomId UUID string, money = int64 minor units + currency, checkIn/checkOut = DATE ("2026-03-01")
        + Naming: commands imperative (ReserveRoom, ConfirmReservation, CapturePayment),
          events past tense (RoomReserved, PaymentAuthorized, BookingConfirmed)
    9. Status updates must check current status (current code = happy path only)
        + booking UpdateBookingStatus: conditional update WHERE id = $1 AND status = ANY($expected)
        + Check RowsAffected: 0 rows = not found / already applied / invalid transition -> handle explicitly
        + Same rule for reservation status + payment status updates
        + Gives idempotency + protection against late/duplicate/out-of-order messages
    10. Small additions
        + Price never comes from client: taken from room svc inventory -> stored in booking -> Payment amount read from booking
        + PSP redirect / success page = client UX only, NOT proof of payment (webhook is the source of truth);
          client polls GET /bookings/{id}
        + Saga recovery worker: scan saga_instances IN_PROGRESS / COMPENSATING past deadline -> resume or compensate
        + ENUM vs TEXT + CHECK (see below): use TEXT + CHECK for fast-growing lists like saga_message_log.message_type;
          keep ENUM for stable domain statuses (project convention)

    Why TEXT + CHECK is easier to extend than Postgres ENUM
        + ENUM add value: ALTER TYPE x ADD VALUE 'NEW' -> OK, but the new value can't be used in the same tx
          that added it (and before PG12 it can't run inside a tx block at all) -> awkward in migrations
        + ENUM remove value: NOT supported -> create new type, ALTER every column USING col::text::new_type, drop old type
          (rewrites the table, heavy lock on big tables)
        + ENUM reorder: not supported (sort order = declaration order)
        + TEXT + CHECK: add/remove value = DROP CONSTRAINT + ADD CONSTRAINT in one normal transactional migration
          -> ADD CONSTRAINT ... NOT VALID then VALIDATE CONSTRAINT to avoid a long lock on big tables
        + Go side is the same either way: pgx scans both to string -> type X string + IsValid() still the app-level guard
        + Trade-off: ENUM is compact (4 bytes) + self-documenting type; TEXT + CHECK is flexible -> message types grow
          with every new use case, so they benefit most from TEXT + CHECK

    Suggested revised flow (UC1)
    ```
    POST /bookings  (Idempotency-Key)
      Orch: create saga IN_PROGRESS
      Orch -> Room    gRPC ReserveRoom(saga_id,...)    -> reservation_id, nightly rates, expires_at
      Orch -> Booking gRPC CreateBooking(saga_id,...)  -> booking_id PENDING
          +-- fail -> ReleaseRoom (Kafka cmd, via outbox)
      Orch: step=AWAITING_PAYMENT, deadline=expires_at   <- 201 {booking_id, expires_at}

    POST /bookings/{id}/payment
      Orch -> Payment gRPC CreateIntent(saga_id, amount from booking, capture=manual) <- client_secret

    PSP webhook -> Payment svc (verify sig, dedupe, persist) -> PaymentAuthorized | PaymentFailed
      Authorized -> ConfirmReservation -ok-> CapturePayment -> booking BOOKED -> BookingConfirmed (notify subscribes) -> COMPLETED
                                       +-fail-> CancelAuthorization -> booking failed status -> notify
      Failed     -> ReleaseRoom, booking PAYMENT_FAILED
      Deadline   -> cancel intent at PSP -> ReleaseRoom -> booking expired status
    ```

    Docs disagreeing with the design (to fix)
        + use-cases.md step 5 lists "generate invoice", diagram has update_payment_log and no invoice
        + use-cases.md calls UC1 "(sync)", but everything from step 5 (webhook) is async
        + services.md lists POST /bookings/{id}/payment under Payment svc as "downstream of orchestrator"
          -> it's a gRPC method called by orchestrator, not a REST route
        + architecture_uc_create_order.excalidraw: webhook arrow PSP -> Orchestrator must become PSP -> Payment svc;
          step 6 fan-out -> sequential confirm/capture + BookingConfirmed event (decisions 1, 3)
        + Message-definition.json: update to the envelope in decision 8

    What to do first (order)
        1. Fix design docs: webhook owner (1), timeout + manual capture (2), room-first + booking statuses (6) -> update diagram
        2. Define message envelope + command/event names (8)
        3. Room svc: reservations constraint + expire-stale-overlap + saga_id unique (5, 7)
        4. Build outbox + idempotent consumer together with the FIRST Kafka consumer (4, 5) -> reuse pattern everywhere
        5. Conditional status updates in booking/room/payment repos (9)
        6. Orchestrator: saga state machine + deadline + recovery worker (2, 10)

- Local deployment & test (skip AWS API gateway, Cognito, Lambda)
- AWS deployment (Add Gateway, Cognito, Lambda)
- CICD
- Monitoring, logging
