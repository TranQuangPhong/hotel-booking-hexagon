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
- Impl: Design models -> folder structure -> Impl APIs
    + User service (done)
    + Model + APIs (done)
    + SQL + Logging (done)
- Local deployment & test (skip AWS API gateway, Cognito, Lambda)
- AWS deployment (Add Gateway, Cognito, Lambda)
- CICD
- Monitoring, logging
- Apply CDC
