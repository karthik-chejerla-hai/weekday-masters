# R2 integration with current main

14 November 2026. Baseline main: `e26869a`. Original PR #88 head: `c7987e3`.
Isolated branch: `work/notification-r2-integration`, worktree
`/Users/karthikchejerla/.codex/worktrees/rally-r2-integration-20261114/weekday-masters`.
Original worktrees were not reset or deleted.

## Integration and regression fixes

Current main merged without textual conflicts at `f50f235`. Real execution found
newer guest-RSVP tests still calling the old notifier-taking constructor and
expecting waitlist promotion alerts. R2 deliberately removes these alerts.
Updated those tests to retain actual capacity, promotion order, payer removal,
money-history and concurrency assertions while checking that no waitlist history
is written. The three targeted guest-promotion regressions failed before the
assertion updates and pass afterward.

Independent specification review found a privacy bug in first-rollout account
switching: the Firebase silent lookup converted transient failures into null, and
the new account could finish sign-in without retiring the former legacy address.
The silent path now propagates discovery failures. A persisted
`legacyCheckPending` flag survives failed discovery, so same-account sign-in retry
cannot skip it. Existing operation/snapshot fencing remains in place.

A follow-up case checks a frontend build losing its Firebase config while an
older worker still has a subscription. Missing config is not evidence of absence:
inspect local worker subscriptions and keep sign-in retryable if one survives.
An unconfigured browser with confirmed absence remains usable. Neither path
opens a permission prompt. Three initial regressions and the subsequent
configuration-loss case failed before their fixes; the targeted SDK/device suite
then passed all 49 tests.

## Validation

- Dedicated disposable `postgres:16-alpine` container, `rally-r2-audit-20261114`,
  loopback port 51149, database `rally_r2_test`; no existing application database.
- Full `go test -race -coverprofile=coverage.out -covermode=atomic ./... -count=1`
  passed with TEST_DATABASE_URL set to that database. Database suites executed;
  the optional private-export fixture remains conditional on its private input.
  Backend statement coverage: 72.3%, above the 60% floor.
- Three guest-promotion regressions passed under `-race -v`, with no skips.
- Frontend coverage suite: 55 files, 467 tests passed; build and E2E typecheck
  passed. Coverage floors passed.
- ESLint passed with the existing `SessionDetail.tsx` loadSession dependency
  warning; no new warning added. Build retains its existing chunk-size advisory.
- Desktop/mobile browser suite: all 148 tests passed in the final post-fix run.
- Standards review: no documented-standard breaches or actionable smells.
  Storage-unavailable first-login behavior is a lower-priority limitation:
  swallowing unreadable identity state would weaken privacy and needs separate
  positively established absence plus durable-storage/coordination design.
- Specification review after fixes: no remaining actionable P1/P2 findings.
  Capacity/promotion/guest safety and reminder policy remain in scope as intended.

No production migration, infrastructure activation or real-member notification
send was part of this verification. Local checks do not establish live delivery.
Fresh hosted checks and preview deployment must still validate the published head.
