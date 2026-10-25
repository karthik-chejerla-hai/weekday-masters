# Tasks: Clear and Reliable Notifications

**Input**: spec.md, plan.md, data-model.md, contracts/notifications.md and [release-plan.md](release-plan.md).

**Status**: Revised release sequence; remaining detailed technical review corrections must be completed before implementation. No application work executed.

**Tests**: Required by FR-017. Tasks are ordered by the authoritative six-release sequence, with user-story labels retained. Task IDs were renumbered on 2026-10-13; old review references refer to the previous version.

## Setup

- [x] T001 Record release cutover/configuration and migration ownership in `specs/011-notification-delivery/quickstart.md`, including forward-only client cutover and no backlog replay.

## Release 1: US1 — Accurate member status

- [x] T002 [US1] Add batched active-registration summary and safe metadata in `backend/internal/services/member_push.go` and `backend/internal/handlers/member_push.go`, leaving current delivery unchanged until release 2.
- [x] T003 [US1] Update member summary/API typing in `frontend/src/services/api.ts`, `frontend/src/pages/AdminMembers.tsx` and `frontend/src/components/members/MemberPushPanel.tsx` to show Push set up/No push devices and counts without claiming permission or delivery.
- [x] T004 [P] [US1] Add no-device/default-preference, multi-device and private-token handler coverage in `backend/internal/handlers/member_push_test.go`.
- [x] T005 [P] [US1] Add member status/error/loading UI cases in `frontend/src/components/members/MemberPushPanel.test.tsx` and `frontend/e2e/member-push.spec.ts`.

## Release 1: US2 — Actionable links and history

- [x] T006 [US2] Implement validated destination generation and legacy payload fallback in `backend/internal/services/notification_events.go` and `backend/internal/services/notification_service.go`, including event ID and direct split path.
- [x] T007 [US2] Implement allowlisted destination resolver in `frontend/src/services/notification-destination.ts`; use it in `frontend/src/components/notifications/PushNotifications.tsx` and matching `frontend/public/firebase-messaging-sw.js` routing.
- [x] T008 [US2] Add own-event detail/history support in `backend/internal/handlers/notifications.go`, `frontend/src/services/api.ts`, `frontend/src/components/notifications/NotificationHistory.tsx` and `frontend/src/App.tsx`, linking each history item and exact announcement.
- [x] T009 [US2] Preserve protected local return path through sign-in in `frontend/src/App.tsx`, `frontend/src/context/AuthContext.tsx` and `frontend/src/context/auth-redirect.ts` without changing invitation/approval behavior.
- [x] T010 [P] [US2] Test destination generation and own-event access in `backend/internal/services/notification_service_test.go` and `backend/internal/handlers/notifications_test.go`.
- [x] T011 [P] [US2] Test foreground/background/history navigation and OAuth return safety in `frontend/src/services/firebase-worker.test.ts`, `frontend/src/context/auth-redirect.test.tsx` and `frontend/e2e/notification-navigation.spec.ts`.

## Release 2: US1 — Device-only controls; preserve registrations

- [ ] T012 [US1] Add installation/enrollment lifecycle and token-preserving registration metadata migration in `backend/internal/models/notification.go` and `backend/cmd/migrate/main.go`, preserving history and blocking old-client revival.
- [ ] T013 [US1] Implement explicit enroll, active-only refresh and scoped idempotent revoke in `backend/internal/services/notification_service.go` and `backend/internal/handlers/notifications.go`; distinguish revoked registration from missing device and retain token privacy.
- [ ] T014 [US1] Update `frontend/src/services/api.ts`, `frontend/src/services/notifications.ts` and `frontend/src/services/firebase.ts` for installation identity, successful server registration, scoped revoke and Firebase token cleanup.
- [ ] T015 [US1] Replace master/category controls with device setup in `frontend/src/components/notifications/NotificationSettings.tsx` and `frontend/src/components/notifications/PushNotifications.tsx`; coordinate tabs and permit silent refresh only for active enrollments.
- [ ] T016 [US1] Preserve registration on ordinary logout and revoke the former account’s installation on account switch in `frontend/src/context/AuthContext.tsx`; surface/retry failed removal and prevent stale refresh from rebinding the former account.
- [ ] T017 [P] [US1] Test preservation regardless of old flags, refresh/revoke concurrency, token adoption and ownership boundaries in `backend/internal/services/notification_service_test.go` and `backend/internal/handlers/notifications_test.go`.
- [ ] T018 [P] [US1] Test per-device disable across reload/tab/sign-in/account-switch and blocked/unsupported setup in `frontend/src/components/notifications/NotificationSettings.test.tsx`, `frontend/src/components/notifications/PushNotifications.test.tsx` and `frontend/e2e/push-enablement.spec.ts`.

## Release 2: US6 — Retire waitlist emission

- [ ] T019 [US6] Remove waitlist notification generation/notifier wiring from `backend/internal/services/rsvp_service.go` and `backend/cmd/server/main.go`, retaining promotions/locks/admin exceptions and historical type readability in `backend/internal/models/notification.go`.
- [ ] T020 [P] [US6] Update waitlist tests for no notification side effects while retaining promotion/concurrency assertions in `backend/internal/services/rsvp_service_test.go` and constructor/helper wiring in `backend/internal/services/main_test.go`.

## Release 3: Durable delivery foundation

- [ ] T021 Define additive event-key/destination/outbox/attempt schema in `backend/internal/models/notification.go`; register explicit migrations in `backend/cmd/migrate/main.go` and tables in `backend/internal/testsupport/db.go`.
- [ ] T022 Implement transaction-aware event/outbox insertion and unique originating-action identities in `backend/internal/services/notification_outbox.go`.
- [ ] T023 Add fake-provider/controlled-clock concurrency, rollback, lease recovery, partial acceptance and uncertain-send regression tests in `backend/internal/services/notification_dispatcher_test.go`.
- [ ] T024 Implement bounded claims, leases, per-device outcomes/retries/expiry and stop/membership/enrollment revalidation in `backend/internal/services/notification_dispatcher.go`; use `backend/internal/services/notification_service.go` for FCM delivery without holding DB locks.
- [ ] T025 Add private bounded HTTP worker entry point in `backend/cmd/notification-worker/main.go` and image binary in `backend/Dockerfile`, with no public member/admin routes.
- [ ] T026 Configure private worker deployment, Scheduler OIDC invocation each minute, least-privilege IAM, per-environment stops and independent backlog/failure monitoring in `.github/workflows/deploy.yml`, `.github/workflows/preview-deploy.yml` and `DEPLOY.md`.
- [ ] T027 Migrate existing send callers to queue via `backend/internal/services/notification_service.go`, using active registrations after release 2 while retaining trigger timing until its own cutover and preventing simultaneous old/new scheduler ownership in `backend/cmd/server/main.go`.
- [ ] T028 Align fixed notification-threshold reporting and all nudge eligibility at cutover in `backend/internal/handlers/ledger.go`, `backend/internal/models/club.go`, `backend/cmd/migrate/main.go`, `frontend/src/pages/Money.tsx` and `backend/internal/handlers/openapi.yaml`; retain the Club column for compatibility and do not add a threshold editor.

## Release 4: US3 — RSVP reminders and deadlines

- [ ] T029 [US3] Replace default deadline arithmetic and recurring/date/start-time edit behavior in `backend/internal/utils/time.go` and `backend/internal/services/session_service.go`; retire new custom-deadline input controls in `backend/internal/handlers/sessions.go` and `frontend/src/pages/AdminSessions.tsx` normalizing all still-open upcoming deadlines to 48h and preserving already-closed windows.
- [ ] T030 [US3] Add cutoff-based backfill of all still-open upcoming deadlines including custom ones and skipped past reminder stages in `backend/cmd/migrate/main.go`; preserve closed/cancelled/historical deadlines.
- [ ] T031 [US3] Replace attendance/deadline schedules with due-stage scanner and unique no-RSVP recipient events in `backend/internal/services/scheduler_service.go`; revalidate immediately before delivery through `backend/internal/services/notification_dispatcher.go`.
- [ ] T032 [US3] Remove old reminder configuration/cron startup and update exposed copy in `backend/internal/config/config.go`, `backend/cmd/server/main.go`, `frontend/src/components/notifications/PushNotifications.tsx` and `frontend/src/components/notifications/NotificationSettings.tsx`.
- [ ] T033 [P] [US3] Add elapsed-hour/DST/default recurring/edit and all-open-window normalization and closed-window preservation tests in `backend/internal/utils/time_test.go` and `backend/internal/services/session_service_test.go`.
- [ ] T034 [P] [US3] Add deterministic reminder boundary/response/concurrent-worker/outage/reschedule/cancellation/stop tests in `backend/internal/services/scheduler_service_test.go`.

## Release 5: US4 — Charges and reversal corrections

- [ ] T035 [US4] Add shared integer-cent balance classification/message fragments at 2000 cents in `backend/internal/services/notification_events.go`.
- [ ] T036 [US4] Inside settlement transaction, aggregate participant/host charges and snapshot balances after ledger posting, then create unique session-charge events/outbox rows in `backend/internal/services/settlement_service.go`; cover expense confirmation through `backend/internal/services/expense_service.go` and retire `notifyLowBalances` post-commit send.
- [ ] T037 [US4] Define transaction-aware reversal corrections and pending-receipt suppression in `backend/internal/services/ledger_service.go`, `backend/internal/services/settlement_service.go` and `backend/internal/services/notification_events.go`; identify the original settlement in split destinations and add reversal/re-settlement regression coverage in `backend/internal/services/settlement_service_test.go`.
- [ ] T038 [US4] Add exact aggregation/zero-cost/negative-zero-1999-2000/snapshot/rollback/concurrent-confirmation/provider-failure tests in `backend/internal/services/settlement_service_test.go` and `backend/internal/services/expense_service_test.go`.
- [ ] T039 [US4] Verify direct split receipt rendering and normal settlement UI success during delivery failure in `frontend/src/components/settlement/SettlementBreakdown.test.tsx` and `frontend/e2e/notification-navigation.spec.ts`.

## Release 5: US5 — Manual balance nudges

- [ ] T040 [US5] Refactor `backend/internal/services/notification_service.go` SendBalanceNudge for shared classification, transaction-safe balance read/request identity/outbox insertion and manual-only cooldown; recheck balance eligibility in `backend/internal/services/notification_dispatcher.go`.
- [ ] T041 [US5] Return precise nudge outcomes in `backend/internal/handlers/notifications.go`; update `frontend/src/services/api.ts`, `frontend/src/pages/AdminMembers.tsx` and `frontend/src/components/money/BalancesList.tsx` with fixed-threshold eligibility and queued/suppressed feedback.
- [ ] T042 [P] [US5] Test nudge request/cooldown concurrency, fixed threshold and suppression in `backend/internal/services/notification_service_test.go` and `backend/internal/handlers/notifications_test.go`.
- [ ] T043 [P] [US5] Test manual nudge eligibility and truthful outcomes in `frontend/src/pages/AdminMembers.test.tsx` and `frontend/src/components/money/BalancesList.test.tsx`.

## Release 6: US6 — Announcements and cancellations

- [ ] T044 [US6] Move announcement publication/audience/request identity/fan-out into one service transaction in `backend/internal/services/announcement_service.go` and invoke it from `backend/internal/handlers/notifications.go`.
- [ ] T045 [US6] Add outbox intent to existing atomic cancellation history creation and remove immediate delivery in `backend/internal/services/session_service.go`, reusing existing recipient events.
- [ ] T046 [P] [US6] Test publication/fan-out rollback/retry/restart, cancellation and send-time membership in `backend/internal/services/announcement_service_test.go` and `backend/internal/services/session_cancellation_test.go`.

## Final validation and documentation

- [ ] T047 Update final API/category/compatibility contracts in `backend/internal/handlers/openapi.yaml`, profile/admin copy in `frontend/src/components/members/MemberPushPanel.tsx` and notification architecture in `AGENTS.md` and `.specify/memory/constitution.md`.
- [ ] T048 Run and record quickstart/CI checks and isolated preview migration/worker rehearsal in `specs/011-notification-delivery/quickstart.md`, verifying no token leakage, stale backlog replay or production-message side effects.

## Dependencies and release gates

Use release-plan.md for scope and independent acceptance demonstrations. Execute releases 1 → 2 → 3 → 4 → 5 → 6; do not activate new queue-dependent triggers before release 3 is verified. Threshold reporting is aligned in release 3 before charge/nudge activation. Waitlist generation and its regression updates ship together in release 2. All API/schema/client changes and relevant tests needed by a release ship in that release, rather than deferring them to final documentation work.

Each migration records durable completion and is safe to rerun. Forward-only means no old preference-policy adapter, no opt-out translation and no forced re-enrollment. Historical notification/ledger data and existing token registrations remain intact. Runtime/preview failures are corrected with forward fixes; operational delivery stops still apply.

[P] test work can be authored in separate files after its API contract is fixed. Shared service/model/client changes integrate sequentially. US1 backend/frontend tests, US2 server/browser routing tests, US3 deadline/scanner tests, US4 receipt/browser tests, US5 backend/UI nudge tests and US6 announcement/waitlist tests provide the parallel opportunities. Do not run dependent tests before their required schema exists.

MVP: release 1, accurate status and working navigation. Every later release is a separate reviewable increment with release-specific backend/frontend/browser validation. Remaining technical review corrections include exact protocol/state schemas, migration integration, provider TTL, preview cleanup and announcement UI/idempotency details; these are not waived by the revised product policy.
