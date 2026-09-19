Hotel booking system

1. AWS API gateway

2. User service
- Store user profile only
- AWS Cognito handles sign up, login & ROLE
- AWS Lambda to sync Cognito -> User service DB

3. Room service
- Room info (type, price, status)
    + GET /rooms
    + GET /rooms/{id}
    + POST /rooms/
    + PUT /rooms/{id}
    + DELETE /rooms/{id}
- Inventory (year, month, day)
    + Reserve room          - gRPC              - TODO
    + Release room          - kafka consumer    - TODO

4. Booking service
- GET /bookings
- GET /bookings/{id}
- POST /bookings                  - gRPC                - downstream of orchestrator
- POST /bookings/{id}/modify      - kafka consumer      - downstream of orchestrator
- POST /bookings/{id}/cancel      - kafka consumer      - downstream of orchestrator

5. Payment service
- Receive payment request from user
    + POST /bookings/{id}/payment   - start tnx     - downstream of orchestrator
    + POST /bookings/{id}/refund    - refund        - downstream of orchestrator
    + POST /bookings/{id}/invoice   - invoice       - downstream of orchestrator
- Integrate 3rd party payment provider

6. Notification service
- Send notification email
- AWS SES (simple email service)

7. Orchestrator
- Saga coordinator
    + POST /bookings                (forward to booking svc)
    + POST /bookings/{id}/payment   (forward to payment svc)
    + POST /bookings/{id}/modify    (forward to booking svc)
    + POST /bookings/{id}/cancel    (forward to booking svc)
    + POST /bookings/{id}/refund    (forward to payment svc)
- Kafka as system backbone

Techstack:
1. AWS API gateway
2. User Service: Golang + PostgreSQL (Lưu ID, Email, Role đồng bộ từ Cognito via Lambda).
3. Room Service: Golang + PostgreSQL + Redis (Distributed Lock).
4. Booking Service: Golang + PostgreSQL.
5. Payment Service: Golang + PostgreSQL + Stripe (sandbox).
6. Notification Service: Golang + MongoDB (Lưu log mọi email/thông báo) + AWS SES.
7. Orchestrator: Golang + PostgreSQL.