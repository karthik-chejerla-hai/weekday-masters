# Notification Data Model

Additive migrations only. All primary keys use existing UUID conventions. Ledger tables are unchanged.

## Existing Notification extended as recipient event

Fields: existing ID, user, title, body, category, read/created dates; add event_key (unique when present), source_kind/source_id, destination_path, due_at, expires_at, suppressed_reason and immutable facts in data. Facts include session_id, settlement_id, charge_cents, balance_after_cents and balance_band when relevant. Categories: rsvp_reminder, session_charge, balance_alert, admin_announcement. Keep old category strings readable for historical records.

Uniqueness: RSVP session/member/stage; charge settlement/member; reversal settlement/member/reversal-transaction; announcement announcement/member; nudge actor/request/member. Category is not a delivery channel. Balance warning within a charge is facts/wording, not a second event.

History/read state never proves delivery. Legacy PushSent means at least one device was accepted; expose new precise status instead of renaming it delivered.

## Push device enrollment

Stable device_id and user_id, lifecycle active/revoked, registration timestamps/device name; optional legacy-adoption metadata. Link active enrollment to its FCM registration, keeping raw addresses private. Revocation deletes the active token and retains lifecycle tombstone; automatic refresh requires active enrollment, explicit enroll can restore it. Unique installation ownership must prevent shared-browser cross-account delivery. A legacy token may be adopted once only by its owning approved member while active.

An active enrollment with a registered delivery address counts as Push set up. Missing permission cannot be observed remotely; no fabricated permission field. No independent master enable flag or per-category settings in the new policy.

## NotificationOutbox

ID, notification_id (unique), due_at, expires_at, state, lease_owner, lease_until, attempts, next_attempt_at, last_error_code, created/updated dates. States: pending -> claimed -> completed; pending/claimed -> suppressed/expired; claims can recover after lease expiry if no send began. New states and indexes must support due polling and safe concurrency.

Create inside the same transaction as event/origin action. No raw provider call inside that transaction. Pending notification intent survives process restart.

## NotificationDeliveryAttempt

ID, notification_id, enrollment_id, channel=push, state, attempt_count, next_attempt_at, lease/send start, provider_accepted_at, safe error code. Unique notification/enrollment/channel; an append-only attempt log or sequence can record individual retries. States: pending -> sending -> accepted/transient_failure/permanent_failure/uncertain/suppressed/expired. A timed-out sending lease becomes uncertain; accepted/uncertain attempts are not automatically resent.

Partial success retries only failed devices. Provider-unregistered failures remove the address and deactivate the enrollment; token/config/payload failures must be classified accurately rather than deleting every failing token.

## Reminder stage/cutover bookkeeping

Session/member/stage identity and terminal skipped/sent status may live in notification/event bookkeeping. A rollout cutoff records stages already due as skipped, independently of whether an event was shown. Stage identity is stable across rescheduling; unsent due/expiry times follow the current session. Do not send both stages together after an outage.

## Announcement and manual request identity

Existing Announcement remains the publication/content object. Add idempotency request ID with actor scope. Audience recipient events and outbox rows are committed with the publication, including cancellations.

Manual nudge request identity stores/reuses its original event result; cooldown is per member over manual-nudge events only. Approved membership and account balance must be checked inside its serialized action.

## Migration rules

- Preserve all existing token rows/ownership; add active lifecycle metadata without deleting or rotating tokens, regardless of old preferences. Assign installation identity automatically on a browser visit; unvisited registrations remain deliverable. Retire preference policy rather than migrating opt-outs. Subsequent explicit device revocations still persist.
- Do not delete notification history or ledger records. Do not backfill charge events for old settlements/imports.
- Migrate open future sessions only if their existing RSVP window is still open including custom deadlines; write resolved starts_at - 48h. Preserve closed windows on later edits.
- Fixed threshold: 2000 cents in shared notification policy; retained Club threshold reporting and nudge UI must match this policy after cutover.
- Add new tables to `backend/internal/testsupport/db.go` truncation list, respecting foreign keys.
