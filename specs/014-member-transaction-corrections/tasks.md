# Implementation status and remaining tasks

Audited 2026-10-25 against the current checkout and GitHub's merged PR list.
Checked items are complete at the stated scope. An existing primitive does not
mean its new shared-expense or member-correction integration is complete.
Evidence and requirement-by-requirement gaps: [implementation-status.md](implementation-status.md).

This remains a preliminary work outline. Detailed contracts and dependency-ordered
execution tasks are still required before implementation.

## Specification — done

- [x] S001 Formalize the specification and review its requirements.
- [x] S002 Specify session stock valuation, inactive-member and history-date rules.
- [x] S003 Record the quality checklist and resolved review findings.
- [x] S004 Reconcile the proposal with shipped PRs and separate existing work from gaps.

## Already implemented — reuse without rebuilding

- [x] D001 Integer-cent ledger posting, balanced movements and club-position checks
  with account locks in stable ID order — PR #24; FR-016 foundation.
- [x] D002 Immutable ledger entries and original/reversal transaction links —
  PR #24; FR-011/012/019 foundation.
- [x] D003 Exact transaction reversal, including top-up bank/member effects and
  shuttle units, with a second reversal rejected — PR #24; FR-012 foundation.
- [x] D004 Settlement reversal restores original charges/assets and permits
  resettlement on the successful path — PR #24; atomicity remains R001.
- [x] D005 Deterministic largest-remainder splitting and exact stock unit/value
  arithmetic — PR #24; reuse for FR-004/014/015, not a new split engine.
- [x] D006 Settlement rate snapshots, base/extra-hour participant splits,
  hosted guests and comped-share surplus costing — PR #24; FR-009/014/015 foundation.
- [x] D007 Personal/all-member ledger views, top-up filtering, inactive labels
  and full-history running balances before filtering/paging — PR #36;
  existing portions of FR-001/019/021/022.
- [x] D008 Grouped native/imported session rows before pagination, expandable
  member shares, gross paid/owed information and balances — PR #36;
  existing session portions of FR-006/020. New shared-expense rows remain R009.
- [x] D009 Actual-count expense preview/confirm, attendance-bound cost fingerprint,
  stale-cost rejection and serialized stock consumption — PR #42;
  FR-018 foundation, not new correction/balance-bound previews.
- [x] D010 Sydney play-date personal spend, gross imported session charges,
  hosted guest charges and exclusion of reversed charges/non-session spending —
  PR #42; existing portion of FR-023.
- [x] D011 Audited bank/court asset-balance correction with review and historical
  audit restrictions — PR #67; existing workflow preserved by FR-025.
- [x] D012 Member top-up disclosure and header balance refresh after recording —
  PR #73; existing entry point and refresh portion of FR-002/024.
- [x] D013 Club asset disclosure/purchase forms and position refresh — PR #74;
  existing asset UI preserved, not a new form to implement here.
- [x] D014 Asset-purchase correction: mandatory reason, atomic exact reversal and
  linked replacement, original date retention, same-payload retry, competing-write
  protection, stock guards, admin access and review UI — PR #102;
  existing asset scope of FR-010/011/017/019/025.
- [x] D015 Import/audit reversal exclusions and imported-session settlement guard —
  PRs #36/#43/#67; existing restrictions to preserve under FR-025.

## Remaining implementation

- [x] R001 Make session reversal atomic with settlement state; close generic
  session reversal and reversal-of-reversal bypasses (FR-012/013/017/025).
- [ ] R002 Complete detailed schema/contracts, preview binding, lock-order design
  and dependency-ordered execution tasks; reuse D001–D015.
- [ ] R003 Add durable submission identity/payload replay for new member operations;
  retain successful retries before stale-review checks (FR-017/018).
- [ ] R004 Add top-up correction preview/confirm for member, amount, date and note;
  existing RecordTopup and generic reversal do not provide this (FR-009–FR-012).
- [ ] R005 Add shared-expense/version/allocation models, explicit migration and
  test-harness truncation; retain zero-net allocations (FR-003–FR-008/020).
- [ ] R006 Add shared-expense exact preview/confirm using D005, with personal,
  multiple-payer, club funding and custom shares (FR-003–FR-008/016/018).
- [ ] R007 Add shared-expense correction/reversal and atomic rollback/concurrency
  coverage, reusing D002/D014 patterns (FR-010–FR-012/017).
- [ ] R008 Add mobile shared-expense entry and member correction/reversal review
  controls, required reasons and exact before/after outcomes (FR-002/006/010/012/024).
- [ ] R009 Extend D007/D008 with shared-expense grouping, zero-net personal history,
  correction links and Sessions/Shared expenses filters including their inverses
  (FR-019–FR-021). Existing grouped game history is already done.
- [ ] R010 Implement member-operation effective dates and recording-time display,
  preserving full-history running balances from D007 (FR-019/022).
- [ ] R011 Add native-session correction from saved inputs, preserving original
  attendance-only costs and implementing shuttle delta valuation (FR-009/013–FR-015).
- [ ] R012 Apply approved-admin mutation access, inactive-original-member rules,
  exact reviewed-balance binding and new-type restrictions (FR-001/007/018/025).
- [ ] R013 Integrate new operations with existing balance/position/spend refresh;
  verify new shared expenses stay excluded from D010 and send no new notifications
  (FR-023/024/026). Existing spend queries/header event need reuse, not replacement.
- [ ] R014 Run new scratch-database race/rollback/access tests, frontend tests,
  lint, build and mobile browser QA against all acceptance scenarios.

## PR 1 — implemented

[pr1-reversal-safety.md](pr1-reversal-safety.md) records the implementation,
contracts, lock order and regression coverage. R001 is complete: both reversal
endpoints share an atomic session path; reversal-of-reversal is rejected. Approved
admin access is enforced for both routes. R012 remains open for the remaining
new mutation types and inactive-participant/preview rules.

Full backend `go test -race ./...` passed against the dedicated disposable
PostgreSQL database, including all database-backed packages. No frontend/schema
changes are needed for PR 1. Later PR work remains unchecked above.

## Verification during the earlier baseline audit

- Passed `go test ./internal/services/money` (cached).
- Passed the existing LedgerBrowser, LedgerGameRow, AssetPurchaseHistory and
  TopupForm test files: 4 files, 25 tests.
- Inspected the existing database-backed tests for ledger, stock restoration,
  history, expense previews, spend and asset corrections. They were not rerun
  against a scratch database during this documentation audit.
- Full feature validation remains R014; passing existing tests does not establish
  support for new shared expenses or member/session correction flows.
