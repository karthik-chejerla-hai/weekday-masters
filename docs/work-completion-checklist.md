# Work completion and dead-code removal checklist

Updated: 14 November 2026 (Australia/Sydney).

Execute the numbered chunks in order. Within a chunk, check an item only when
its result is recorded below or linked from the relevant feature evidence.
Implemented, locally verified, CI green, merged and activated are separate states.
This checklist does not require completing every future money feature before
removing unrelated unused helpers; intentionally retained foundations stay intact.

## 1. Preserve and reconcile local work

- [x] Inventory the original 19 registered worktrees and associate the six open PRs.
- [x] Identify historical merged work: mobile UI #35, browser telemetry #83,
  notification R1 #86.
- [x] Compare the two uncommitted R1 push-test navigation changes with this
  checkout: both `/dashboard` fallbacks already exist here.
- [x] Save a reproducible inventory and the R1 patch before touching integration branches.
- [x] Preserve the local-only issue #99 scheduled-reminders spec and checklist
  in this checkout, leaving the original files untouched.
- [x] Map issue #99/#96/#97 requirements to notification R3/R4; record remaining
  acceptance gaps and superseded timing policy instead of building a second worker.

## 2. Finish notification integration — R2 through R6

Use latest PR heads, not stale local release branches. Preserve device ownership,
guest RSVPs, current deadline rules, financial locking and truthful delivery state.
Use synthetic recipients and disposable databases for verification.

### 2.1 R2 — device setup and registration retention (#88)

- [x] Create an isolated integration worktree from the latest #88 head.
- [x] Integrate current main and resolve any incompatibilities within R2's scope.
- [x] Read unresolved review threads and validate reported issues.
- [x] Run real-database backend race tests and relevant frontend/device/browser checks.
- [x] Record review and validation evidence; publish scoped fixes if needed.
- [x] Obtain fresh successful CI/preview checks on the intended PR head.
- [ ] Complete integration through the normal release process; record resulting SHA.

### 2.2 R3 — durable delivery and private worker (#89; after R2)

- [x] Integrate the updated R2 base and resolve conflicts.
- [x] Investigate recorded preview branch-creation HTTP 401; fix or record the
  concrete external dependency without exposing credentials or weakening checks.
- [x] Verify outbox fencing, concurrent claims, retries, uncertain outcomes,
  private invocation, migration idempotence and preview isolation.
- [ ] Obtain fresh green CI/preview after completed review, tests and publication.
- [ ] Complete integration and record SHA; retain explicit worker activation state.

### 2.3 R4 — RSVP reminders (#90; after R3)

- [x] Integrate updated R3 and resolve conflicts, preserving configurable deadlines.
- [x] Verify 96h/60h stages, RSVP eligibility, expiry, Sydney DST, rescheduling,
  rollout/backlog behavior and legacy-scheduler cutover.
- [ ] Review, validate, obtain fresh green CI/preview, complete integration and record SHA.

### 2.4 R5 — charges, balance alerts and nudges (#92; after R4)

- [x] Integrate R4; verify atomic immutable charge receipts, reversal corrections,
  historical settlement navigation and nudge retry/cooldown semantics.
- [x] Verify compatibility with current expense/guest/financial-lock behavior.
- [ ] Review, validate, obtain fresh green CI/preview, complete integration and record SHA.

### 2.5 R6 — announcements and cancellation (#93; after R5)

- [x] Integrate R5; verify atomic audience/history/outbox writes, retries and
  truthful saved/queued/suppressed feedback.
- [ ] Review, validate, obtain fresh green CI/preview, complete integration and record SHA.
- [ ] Reconcile issue #80/#81/#96/#97/#99 acceptance and stale status documents.
- [ ] Record worker/IAM/Scheduler/monitoring/rollback readiness separately from
  actual deployment or activation. Do not claim live delivery was verified by mocks.

## 3. Finish the existing profile-safety increment (#112)

This is independent of the notification PR dependency chain. Its integration can
occur earlier when ready; revalidate affected financial/notification seams afterward.

- [ ] Revalidate F001's profile ownership, atomic account naming and pre-F005
  lock-order regressions on a scratch database.
- [ ] Review #112, verify current CI and complete integration; record SHA.
- [ ] Update the F001 evidence and next-packet status without marking whole
  foundation milestones complete.

## 4. Continue money corrections in bounded feature increments

Follow `specs/014-member-transaction-corrections/foundation-work-packets.md`.
These are planned packets, not thirteen separate discovered unfinished branches.

- [ ] F002: serialized membership/last-admin transitions and invite adoption.
- [ ] F003: target-member locking for RSVP writes.
- [ ] F004: caller-transaction RSVP removal/promotion effects.
- [ ] F005: freeze settlement membership before session/account locks.
- [ ] F006: atomic member removal and RSVP effects (after F001–F005).
- [ ] F007: shared exact-inverse helpers (after F006).
- [ ] F008: reviewed top-up reversal services (after F007).
- [ ] F009: reviewed native-session reversal services (after F008).
- [ ] F010: approved-member transaction detail (after F008/F009).
- [ ] F011: reviewed top-up HTTP/API cutover (after F010).
- [ ] F012: reviewed session HTTP/API cutover (after F011).
- [ ] F013: cross-route authorization, replay and rollback compatibility (after F012).
- [ ] F014: exact-payload frontend review/retry hook (after F011/F012).
- [ ] T015–T024: top-up correction/reversal UI, review, history and refresh.
- [ ] T025–T036: shared-expense entry, allocations and grouped history.
- [ ] T046–T051: shared-expense correction/reversal (after entry).
- [ ] T037–T045: native-session corrections (after foundation/top-up review;
  independent of shared expenses, default sequence after shared correction).
- [ ] T052–T056: complete history/filter/date/pagination matrix.
- [ ] T057–T060: full real-DB race/migration, desktop/mobile and documentation gate.

Each packet includes implementation, meaningful regression tests, review and CI.
Keep the MoneyOperation model/migrations, member-money protocol/handlers and reversal
wrappers while their feature is active. If later scope is parked, record that
decision explicitly; cleanup must not silently cancel it.

## 5. Repeat removal audit on the integrated baseline

- [ ] Search latest open PRs and remaining worktrees for every candidate again.
- [ ] Run frontend full-project/production and Go command/test reachability checks.
- [ ] Preserve ORM hooks, embedded/build assets, standalone commands and registered
  API endpoints unless separately justified.
- [ ] Record a final removal list with definitions, callers and feature dependencies.

This chunk can proceed after chunks 1–3 with chunk 4 explicitly retained or parked;
unrelated helper cleanup does not depend on building the whole future money product.

## 6. Remove confirmed leftovers in small PRs

- [ ] Backend: `GetConfirmedPlayers`, `testsupport.Available`, unused `WithUser`
  context path and its dedicated test; keep live Gin context and `dbAvailable`.
- [ ] Frontend: unused `useApi` query hook/tests, `Account`/`AccountKind`, and the
  seven API wrappers if still unused; keep `useApiMutation` and backend endpoints.
- [ ] Notification: reassess `getHistory`, `markAsRead`, unused type re-export and
  obsolete token wrapper after R2 cutover. Keep `disablePushNotifications` and
  `FormatTimeForDisplay`.
- [ ] Config/docs: remove unread `Config.Timezone` and matching environment/docs/
  deployment flags together, including incoming worker flags; keep Sydney behavior.
- [ ] Optional: move/simplify test-only `SumShares`, `Stock.Add`, `UnitCostCents`
  and `EndOfDay` without losing useful stock/time regression coverage.
- [ ] Run affected checks per PR and full checks on the final integrated result.

## Evidence and current handoff

- 13 November audit: frontend build and 426 tests passed; Go tests passed with DB
  cases skipped. This is baseline evidence only, not validation of integration work.
- 14 November: checklist created; preservation and R2 integration work started.
- Preservation: original #99 Markdown copied unchanged to
  `specs/012-scheduled-reminders/`; requirements mapping and unresolved timing,
  suppression and catch-up differences are in `reconciliation.md` there.
  No policy change is implied by the mapping; R2 does not depend on that decision.
- Local backups: `tmp/dead-code-audit/2026-11-14/` contains worktree/status snapshots,
  the two-line R1 patch and validation logs. Originals are untouched.
- R2 integration worktree:
  `/Users/karthikchejerla/.codex/worktrees/rally-r2-integration-20261114/weekday-masters`,
  branch `work/notification-r2-integration`. Current main merged at `f50f235`.
  There were no Git conflicts, but real-DB execution exposed three stale guest
  constructor calls and three obsolete promotion-alert assertions; those are
  adapted while retaining actual promotion/capacity checks.
- Independent review found a first-login legacy-token discovery failure could
  bypass former-account cleanup. Three new regression cases failed before the
  fix; the SDK lookup now propagates failures and persisted discovery state is
  retried before sign-in completes. A fourth regression covers configuration loss
  with a surviving browser subscription. Targeted Firebase/device suite: 49 passed.
- R2 final local checks: full PostgreSQL 16 backend race suite passed, 72.3%
  coverage; 467 frontend tests, build, lint/coverage and E2E typecheck passed;
  148 desktop/mobile browser tests passed. Lint retains one pre-existing
  `SessionDetail` dependency warning. Both independent reviews found no remaining
  actionable P1/P2 issues after fixes.
- Published R2 integration: merge `f50f235` and privacy/guest fix `0d3734e` are
  pushed to PR #88. Evidence is
  `specs/011-notification-delivery/r2-integration-validation.md` on that PR.
- Fresh #88 CI at `0d3734e`: backend/frontend coverage, desktop/mobile browser
  tests, all CodeQL analyses, aggregate coverage and both preview deployments
  passed. GitHub reports CLEAN. Production/image/badge jobs are intentionally
  skipped on this PR. Evidence: Actions runs `43523175597`, `43523175503` and
  `43523167800`; [PR #88](https://github.com/karthik-chejerla-hai/weekday-masters/pull/88).
  The PR is merge-ready; no main merge or production activation was performed.
- Fresh #88 preview passed database branch creation and migrations; the old #89
  HTTP 401 should be rechecked on its updated head before changing credentials.
- Non-checkout R3 merge probe against updated R2 found ten conflict files:
  database migrations, notification service/cancellation tests, browser worker,
  Firebase, notification destinations, device state/tests, and two release docs.
  R3 must retain the R2 discovery-retry privacy fix during resolution; no R3
  source worktree was changed by this probe.
- 14 November stack implementation: R3/R4/R5/R6 are integrated on isolated
  branches and pushed to #89/#90/#92/#93. Actual `gh stack link` created GitHub
  stack #113 with #88 as the unmerged bottom dependency; merged #86 is excluded.
  Verified each stack entry and base through GitHub's stack API.
- Incremental local validation: full real-DB race suites passed on all four
  releases (71.9% / 73.1% / 73.3% / 73.3% coverage). Frontend suites passed
  469 / 470 / 479 / 485 tests; the combined top passed 154 desktop/mobile browser
  tests. Build/lint/coverage/typecheck passed. Existing lint/chunk advisories remain.
- Reviews: separate standards/spec reviews cleared all four releases after R3
  producer/status fixes and R5 generic-reversal/config regression coverage.
- Historical #89 preview 401 did not reproduce: its fresh preview created the
  database branch and migrated it successfully. Hosted checks/previews for all
  published heads are being monitored; no main merge/production activation.
- Combined testing instructions are in `docs/notification-stack-testing.md` on
  #93. Its newest successful preview contains every layer together; preview
  member push stays stopped. Main merges and live worker activation remain open.
- Current next action: finish hosted CI/preview checks for the complete stack,
  update this evidence, then hand over the #93 preview for testing.
