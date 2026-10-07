# WhatsApp balance alerts

## Accepted scope

Private, one-way utility alerts to 6–8 Australian players. Admins and players use
existing phone editors. Each player explicitly enables WhatsApp for the saved
number. A changed number needs fresh consent. Existing low-balance threshold and
zero define separate crossings; no repeat while a balance stays below its boundary.
A top-up naturally rearms a crossing. A direct fall into debt sends only the debt alert.

Push goes first. An authenticated foreground client or a background worker proves
receipt using a short-lived, notification-specific random capability. Provider
acceptance alone never suppresses WhatsApp. Any device receipt cancels fallback.
Without a usable push device, WhatsApp is due immediately. Otherwise it is due in
15 minutes. A minute worker processes durable pending rows. Recovery, removal,
opt-out, changed number, notification pause and stale alerts prevent sending.
Late push arrivals can still cause duplicates. Receipt does not mean the player read it.

Budget: AUD 5 per Sydney calendar month, at most 200 requests, reserving at least
2 whole cents per request (round up the tax-inclusive utility rate). Failed or
uncertain requests retain their reservation. No automatic provider retries after
a request is claimed. This bounds app requests, not unrelated charges on Meta's
account. Templates and billing currency must be verified before enabling.

## Plan and tasks

1. Add durable fallback and receipt fields through explicit migration.
2. Add opt-in bound to a normalized Australian mobile number.
3. Implement a direct Cloud API template sender, with a bounded timeout.
4. Serialize budget reservation and dispatch claims across server instances.
5. Add web receipt handlers and fallback cancellation.
6. Use locked settlement balance snapshots to identify threshold crossings.
7. Test delivery, concurrency, budget, permission and worker behavior.
8. Document Meta setup, approved templates and deployment configuration.

No production activation or real message sending is part of development checks.

## Validation completed

- Full Go suite with a disposable PostgreSQL database; race detector passed.
- Final service/handler race checks include scheduler authentication, receipt expiry,
  recovered-then-low balances and the 200-message cap.
- Full backend coverage: 72.8%, above the 60% floor.
- Frontend: 47 test files, 306 tests passed; configured coverage floors passed.
- Frontend production build and lint passed. One pre-existing React hook warning
  remains in `SessionDetail.tsx`; Vite also reports the existing large bundle warning.
- OpenAPI and deployment YAML parsed successfully; Git whitespace checks passed.
- Real Meta delivery and real-device receipt checks remain activation prerequisites.
