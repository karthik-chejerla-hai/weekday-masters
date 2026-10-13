# Implementation Plan: Clear and Reliable Notifications

**Branch**: `codex/notification-delivery` | **Date**: 2026-10-13 | **Spec**: [spec.md](spec.md)

**Input**: Confirmed device-only controls, four categories, 96/60-hour reminders, 48-hour deadline and combined session charge/balance alert.

## Summary

Refactor the existing notification services incrementally. Keep PostgreSQL as the source of notification intent and add a transactional outbox plus per-device delivery attempts. Business services create typed events; one dispatcher applies membership, channel availability, enrollment and stop policy. A Cloud Scheduler invocation every minute wakes a private Cloud Run notification worker to evaluate due reminders and drain work. No new queue broker or notification microservice architecture is needed; the worker shares the backend image/code and database, but has a private service identity.

Do not rely on the existing hourly in-process cron for production reliability. The checked-in deploy does not establish background CPU/minimum-instance guarantees; actual deployed configuration was not inspected. External scheduled invocation remains effective independently of browser traffic. Cloud Scheduler can execute more than once, so PostgreSQL uniqueness and leases are required.

## Technical Context

**Language/Version**: Existing Go module, React 18/TypeScript/Vite; do not upgrade dependencies for this feature.

**Primary Dependencies**: Gin, GORM, PostgreSQL, Firebase Admin/FCM, existing Auth0 client; Google Cloud Scheduler and Cloud Run for scheduled invocation.

**Storage**: Existing PostgreSQL database, additive notification/enrollment/outbox schema via `cmd/migrate`.

**Testing**: Go unit/service/handler tests, scratch-database concurrency/restart tests, Vitest and Playwright routing/setup tests.

**Target Platform**: Existing Firebase-hosted browser/PWA and Cloud Run backend, including supported mobile PWA push.

**Project Type**: Existing web application with a second private worker entry point in the backend image.

**Performance Goals**: Due work processed within five minutes in healthy operation. External worker invocation every minute, bounded batches and runtime; actual device arrival is outside this guarantee.

**Constraints**: Integer cents; ledger remains append-only; no network calls while holding money locks; approved members only; no automatic email/SMS; existing global stops and preview-only diagnostic exceptions retained.

**Scale/Scope**: Single club and current roster; batch queries rather than per-member status requests. No generic channel preference matrix or distributed event bus.

## Constitution Check

Pre-research and post-design checks pass with these explicit provisions:

- I: Handlers parse/authenticate; event generation, policy and dispatch live in services using `database.DB`; no DI container.
- II: Additive schema and explicit backfills run only through `backend/cmd/migrate/main.go`.
- III: Device/history routes require approved membership; member diagnostics/nudges/announcements require approved admin access. Worker routes use cloud service identity authentication on a separate private service, not a public/admin bypass.
- IV: Use stored `starts_at` and resolved deadlines; elapsed-hour arithmetic handles Sydney DST.
- V–VII: Capture exact integer charge/balance facts inside the settlement transaction; `LedgerService` remains the only ledger writer. Notification changes do not alter charges, balances, accounting invariants or reversals.
- VIII: New database tables join `truncateAll`; tests skip without `TEST_DATABASE_URL`; concurrency tests run in CI.
- IX: API calls stay in singleton Axios client; local components/hooks and AuthContext; no global store.
- Existing constitution wording allows independently optional push/email providers. This design supplies automatic push only and retains explicit invitation email; neither becomes a startup dependency. A documentation clarification accompanies the final rollout.

## Project Structure

### Documentation (this feature)

```text
specs/011-notification-delivery/
  spec.md
  plan.md
  research.md
  data-model.md
  contracts/notifications.md
  quickstart.md
  tasks.md
  checklists/requirements.md
```

### Source Code (existing plus proposed additions)

```text
backend/
  cmd/migrate/main.go
  cmd/server/main.go
  cmd/notification-worker/main.go                 (new private HTTP worker)
  Dockerfile
  internal/models/notification.go
  internal/services/notification_service.go
  internal/services/notification_outbox.go         (new)
  internal/services/notification_dispatcher.go     (new)
  internal/services/notification_events.go         (new)
  internal/services/scheduler_service.go
  internal/services/member_push.go
  internal/services/settlement_service.go
  internal/services/session_service.go
  internal/services/rsvp_service.go
  internal/handlers/notifications.go
  internal/handlers/openapi.yaml
  internal/testsupport/db.go
frontend/
  src/components/notifications/NotificationSettings.tsx
  src/components/notifications/PushNotifications.tsx
  src/components/notifications/NotificationHistory.tsx (new)
  src/components/members/MemberPushPanel.tsx
  src/pages/AdminMembers.tsx
  src/services/api.ts
  src/services/notifications.ts
  src/services/notification-destination.ts         (new)
  src/context/AuthContext.tsx
  src/App.tsx
  public/firebase-messaging-sw.js
.github/workflows/deploy.yml
.github/workflows/preview-deploy.yml
```

**Structure Decision**: Extend existing backend/frontend boundaries. The worker is an execution mode of the same backend, privately deployed for cloud-authenticated scheduled requests. It does not expose the public member/admin router.

## Trigger and Delivery Design

1. **RSVP**: On each scheduled invocation, query open sessions with due 96/60-hour stages. Select approved members with no RSVP. Insert recipient events using a unique key `(session_id, user_id, stage)`. The scanner derives times from current stored `starts_at`; claim/delivery revalidates start, status, membership, deadline and response.
2. **Split**: In the transaction that writes the settlement and ledger movements, aggregate all charge lines per member and snapshot the resulting balance using that transaction. Insert one `session_charge` event and outbox row per included member, including zero-cost receipts. Uniqueness uses `(settlement_id, user_id)`. Both legacy settlement and expense confirmation converge on `SettlementService`.
3. **Nudge**: Keep the current serialized 24-hour manual cooldown. Lock relevant player account using the ledger lock order before reading the balance, in addition to existing member serialization; do not write the ledger. Save an event and outbox row atomically. Use an admin request ID to distinguish retries from new actions.
4. **Announcement/cancellation**: Move announcement business logic out of the handler. Persist announcement, publication audience, recipient events and outbox rows together. Cancellation already persists much of this atomically; enqueue those records rather than producing duplicate events.
5. **Delivery**: Claim due rows with `FOR UPDATE SKIP LOCKED` and a renewable/recoverable lease. Release database locks before FCM. Snapshot active recipient devices; create unique event/device attempts. Recheck stops, membership, reminder/nudge eligibility and device enrollment before each send. Record per-device provider outcomes; retry only transiently failed devices.

A split can optionally request a bounded post-commit dispatch for prompt delivery, but worker processing is sufficient and mandatory. Do not start a detached goroutine and assume it finishes after an HTTP response on Cloud Run.

### Retry and Expiry Defaults (proposed operational policy)

- Event uniqueness guarantees one logical history item, not exactly-once device presentation.
- Per-device retries: initial attempt plus three retries after 1, 5 and 15 minutes, respecting provider backoff advice and event expiry. Accepted devices are never intentionally resent.
- If a send may have succeeded but acceptance persistence failed, label the attempt uncertain; do not automatically resend it. An expired lease with an in-flight send becomes uncertain. Recover unsent leased work safely.
- Use stable event notification tags to reduce repeated presentation where the platform supports it; do not claim this proves exactly-once delivery.
- Reminder expiry: first stage expires at the second due time; second expires at the RSVP deadline. If both are overdue, create only the latest useful stage and mark the earlier stage skipped. On rollout, record past due stages as skipped to avoid a backlog.
- Session time edits before sending replace due/expiry facts. A stage already sent is not sent again merely because the date changes; a later unsent stage follows the new start. Eligibility is checked immediately before sending, acknowledging a response/cancellation racing with a provider call cannot retract a message already submitted.
- Other categories expire after 24 hours. Stops, no devices and removed membership produce terminal suppressed delivery state, rather than a future backlog. Event history is still retained. An admin nudge reports suppression visibly.
- Event history is available without devices; pushes are not a substitute for an in-app record. Add a lightweight history view with links; no new category switches.

## Device Setup and Migration

The status truth is **active enrolled registrations**, not old booleans. The local browser can additionally check permission and registration success. Admins see registration metadata only, never raw tokens or current-permission claims.

- Give each installation a stable opaque device identifier associated with the current member. Explicit enroll registers/rotates the FCM address and creates active enrollment. Disable revokes that enrollment and removes its active token; retain a tombstone to block silent refresh. This is registration lifecycle state, not a separate account push preference.
- Automatic refresh is allowed only for a known active enrollment. Refresh cannot reactivate a revoked/absent enrollment. Explicit enroll is the only transition back to active; local suppression also prevents repeated repair prompts/registration loops. Coordinate tabs via storage events or BroadcastChannel.
- Disable only the selected installation. Ordinary sign-out preserves enrollment and tokens. Account switching revokes the former member’s registration on this installation before transferring delivery to a different member; stale requests must not rebind it to the former member. Failed revocation is surfaced and retried rather than reported as success.
- Preserve every existing token and its ownership at rollout, irrespective of old preference booleans. Add registration lifecycle/installation metadata without revoking, rotating or recreating those tokens. Automatically attach an installation identity when its authenticated browser returns; do not duplicate registrations or request permission again. Devices that never revisit remain deliverable by their current token.
- Legacy master/category preferences have no effect after the device-policy cutover. No opt-out translation, opt-out migration or re-enrollment campaign is built. Existing registered devices receive all four categories as their triggers are introduced; global deployment/club stops still apply.
- Accurate status ships first without changing delivery. The device-policy release atomically switches delivery eligibility to active registrations, retires old preference mutations and updates the frontend. Stale clients receive an update-required response for obsolete setup mutations; historical payloads remain readable. New scoped revocation cannot be undone by an obsolete refresh.
- Retain obsolete columns as inert storage during rollout; do not build a backward preference-policy adapter. Retire ambiguous tokenless unregister mutations and maintain existing token rows. Enrollment/refresh/revoke use the new device protocol. Existing notification history is preserved.

## Deadlines and Threshold Cutover

- Replace all session create/update/recurring deadline builders with `starts_at - 48h` using a shared helper.
- A one-time migration updates every open future session whose existing deadline is still in the future, including custom deadlines, with one snapshotted migration cutoff. Never reopen already-closed windows; preserve historical/cancelled sessions. For later edits, retain an already-closed window rather than reopening it; an explicit date/start change to an open session converts its deadline to 48h with a preview. New custom-deadline controls are retired. Reminder delivery always respects the actual deadline of preserved legacy sessions.
- Remove retired 24/12-hour attendance and 6-hour deadline schedules; cutover uses one scheduler ownership/config switch so old and new processes cannot both enqueue.
- The agreed fixed notification threshold is 2000 cents. Automatic messages, manual eligibility and money/member nudge controls share this value. Retain the Club column for compatibility but align its returned value and existing nudge eligibility to 2000 cents with an explicit migration/contract update. No threshold editing is exposed in the current inspected admin UI.

## Destinations and Future Channels

Use a server-produced, validated local destination on each event: RSVP `/sessions/:id`; charge `/sessions/:id/settlement`; manual nudge `/money`; announcement exact in-app history item; cancellation `/sessions/:id`. Expose event IDs to correlate clicks/read state. Only allowlisted local destinations are accepted; never trust arbitrary external URLs from notification data or OAuth state. Foreground toast, service worker, provider link and history use the same contract. Restore the intended path through Auth0 login while preserving the existing invitation `/welcome` flow and normal membership authorization.

Events contain business facts, not push-specific preferences. Push delivery reads enrollments. A future email/SMS adapter would introduce verified channel contact, explicit enrollment/consent, delivery attempts and its own provider. Do not build email/SMS schemas, settings or adapters now, and do not auto-fallback to existing profile contacts.

## Incremental Delivery

The authoritative six-release sequence and release acceptance gates are in [release-plan.md](release-plan.md). All existing device registrations survive every release without re-enrollment. Product changes apply from their respective cutovers; old preference policy is not maintained after release 2. Persist migration completion/cutover markers; do not reapply business backfills on every deployment.

1. Accurate member status and actionable notification links.
2. Device-only controls with registrations preserved; retire flags and waitlist emission together.
3. Durable delivery queue, per-device attempts and private scheduled worker.
4. RSVP reminders at 96/60 hours and default deadline at 48 hours.
5. Combined session charges/balance alerts and unified manual nudges.
6. Transactional announcements/cancellations and final cleanup.

Confirmed: ordinary sign-out preserves registration; every still-open future RSVP window becomes 48h, including custom deadlines; reversals send Session charges corrections and suppress pending original receipts. Remaining technical review findings must be resolved in the detailed handoff before implementation; this revision does not claim that the prior review is fully remediated.
