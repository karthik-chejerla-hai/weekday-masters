# Validation, 4 October 2026

## Automated checks

- Full backend suite passed with the race detector against the separate scratch database. Aggregate statement coverage: 66.4%.
- Frontend: 196 tests passed in 30 files. Statement coverage: 72.24%; line coverage: 73.36%.
- Frontend production build passed. The existing large-bundle warning remains.
- ESLint passed with no errors. The existing `loadSession` dependency warning in `SessionDetail.tsx` remains.

Invitation tests cover admin access, both notification stops, the separate admin-test opt-in, fixed test recipient, stale previews, duplicate requests, concurrent requests, cooldown, provider failures, unknown results, HTML escaping, and the SendGrid request contract. First-sign-in tests prove that claiming a member preserves the member ID, balance, ledger records, and existing IN RSVP. Wrong and unverified email tests prevent an incorrect claim.

## Browser checks

The local review database was migrated explicitly. The member review panel shows the recipient, sender, subject, desktop/mobile HTML previews, separate test controls, paused member-send controls, and stored attempt status.

The email's sign-in link opens `/welcome`. Google's account chooser and sign-in with the owner's existing personal account passed. The app returns to `/welcome` and shows the signed-in account. This check found and fixed a router synchronization issue after the Auth0 callback. Other members' Google accounts were not accessed.

Review screenshot: `tmp/splitwise-import/invitations-panel.jpg` (local, ignored).

## Live email limit

A local test attempt to the owner's admin address failed. Read-only checks confirmed a valid SendGrid API key with `mail.send` permission and a verified configured sender. A non-delivering SendGrid sandbox validation returned HTTP 401 with `Maximum credits exceeded`. The app now reports this limit explicitly for new attempts.

Actual inbox delivery and rendering remain unverified. Restore the SendGrid sending allowance before another inbox test. No plan or billing changes were made. Provider evidence and test logs remain in the ignored `tmp/splitwise-import/` directory.

## Release state

This feature is available locally only. No commit, push, production migration, or deployment was performed. Member invitations and routine notifications remain paused. Explicit test copies are enabled only in the local review server. No member invitations were sent.
