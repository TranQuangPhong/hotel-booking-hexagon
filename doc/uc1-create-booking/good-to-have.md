# UC1: good to have (not in v1)

v1 is correct without these: no double booking, no charge without a confirmed room, no room held forever.
Each item says **what v1 does instead**, so you know the exact limitation you accept.
Add them once M1–M6 work end to end, roughly in the order listed (most useful first).

## Cut features (G#)

| ID | Good to have | What v1 does instead (the accepted limitation) |
|---|---|---|
| **G1** | **Void the authorization on expiry**: a `CancelPayment` step before `ReleaseRoom`, payment `CANCELLED`, and a "cancelled before intent existed" marker for the Pay-at-deadline race | A late authorization is ignored and never captured. The hold stays on the user's card until the bank drops it (up to ~7 days). The user isn't charged |
| **G2** | `Idempotency-Key` header on `POST /bookings` (+ `422` if the same key comes with a different body) | A double-click sends a 2nd request, which gets `409 ROOM_UNAVAILABLE` from the exclusion constraint. The 1st succeeds |
| **G3** | gRPC retries on `UNAVAILABLE` / `DEADLINE_EXCEEDED` (same `saga_id`) | Any gRPC error → phase D release; the user tries again |
| **G4** | Recovery worker for sagas stuck after payment (re-send the current command with a new `messageId`) | Outbox + Kafka redelivery already keep async steps moving. A saga stuck from a bug can be found with `status = 'IN_PROGRESS' AND updated_at < now() - interval '5 min'` and fixed by hand |
| **G5** | Inbox tables: `processed_messages`, `psp_webhook_events` (dedupe + webhook audit) | Handlers are idempotent through the status check (README rule 3) |
| **G6** | Room-side safety net: a hold sweeper, stale-hold cleanup in `ReserveRoom`, `HOLD_GRACE`, reservation `EXPIRED`, and the `ReservationConfirmFailed` branch in the saga | Only the orchestrator's deadline worker releases holds. If the orchestrator is down, holds stay until it's back, then get released |
| **G7** | Envelope extras: `kind`, `schemaVersion`, `correlationId` (one ID across logs of all services), `causationId` | `sagaId` is the correlation ID in logs |
| **G8** | `saga_message_log` IN rows (audit of every received message) | Only OUT rows (outbox); the handler logs what it received |
| **G9** | Notification: MongoDB log, SES (Phase 3), "hold expired" email on `BookingExpired`, retries | Log line on `BookingConfirmed` |
| **G10** | Automatic handling of capture failure: release the `CONFIRMED` room, expire the booking, notify | Saga `FAILED` + error log; fix by hand (rare: we capture seconds after authorization) |
| **G11** | Dead-letter topic `<topic>.dlq` for poison messages | A handler that keeps failing doesn't ack, so the partition stops. Visible in logs |
| **G12** | Cleanup of published outbox rows | Tables grow; fine locally |
| **G13** | Reconciliation job that polls Stripe for `CREATED` payments older than N min (missed webhook) | The deadline releases the room; the user isn't charged |
| **G14** | "Grace confirm": at the deadline, first ask Stripe whether the intent is already `requires_capture` | Deadline wins; a payment authorized seconds before it is ignored (see G1) |
| **G15** | `booking.payment_status = CANCELLED` on expiry | Stays `PENDING`; `status = EXPIRED` already tells the truth |

## Edge cases still open (E#)

| ID | Case | Direction |
|---|---|---|
| **E1** | `CreateBooking` times out, the orchestrator releases (`ExpireBooking` finds no row), then the slow `CreateBooking` commits → an orphan `PENDING` booking | booking svc records "expired" for that `saga_id` so a later create is rejected (a tombstone). Very rare locally |
| **E2** | Several sagas for one booking (UC2 cancel, UC3 refund) | Kafka key `sagaId` no longer orders per booking. Revisit the key (`bookingId`) and allow only one active saga per booking. Decide with UC2 |
| **E3** | Price changes between search and booking | The price is locked at `ReserveRoom`. The client sees the final total in the `201`. A quote mechanism comes later |
| **E4** | Inventory days with different currencies in one stay | `ReserveRoom` rejects; enforce one currency per room in the admin API |
| **E5** | Hold expires while the user is on the Stripe form | v1: the payment may still be authorized but is ignored, and the client poll shows `EXPIRED` ("hold expired, book again"). With G1, Stripe rejects it directly |
