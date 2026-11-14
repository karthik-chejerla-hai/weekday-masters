# Manual RSVP reminders (#114)

Approved admins can choose **Remind players to RSVP** on upcoming session cards
on the dashboard and sessions page. A confirmation shows the current count of
approved members with no RSVP; the send rechecks that audience under the session
lock. All stored responses (IN, OUT, maybe and waitlisted) exclude the member.
Admins with no response are included.

Reminders require an open session whose start and stored RSVP deadline are both
in the future. One manual batch with a nonempty audience starts a shared one-hour
cooldown for that session, across all admins. Automatic reminder stages are
independent. An empty audience creates no notifications and consumes no cooldown.
A nonempty batch consumes the cooldown even if all pushes are suppressed.

The action saves a batch, member history and durable outbox intent atomically.
The existing private notification worker delivers pushes; the API does not call
the provider. Deployment/club stops and device enrollment rules apply. The result
reports publication-time queued and suppressed counts, not delivery or receipt.
Provider TTL is capped at FCM's 28-day maximum without shortening the stored event
lifetime. Eligibility is rechecked by the worker, including responses, membership,
start, deadline and cancellation. Manual reminders use the standard RSVP message and
session navigation. No automatic email is sent.

## API

Both endpoints require approved admin authorization:

- `GET /api/admin/sessions/:id/rsvp-reminder` returns `audience_count` and nullable
  `next_allowed_at`. It creates no notification or batch.
- `POST /api/admin/sessions/:id/rsvp-reminder` accepts `{ "request_id": "<uuid>" }`
  and returns the batch with `audience_count`, `queued_count`, `suppressed_count`
  and nullable `next_allowed_at`. Retrying the same request returns its immutable
  result without creating new notifications, including after the deadline or session
  deletion. Authorization and original actor/session ownership still apply.

A reused request ID for another actor or session returns 409. Ineligible sessions
return 409; a missing session returns 404; cooldown returns 429 with
`next_allowed_at`. Retry an uncertain network result with the original request ID.
After an uncertain send, the UI keeps **Retry confirmation** available even if the
deadline has since passed;
new submissions remain disabled. The dashboard preserves a pending confirmation
when the session starts and the card switches to score entry.

## Schema and verification

The existing explicit migration command adds `rsvp_reminder_batches`. It is an
additive audit/idempotency table, with no member or device backfill. Batch records
retain their identifiers if a session is subsequently deleted. Apply migrations
before deploying the API, as usual. No extra scheduler configuration is needed.

Backend tests use a disposable PostgreSQL database and synthetic devices/provider;
frontend tests cover confirmation, cancellation, retry IDs, suppression feedback,
admin visibility and disabled states. Post-deadline player RSVP changes belong to
#115 and are outside this feature.
