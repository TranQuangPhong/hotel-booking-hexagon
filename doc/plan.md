1. User service
- Make it simple (id, name, role, email)

2. Room
- Room & inventory
- Model
- APIs: rooms list, room details, create/update/delete room - admin
- Saga (phase 2: after done all APIs for all services)
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
- Local deployment & test
- AWS deployment
- CICD
- Monitoring, logging
- Apply CDC
