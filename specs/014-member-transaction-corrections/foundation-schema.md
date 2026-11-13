# Foundation increment: durable operation model and migration

Date: 2026-10-26. Branch: `codex/member-money-model`, stacked on design PR #104.
Scope: **T002–T003 complete**, not the entire reviewed-operation foundation.
No new financial endpoints, confirmation service or correction UI are implemented.

## Storage guarantees

- MoneyOperation stores submission UUID, actor/action/target, canonical JSON/hash,
  accepted response and a database-generated sequence for equal-time ordering.
- Transaction operation links/steps are nullable for legacy rows. Complete links
  require step 0 or 1, a valid operation FK and uniqueness per operation/step.
- The target FK is DEFERRABLE INITIALLY DEFERRED, preventing reservation from
  taking a premature original-row KEY SHARE lock before competing FOR UPDATE.
- Reservations may be incomplete inside their transaction. A deferred constraint
  trigger queries their final state and prevents incomplete rows from committing.
- A before-update/delete trigger protects completed operations. Replay will read
  the accepted response; it must not update the accepted audit record.
- Actor/action/target, hash, JSON object and response-pair checks reject malformed
  storage. The future service still owns authorization, payload comparison,
  freshness and type-specific business rules.

The explicit migrator creates the sequence before AutoMigrate, then adds cyclic
FKs/indexes/checks/triggers after both tables exist. Existing money rows/entries
are not rewritten. `testsupport.truncateAll` includes money_operations.

## Verification executed

New tests first failed on the old schema: `migration did not create money_operations`.
Then passed against a dedicated loopback PostgreSQL 16 disposable database:

```bash
# TEST_DATABASE_URL was explicitly set to the disposable test database.
go test -race ./internal/database -run TestMemberMoney -count=1
go test -race ./...
```

The full suite finished successfully, including existing DB-backed handlers,
services, imports, asset correction, ledger and settlement tests. Unchanged
packages used Go's cache on the final run; they passed with the same scratch DB
configuration in the preceding full run. No frontend code changed.

The migration tests verify:

1. Populated pre-feature top-up/ledger entries survive upgrade byte-for-byte;
   two consecutive migrations succeed.
2. Target FK is deferred at runtime and in catalog metadata; missing targets
   still fail at commit.
3. Two independent reservations precede either original lock; taking the first
   original FOR UPDATE succeeds with a bounded lock timeout.
4. Pending reservations roll back at commit; completed response/payload updates
   and deletion are refused.
5. UUID/sequence generation, duplicate submission identity, FK/pair/step uniqueness,
   response object/status and payload constraints behave as specified.

`git diff --check` passed. Tests use the normal RequireDB harness and isolated
package schema; they skip when TEST_DATABASE_URL is unset. No production database
or existing application database was used.

## PR #104 review follow-ups

The requested subagent review found no blocking design issues. Two clarifications
were applied to the contracts/task acceptance in this increment:

- Current approved-admin authorization is required **before returning a successful
  replay**, including when the caller was removed or demoted after the first save.
- Every new reviewed inverse, including a native-session inverse, retains the
  exact original effective occurrence timestamp.

The service/HTTP regressions for those rules belong to pending T004/T011/T013;
this schema increment does not claim to enforce HTTP authorization or date policy.
Next task: T004's replay/hash/rollback/original-lock contention service tests,
followed by T005–T006 protocol implementation.
