# Validation

Completed on 2026-10-04.

- Full backend suite passed with PostgreSQL scratch data and the race detector. Total statement coverage: 65.6%.
- All 26 frontend test files passed: 179 tests. Coverage: statements 71.2%, branches 68.7%, functions 64.62%, lines 72.41%.
- Frontend production build passed. Existing bundle-size warning remains.
- ESLint passed with no errors. Two existing hook warnings remain in AuthContext and SessionDetail.
- `git diff --check` passed.
- Browser review used the seeded local database. Both roles show the same dated asset cards. Member preview hides purchase forms.
- The real imported ledger displays original titles, source badges and category icons. Mine/all and top-up filters work. The browser loaded the second page beyond 50 records.
- A 390-pixel viewport preserves a single horizontally scrollable asset row. All three cards remain reachable. The viewport override was then cleared.
- Tests cover 113 entries across three pages, per-account running balances, source classification, inactive participants, Sydney dates, audit/movement separation, reversed audit exclusion, stale request results, retries, and a new transaction shifting an older page.
- The production site, ledger data, notification settings, and invitations were not changed. No commit or push was made.

## Review

The local server uses `rally_import_review` on port 55439 and API port 18080. The UI is available at http://localhost:5173/money and is left in member preview. Local notifications are disabled. Test suites use only the separate `rally_import_test` database.

Private screenshots and test logs are retained in ignored `tmp/splitwise-import/shared-*` files.

## Interpretation

“All” displays all member-account movements, including inactive participants, without duplicating bank/surplus legs. Imports use explicit top-up titles and incoming payments to the club to identify top-ups. Unclear source titles remain uncategorized. A positive game or food movement is not itself a top-up. Asset snapshot dates are not advanced by purchases or games; stock movement dates are shown separately when different.

## Grouped game and dollar-first card revision

Completed locally on 2026-10-04.

- Full backend suite passed with the race detector against the scratch database. Total statement coverage: 66.7%.
- All 27 frontend test files passed: 183 tests. Coverage: statements 71.46%, branches 69.52%, functions 65.09%, lines 72.66%.
- Production frontend build passed. ESLint reported no errors and the same two existing hook warnings. The existing bundle-size warning remains.
- Regression tests cover gross payer shares, combined extra-hour charges, inactive participants, native comped members and guests, reversals, distinct same-day native sessions, and 53 complete games across a 50-item page boundary. Read-only checks preserve the ledger row count.
- Handler tests verify the activity endpoint uses approved-member access checks and rejects invalid filters. Existing raw history endpoints remain compatible.
- Browser review confirmed the Ledger label, one expandable Game 29/09 row with all seven member shares and balances, collapse behavior, mine/all and top-up filters, and loading 100 of 451 activities.
- The Club assets view highlights all three monetary values. The shuttle count appears beneath its dollar value. All three dates remain visible. Member preview hides purchase forms.
- Screenshots are saved in ignored `tmp/splitwise-import/grouped-ledger.jpg` and `tmp/splitwise-import/dollar-first-assets.jpg`. Test logs use the `grouped-ledger-` prefix in that folder.
- The local application remains available at http://localhost:5173/money. Production and notification settings were not changed. No invitations, commits, or pushes were made.
