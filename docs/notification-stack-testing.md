# Test the complete notification stack

14 November 2026. GitHub stack **#113**, created with `gh stack link`:

`main → #88 → #89 → #90 → #92 → #93`

PR #88 is the unmerged device-setup dependency. The requested four releases are
#89, #90, #92 and #93. The **PR #93 preview** contains all five changes together.
Use the preview link posted by the latest successful Preview Deploy workflow on
#93; older preview comments can have expired. `gh stack checkout 113` imports
the actual GitHub stack into a separate clean clone for local testing.

## What is implemented

- #89: private scheduled production/draining, durable fenced per-device work,
  retry/expiry/uncertain outcomes, health monitoring, fixed $20 reporting, and
  safe approved-admin delivery-status reads with a refresh control after nudges.
  The API no longer owns a reminder timer, including in this intermediate release.
- #90: persisted 96h/60h RSVP reminders for members without any response, bounded
  by actual stored deadlines. Initial club deadline default remains 50 elapsed
  hours; custom deadlines/DST and guest/game deletion guards are retained.
- #92: immutable session-charge/balance receipts, exact historical split links,
  atomic receipt suppression and correction on either reversal route, and
  idempotent manual nudges with a 24-hour cooldown. Hosted guest charges remain
  attributed to their payer. Provider failure cannot roll back committed money.
- #93: atomic announcement/cancellation publication, immutable audience/history/
  outbox intent, stable request IDs through retry/reload, and truthful saved,
  queued and suppressed counts.

## Manual preview sequence

1. Sign in as an admin and open Profile. Verify explicit browser push setup and
   disable controls; ordinary sign-out must retain an existing registration.
   Verify switching accounts completes cleanup or presents a retryable failure.
2. In Admin, inspect the club deadline setting and edit a synthetic future
   session's deadline. Check Sydney formatting and retained custom values; no
   notification rollout should rewrite all stored deadlines.
3. Use the synthetic seeded roster and sessions to record a played-session
   expense. Check exact player/hosted-guest shares and balance/spend refresh.
   Inspect its history and original split after a reversal and re-settlement.
4. Nudge a synthetic low-balance member from Balances or Manage Members. Check
   saved/queued/suppressed feedback, use **Check push status**, and verify a
   second submission respects the cooldown rather than creating another nudge.
5. Publish a clearly labelled test announcement. Refresh or retry after a lost
   response; the same request must retain its original publication/audience.
6. Cancel a synthetic future session. Confirm reason/history/count feedback,
   retained RSVPs, blocked edits and unaffected other recurring occurrences.
7. As an ordinary approved member, verify own notification history/detail and
   intended links. Admin delivery diagnostics and publishing controls must be
   unavailable. Foreign notification IDs must not expose another member's event.

Preview delivery stops remain active. Recorded history or a suppressed outcome
does not mean a member received a push. The private worker, OIDC authorization,
time advancement, concurrency and provider failures are exercised by synthetic
tests; real scheduled delivery and live device receipt require separate explicit
operational activation. No production merge or activation was performed here.

## Executed local validation

Each release used its own disposable PostgreSQL 16 database, with full
`go test -race -coverprofile=coverage.out -covermode=atomic ./... -count=1`.
Database-backed tests executed, rather than being counted as skipped passes.

- R3 (`eb985da`): backend coverage 71.9%; 469 frontend tests passed.
- R4 (`059e3f9`): backend coverage 73.1%; 470 frontend tests passed.
- R5 (`650230a`): backend coverage 73.3%; 479 frontend tests passed.
- R6 (`65edc6c`, same application code as this documentation update): backend
  coverage 73.3%; 485 frontend tests and all 154 desktop/mobile browser tests passed.
- Frontend build, coverage floors, lint and E2E typecheck passed at each release.
  Lint retains the existing SessionDetail effect-dependency warning; no new
  warning was added. Build retains the existing chunk-size advisory.
- Mocked worker deployment/cleanup and registry-retention protocol tests passed.
- Independent standards/spec reviews cleared each incremental release after
  fixing the R3 execution-owner/status-visibility gaps and the R5 stale generic
  reversal test. Both reversal routes verify pending correction configuration,
  original suppression/history, duplicate rejection and balanced money.

Hosted CI/preview results are tracked in each PR and the completion checklist.
Local checks do not establish deployed IAM readiness or live notification receipt.
