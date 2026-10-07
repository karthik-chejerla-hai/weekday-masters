# WhatsApp setup and operation

## Meta account setup

The phone app alone cannot send these automatic alerts. Set up a WhatsApp
Business Platform account and a Cloud API business phone number. Confirm whether
Meta's supported onboarding can retain your existing phone app number; otherwise
use a separate sender. Do not disconnect your current number until this is checked.
Use Meta's test number and permitted test recipients before production activation.

Create a system-user access token with `whatsapp_business_messaging`. Keep it in
GitHub secrets or the runtime secret manager, never in the frontend or repository.
Set the phone number ID (not the phone number), an active Graph API version, and
the exact approved template language. Confirm the account bills in AUD.

Request UTILITY approval for two templates. Each has one body parameter, the
signed AUD balance, and no promotional content. Proposed wording:

- `rally_balance_low`: Your Rally club balance is {{1}}. It is below the club's low-balance limit. Please check your account for top-up details.
- `rally_balance_negative`: Your Rally club balance is {{1}}. Your account is in debt. Please check your account for top-up details.

A static button may open the production `/money` page. Do not add dynamic button
parameters without updating the sender. Match `WHATSAPP_LANGUAGE` to the approved
language code. Recheck the template category in Meta before turning sending on.
The sender has no provider subscription. Provider acceptance is recorded as
`accepted`, not `delivered`; no inbound chat or delivery webhook is required here.

## Runtime configuration

See `backend/.env.example`. All fields must be valid and `WHATSAPP_ENABLED=true`
before the sender initializes. `NOTIFICATIONS_DISABLED` and the database club
pause remain authoritative. Preview environments must keep notifications disabled.
The existing explicit migrator adds the new notification/preference columns;
run it before the new server starts. There are no new tables or background migrations.

GitHub deployment variables: `WHATSAPP_ENABLED`, `WHATSAPP_PHONE_NUMBER_ID`,
`WHATSAPP_GRAPH_VERSION`, `WHATSAPP_LOW_TEMPLATE`, `WHATSAPP_NEGATIVE_TEMPLATE`,
`WHATSAPP_LANGUAGE`, `WHATSAPP_RESERVED_CENTS`.
GitHub secrets: `WHATSAPP_ACCESS_TOKEN`, `NOTIFICATION_WORKER_TOKEN`.
The deployment workflow passes these only to the backend.

Players save an Australian mobile number in Profile and explicitly enable
WhatsApp balance alerts there. Admins can edit numbers in Members. A number edit
revokes consent. The admin cannot silently consent for another player. Existing
accounts default to WhatsApp off. A settings change only affects new alerts;
turning WhatsApp off also prevents pending messages when the worker checks them.

## Reliable scheduling on Cloud Run

In-process cron checks pending messages each minute while the process has CPU.
Cloud Run may suspend CPU or scale to zero. Configure one external scheduler to
POST `/api/internal/whatsapp/dispatch` every minute, with a 60-second request timeout
and `X-Notification-Worker-Token` containing the dedicated random secret. An empty
or shorter-than-32-character configured secret disables the endpoint. Store the
scheduler secret securely; never put it in a URL. This endpoint permits only
pending dispatch, not arbitrary recipients or content. Multiple invocations are safe.
Cloud Scheduler is one option; check the project's free-job allowance and billing.
No scheduler, runtime secret, deployment or real message was created by this change.

A due message normally runs at the next minute check, so the wait is 15–16 minutes.
The queue survives restarts. Alerts older than 24 hours are cancelled. Pauses leave
pending alerts queued until processing resumes or they expire. Budget-blocked alerts
are not carried into the next month. Late pushes or lost receipt requests can still
result in a message on both channels. A receipt means device processing, not reading.

## A$5 budget

The hard app limits are A$5 and 200 provider requests per Australia/Sydney calendar
month. `WHATSAPP_RESERVED_CENTS` reserves a conservative whole-cent amount for each
attempt, minimum 2. With the estimated A$0.0148 rate plus GST, 2 cents leaves margin;
verify the current AUD rate card before enabling. Increase the reserve if rates rise.
Never reduce it below the rounded-up tax-inclusive utility price. A 2-cent reserve
and 200-message limit reserve at most A$4 per month. Higher reserves reach A$5 sooner.

The budget uses database locking across replicas. Failed, crashed and uncertain
requests consume their reservation. They are not retried automatically because the
provider may have accepted a request before the connection failed. Push continues
when WhatsApp reaches its limit. Other senders, provider subscriptions, foreign
currency billing, scheduler charges, or a template reclassified by Meta are outside
this estimate. This is an app-side spending bound under the configured rate, not a
Meta-account billing cap. Keep these templates utility-only and verify their rates.

Notification history exposes `whatsapp_status` and `push_received_at`. Phone numbers,
provider IDs and receipt hashes are excluded. Pending sends become `cancelled`,
`budget_blocked`, `claimed`, `accepted`, or `uncertain`. A lingering `claimed` row means
processing stopped after budget reservation; inspect it before any manual intervention.
Never reset a claimed/uncertain record just to retry it without checking Meta first.

## Verification

Use a disposable PostgreSQL database for `go test -race ./...`. The tests cover
push acceptance versus receipt, invalid/expired capabilities, duplicate workers,
monthly reservation limits, opt-out, number edits, removed players, stale messages,
balance recovery and threshold crossings. Run `npm run test:coverage`, `npm run lint`
and `npm run build`. Browser-worker tests simulate cold starts and failed receipts.
Before production activation, test a real registered device with the app both open
and closed, a missing device, the 15-minute fallback, and a recovered balance using
Meta test recipients. Local tests never send real WhatsApp messages.
