# Contracts: Club Ledger and Session Settlement

Conventions shared by every endpoint in this directory.

## Route groups

Every endpoint records the Gin group it registers on. Principle III makes this a security
decision, so it is stated in the contract rather than left to be inferred from the path.

| Group | Requirement | Used for |
|---|---|---|
| `approved` | authenticated, `membership_status = approved` | all reads |
| `admin` | `role = admin` | every write, plus the club position |

There are no public or `protected`-only endpoints in this feature. Reads sit on `approved`
because the spec makes balances and session breakdowns visible to all members (FR-026,
FR-027); the club's asset position is admin-only (FR-029).

## Money encoding

Amounts cross the wire as **integer cents** in fields suffixed `_cents`. No endpoint accepts
or returns a decimal or float for money. The frontend divides by 100 at the point of
display only.

Shuttle quantities cross as whole `units`.

## Errors

Errors return a JSON body with a stable machine-readable `code`, so the frontend can react
to specific conditions rather than matching on message text:

```json
{ "code": "shuttle_stock_short", "message": "…", "details": { … } }
```

| Code | HTTP | Meaning |
|---|---|---|
| `shuttle_stock_short` | 422 | Settlement needs more shuttles than stock holds. `details` carries `required_units`, `available_units`. Drives the inline purchase prompt. |
| `session_already_settled` | 409 | A live settlement exists. Reverse it first. |
| `transaction_already_reversed` | 409 | That transaction has already been reversed. |
| `invariant_violated` | 500 | The club-position identity failed; the transaction was rolled back and nothing was written. Should never surface. |
| `not_settleable` | 422 | The session is cancelled, or has not yet finished. |

## Idempotency and concurrency

Settlement is guarded by the session row lock and a partial unique index (research.md R8).
Two concurrent settle requests for one session result in exactly one settlement; the loser
receives `session_already_settled`.

## Pagination

List endpoints take `limit` (default 50, max 200) and `offset`, and return
`{ "items": [...], "total": n }`.
