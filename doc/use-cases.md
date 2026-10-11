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
        - Payment svc calls api PSP (sends txn details -> creates checkout session + get hosted payment page URL)
        - Payment svc persists payment record (CREATED)
        - PSP responses -> payment svc response -> orches -> client
    3.4. Orches responses to client (payment page URL)
4. Client executes payment (PSP hosted page, client is redirected to it)
    - After finish, PSP redirects client to booking page -> polls GET /bookings/api/v1/{id}
5. PSP webhook -> Payment svc (only when the card is authorized; a declined card stays on the PSP page, client retries there)
    5.1. Payment svc
        - Receive webhook
            + Vefify PSP signature
            + Find payment record by saga_id (sent to PSP as metadata at 3.3)
            + Dedup: record already AUTHORIZED -> do nothing (dedup by PSP event.id = later, G5)
            + Update payment record -> AUTHORIZED, save PSP payment intent id
            + Return 200 to PSP
        - Publish PaymentAuthorized event -> orchestrator consumes
    - Update inventory
    - Notify client
    - Generate invoice

Special cases:
Late payment vs expired hold
    + Every reservation has a timeout (expires_at)
    + Orchestrator is the single owner of the timeout: saga deadline = reservation.expires_at
        -> on deadline (v1): ReleaseRoom -> ExpireBooking. A payment authorized after that is ignored, never captured
        -> later (G1, G14): expire the checkout session at PSP first; if payment already authorized -> continue confirm path
    + Room svc sweeper = safety net only (longer TTL than the saga deadline) -> later (G6)
    + Stripe capture_method=manual (authorize -> confirm room -> capture)
        -> confirm fails => cancel authorization (no refund, no fee) -> later (G6); in v1 confirm can't fail
        -> capture right after confirm (card auth holds expire after ~7 days)

Usecase 2: Cancel booking order
- TODO

Usecase 3: Payment refund
- TODO
