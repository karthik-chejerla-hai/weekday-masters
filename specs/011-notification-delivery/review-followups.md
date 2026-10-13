# Review follow-ups for implementation

The full cross-artifact review was read-only and found 12 issues in the earlier backlog. Product decisions and task ordering have since been revised. Do not use old task IDs from that report: the current backlog has 48 renumbered tasks. This document captures remaining implementation prerequisites; it is not a claim that the full handoff has passed a fresh review.

## Resolved product/order decisions

- All existing registrations carry forward regardless of old flags; no legacy opt-out migration or forced re-enrollment.
- Ordinary sign-out preserves registration; account switching retires the former account on the browser.
- Normalize all still-open upcoming deadlines to 48h, including custom ones; closed windows remain closed.
- Reversals generate Session charges corrections and suppress pending original receipts.
- Six releases have one authoritative execution sequence in release-plan.md/tasks.md. Waitlist emission removal/tests ship together in release 2; threshold reporting aligns in release 3 before charge/nudge activation.

## Release 1 prerequisites (resolve before coding this release)

1. Specify exact member-summary and own-history/detail endpoint paths, response fields, status/error codes and approved-route registration in `backend/cmd/server/main.go`. Reuse existing API conventions. Preserve token privacy and caller ownership.
2. Define an accessible history entry/list/detail flow and missing/deleted resource states. The current split page collapses all errors into “not settled”; make error handling appropriate to notification navigation without changing money.
3. Define the additive click contract and legacy payload fallback across backend FCM links, foreground toast, service worker and history. Keep current `notification_type`/data serialization conventions rather than silently renaming existing API fields.
4. Specify allowlisted Auth0 return handling and invitation preservation. Test signed-out, pending/removed, foreign-event and malicious/external URL cases.
5. Session-charge links need to identify the original settlement once reversal support lands in release 5. Keep the release 1 path extensible, but do not implement receipts/reversals or manufacture backward historical charge notifications now.

## Release 2 prerequisites

- Define device identity/ownership uniqueness, version/generation checks and stale enrollment/refresh/revoke ordering. A tombstone alone cannot stop a delayed request after account switching.
- Preserve token rows, including devices whose browsers never revisit. Automatic installation adoption must not request permission, change owner or create duplicates.
- Coordinate the new frontend/device-policy cutover: no obsolete preference flag may block an explicitly active device afterward. Obsolete setup mutations should require a client update rather than revive revocations or delete all devices.
- Test explicit disable, sign-out retention, account switch, token rotation and repeated migration. Add schema through shared `database.Migrate`, not only the CLI entry point.

## Release 3 prerequisites

- Register schema/backfills in `backend/internal/database/db.go` so CLI and the testsupport harness share them. Persist versioned completion records and one captured cutoff for each business migration; repeated deployments must not reset device lifecycle or skip new reminder stages.
- Specify outbox and per-device state enums, aggregate outcomes for partial acceptance and conditional lease fencing. A stale worker must not overwrite a newer lease; ambiguous provider submission is uncertain and not auto-resent.
- Apply Web Push TTL using remaining event lifetime on every attempt. Queue expiry does not expire messages already accepted by FCM. Test exact boundaries and no submission after expiry.
- Define isolated production/PR worker names, database URLs, Scheduler targets and IAM; add worker/Scheduler teardown to `.github/workflows/preview-cleanup.yml`. Never let preview deployment retarget the production scheduler.
- Define which old reminder process is retired when worker execution begins, then keep old timing until release 4; do not run two execution owners.

## Release 4 prerequisites

- Normalize all still-open upcoming windows to 48h using resolved starts; preserve already-closed/cancelled/historical windows. State short-notice creation and later date/start edit behavior explicitly.
- Resolve catch-up, rescheduling and stage-expiry semantics against actual deadlines. Past-due rollout stages are skipped once, not at every deployment. Test DST and exact boundaries.

## Release 5 prerequisites

- Implement correction/event intent in the same transaction as ledger reversal using a transaction-aware LedgerService entry point, preserving append-only money and global account lock order. The current reversal service is not a sufficient atomic notification boundary.
- Suppress pending original receipts and target historical settlement identity after reversal/re-settlement. Do not change the original stored charge/balance snapshots.
- Define stable admin nudge request IDs, same-ID/different-payload conflict, cooldown behavior on retries and no-device/paused outcomes. Add client typing, retained retry identity and truthful feedback.

## Release 6 prerequisites

- Add announcement frontend/client changes in `frontend/src/pages/Admin.tsx` and `frontend/src/services/api.ts`, not just backend fan-out. Retain request ID across retries, reject conflicting reuse and report queued/suppressed/provider-accepted outcomes rather than “sent to all.”
- Save announcement/cancellation audience and outbox work atomically. Finish exact contracts and final four-category integration checks.

## Validation and governance

Each release includes relevant tests, route/security checks, API documentation and migration integration before merge. Update spec-quality checklist accurately as design gaps are resolved; do not blanket-check it to bypass review. No code tests were run during this planning handoff. Changes to constitutional principles require a separately explicit amendment; routine notification documentation must not silently weaken the constitution.
