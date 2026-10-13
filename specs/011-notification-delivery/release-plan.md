# Notification Release Plan

Updated 2026-10-13. Six forward-only releases; existing registrations remain intact. No dates are committed.

Confirmed: old master/category opt-outs are ignored; ordinary sign-out preserves registration; all still-open upcoming deadlines normalize to 48h; reversals send Session charges corrections and suppress pending original receipts.

## Release 1 — Accurate status & working links

Know who has push set up, and open the right page from an alert.

**What goes out**

- Member rows show Push set up or No push devices, based on registered devices, with an accurate count.
- Notification clicks work in foreground, background and in-app history.
- Protected links survive sign-in. Session-charge links open See the split; announcements open their exact message.

**Delivery work**

- Batch member registration summaries; keep raw tokens private.
- Add notification history/list and detail navigation, consistent destination routing and safe OAuth return paths.
- Prepare the split destination for charge receipts introduced in release 5. Existing delivery policy remains until release 2.

**Acceptance checks**

- No devices never shows push setup, even when old flags are true.
- Every click surface reaches the intended authorized destination; invitation sign-in still works.

**Dependency**: Starting release.

## Release 2 — Simple device setup

One device setup control. Existing devices keep working.

**What goes out**

- Keep every existing token and its owner. No permission prompt, opt-out migration or re-registration campaign.
- Remove account-wide and category switches. Existing registrations receive the supported alert types as each trigger ships.
- Enable or disable this device without affecting another. Signing out keeps registration; switching accounts retires the previous account on this browser.
- Stop new waitlist notifications while preserving waitlist promotion and past notification history.

**Delivery work**

- Add installation identity automatically when an existing registered browser returns; unvisited devices remain deliverable.
- Distinguish explicit enrollment from refresh; persist revocation and reject stale refresh/account-switch requests.
- Switch delivery policy to device registrations and retire preference mutations in one coordinated backend/frontend cutover.

**Acceptance checks**

- Before/after migration tokens and ownership match, including accounts with old flags off.
- Disabled device stays disabled across reload, tabs and sign-in; another device still works.
- Ordinary sign-out preserves registration; waitlist promotion still works without alerts.

**Dependency**: After release 1.

## Release 3 — Reliable delivery

Saved work survives restarts, with honest delivery outcomes.

**What goes out**

- Existing alerts use a saved delivery queue and per-device outcomes.
- Failed devices receive bounded retries; accepted devices are not intentionally resent.
- Admins see queued, suppressed, failed, uncertain or provider-accepted outcomes. Acceptance never claims the member saw the message.

**Delivery work**

- Add transactional outbox, migration completion records and per-device attempts through database.Migrate, including the test harness.
- Run a private scheduled worker every minute; isolate production and preview databases/services and clean up PR workers.
- Define lease fencing and aggregate outcomes, expiry/FCM TTL, notification stops and independent backlog/failure monitoring.
- Retain existing reminder timing until release 4; replace its execution owner without running two schedulers.

**Acceptance checks**

- Concurrent workers/restarts do not duplicate logical events.
- Partial success retries only failed devices; ambiguous sends stay uncertain.
- Repeat migrations do not reset registration state or replay work; delivery stops remain effective.

**Dependency**: After release 2; prerequisite for releases 4–6.

## Release 4 — RSVP reminders & 48-hour deadline

Two reminders for members who have not responded, before RSVP closes.

**What goes out**

- Send a first reminder 96 hours before play and a second 60 hours before play, only when no RSVP exists.
- IN, OUT and waitlisted responses all stop reminders; clicks open the session.
- Set new deadlines to 48 hours before play. Update all still-open upcoming windows, including custom deadlines.
- Keep already-closed windows closed. Retire the old 24/12-hour attendance reminders and 6-hour deadline alerts.

**Delivery work**

- Use resolved session start and elapsed-hour arithmetic across Sydney daylight saving.
- Recheck response, membership, cancellation and deadline before submission; expire stale work at the provider too.
- Save a one-time rollout cutoff; skip overdue stages at rollout and send only the latest useful stage after an outage.

**Acceptance checks**

- Exact 96/60/48-hour boundaries and DST pass controlled-clock tests.
- Responded, removed, cancelled and closed cases produce no new reminder.
- Old and new schedules do not overlap; migration never reopens a closed window.

**Dependency**: After release 3.

## Release 5 — Session charges & balance nudges

One clear receipt after a split, with any balance warning included.

**What goes out**

- Each included member gets one Session charges notification with total charge and balance after that split.
- Include a negative warning below $0, or low-balance warning from $0 to below $20. Exactly $20 gets no warning.
- Open See the split directly, identifying the original settlement when corrections exist.
- Manual nudges use the same balance wording and $20 threshold, keeping the 24-hour manual cooldown.
- A reversed split sends a correction under Session charges; suppress pending original receipts. Re-settlement produces a new distinct receipt.

**Delivery work**

- Save receipts and immutable balance snapshots in the settlement transaction; aggregate regular/extra-hour and hosted-guest charges.
- Save reversal corrections atomically with ledger reversal and invalidate pending original work without altering history.
- Add stable admin request IDs and truthful nudge feedback; align threshold reporting before activation.

**Acceptance checks**

- Amounts match committed ledger facts; failed/duplicate confirmations create no extra receipt.
- Test negative, $0, $19.99 and $20 boundaries, guests, zero-cost receipts and delayed delivery.
- Reversal/re-settlement and concurrent manual nudges are covered; provider failure never rolls back money.

**Dependency**: After releases 1 and 3; may follow release 4.

## Release 6 — Reliable announcements & final cleanup

Publish a club message once and deliver it through the shared system.

**What goes out**

- Ad hoc announcements target the approved publication audience and open the exact message.
- Session cancellations remain announcements and open the cancelled session.
- Show queued/suppressed publication outcomes instead of claiming everyone received it.
- Finish retiring old category/settings UI and document the final four-type policy.

**Delivery work**

- Save announcement, audience, recipient events and delivery work together; do the same for existing cancellation events.
- Add frontend request IDs that survive retries and reject reused IDs with conflicting content.
- Finish API contracts, isolated preview checks, release validation and documentation.

**Acceptance checks**

- Retried or concurrent publication creates one logical publication and audience set.
- Cancellation and fan-out survive a process restart; removed members are suppressed at send.
- All four categories and existing registrations pass final end-to-end checks.

**Dependency**: After releases 1 and 3; final integration after release 5.

## Readiness

Product decisions are confirmed. Detailed implementation handoff still needs the outstanding technical review corrections; this plan does not claim releases are implemented. Each release requires its own tests/build/lint, repeat-safe migrations, token-preservation check and isolated preview rehearsal.
