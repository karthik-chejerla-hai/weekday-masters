# Implementation audit: transaction corrections and shared expenses

**Audited**: 2026-10-25
**Code baseline**: current checkout `38eed32`; GitHub merged PR list checked live.
**Purpose**: Identify completed work to reuse and the actual remaining delta.

The completed checkboxes in [tasks.md](tasks.md) are scoped capabilities, not
claims that every flow in a broad requirement is implemented. All original
functional requirements remain acceptance targets. The baseline audit itself changed no application code. Subsequent PR 1 work is
recorded separately below.

## Foundation model/migration increment — 2026-10-26

T002–T003 are implemented on `codex/member-money-model`, stacked on design PR #104.
[foundation-schema.md](foundation-schema.md) records the additive operation model,
transaction links, deferred target FK, completion/immutability constraints and
populated-upgrade/concurrency checks. Full backend `go test -race ./...` passed
against a dedicated disposable PostgreSQL 16 database. There are no new HTTP
operations yet. T004–T014 remain pending, so R003/reviewed-operation foundation
is not complete; top-up correction UI has not started.

## Design update — 2026-10-26

PR #103 is merged as `9b83151`. R002 is now complete as a design deliverable:
[implementation-design.md](implementation-design.md), research.md, data-model.md,
contracts/, quickstart.md and tasks.md define schema, reviewed operations/replay,
lock order, stock valuation, history and dependency-ordered PRs. The next work is
PR 2's reviewed-operation foundation before the top-up correction UI slice.
At design completion all later implementation tasks were unchecked; see the
subsequent model/migration update above. The requirement-by-requirement
section below deliberately preserves the earlier audit baseline.

## Subsequent PR 1 implementation

[pr1-reversal-safety.md](pr1-reversal-safety.md) supersedes the baseline gaps for
atomic reversal and reversal-of-reversal. R001 is merged: generic
session reversal dispatches to the atomic settlement path, both endpoints use
approved-admin checks, and inverse transactions are not reversible. This closes
the reversal safety portions of FR-001/012/013/017/025. Full session replacement,
review controls, durable request replay and new expense flows remain pending.
The complete backend race suite passed against a disposable PostgreSQL database.

The requirement statuses below preserve the **pre-implementation baseline** at
`38eed32`; use this update and tasks.md for current PR 1 status.

## Confirmed shipped work

- [PR #24 — Club ledger and session settlement](https://github.com/karthik-chejerla-hai/weekday-masters/pull/24):
  D001–D006 — cent arithmetic, invariant enforcement, immutable posting, exact
  reversal, stock arithmetic, deterministic splitting, rate snapshots and native
  settlement costing/reversal.
- [PR #36 — Import history and share member money views](https://github.com/karthik-chejerla-hai/weekday-masters/pull/36):
  D007/D008 and part of D015 — full member history, inactive labels, filters,
  running balances, grouped session activity and imported-history safeguards.
- [PR #42 — Expense assistant and personal spend](https://github.com/karthik-chejerla-hai/weekday-masters/pull/42):
  D009/D010 — actual-count preview/confirm, stale-preview rejection, concurrency
  protection and session-only personal spending.
- [PR #43 — Exclude imported sessions from outstanding expenses](https://github.com/karthik-chejerla-hai/weekday-masters/pull/43):
  D015 — preserve imported session history without permitting duplicate native charges.
- [PR #67 — Audited admin asset balance corrections](https://github.com/karthik-chejerla-hai/weekday-masters/pull/67):
  D011/D015 — reviewed bank/court audits and snapshot restrictions.
- [PR #73 — Member top-ups on request](https://github.com/karthik-chejerla-hai/weekday-masters/pull/73):
  D012 — existing top-up disclosure and header refresh.
- [PR #74 — Asset details and forms on demand](https://github.com/karthik-chejerla-hai/weekday-masters/pull/74):
  D013 — existing asset details/purchase UI.
- [PR #102 — Correct recorded club asset purchases](https://github.com/karthik-chejerla-hai/weekday-masters/pull/102):
  D014 — reviewed immutable purchase correction, links/reasons, atomic replacement,
  retry/conflict behavior and stock guards. GitHub confirms this PR is merged;
  it is not pending work for this feature.

## Requirement-by-requirement status

- **FR-001 — Partial; D007/D014.** Approved-member money reads and approved-admin
  asset-correction writes exist. Add access checks for new shared/member mutation
  routes. Legacy generic reversal/top-up routes use RequireAdmin without the
  additional explicit RequireApproved check found on asset corrections; do not
  assume complete approval enforcement from the admin route prefix.
- **FR-002 — Partial; D012.** Record top-up exists with disclosure. Record shared
  expense, its form and review do not exist.
- **FR-003 — Not implemented.** No native shared-expense funding/payer model.
  Imported paid-on-behalf history is readable but is not an entry flow.
- **FR-004 — Partial; D005.** Exact, deterministic, selection-order-independent
  largest-remainder arithmetic exists. Integrate it with shared paid/owed totals
  and custom shares; do not write another split algorithm.
- **FR-005 — Not implemented as an expense flow.** LedgerService can post balanced
  member-only movements, but there is no validated personal/bank-funded bill service.
- **FR-006 — Partial; D008/D014.** Grouped session details distinguish payments,
  gross shares and net changes; asset correction reviews show asset deltas.
  Shared-expense and member correction before/after review is missing.
- **FR-007 — Not implemented for these new mutations.** Existing reads retain
  inactive labels. Corrections need the original-versus-new participant rule.
- **FR-008 — Not implemented.** Existing session/default-participant resolution
  is not optional context selection for a shared expense.
- **FR-009 — Partial; D006/D009.** Initial settlement forms and saved-rate/line
  data exist. No top-up correction endpoint/editor or saved-settlement correction
  flow exists. The expense assistant prepares initial settlements, not meal bills.
- **FR-010 — Partial pattern only; D014.** Asset purchases already require a
  reason and reject unchanged amount/count. Shared-expense correction is missing;
  do not count the asset-only implementation as the member feature.
- **FR-011 — Partial; D002/D014.** Transaction links and atomic immutable
  reversal/replacement exist for asset purchases. Apply them to top-ups,
  shared-expense allocations and session records as indivisible operations.
- **FR-012 — Partial; D003/D014.** Exact single reversal is implemented. Generic
  reversal currently accepts an absent reason, lacks reviewed member controls,
  and does not reject reversal-of-reversal by kind. Add the new reason/review,
  current-version and type-specific behavior.
- **FR-013 — Partial; D004.** Successful session reversal restores original money
  and marks the settlement reversed. The two writes are not atomic, and the
  generic reversal route bypasses settlement status. Full correction remains.
- **FR-014 — Partial; D006.** Existing costing correctly handles bands, guests,
  comped amounts and surplus. Attendance-only redistribution of frozen recorded
  costs is missing; calling the ordinary costing path would revalue stock.
- **FR-015 — Partial; D005/D006.** Saved rates and exact stock consumption exist.
  Correction-specific preserved/delta valuation is missing. Reuse the arithmetic
  rather than repricing all original consumption from current stock.
- **FR-016 — Existing enforcement complete; new integration unverified; D001.**
  LedgerService.PostWithin checks balanced movements and club position under
  ordered account locks; player debt is supported. This core is done. New money
  flows must use it and add their own invariant acceptance tests.
- **FR-017 — Partial; D003/D009/D014.** Reversal uniqueness, serialized settlement
  and asset same-payload retry/conflict handling exist. A durable submission key
  and payload replay for all new member mutations, and atomic session undo,
  still need implementation. An already-reversed error is not a successful retry.
- **FR-018 — Partial; D009.** Actual-count expense fingerprints bind attendance,
  costs and rates and reject changed previews. New correction previews must also
  bind original version, memberships and displayed current/new balances. Do not
  claim the current cost fingerprint already covers those balance outcomes.
- **FR-019 — Partial; D002/D007/D014.** Original/reversal/replacement relationships
  exist in storage; asset UI shows reasons, recorder/time and correction IDs;
  member history shows reversed badges. Member transaction IDs/links, complete
  correction navigation, recording-time presentation and shared versions remain.
- **FR-020 — Partial; D008.** Group-before-page and full expansion exist for native
  and imported session rows. Shared-expense allocation grouping and zero-net
  participant discovery remain; extend the existing activity reader.
- **FR-021 — Partial; D007.** Personal/all-member and top-up filters are done.
  Sessions/Shared expenses type filters and original-type reversal inclusion
  are missing; the current top-up filter excludes ordinary reversal rows.
- **FR-022 — Partial; D007.** Full effective-history running balances before
  filtering/paging are done. New correction effective-date semantics and their
  recording-time explanation remain. Generic inverses currently use now; asset
  replacements retain purchase dates. Neither fulfills the new member rule.
- **FR-023 — Existing session-spend behavior complete; new integration unverified;
  D010.** Spend uses Sydney play dates and live native/imported session charges,
  not net credits, top-ups or non-session expenses. Tests cover reversal and
  replacement. Reuse this query and verify the new meal type never enters it.
- **FR-024 — Partial; D012/D013.** Top-up header refresh and asset/position reload
  exist. Wire new operations to the same refresh mechanism, add spend/history
  updates and retain form/review data on new-flow errors.
- **FR-025 — Partial; D011/D014/D015.** Import/audit safeguards and asset-purchase
  corrections are implemented and done. Close the generic session/type bypasses
  and verify all new routes preserve the existing protected-history boundaries.
- **FR-026 — Pending integration.** No shared-expense/correction notification flow
  exists yet. Preserve current ordinary settlement notifications and ensure the
  new orchestration does not invoke them for intermediate or final corrections.

## Evidence in the checkout

### Core ledger and settlement

- [ledger_service.go](../../backend/internal/services/ledger_service.go): PostWithin,
  RecordTopup, ReverseTransaction and invariant/account-lock enforcement.
- [ledger_service_test.go](../../backend/internal/services/ledger_service_test.go):
  TestPostRefusesUnbalancedMovements, TestRecordTopupMovesBankAndPlayerTogether,
  TestReverseRestoresBalancesAndKeepsBothEntries, TestReverseRefusesASecondReversal,
  TestReverseUnwindsShuttleUnits and TestIntegrityAfterAFullSequence.
- [ledger.go](../../backend/internal/models/ledger.go): immutable entry structure,
  unique reversal/correction links, correction reason and recording dates.
- [split.go](../../backend/internal/services/money/split.go) and
  [stock.go](../../backend/internal/services/money/stock.go): deterministic seeded
  remainder allocation, exact value/unit consumption and zero-stock behavior.
- [settlement_service.go](../../backend/internal/services/settlement_service.go)
  and [settlement_service_test.go](../../backend/internal/services/settlement_service_test.go):
  saved rates, base/extra-hour splits, guests, comping, stock restoration,
  resettlement and TestSettleIsSafeUnderConcurrency. ReverseSettlement commits
  ledger undo before updating state, so D004 is explicitly limited to the
  successful path.

### History, analytics and preview

- [ledger_history.go](../../backend/internal/services/ledger_history.go) and
  [ledger_history_test.go](../../backend/internal/services/ledger_history_test.go):
  full-history balance windows, source-aware classification and personal/top-up
  filters, including inactive members and pagination.
- [ledger_activity.go](../../backend/internal/services/ledger_activity.go) and
  [ledger_activity_test.go](../../backend/internal/services/ledger_activity_test.go):
  TestGameActivityGroupsSourceBeforePagingAndKeepsGrossShares,
  TestGameActivityNativeGuestCompedAndSameDaySessions and
  TestGameActivityOverFiftyKeepsEverySplitOnOnePage.
- [ledger_spend.go](../../backend/internal/services/ledger_spend.go) and
  [ledger_spend_test.go](../../backend/internal/services/ledger_spend_test.go):
  source-limited session spending, Sydney boundaries and
  TestPersonalSpendNativeGuestsReversalsAndReplacement. That test explicitly
  acknowledges the generic reversal path does not mark settlement state.
- [expense_service.go](../../backend/internal/services/expense_service.go) and
  [expense_service_test.go](../../backend/internal/services/expense_service_test.go):
  initial actual-count expense review/confirm, TestExpenseRejectsStalePreview,
  TestExpenseConcurrentConfirmation and
  TestExpenseDifferentSessionsCannotConsumeTheSameStock.

### Existing correction and frontend flows

- [balance_correction.go](../../backend/internal/services/balance_correction.go):
  reviewed asset audits; [balance_correction_test.go](../../backend/internal/handlers/balance_correction_test.go)
  checks their HTTP access.
- [asset_correction.go](../../backend/internal/services/asset_correction.go) and
  [asset_correction_test.go](../../backend/internal/services/asset_correction_test.go):
  audit/retry, consumed-stock rollback, validation, competing corrections and
  correction-versus-reversal tests. Existing same-payload asset retry is not a
  reusable durable request-ID protocol for all new member operations.
- [ledger handler](../../backend/internal/handlers/ledger.go) and
  [handler tests](../../backend/internal/handlers/ledger_test.go): existing
  approved reads, admin writes, generic reverse, shared filters and spend access.
- [Money.tsx](../../frontend/src/pages/Money.tsx),
  [LedgerBrowser.tsx](../../frontend/src/components/money/LedgerBrowser.tsx),
  [LedgerGameRow.tsx](../../frontend/src/components/money/LedgerGameRow.tsx) and
  [AssetPurchaseHistory.tsx](../../frontend/src/components/money/AssetPurchaseHistory.tsx):
  top-up disclosure/refresh, member filters, expanded shares and asset correction
  entry/review UI. Member correction and shared-expense entry controls are absent.

## Verification limits

During this audit, `go test ./internal/services/money` passed (cached), and four
existing frontend files passed all 25 tests: LedgerBrowser, LedgerGameRow,
AssetPurchaseHistory and TopupForm. Existing database-backed tests were inspected
as evidence but not rerun against a scratch database. Full new-feature validation
remains unchecked. No completed checkbox claims runtime support for an unbuilt
shared-expense or member-correction flow.
