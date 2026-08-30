# Implementation Plan: Club Ledger and Session Settlement

**Branch**: `001-club-ledger-settlement` | **Date**: 2026-08-24 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-club-ledger-settlement/spec.md`

## Summary

Add a double-entry ledger to Rally covering player balances and the three places club
money actually sits — the bank, prepaid court credit at the venue, and shuttles in the bag
— plus a settlement flow that turns a played session into per-player charges.

The technical shape is: five new models behind one `LedgerService` that is the single
writer of money, a `SettlementService` that composes session costing on top of it, and a
money section in the frontend. All amounts are `int64` cents. Ledger entries are written
once and never modified; corrections post a reversing transaction. Every money-moving write
runs inside one database transaction that locks the accounts it touches, and asserts the
club-position identity before committing.

Delivery follows the spec's four prioritised stories. P1 (ledger, balances, top-ups) is
independently shippable and on its own replaces Splitwise.

## Technical Context

**Language/Version**: Go 1.22 (backend), TypeScript 5.6 / React 18 (frontend)

**Primary Dependencies**: Gin, GORM, `google/uuid`, Auth0 JWT via JWKS, FCM, SendGrid;
React Router 6, Axios, Tailwind 3, date-fns / date-fns-tz

**Storage**: PostgreSQL 16. Schema applied by `cmd/migrate` via `AutoMigrate`, plus raw SQL
for the two constraints AutoMigrate cannot express (see research.md R8).

**Testing**: Go standard-library tests against a scratch PostgreSQL via the existing
`internal/services/main_test.go` harness (`requireDB(t)`); Vitest + Testing Library on the
frontend; Playwright for end-to-end.

**Target Platform**: Cloud Run (australia-southeast1) behind Firebase Hosting; PWA on
mobile browsers.

**Project Type**: Web application — separate `backend/` and `frontend/` trees.

**Performance Goals**: None beyond interactive response. Club scale is ~20 members and ~52
sessions a year; the ledger reaches low thousands of rows over five years. Balances are
derived from entries rather than cached (research.md R4) because at this scale the
aggregate is free and drift is impossible.

**Constraints**: Amounts exact to the cent, no floating point. Ledger append-only.
Settlement atomic and idempotent under concurrency. All times Australia/Sydney.

**Scale/Scope**: ~20 users, ~52 sessions/year, ~5k ledger entries over five years.
Roughly 5 new backend models, 2 new services, ~14 new endpoints, 1 new frontend section
plus a tab split on the existing sessions page.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Checked against `.specify/memory/constitution.md` v1.0.0.

| Principle | Gate | Status |
|---|---|---|
| I. Layered backend | Money logic lives in `LedgerService` / `SettlementService`; handlers only parse and format. Services use `database.DB`. UUID PKs via `BeforeCreate`. | PASS |
| II. Explicit migrations | New models registered in `database.Migrate()`; the two raw-SQL constraints and the session backfill run there too. Server never migrates. | PASS |
| III. Middleware boundary | Reads register on `approved`; every write registers on `admin`. Contract files record the group per endpoint so review can check it. | PASS |
| IV. Sydney time, resolved | Feature adds `starts_at` / `ends_at` to `Session` and backfills them, removing the read-time `HH:MM` parse in `SchedulerService`. See note below. | PASS (remediates existing violation) |
| V. Money is integer cents | `int64` throughout; per-shuttle cost derived from stock at consumption; largest-remainder split. | PASS |
| VI. Append-only ledger | No update or delete path on `LedgerEntry`. Corrections are reversals. Settlement guarded against re-settlement. | PASS |
| VII. Invariants enforced and tested | Identity asserted inside every posting transaction; accounts locked in deterministic order; concurrency test mirrors the RSVP capacity test. | PASS |
| VIII. Tests skip without DB | New service tests use `requireDB(t)`; pure functions (split, costing) tested without a database. | PASS |
| IX. Frontend without global store | Money screens use `useApi`; header balance uses `useApi`; all calls through `services/api.ts`. No new state library. | PASS |

**Note on Principle IV** — the spec assumed billing durations depend on resolved session
timestamps. They do not: billed hours are an explicit input at settlement (FR-016), not
derived from the session's scheduled times. The migration is still required, for two other
reasons: the History tab needs a dependable past/upcoming boundary, which today is a bare
`session_date >= today` comparison, and `SchedulerService.parseSessionDateTime` currently
reconstructs a timestamp from `session_date` plus an `HH:MM` string on every read, which
Principle IV forbids. Recording this here so the justification is accurate and the work is
not mis-sequenced as a billing blocker — it is not one, and it can land independently.

**Post-Phase 1 re-check**: PASS. No new violations introduced by the data model or
contracts. Complexity Tracking below remains empty.

## Project Structure

### Documentation (this feature)

```text
specs/001-club-ledger-settlement/
├── plan.md              # This file
├── research.md          # Phase 0 output — 10 resolved decisions
├── data-model.md        # Phase 1 output — entities, fields, invariants
├── quickstart.md        # Phase 1 output — how to run and validate
├── contracts/
│   ├── README.md        # Conventions: money encoding, errors, route groups
│   ├── ledger.md        # Accounts, balances, transactions, reversal
│   ├── settlement.md    # Preview and settle a session, session history
│   └── settings.md      # Club rate and threshold settings
├── checklists/
│   └── requirements.md  # Spec quality checklist (all passing)
└── tasks.md             # Phase 2 output — NOT created by /speckit.plan
```

### Source Code (repository root)

```text
backend/
├── cmd/
│   ├── migrate/main.go              # unchanged; picks up new models via database.Migrate
│   └── server/main.go               # + route registration on approved / admin groups
└── internal/
    ├── database/db.go               # + new models, raw-SQL constraints, session backfill
    ├── models/
    │   ├── account.go               # NEW  Account
    │   ├── ledger.go                # NEW  Transaction, LedgerEntry
    │   ├── settlement.go            # NEW  Settlement, ChargeLine
    │   ├── club.go                  # + rate and threshold settings
    │   └── session.go               # + starts_at / ends_at
    ├── services/
    │   ├── ledger_service.go        # NEW  sole writer of money
    │   ├── settlement_service.go    # NEW  session costing on top of the ledger
    │   ├── money/                   # NEW  pure helpers: split, weighted-average cost
    │   ├── notification_service.go  # + balance reminder types
    │   └── session_service.go       # + resolved timestamps on create/update
    ├── handlers/
    │   ├── ledger.go                # NEW
    │   └── settlement.go            # NEW
    └── utils/time.go                # + ResolveSessionTimes

frontend/
└── src/
    ├── pages/
    │   ├── Money.tsx                # NEW  Balances | My ledger | Club assets
    │   ├── Sessions.tsx             # + Upcoming | History tabs
    │   └── AdminSettlement.tsx      # NEW  the settlement form
    ├── components/
    │   ├── money/                   # NEW  BalanceChip, LedgerList, PositionPanel
    │   ├── settlement/              # NEW  ParticipantRow, GuestRow, CostSummary
    │   └── layout/{Header,Navigation}.tsx  # + balance chip, + Money nav item
    ├── services/api.ts              # + ledger and settlement calls
    └── types/index.ts               # + Account, Transaction, Settlement, ChargeLine
```

**Structure Decision**: Web application, using the existing `backend/` + `frontend/` split.
No new top-level directories. The one new backend package is `internal/services/money`,
which holds the pure arithmetic — largest-remainder allocation and weighted-average stock
costing — so it can be exhaustively unit-tested without a database, per Principle VIII.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations. Table intentionally empty.
