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

## Unique names and direct guest editing

- Full backend tests passed with PostgreSQL and the race detector. Coverage meets the 60% floor. Duplicate creates, duplicate renames, case and whitespace variants, OUT guests, and simultaneous duplicate requests are covered. A guest can still attend different sessions under the same name.
- All 413 frontend tests passed across 54 files, including coverage thresholds. The production build, lint, and browser-test type check passed. Existing effect-dependency and bundle-size warnings remain.
- Desktop and mobile Chromium tests open Edit from a guest row, confirm all saved details, change only the payer, reload, and confirm one guest and one occupied place. They change OUT and back IN through the same editor, cancel an unsaved change, and follow the duplicate-name warning to the saved RSVP.
- Browser tests use mocked HTTP data. The database tests separately prove persisted uniqueness and payer reassignment.
- Updated editor screenshots are saved outside the repository under `/tmp/rally-guest-edit-screens/` and attached to the PR.

## Complete guest RSVP removal

- The full backend suite passed with PostgreSQL and the race detector. Removal tests cover admin-only access, IN and OUT removal, one waitlist promotion and notification, concurrent removal, session scoping, and refusal for closed, cancelled, started, settled, or reversed sessions.
- Removal clears the guest from settlement defaults and permits removal of a payer with no remaining obligations. It writes no ledger entries. A separate regression test confirms that deleting an old duplicate preserves the other booking and its payer.
- All 416 frontend tests passed with coverage thresholds. Build, lint, and browser-test type checks passed. Existing effect-dependency and bundle-size warnings remain.
- Desktop and mobile browser tests remove the guest with one click, reload, and add the same name again. The main form clears its selection when the separate editor removes the selected guest. Removal failures retain the guest and display an error.
- Browser tests use mocked HTTP data; service and handler tests verify real database deletion. Removal screenshots remain outside the repository in `/tmp/rally-guest-remove-screens/` and are attached to the PR.

Immediate removal follow-up: 33 affected component/page tests and both desktop/mobile guest browser tests passed after removing the confirmation step. Build, lint, and browser-test type checks passed with the existing warnings. Updated screenshots are under `/tmp/rally-guest-direct-remove-screens/`.
