# Specification review

**Reviewed**: 2026-10-25
**Result**: Ready for detailed planning, with explicit proposed product defaults.
PR 1 reversal safety is now implemented; later application work has not started.
This document records the original requirements review; implementation and test
results are recorded in pr1-reversal-safety.md.

**Implementation reconciliation**: The follow-up audit confirms substantial
existing work from merged PRs #24, #36, #42, #43, #67, #73, #74 and #102. See
[implementation-status.md](implementation-status.md) for exact coverage and
[tasks.md](tasks.md) for 15 completed reusable capabilities and the remaining
delta. The original “implementation has not started” status referred to new
feature work,
not the existing ledger, history, analytics or asset correction flows. The
subsequent PR 1 result is recorded in [pr1-reversal-safety.md](pr1-reversal-safety.md).

## Findings resolved in the specification

1. **Session reversal could leave money and status inconsistent.** At baseline,
   `SettlementService.ReverseSettlement` called the independently committed
   ledger reversal before updating settlement status. The generic transaction
   reversal path also accepted session transactions without coordinating that
   status. FR-013, FR-017 and FR-025 require one indivisible operation and
   prohibit generic-route bypasses. PR 1 now implements those reversal safeguards.
2. **“Recalculate” could silently reprice shuttles.** Current settlement costing
   consumes from the stock available when called; blindly reusing it on a past
   session could change its cost even when only attendance changed. FR-014/015
   and the valuation policy preserve original usage value and value only extra
   units from current stock. Legacy and actual-count calculations stay distinct.
3. **Inactive participants had no defined correction behavior.** FR-007 permits
   exact reversal and correction of original participants while requiring newly
   introduced members to be approved. This enables genuine historical fixes
   without opening new expenses to inactive accounts.
4. **Correction dates and displayed balances were ambiguous.** FR-019/022/023
   distinguish effective occurrence, session play date and actual recording
   time. New member-operation inverses use the original effective time;
   replacements default to it. Backdated effective balances are explicitly
   recomputed. Existing asset-correction behavior stays outside this change.
5. **Net movements cannot explain a payer's share.** FR-004/006/020 require
   retained paid and owed amounts, including zero-net members and bills. The
   worked examples distinguish member-funded and bank-funded expenses.
6. **Retry checks could conflict with stale-review checks.** FR-017/018 require
   retrieval of a successful identical submission before rejecting a stale
   review. Different contents under the same submission identity are rejected.
7. **History filters could hide reversal effects or misclassify credits.**
   FR-020/021/022 retain full breakdowns and running balances, associate inverses
   with original types and prohibit treating a payer's credit as a top-up.
8. **The draft lacked formal outcomes and bounded scope.** Added prioritized
   independently testable stories, 26 numbered requirements, seven measurable
   outcomes and exclusions. Plans/tasks now state their preliminary status.

## Acceptance coverage

- Top-ups: Story 1 covers FR-009–FR-012, FR-016 and FR-019.
- Shared expense creation: Story 2 covers FR-002–FR-008 and FR-016;
  edge cases cover single-member/all-zero-net bills and invalid values.
- Session corrections: Story 3 covers FR-009 and FR-013–FR-018, including
  legacy meaning, original rates, comped shares and stock valuation examples.
- Shared expense corrections: Story 4 covers FR-010–FR-012 and funding changes.
- Read experience: Story 5 covers FR-019–FR-023 and approved-member visibility.
- Permissions/history boundaries: Story 5 scenario 4 and edge cases cover
  FR-001/007/025, including protected historical types and reversal-of-reversal.
- Cancellation, refresh, errors and no notifications: FR-024/026 plus the review
  cancellation/error edge cases define verification cases for detailed tasks.
- SC-001 defines a later mobile usability check; SC-002–SC-007 define exactness,
  traceability, concurrency, history, authorization and stock acceptance targets.

## Proposed defaults for product review

Admin-only writing; all approved members may read breakdowns. Equal/custom
amounts, multiple personal payers and a separate club-bank funding option are
included. No mixed funding, percentage splits, automated transfers/refunds,
unrelated balance adjustments or new notification sends. Optional native-session
context does not make a meal part of badminton spending. These defaults are
documented rather than left for an implementer to guess.

## Code consulted

- `backend/internal/services/ledger_service.go`: posting, cash movements,
  generic reversal and accounting identity.
- `backend/internal/services/settlement_service.go`: saved-rate costing,
  session/account locks, guest/comped and extra-hour splits, reversal status.
- `backend/internal/services/money/stock.go`: exact unit/value consumption.
- `backend/internal/services/asset_correction.go`: linked reversal/replacement,
  serialized correction, original dates and stock guards.
- `backend/internal/services/ledger_history.go`, `ledger_activity.go` and
  `ledger_spend.go`: effective running balances, grouping, paid/gross shares
  and session-only spending.
- `backend/internal/handlers/ledger.go`: existing generic reversal route.
- Frontend Money, TopupForm, LedgerBrowser, LedgerGameRow and SessionSettlement:
  current entry points, expansion and missing member correction controls.
- `.specify/memory/constitution.md`: exact cents, immutable ledger and invariant
  requirements, explicit migrations and specification workflow.

## Design completion — 2026-10-26

The design work below has now been completed in
[implementation-design.md](implementation-design.md), data-model.md, research.md,
contracts/, quickstart.md and the detailed T001–T060 tasks. PR #103 is merged.
Implementation remains a separate next step; no feature mutation is built here.

## Original design follow-up (now completed)

Produce detailed contracts, schema/migration design, preview binding rules,
lock order, history ordering and a dependency-ordered execution task list from
the reviewed requirements, using the completed D001–D015 baseline rather than
rebuilding it. Validate this design against the same constitution.
No unresolved product-rule blocker remains in this draft; proposed defaults
remain open to user edits before implementation.
