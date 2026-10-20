# UC1: good to have (not in v1)

v1 is correct without these: no double booking, no charge without a confirmed room, no room held forever.
Each item says **what v1 does instead**, so you know the exact limitation you accept.
Add them once M1–M6 work end to end, roughly in the order listed (most useful first).

## Cut features (G#)

| ID | Good to have | What v1 does instead (the accepted limitation) |
|---|---|---|
| **G1** | **Close the payment on expiry**: a `CancelPayment` step before `ReleaseRoom` (expire the Checkout Session, void the authorization if there already is one), payment `CANCELLED`, and a "cancelled before session existed" marker for the Pay-at-deadline race | Stripe's page stays usable after the hold expires (session lifetime: 24 h by default, 30 min minimum). A late authorization is ignored and never captured. The hold stays on the user's card until the bank drops it (up to ~7 days). The user isn't charged |
| **G2** | `Idempotency-Key` header on `POST /bookings` (+ `422` if the same key comes with a different body) | A double-click sends a 2nd request, which gets `409 ROOM_UNAVAILABLE` from the exclusion constraint. The 1st succeeds |
| **G3** | gRPC retries on `UNAVAILABLE` / `DEADLINE_EXCEEDED` (same `saga_id`). Retries make a repeated `ReserveRoom` real, so it should then compare the stay: same `saga_id` with another room or dates → `INVALID_ARGUMENT` | Phase A: any gRPC error → phase D release, and the user books again. Phase B (payment): the client gets `503` and calls again; the hold is kept. A repeated `ReserveRoom` (in v1 only a hand-written call, e.g. grpcurl) returns the stored hold even if the room or dates differ, labelled with the requested room's number and type |
| **G4** | Recovery worker for sagas stuck after payment (re-send the current command with a new `message_id`) | Outbox + Kafka redelivery already keep async steps moving. A saga stuck from a bug can be found with `status = 'IN_PROGRESS' AND updated_at < now() - interval '5 min'` and fixed by hand |
| **G5** | Inbox tables: `processed_messages`, `psp_webhook_events` (dedupe + webhook audit) | Handlers are idempotent through the status check (README rule 3) |
| **G6** | Room-side safety net: a hold sweeper, stale-hold cleanup in `ReserveRoom`, `HOLD_GRACE`, reservation `EXPIRED`, and the `ReservationConfirmFailed` branch in the saga. Once room svc relies on `expires_at`, `ReserveRoom` also rejects a past one for a new hold | Only the orchestrator's deadline worker releases holds. If the orchestrator is down, holds stay until it's back, then get released. `expires_at` is stored but room svc never reads or checks it (a past value, even 1970, is accepted) |
| **G7** | Envelope extras: `kind`, `schema_version`, `correlation_id` (one ID across logs of all services), `causation_id` | `saga_id` is the correlation ID in logs |
| **G8** | `saga_message_log` IN rows (audit of every received message) | Only OUT rows (outbox); the handler logs what it received |
| **G9** | Notification: MongoDB log, SES (Phase 3), "hold expired" email on `BookingExpired`, retries | Log line on `BookingConfirmed` |
| **G10** | Automatic handling of capture failure: release the `CONFIRMED` room, expire the booking, notify | Saga `FAILED` + error log; fix by hand (rare: we capture seconds after authorization) |
| **G11** | Dead-letter topic `<topic>.dlq` for poison messages | A handler that keeps failing doesn't ack, so the partition stops. Visible in logs |
| **G12** | Cleanup of published outbox rows | Tables grow; fine locally |
| **G13** | Reconciliation job that polls Stripe for `CREATED` payments older than N min (missed webhook) | The deadline releases the room; the user isn't charged |
| **G14** | "Grace confirm": at the deadline, first ask Stripe whether the session's PaymentIntent is already `requires_capture` | Deadline wins; a payment authorized seconds before it is ignored (see G1) |
| **G15** | `booking.payment_status = CANCELLED` on expiry | Stays `PENDING`; `status = EXPIRED` already tells the truth |
| **G16** | Extend the inventory horizon: a monthly job (or the admin inventory API) opens the next month for every room | A room gets 12 months of inventory from its creation month and nothing adds more, so the bookable window shrinks every month; after a year every stay fails with `ROOM_UNAVAILABLE`. Fine locally: DBs are rebuilt |
| **G17** | Max stay length (e.g. 30 nights), rejected with `400` at `POST /bookings` | Any length passes. A stay past the 12-month inventory fails with `ROOM_UNAVAILABLE`, but only after `setRates` sized its slice for the whole stay (about 3 MB for check-out `9999-12-31`) |
| **G18** | The hotel's time zone for "today" in the check-in-in-the-past rule | Room svc uses today's UTC date. At UTC+7, yesterday's date still passes until 07:00 local; at UTC-5, same-day check-in is rejected from 19:00 local |
| **G19** | gRPC servers return `CANCELED` / `DEADLINE_EXCEEDED` for a request the caller cancelled or timed out, and log it below error level | It comes back as `INTERNAL` and is logged as an error. The caller already gave up, so only the logs are noisy (matters once alerts watch error logs, roadmap phase 4) |

## Edge cases still open (E#)

| ID | Case | Direction |
|---|---|---|
| **E1** | `CreateBooking` times out, the orchestrator releases (`ExpireBooking` finds no row), then the slow `CreateBooking` commits → an orphan `PENDING` booking | booking svc records "expired" for that `saga_id` so a later create is rejected (a tombstone). Very rare locally |
| **E2** | Several sagas for one booking (UC2 cancel, UC3 refund) | Kafka key `sagaId` no longer orders per booking. Revisit the key (`bookingId`) and allow only one active saga per booking. Decide with UC2 |
| **E3** | Price changes between search and booking | The price is locked at `ReserveRoom`. The client sees the final total in the `201`. A quote mechanism comes later |
| **E4** | Inventory days with different currencies in one stay | Not checked in v1: `setRates` takes the first month's currency and sums every price. It can't happen yet: all of a room's inventory is created with one rate and nothing edits it. With the admin inventory API, enforce one currency per room there (or reject in `ReserveRoom`) |
| **E5** | Hold expires while the user is on Stripe's page | v1: the payment may still be authorized but is ignored, and the client poll shows `EXPIRED` ("hold expired, book again"). With G1 the session is expired at the deadline, so Stripe's page refuses the payment |
| **E6** | `ReserveRoom` times out, the orchestrator releases (`ReleaseRoom` finds no row, so `RoomReleased` has a null `reservation_id`), then the slow `ReserveRoom` commits → a hold nobody releases (the saga ends `FAILED`) | Rare in v1: every query uses the request ctx, so a slow insert is cancelled around the caller's deadline, and `ReleaseRoom` only arrives later (outbox + Kafka). Fix with G6 (a sweeper releases holds past `expires_at`) or a tombstone like E1 |
