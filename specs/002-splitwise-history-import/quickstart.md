# Splitwise import rehearsal

## Inputs and boundaries

The command defaults to validating files only. `-apply` enables writes. Local targets must use loopback. Preview targets require `IMPORT_ALLOW_PREVIEW=true`. Production targets require an exact `-confirm-target` value, an encrypted connection, an asset snapshot, and disabled notifications. Invitation sending is outside this import and requires separate user confirmation. Do not use `cmd/seed -from` for this migration: that command creates fixture identities and closing balances only.

Keep the export, email mapping, asset snapshot, logs and reports outside version control. The repository's ignored `tmp/splitwise-import/` folder is suitable. Never point `TEST_DATABASE_URL` at the retained review database; tests truncate their tables.

Mapping JSON:

```json
{
  "members": [
    {"source_name": "Source name", "name": "Member name", "email": "member@example.test"}
  ],
  "admin_email": "member@example.test",
  "session_payers": {"2": "Source name"}
}
```

The source name must match its CSV header exactly. Transaction numbers count non-empty rows excluding the header and totals. Explicit payer entries are required for member-funded sessions because the export only contains net balance changes. Unmapped source names become inactive historical participants and player ledger accounts with no user ID. Non-zero closing balances on inactive participants are refused for review.

## Rehearsal

Set `DATABASE_URL` to a **separate local review database**. Then, from `backend/`:

```sh
go run ./cmd/migrate
go run ./cmd/import-splitwise -from /path/to/export.csv -mapping /path/to/members.json
NOTIFICATIONS_DISABLED=true go run ./cmd/import-splitwise \
  -from /path/to/export.csv -mapping /path/to/members.json -apply
```

The import uses one transaction and a database lock. It refuses a non-empty ledger on first application by default. The explicit `-allow-reversed-topups` option permits reviewed setup top-ups only when every entry has an exact reversing entry. It refuses other transaction types, active balances, incomplete reversals and orphan reversals. Original entries remain unchanged. Repeated identical imports verify every persisted cell, charge and ledger movement and create nothing. Changed source files or mappings are refused instead of appended over existing history.

The app reads imported games through its existing session-history and settlement-breakdown routes. Play dates come from the source titles. Titles with no date use the recorded date and receive a warning in the report. Extra-hour source rows join the game on the same play date. Original recorded dates remain on the ledger. No RSVP or invented court, clock-time, rate or shuttle quantity is created.

## Asset confirmation

Until the actual asset figures are supplied, the UI marks club assets as awaiting review. The source's club-pot changes offset participant changes in surplus, so the ledger identity remains valid without pretending the club pot is all cash. This temporary surplus is not a verified statement of club profit or loss.

Provide all four figures at the export cutoff, as integer cents and units:

```json
{"bank_cents": 40000, "court_credit_cents": 15000, "shuttle_units": 21, "shuttle_cents": 8740}
```

These are format examples, not the club's confirmed amounts. Add `-assets /path/to/assets.json` to the same import command. It posts one new transaction, changes no participant balances and marks assets confirmed. Repeating the same snapshot is safe; changing it is refused. Confirm assets before posting new app transactions. A local test that has changed balances should be repeated in a fresh review database.

## Notifications

The import atomically sets `clubs.notifications_paused=true`. All notifications, including announcements and balance alerts, are discarded while this pause is active. No messages are queued for a later burst. The admin dashboard shows the pause.

Also run the server with `NOTIFICATIONS_DISABLED=true`. This bypasses provider initialization and prevents the scheduler from starting. The two controls are independent. After data review and invitation preparation, clearing the saved pause requires the existing authenticated admin endpoint `PUT /api/admin/club` with `{"notifications_paused":false}`. Removing the environment override requires restarting that server. Neither occurs as part of importing or confirming assets.

## Tests

Use a distinct scratch database:

```sh
TEST_DATABASE_URL=postgres://localhost/rally_import_test?sslmode=disable \
RALLY_SPLITWISE_CSV=/path/to/export.csv \
RALLY_SPLITWISE_MAPPING=/path/to/members.json \
go test -race ./...
```

The private-export test independently rereads the CSV and compares each participant cell against posted ledger entries. Generic tests cover atomic rollback, inactive participants, the paid-versus-charged distinction, notification silence, grouped extra hours, concurrent duplicate attempts, asset confirmation and tampering detection.

The frontend retains the existing 50-row display limit. Pagination remains a separate follow-up. Direct links to older imported session breakdowns work.

## Production execution

The user authorized production preparation and import on 4 October 2026. Invitations remain excluded until the user explicitly confirms them.

1. Identify the database used by the production Cloud Run revision. Compare it with the intended connection before any write.
2. Save a private backup with a PostgreSQL client compatible with the server. Restore it into an isolated local database and rehearse the schema migration and import there.
3. Deploy the reviewed backend and frontend changes. Keep `NOTIFICATIONS_DISABLED=true` on the deployed backend before any imported members become visible. Run `cmd/migrate` explicitly against the identified database before starting the new backend.
4. Run the import from the local machine using the three private input files. The initial production ledger must be empty unless explicitly reviewed setup top-ups have exact reversals and `-allow-reversed-topups` is supplied. Existing users are matched by email and retain their IDs and sign-in identities.
5. Check the report and production UI. Keep both notification controls active. Do not send invites.

With `DATABASE_URL` securely set to the identified production database, run from `backend/`:

```sh
NOTIFICATIONS_DISABLED=true go run ./cmd/import-splitwise \
  -env=production -confirm-target='database-host/database-name' \
  -from /private/path/export.csv \
  -mapping /private/path/members.json \
  -assets /private/path/assets.json -apply
```

Use the exact host (including port when present) and database path from `DATABASE_URL`. Never include its password in `-confirm-target`. The command rejects parameters that could override the selected host or database. It writes the source history, asset confirmation, and notification pause in one database transaction. A failed import rolls back those changes; the preceding schema migration is separate.

The production deploy workflow defaults `NOTIFICATIONS_DISABLED` to `true`. An explicit repository variable of `false` is needed to remove this environment stop later, and the persisted club pause still needs to be cleared separately. Neither step is part of this import.

The 4 October production review found two existing setup top-ups. The owner explicitly approved reversing both and marking the separate test member removed. These corrections were rehearsed on the verified production backup using the existing ledger and member services. They preserve original transactions, existing sign-in identity, and pending join requests. The import uses `-allow-reversed-topups` for this reviewed case.
