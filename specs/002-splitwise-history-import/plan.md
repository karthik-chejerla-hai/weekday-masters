# Implementation plan

Use separate import tables to retain source rows, participant identities and net balance movements. Imported sessions are source records exposed through the existing history and breakdown APIs. They do not need Session scheduling fields or RSVP rows. Play dates come from titles, with the closest year to the recorded date when omitted. Date-less titles use a labelled recorded-date fallback. Original source dates remain on the ledger. Extra-hour rows on one play date form a single session.

Mapped participants use normal claimable users and player accounts. Unmapped participants use player ledger accounts with no user, linked to the historical participant only. The current export's inactive closing balances must be zero; refuse otherwise pending an explicit handling decision.

Post each source row through LedgerService.PostWithin in one transaction, after taking a PostgreSQL transaction advisory lock. The club mirror movement is posted to surplus while the asset position is unknown. This preserves the exact member balances and ledger invariant without claiming unknown cash, court credit or shuttle quantities. Mark assets pending prominently. A later verified asset snapshot adds the actual assets and matching surplus, without changing member history.

Store a digest of the source and reviewed mapping. Reject changed inputs or a non-empty ledger on first import. Repeated identical runs verify persisted source rows, charges, identities and ledger entries rather than silently skipping. A member-funded game needs a reviewed payer allocation; never infer this from net values alone.

Pause notification delivery on the club row atomically with import. Add an environment-level kill switch that also avoids provider initialization and cron startup. Import never constructs a notifier. Paused sends are discarded, not queued for replay.

Add generic parser and database-backed tests, including rollback, tampering, re-run and concurrency. Run an opt-in test with the private export and private mapping, plus an independent reconciliation against a retained local rehearsal database. Keep private inputs and outputs under the ignored tmp directory.
