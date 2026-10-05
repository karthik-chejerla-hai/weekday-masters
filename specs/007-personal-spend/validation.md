# Validation

Validated locally on 2026-10-05 with Go 1.26.8 and PostgreSQL 16 in a dedicated
scratch container (`rally-spend-test-db`, loopback port 5547). Existing local
app databases were not used by the test harness.

- Full backend `go test -race ./...`: passed.
- Backend coverage with race detection: 68.5%, above the 60% floor.
- Frontend `npm run test:coverage`: 38 files, 246 tests passed. Coverage floors
  passed (76.46% statements, 75.03% branches, 69.36% functions, 78.23% lines).
- `npm run lint`: passed with one pre-existing missing-dependency warning in
  `SessionDetail.tsx`.
- `npm run build`: passed. Existing large-bundle advisory remains.
- `git diff --check`: passed.

Database tests cover gross imported charges when the member paid for the group,
imported play dates versus posting dates, Sydney New Year and DST dates,
future dates, monthly zeros, more than 200 records, guests, comped participants,
reversed and replaced settlements, both reversal paths, and no read-side writes.
HTTP tests verify approved-member access and caller-only scope even when query
parameters try to select another user or all members.

Frontend tests cover direct Analytics URLs, the fourth tab, monthly amounts,
empty history, loading, failed requests and retry, event-driven refresh, and
discarding a response belonging to a previous signed-in member.

Chrome review used synthetic data in a temporary local preview. The desktop
header shows the balance and spend chip at the top right. At 320px wide, both
chips and all four tabs fit; document width equals viewport width. The chip
switches from Balances to Analytics. Temporary viewport settings were reset.

No schema changes or ledger writes are part of this feature. No commit, push,
deployment or production-data change was made.
