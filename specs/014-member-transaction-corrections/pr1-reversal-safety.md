# PR 1 — Atomic reversal safety

**Status**: Implemented and validated locally on `codex/reversal-safety`.
**Scope**: R001 plus reversal-route approval checks. No schema changes.

## Behavior and contracts

- Both `/admin/transactions/:id/reverse` for a session charge and
  `/admin/settlements/:id/reverse` use the same settlement orchestration.
- Exact inverse entries and `reversed_at`/`reversed_by` commit in one database
  transaction. Errors return no successful transaction object and leave both
  financial records and settlement state unchanged.
- Lock order: session, settlement, original transaction, then affected accounts
  in canonical account-ID order. Ordinary settlement also takes the session
  first. Preliminary routing reads use immutable identities and acquire no locks.
- The generic route resolves the settlement by its exact original transaction,
  never the session's latest live settlement. Repeating an old undo cannot undo
  a newer settlement. Concurrent reversals return one success and conflicts.
- A reversal cannot itself be reversed (`422 reversal_not_allowed`), preventing
  reinstated charges from bypassing live-settlement rules. Imported-history and
  audited-snapshot guards remain in force.
- Both HTTP routes require an approved admin. The dedicated settlement route
  already inherits this in the server; its test registration now matches it.
- Existing empty-description fallback, reversal occurrence timestamps, response
  shape and second-reversal conflict behavior remain compatible. Required reason
  review, durable replay IDs and changed effective dates belong to later PRs.
- No new notifications or frontend controls are introduced.

## Implementation tasks

- [x] Extract exact inverse posting into a caller-transaction helper owned by LedgerService.
- [x] Make settlement ledger undo and status update one atomic operation.
- [x] Dispatch generic session reversals through that same operation.
- [x] Reject reversal-of-reversal with a structured error.
- [x] Check the settlement's charge/session association before writing.
- [x] Enforce approved-admin access on both HTTP reversal paths.
- [x] Add real-database rollback, cross-endpoint contention and resettlement-race tests.
- [x] Add HTTP coverage for generic dispatch, metadata, access, inverse refusal and old-version safety.
- [x] Update API contracts, architecture notes and feature task status.

## Validation

Passed the full backend race suite using a dedicated disposable PostgreSQL 16
container/database (not an application database): `go test -race ./...` with
`TEST_DATABASE_URL` explicitly targeting that scratch database.

New regression coverage includes:

- An injected PostgreSQL trigger error on settlement status update rolls back all
  inverse transactions/entries and preserves every account value/unit balance;
  tested through both service entry points, followed by a successful retry.
- Eight concurrent mixed-entry-point reversals yield exactly one success, restore
  all assets/member balances/comped surplus, preserve original entries and record
  the actor/time. Subsequent settlement succeeds.
- Concurrent reversal and settlement produce either a clean unsettled state or
  one newly settled state, with matching account balances and no partial result.
- Reversal-of-reversal produces no entries or balance changes.
- HTTP generic reversal marks status and actor, allows resettlement, rejects old
  and inverse transactions, and refuses unauthenticated/member/unapproved-admin
  attempts without changing money or settlement status.

## Deliberate limits

This does not repair pre-existing data left inconsistent by an earlier failed
reversal; it prevents new partial reversals. No ledger history is rewritten.
Top-up/member corrections, shared expense entry and full session replacement
remain separate PRs. Existing generic asset reversals retain their prior policy.
