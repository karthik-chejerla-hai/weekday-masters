# Foundation protocol increment — T004–T006

Date: 2026-10-26. Branch: `codex/member-money-protocol`, based on PR #105.
This adds internal service/transport primitives, not reviewed money endpoints.

## Implemented

- A service transaction runner reserves a UUID using INSERT ON CONFLICT, compares
  actor/canonical payload identity, and replays the accepted status/body before
  any kind-specific version/freshness checks.
- Current approved-admin authorization is checked again under a user SHARE lock
  on both first execution and replay. A removed/demoted actor cannot read the
  stored result. New operations lock the sorted union of actor and supplied users
  before entering the kind-specific callback. Replay locks only its actor.
- The callback receives the caller transaction, operation UUID and normalized
  request. It must acquire the correct original/session/account locks, recheck
  the preview and post through LedgerService. These kind-specific orchestrations
  remain T011/T016/later work; the runner does not infer their business rules.
- Reservation, callback postings and completed response commit once. Callback,
  response-validation or deferred-commit errors return no successful result and
  leave no reservation or money change. Completion stamps actual recording time.
- PostInput carries operation ID/step to the existing sole ledger writer. An early
  financial-table intent helper is used by the runner. Adoption across existing
  financial entry points remains T007; it is not claimed complete here.
- Bounded UTF-8 JSON parsing rejects duplicate keys, non-object roots, trailing
  values, fractional/exponent/out-of-range numbers, excessive nesting and unknown
  typed fields. The common handler envelope accepts no client actor field.
- Canonical JSON uses lexicographically sorted object keys and exact integers.
  Type-specific services must normalize meaningful strings/dates/defaults and
  member/line arrays before submission; generic array sorting is deliberately
  not used. Review fingerprints are domain-tagged `member-money-review:v1`.
- Common review/effect DTOs retain old/new member amounts, balances and stock/
  asset deltas. A kind-specific preview still supplies complete relevant state
  from a consistent snapshot; the digest helper alone is not a preview service.

## Tests and evidence

The new service tests were run first against the placeholder seam and failed with
`member money protocol not implemented`. They then passed against the runner.
The tests exercise real PostWithin postings inside the caller transaction:

- identical retry after later balance/correction changes returns the same body;
- changed contents, preview or actor under the same ID cannot execute/disclose;
- removed/demoted actor cannot replay;
- callback failure, invalid response and deferred commit failure roll back money
  and reservation and return no successful result;
- eight concurrent identical requests execute/post exactly once;
- two different reservations reach a barrier before either locks the original,
  then produce one success and one version conflict without duplicate postings;
- exact large integer cents, duplicate keys, input bounds and canonical fingerprints;
- strict handler decoding rejects unknown fields, duplicate/nested keys and
  oversized bodies without registering a route.

Passed `go test -race ./...` with TEST_DATABASE_URL explicitly pointing to a
new disposable PostgreSQL 16 database. All DB-backed services/handlers/migration
packages ran successfully. No existing application/production database was used.
`git diff --check` and task/link checks passed. Frontend code is unchanged.

## Remaining delivery checklist

[tasks.md](tasks.md#delivery-checklist) is the single authoritative checklist.
T001–T006 are complete; **T007–T060 remain pending** (54 tasks). Next is early-lock
integration, followed by membership/RSVP consistency and reviewed reversal services,
routes and client helpers. The foundation is not yet an exposed application flow.

Existing notification behavior and legacy endpoint behavior remain unchanged by
this increment. No notification or new HTTP write is sent from the runner itself.
