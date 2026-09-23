Main use-cases

Usecase 1: Complete booking order (sync)
=> Detailed design (source of truth): doc/uc1-create-booking/README.md
1. Client clicks book a specific room
    - http request -> orches
2. Orchestrator
    2.1. Orches calls gRPC -> room svc
        - room svc tries to reserve room (with TTL)
        - room svc responses to orches
    2.2. Orches calls gRPC -> booking svc (if reservation success)
        - booking svc creates booking order
        - booking svc responses to orches
    2.3. Orches responses to client
3. Payment (Ex: Credit card)
    3.1. Client click payment
        - http request -> orches
    3.2. Orches calls gRPC -> payment svc
    3.3. Payment svc 
        - Payment svc calls api PSP (sends txn details -> creates payment intent + get session key)
        - PSP responses -> payment svc response -> orches -> client
    3.4. Orches responses to client
4. Client executes payment (PSP site)
    - After finish, PSPS redirects client to GET /booking/{id}
5. PSP webhook -> Payment svc
    5.1. Payment svc
        - Receive webhook
            + Vefify PSP signature
            + Dedup by PSP event.id
            + Persist payment record
            + Return 200 to PSP
        - Publish payment result event (PaymentAuthorized / PaymentFailed) -> orchestrator consumes
    - Update inventory
    - Notify client
    - Generate invoice

Special cases:
Late payment vs expired hold
    + Every reservation has a timeout (expires_at)
    + Orchestrator is the single owner of the timeout: saga deadline = reservation.expires_at
        -> on deadline: cancel PaymentIntent at PSP first, then ReleaseRoom
        -> if cancel fails because payment already authorized -> continue confirm path
    + Room svc sweeper = safety net only (longer TTL than the saga deadline)
    + Stripe capture_method=manual (authorize -> confirm room -> capture)
        -> confirm fails => cancel authorization (no refund, no fee)
        -> capture right after confirm (card auth holds expire after ~7 days)

Usecase 2: Cancel booking order
- TODO

Usecase 3: Payment refund
- TODO
