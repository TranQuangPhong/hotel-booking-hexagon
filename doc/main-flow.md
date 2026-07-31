Usecase 1: Create booking order
1. Client clicks a specific room to book -> rest api request create booking -> orchestrator
2. Orchestrator publishes cmd_create_booking -> booking service
3. Booking service creates booking
    3.1. Success: publishes event_booking_creation_success -> orchestrator
    3.2. Fail:
        - Save FAIL status
        - publishes event_booking_creation_fail -> orchestrator
4. Orchestrator (evaluates booking creation result)
    4.1. On event_booking_creation_success
        - Update saga status (BOOKING_CREATION_SUCCESS)
        - publishes cmd_reserve_room -> Room service
    4.2. On event_booking_creation_fail
        - Update saga status (BOOKING_CREATION_FAIL)
        - Stop flow
5. Room service reserves room
    5.1. Success: publishes event_reservation_success -> orchestrator
    5.2. Fail: publishes event_reservation_fail -> orchestrator
6. Orchestrator (evaluates room reservation result)
    6.1. On event_reservation_success
        - Update saga status (RESERVATION_SUCCESS)
        - Publishes cmd_update_booking_status_on_reservation_result -> booking service (status RESERVATION_SUCCESS)
        - Publishes cmd_make_payment_ready -> Payment service (now client is able to send request to execute payment transaction)
        - Optional: Publishes cmd_notify_payment_required (skip for now to make system simple)
    6.2. On event_reservation_fail
        - Update saga status (RESERVATION_FAIL)
        - Publishes cmd_update_booking_status_on_reservation_result -> booking service (status RESERVATION_FAIL) / Saga rollback booking status
7. Client sends request to execute payment transaction (Ex: Credit card)
    7.1. Success: publishes event_payment_success -> orchestrator
    7.2. Fail: publishes event_payment_fail -> orchestrator
8. Orchestrator (evaluates payment result)
    8.1. On event_payment_success
        - Update saga status (PAYMENT_SUCCESS)
        - Publishes cmd_update_booking_status_on_payment_result -> booking service (status PAYMENT_SUCCESS)
        - Publishes cmd_update_room_status_on_payment_result -> Room service (BOOKED)
        - Publishes cmd_notify_booking_result -> Notification service (Booking success)
    8.2. On event_payment_fail
        - Update saga status (PAYMENT_FAIL)
        - Publishes cmd_update_booking_status_on_payment_result -> booking service (status PAYMENT_FAIL) / Saga rollback booking status
        - Publishes cmd_update_room_status_on_payment_result -> Room service (Release room) / Saga rollback room reservation status
        - Publishes cmd_notify_booking_result -> Notification service (Booking fail)


Usecase 2: Cancel booking order


Usecase 3: Payment request (client requests payment after done reserving room)
