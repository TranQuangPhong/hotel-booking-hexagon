# UC1 edge cases (parked)

The main design already absorbs many failure modes through four mechanisms: idempotency by `sagaId`, outbox, inbox, and guarded status updates. This list covers what is **not** fully designed yet, with the intended direction so the implementation doesn't block it.

**When:** `P1` = handle during Phase 1 implementation · `later` = after the happy path works end to end · `P3` = with AWS work.

| ID | Case | Intended handling | When |
|---|---|---|---|
| **E1** | `ReserveRoom` gRPC times out (unknown outcome) | retry with the same `sagaId` (idempotent). If still failing: saga → `RELEASING_ROOM` (`ReleaseRoom` is safe even if nothing was reserved, because room svc replies `RoomReleased` for an unknown saga) → FAILED, client gets 503 | P1 |
| **E2** | `CreateBooking` times out, but the booking **was** created | the compensation path also sends `ExpireBooking` **by `sagaId`** when `bookingId` is unknown, and booking svc expires whatever it has for that saga. Needs `ExpireBooking` to accept `sagaId` | later |
| **E3** | Orchestrator crashes mid-Phase A (client got an error) | the client retries with the same `Idempotency-Key` and gets the current result. Separately, the recovery worker resumes the saga. Possible outcome: the client sees an error but the hold exists and expires at the deadline. Acceptable | P1 |
| **E4** | User clicks **Pay** at the exact moment the deadline fires | `CancelPayment` is keyed by `sagaId`. If no payment exists yet, payment svc stores a `CANCELLED` row first, so a `CreatePaymentIntent` that arrives later is rejected (`PAYMENT_CANCELLED`). If the intent was created first, cancel voids it | P1 |
| **E5** | Card authorized seconds before the deadline, but the deadline worker won | the orchestrator ignores the late `PaymentAuthorized`, and the cancel voids the auth, so the user isn't charged but sees "expired". Improvement: before cancelling, payment svc asks Stripe for the intent status, and if it's `requires_capture`, reply `PaymentAuthorized` instead (a "grace confirm") | later |
| **E6** | `ReservationConfirmFailed`: the hold was swept while the orchestrator was down longer than `HOLD_GRACE` | designed: → `CANCELLING_PAYMENT` → release (no-op) → expire booking. The user isn't charged; notify with apology | P1 (path exists) |
| **E7** | `PaymentCaptureFailed` (authorization expired or was revoked) after the room is `CONFIRMED` | needs `ReleaseRoom` from `CONFIRMED` + expire booking + notify. Rare, because we capture seconds after the auth. For now the saga stays in `CAPTURING_PAYMENT` → alert + manual fix | later |
| **E8** | Stripe webhook never arrives (misconfigured, outage) | the deadline worker cancels, so the user isn't charged (the auth is voided). Improvement: payment svc reconciliation job polls Stripe for `CREATED` payments older than N min | later |
| **E9** | Poison message (a consumer keeps failing on it) | retry N times, then publish to `<topic>.dlq`, alert, and ack. Before that, a failing handler just doesn't ack, so the partition is blocked, which is visible in logs | later |
| **E10** | Same `Idempotency-Key` reused with a **different** body | compare a hash of the request stored in the saga context → `422 IDEMPOTENCY_KEY_REUSED` | later |
| **E11** | Several sagas for one booking (UC2 cancel, UC3 refund) | the Kafka key `sagaId` no longer orders messages per booking. Revisit the key (`bookingId`) and add a guard such as "no new saga while one is active on this booking" (the index on `saga_instances.booking_id` exists) | with UC2 |
| **E12** | Price changes between search and booking | the price is locked at `ReserveRoom` (stored in booking `nightly_rates`). The client sees the final `totalAmount` in the `201` and on the payment form. No quote mechanism yet | later |
| **E13** | Hold expires while the user is on the Stripe form | the intent is cancelled, so Stripe.js fails the confirm. The client shows "your hold expired, book again" on error or while polling | P1 (client) |
| **E14** | Notification fails (SES down) | retry with backoff inside notification svc. The booking is already confirmed and the saga doesn't wait for email | later |
| **E15** | Auth in Phase 1 (no Gateway/Cognito) | a `UserFromRequest` port: dev adapter reads `X-User-Id/Name/Email/Phone` headers or an unsigned JWT; the Cognito JWT adapter comes in Phase 3 | P1 |
| **E16** | Currency mix within one stay (inventory days with different currencies) | `ReserveRoom` rejects with `FAILED_PRECONDITION`. Enforce one currency per room in admin APIs | later |
| **E17** | Outbox / message-log table growth | a periodic delete of published rows older than N days; the saga log may be kept longer for audit | later |
