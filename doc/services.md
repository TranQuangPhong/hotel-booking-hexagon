Hotel booking system

1. AWS API gateway

2. User service
- Store user profile only
- AWS Cognito handles sign up, login & ROLE
- AWS Lambda to sync Cognito -> User service DB

3. Room service
- Room info (type, price, status)
    + View list of rooms
    + View room details
    + Create/update/delete room
- Inventory (month, day(slots))
    + Reserve room

4. Booking service
- Create booking order
- Track order status
- Cancel order

5. Payment service
- Receive payment request from user
- Integrate 3rd party payment provider

6. Notification service
- Send notification email
- AWS SES (simple email service)

7. Orchestrator
- Saga coordinator
- Kafka as system backbone

Techstack:
1. AWS API gateway
2. User Service: Golang + PostgreSQL (Lưu ID, Email, Role đồng bộ từ Cognito via Lambda).
3. Room Service: Golang + PostgreSQL + Redis (Distributed Lock).
4. Booking Service: Golang + PostgreSQL.
5. Payment Service: Golang + PostgreSQL + Stripe (sandbox).
6. Notification Service: Golang + MongoDB (Lưu log mọi email/thông báo) + AWS SES.
7. Orchestrator: Golang + PostgreSQL.