# Implementation plan

Add approved-member GET routes for position and filtered ledger entries. Preserve existing admin routes for compatibility and all existing write protections. No schema migration or ledger mutation is required.

Project ledger metadata from transactions and optional Splitwise source records. Compute balances partitioned by player account over complete history, then apply scope/type filters and pagination. Use the same complete stable ordering for balances and pages. Classify imported rows from their session flag, category, explicit title, and club payment direction; never equate a positive movement with a top-up.

Derive asset dates from ledger movement dates in Australia/Sydney. For import snapshot transactions, use their source cutoff date, not synthetic import ordering timestamps. The stock audit date uses unreversed opening/import snapshots only.

Create a ledger browser with independent load/error state, default mine scope, top-up switch, and load-more paging. Ignore obsolete request results after filter changes or unmount. Render titles, source badges, type icons, member names, and labelled running balances. Keep the Club assets page identical between roles except admin forms. Use three compact stacked cards on small screens and one row on desktop.

Constitution review: read-only changes preserve integer cents, append-only records and ledger identity. Routes bind to the approved group. No migrations, global frontend store, provider sends, or new dependencies.

Validation: scratch-database service and handler tests for filters, dates, stable paging, balances, and access. Frontend tests for both roles, filters, stale requests, retry, paging, titles and icons. Run backend suite, frontend tests, lint and build. Inspect local UI with imported data in admin and member preview modes.

## Grouped game revision

Add a separate approved-member activity read route while preserving raw ledger entry endpoints. The service reads full member history (already needed for running balances), source session records and native settlements in a consistent, read-only database snapshot. At this club's scale, grouping the few thousand immutable rows in memory keeps the arithmetic and source-specific rules explicit. Build game groups, calculate account balances at each group's last posting, then filter and paginate complete activities. Return either an account entry or a game with its account shares. Do not infer expense shares by negating net payer credits.

Use a native button with aria-expanded/aria-controls for each game. Show total charged on the parent, caller share in mine mode, and a labelled participant table on expansion. Play dates and recorded dates remain distinguishable. Aggregate native guest charges to their host account and show guest names. Preserve zero-charge native members and source participants with zero net movement. Test groups at page boundaries and preserve legacy API behavior.

## Imported history correction

Share the imported-game grouping and schedule-match CTE between history and settlement resolution. Match sessions.session_date to imported played_date only when no native settlement exists, including reversed settlements. Remove matches from the native history branch before pagination. Resolve old schedule IDs to the canonical imported record for the existing breakdown, and reject settlement previews/writes for imported games. The write guard runs inside the session-lock transaction. This requires no data deletion, ledger adjustment or migration.

Use a one-column asset grid below the desktop breakpoint. Reduce mobile padding, icon size and vertical spacing. Restore the existing large card treatment on desktop. Verify the real local UI at narrow mobile widths and desktop after the service tests, frontend tests, lint and build pass.
