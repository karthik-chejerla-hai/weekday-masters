# Implementation design: transaction corrections and shared expenses

**Date:** 2026-10-26. **Branch:** `codex/member-correction-design`.
**Baseline:** PR #103 merged as `9b83151`.
**Status:** Design complete; later implementation tasks remain unbuilt.

## Outcome and scope

Give approved admins reviewed top-up corrections/reversals, shared bill entry and
corrections, and whole native-session corrections. Every confirmation binds exact
before/after outcomes, is safe to retry, and preserves immutable history. Reuse
D001–D015 and completed R001; do not rebuild ledger arithmetic, initial settlement,
asset purchase correction, personal spend or the existing member history browser.

This change is documentation only. It does not implement financial operations,
run migrations or change production money. The local Spec Kit `plan.md` follows
the repository's existing ignore rule; this tracked document is its reviewable
architecture/delivery summary. All detailed decisions are in the linked artifacts.

## Technical context

Go module declares Go 1.22; Gin + GORM + PostgreSQL 16. React 18 + TypeScript +
Vite + Tailwind; Auth0 identity; Cloud Run backend and Firebase Hosting frontend.
No new dependency or global state library is required. Backend uses global
`database.DB` and explicit caller transactions, with business rules in services.
Tests use Go scratch-database/race harness, Vitest/Testing Library and existing
Playwright fixtures. Amounts are integer cents; Sydney is the date authority.

This is a small single-club web app. Bound forms/JSON to 100 member/line rows and
128 KiB. History retains server pagination (default 50, max 200). Review and confirm
must use bounded account/user queries rather than one query per form row. No new
throughput or latency SLA is invented; acceptance includes a four-person mobile
bill entered and reviewed in at most two minutes and no unrelated account locking.
Correctness and stable outcomes take priority over micro-optimizing this scale.

## Components and source locations

- `backend/internal/services/member_money_service.go`: proposed common reviewed
  orchestration, canonical preview/retry protocol and supported type dispatch.
- `backend/internal/services/shared_expense_service.go`: proposed paid/owed/funding
  validation and immutable shared version construction.
- `backend/internal/services/settlement_correction.go`: proposed frozen-cost
  correction orchestration; extract allocation from existing settlement costing.
- `backend/internal/services/money/`: reuse split/stock; add checked proportional
  arithmetic and supplied-band allocation. No database in this package.
- `backend/internal/models/{money_operation,shared_expense}.go`: proposed additive
  records; extend existing transaction/settlement/line models, no cached balances.
- `backend/internal/database/db.go`, `backend/internal/testsupport/db.go`: explicit
  migrations/constraints and every new table's scratch reset registration.
- `backend/internal/handlers/{member_money,shared_expense}.go`, server route wiring
  and `openapi.yaml`: transport only; approved reads and approved-admin mutations.
- `ledger_history.go`/`ledger_activity.go`: extend the existing reader and ordering,
  preserving imports and full-history balances; retain raw entry endpoints.
- `user_service.go`/`rsvp_service.go`: transaction-aware removal/status consistency,
  bounded to the locking integration needed by financial confirmation.
- `frontend/src/components/money/`: proposed MoneyOperationReview,
  TopupCorrection, SharedExpenseForm/row and SessionCorrection controls alongside
  current components. `pages/Money.tsx` and `pages/SessionSettlement.tsx` host them.
  All requests remain in `services/api.ts`; reuse existing balance-change event.

These are intended implementation file paths, not claims those new files exist.

## Detailed artifacts

- [research.md](research.md): decisions, alternatives and current-code evidence.
- [data-model.md](data-model.md): fields, states, constraints and additive migration.
- [contracts/operations.md](contracts/operations.md): endpoints, inputs, previews,
  canonical hashes, retry responses, errors and legacy-path transition.
- [contracts/locking-and-valuation.md](contracts/locking-and-valuation.md): global
  lock protocol, cooperating existing paths, removal and frozen stock algorithm.
- [contracts/history.md](contracts/history.md): read shape, tie ordering, zero-net
  visibility, filters, running balances and frontend interaction contract.
- [quickstart.md](quickstart.md): validation scenarios and safe local run guide.
- [tasks.md](tasks.md): dependency-ordered executable tasks and requirement map.
- [validation.md](validation.md): executed documentation checks and resolved
  source-based design findings; runtime implementation remains untested/unbuilt.

## Constitution check — before research and after design

Both passes: PASS, with no exception or amendment requested.

I: Handlers parse; services own rules; existing global DB pattern retained.
II: Additive schema/constraints go through `cmd/migrate`, never server startup.
III: Approved-member reads and approved-admin writes bound at route registration;
service actor checks and membership locks protect confirmation too.
IV: Resolved Sydney effective timestamps, separate recording time and play date;
DST acceptance checks required.
V: Exact bounded integer cents and checked proportional math; saved stock pairs,
largest-remainder allocation, no stored/typed rounded unit price.
VI: Append inverse/replacement and immutable allocations/lines; originals retained.
VII: One ledger writer, complete account union locks, invariant checks after both
postings, final stock guards, failure/concurrency tests specified.
VIII: New DB tests use RequireDB and register every new table; disposable DB only.
IX: Existing AuthContext/hooks/API client and balance event; no global store.

Cross-cutting user/RSVP changes are necessary to freeze membership and keep removal
from racing financial confirmation. They do not introduce a repository abstraction
or replace unrelated membership/product flows. Cross-cutting table intent locks
prevent the documented importer/account lock inversion while ordinary writers
remain compatible. See research decisions 3–4; both must be tested, not assumed.

## Delivery sequence and release gates

PR numbers below are sequence labels, not assigned GitHub numbers.

1. **PR 1 — reversal safety:** merged #103. Complete; preserve its regression suite.
2. **PR 2 — reviewed operation foundation:** MoneyOperation/migration, canonical
   preview/replay, common review DTO and locking integration; reviewed reversal
   support for existing top-up/session types and coordinated legacy API dispatch.
   No new correction UI yet. Foundation tests prove rollback, replay, membership,
   original-lock contention, import lock ordering and existing asset compatibility.
3. **PR 3 — top-up correction slice (first user-facing increment):** correction
   preview/confirm, saved editor, reason/review, existing top-up/session reversal
   entry controls, transaction detail/links, minimal new effective-date history
   order and refresh. Ship a usable flow, not a backend-only top-up editor.
4. **PR 4 — shared expense entry:** immutable versions/allocations, all funding
   and split modes, zero-net history and entry/review UI, session suggestions,
   required shared filter and spend-exclusion tests. Recognize shared type in all
   undo routes before enabling creation. If needed split schema/backend and UI
   commits inside this PR, but don't expose an incomplete form.
5. **PR 5 — shared expense correction/reversal:** whole-version preview and atomic
   replacement, funding/metadata changes, reason/audit chain and read controls.
6. **PR 6 — native-session correction:** mode-preserving editor, exact stock delta
   valuation, frozen-cost allocation, comped/guest support for legacy, atomic live
   replacement, spend/history refresh. Requires foundation plus US1 review/history;
   it is not technically blocked by shared entry and may be scheduled earlier
   once those prerequisites land. This numbered order is the default delivery order.
7. **PR 7 — final integration verification:** close the full history/type/paging,
   mobile/accessibility, date/recording explanations and concurrency matrix. Basic
   audit/history/refresh/tests ship with each slice, not deferred to this last PR.

Each schema change lands with its dependent backend and reset harness in the same
PR. Each PR runs relevant backend race/HTTP tests and frontend tests/lint/build;
coverage floors remain intact. Enable new-kind writes only after capable backend
revisions are serving traffic. No new notifications; regular initial settlement
notifications are preserved. No plan step authorizes production financial edits.

## Completion boundary

R002 is satisfied by these contracts/models/design and generated tasks. It does
not imply R003–R014 are implemented. The next implementation action is PR 2's
foundation; top-up correction begins only after those safety prerequisites pass.
The historical 2026-10-25 audit remains a baseline and is not rewritten as current
feature support. New decisions are linked from it and the specification.
