# Implementation plan

1. Add `BalanceCorrection`, an immutable audit snapshot keyed by request UUID,
   and `balance_correction` transaction kind. Register the table in the explicit
   migrator and scratch-database cleanup.
2. Extend LedgerService with atomic reconciliation. Lock asset and surplus in
   existing UUID order, check the reviewed balance, call `PostWithin`, then append
   the audit snapshot in the same transaction. Preserve integer-cent arithmetic
   and the existing accounting invariant.
3. Add approved-admin POST and paged GET `/admin/balance-corrections` routes.
   Derive actor identity from authentication and document contracts in OpenAPI.
4. Add a review/save form and paged audit history to Money → Club assets.
   Keep a request ID across retries, reject malformed decimal input and reload
   club assets after save or stale-balance rejection.

No new provider or dependency is needed. Normal deployment runs `cmd/migrate`
before starting the new server. This implementation does not modify production.
