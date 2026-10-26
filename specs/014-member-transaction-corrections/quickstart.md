# Validation guide: member corrections and shared expenses

This describes acceptance for the planned implementation. New endpoint paths and
test files below do not exist on the PR #103 baseline; do not interpret this guide
as a successful feature test run. No production dataset or notification send is
needed. Detailed payloads/rules: [contracts/operations.md](contracts/operations.md).

## Prerequisites and isolated setup

Use the repository's normal Go/Node toolchains and Docker PostgreSQL 16. Create a
new disposable database container using the AGENTS.md scratch database recipe,
choosing an unused port. Set `TEST_DATABASE_URL` only to that database. The harness
truncates every listed table and must never target an app/production database.
Keep credentials in the environment and redact them from captured output.

For manual UI testing use a separate loopback application database, apply
`cd backend && go run ./cmd/migrate`, then synthetic `go run ./cmd/seed -env=local`.
Use existing local Auth0/Playwright fixtures, not production identities. Keep
notifications disabled. The form uses no assistant/provider key. Configure the
backend/frontend following AGENTS.md; never assume `.env` points to loopback.

## Design-only checks (runnable now)

From the repository root (on a fresh checkout, reconstruct the ignored local
plan entry point first):

```bash
cp specs/014-member-transaction-corrections/implementation-design.md \
  specs/014-member-transaction-corrections/plan.md
git diff --check
SPECIFY_FEATURE_DIRECTORY=specs/014-member-transaction-corrections \
  .specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks
```

Check artifact links, unique task IDs, checklist formatting and FR-001–FR-026
coverage. Spec Kit's local `plan.md` is ignored intentionally; the tracked design
summary is `implementation-design.md`. These checks validate the documentation,
not financial implementation or runtime correctness.

## Implementation checks (as each slice lands)

```bash
cd backend
go test ./internal/services/money
go test -race ./...
```

The second command needs `TEST_DATABASE_URL` to execute DB tests rather than skip.
Use targeted service/handler test names during each slice, then full suite for CI.

```bash
cd frontend
npm test
npm run lint
npm run build
npm run test:e2e
```

Use existing Playwright configured local servers and synthetic fixture conventions;
validate desktop and 360px mobile widths. Existing CI enforces coverage floors.

## Acceptance scenarios and precise expected outcomes

1. **Top-up correction:** Starting from recorded Alex +$50/bank +$50, correct to
   +$40: review/final deltas Alex -$10, bank -$10. Change recipient to Bea: Alex
   -$40, Bea +$40, bank $0. Reverse replacement: Bea -$40, bank -$40. Original,
   each inverse and replacement remain linked; reason and recording time visible.
   Spend chip unchanged; header/history/position refresh without manual reload.
2. **Reviewed staleness:** Preview, then post to an affected account or change a
   relevant membership. Confirm must return preview_changed or authorization/version
   refusal with zero new financial records. A posting then equal inverse still
   invalidates via entry count. An unrelated personal expense must not invalidate
   a personal bill touching disjoint members. Keep inputs and require new review.
3. **Retry and contention:** Send one accepted confirmation ten times: same 201
   DTO/IDs and one operation. Retry after subsequent balance/version changes:
   still the same accepted result for same actor. Change payload/actor under its
   ID: conflict/forbidden, no response disclosure. Two distinct IDs correcting or
   reversing one original: exactly one success. Synchronize after both reservation
   inserts to catch FK lock-upgrade mistakes; use deferred target FK.
4. **Atomic rollback:** Inject a database failure on second posting, allocation
   insertion, operation response completion or settlement status update. Assert
   no operation/inverse/replacement/allocation persists; balances/stock/live state
   match the original. Retry can succeed after fault removal. Original entries
   and allocation bytes remain unchanged throughout.
5. **Shared bill:** Alex paid $120 for four: +$90/-$30/-$30/-$30, no asset/surplus
   changes. Alex $80 + Bea $40: +$50/+$10/-$30/-$30. Bank-funded: bank -$120,
   each member -$30; other assets/surplus unchanged. Reject mismatched totals.
6. **Remainders and zero-net:** $10/three = $3.34/$3.33/$3.33 in stable seeded
   order across repeated previews/selection reorder/retries. Test one cent across
   three participants, payer-only person and each paid=owed; retain participates,
   allocations and zero legs, find everyone in personal history. One-person expense
   and all-zero-net metadata correction/reversal remain traceable with no invented
   nonzero cash movement. Key-only session churn is no-change, not a cent resplit.
7. **Shared correction:** Change $120 Alex-funded to club-funded: Alex -$120,
   bank -$120, others zero. Then reverse replacement exactly. Category-only revision
   remains auditable; no-change rejected. Inactive original members may remain or
   be removed; newly introduced inactive members are refused. No emails or pushes.
8. **Session attendance:** Original $120/four → three: old fourth +$30 and each
   retained member -$10; court/shuttle deltas zero even after purchases/rate changes.
   Verify exact legacy hosted-guest and comped-surplus behavior, actual-count mode
   restrictions, band subsets and combined split when everyone stays.
9. **Session stock:** Old six/$24 → four: new $16, return two/$8. Old six/$24 →
   eight with current twelve/$60: new $34, current stock loses two/$10. Test zero,
   full remaining consumption, legacy base-return/extra-increase sequence, shortages,
   high-value overflow and unsupported legacy totals. Later settlements unchanged.
10. **History:** Populate >50 activities, all filter/scope combinations, inactive,
    zero-net, corrected/inverse records and imported regular+extra-hour groups.
    Compare raw full-history running balances to expanded event balances, using
    identical order, before filters/paging. Freeze recording timestamps to test
    inverse-before-replacement and chained operation sequence ties. Backdate across
    Sydney DST transitions; show effective date and actual recording time separately.
    Existing asset correction dates/order and imports remain unchanged.
11. **Membership/locks:** Concurrent removal and confirm cannot newly charge an
    unapproved participant. Removal's final balance check and RSVP changes are
    atomic; failed removal leaves RSVPs unchanged and sends no promotion. Test profile
    updates cannot restore stale role/status. Exercise importer/table intent against
    settlement, corrections/audits, and ordinary posting; no table/account deadlock.
12. **Access and boundaries:** Unauthenticated, ordinary member, pending/removed
    admin cannot preview/confirm writes. Supported legacy undo without review is
    refused. Imported/opening/withdrawal/audit/reversal kinds retain restrictions;
    existing asset correction still works. Shared session link cannot bypass spend
    exclusion or permit referenced-session deletion. Ordinary settlement still
    sends its existing post-commit notifications; new operations send none.

For every accepted financial step assert the signed transaction residual and
club-position residual are zero and stock valid. Compare proposed and actual net
effects exactly. UI cancel must post nothing; slow/failed network must retain
inputs and exact retry payload. Test labeled controls, keyboard focus and mobile
layout with no horizontal page scrolling.
