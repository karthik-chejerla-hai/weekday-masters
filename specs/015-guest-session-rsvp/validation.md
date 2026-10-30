# Validation

Validated on 2026-10-31 against a separate local PostgreSQL 16 database.

- Backend: `go test -race -coverprofile=... -covermode=atomic ./...` passed, including database-backed tests. Statement coverage: 72.8%, above the 60% floor.
- Frontend: all 409 unit/component tests passed. Coverage thresholds passed (78.19% statements, 77.41% branches, 71.77% functions, 81.05% lines).
- After updating admin session counts and form styling, all 38 affected component tests passed.
- Production frontend build passed. Lint passed with the existing SessionDetail effect dependency warning. The build retains its existing large-bundle warning.
- Browser tests passed on desktop Chromium and mobile Chromium. They cover creating a guest, reading the saved guest after reload, editing to OUT, and checking for horizontal overflow.
- Browser tests use mocked Auth0 and HTTP data. Backend integration tests separately exercise real PostgreSQL writes, access checks, and concurrent capacity claims.
- Confirmed guests appear in member-visible lists and counts. Payer RSVPs remain independent. Setting a guest OUT promotes the first waiting member.
- Guest saves do not write ledger entries. Advanced settlement defaults retain guest names and paying members. Standard expenses cannot silently omit confirmed guests.
- Screenshots remain outside the repository under `/tmp/rally-guest-rsvp-screens/`.

## Replacement payer before member removal

- The full backend suite passed again with PostgreSQL and the race detector after the removal guard and lock order change.
- Regression tests cover two guests sharing a payer, refusal without changing membership or RSVP, reassignment of every guest, and settlement charging only the replacement payer.
- Upcoming, past unsettled, and reversed sessions block removal. OUT guests, cancelled sessions, and settled history with a cleared balance do not block removal.
- Concurrent guest creation or reassignment and payer removal cannot leave an IN guest assigned to a removed member. Existing removal and waitlist promotion tests also pass.
- All 15 AdminMembers component tests passed, including the replacement payer error and retention of the member in the active list.
