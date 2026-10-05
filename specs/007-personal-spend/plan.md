# Implementation plan

1. Add one caller-scoped aggregate read on LedgerService. Combine native charge
   lines and reviewed imported gross charges. Aggregate by Sydney play month.
   No schema, ledger writes, dependencies or global state changes are needed.
   Count distinct native session IDs for own attendance. Count imported positive
   shares once per import/play date to match existing game grouping.
2. Register `GET /accounts/me/spend` on the approved route group. Document the
   response in OpenAPI and add matching frontend types and API method.
3. Add a cancellable component hook for personal spend, refreshed by the existing
   balance-change event and member identity. Reuse it in the header and Analytics.
4. Add the fourth tab with URL-backed selection, total cards and a monthly view.
   Show monthly session counts alongside spend. Use joined label/value badges
   for the header balance and spend links on desktop and mobile.
5. Verify money semantics and access with database tests. Test frontend loading,
   failures, direct links and refresh. Run backend tests, frontend tests, lint
   and build. Check responsive rendering with synthetic data if possible.

## Risks

Imported net credits can conceal the payer's own charge. Retained gross charges
are mandatory. Reversals can use either settlement or transaction routes, so
both reversal signals must exclude the original. Existing uncommitted expense
assistant changes must remain intact. Tests must use a dedicated scratch DB.
