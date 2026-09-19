Main use-cases

Usecase 1: Complete booking order (sync)
1. Client clicks book a specific room
    - http request -> orches
2. Orchestrator
    2.1. Orches calls gRPC -> room svc
        - room svc tries to reserve room
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
        - PSP responses & payment svc response -> orches
    3.4. Orches responses to client
4. Client executes payment (PSP site)
5. PSP webhook
    - Update inventory
    - Notify client
    - Generate invoice


Usecase 2: Cancel booking order
- TODO

Usecase 3: Payment request (client requests payment after done reserving room)
- TODO
