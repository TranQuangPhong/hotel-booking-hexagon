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

- Impl booking-service: Models -> folder structure -> APIs [NEXT]
- Impl orchestrator-service: folder structure
- Update design, change topics organization (booking.cmd, room.event, payment.event... & use "type", "reason" to handle logic)
- Impl event (msg structure) module
- Impl Usecase 1: Create booking order -> full flow: orchestrator -> booking -> room -> payment -> notify [NEXT]

- Local deployment & test (skip AWS API gateway, Cognito, Lambda)
- AWS deployment (Add Gateway, Cognito, Lambda)
- CICD
- Monitoring, logging
- Apply CDC
