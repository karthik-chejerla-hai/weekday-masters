# Invitation review and testing

Apply the explicit migration before running the updated server. It adds `invitation_deliveries`; it does not change the member or ledger tables.

Open **Manage members > Not signed in > Send invitation**. This opens a review panel and sends nothing. Check the recipient, sender, subject and HTML preview. The desktop/mobile controls change the preview width. **Open sign-in link** opens `/welcome` using the server's `FRONTEND_URL`.

The test button sends the same HTML and plain text with a `[TEST]` subject prefix. Its destination comes from the authenticated admin record, never the request body. It does not sign in as the selected member and does not mark their invitation sent. The email explicitly names the member's saved Google address.

## Configuration

- `SENDGRID_API_KEY` needs `mail.send`; the sender must be verified and the account must have sending allowance.
- `SENDGRID_FROM_EMAIL` and `SENDGRID_FROM_NAME` identify the sender shown in preview and inbox.
- `FRONTEND_URL` must be the actual frontend address, HTTPS except loopback local development. A local test link works only on the machine running the local app.
- `INVITATION_TEST_EMAILS_ENABLED` defaults to `false`. Setting it to `true` permits explicit admin test copies even with routine notifications paused. It starts no scheduler and sends nothing automatically.
- Member invitations remain blocked by either `NOTIFICATIONS_DISABLED=true` or `clubs.notifications_paused=true`. Do not clear these controls until the owner authorizes member invitations and normal notifications.
- GitHub deployment keeps both defaults: notifications disabled, invitation test sending disabled unless the repository variable is explicitly true.

## Results and retries

The provider's HTTP 202 means accepted for delivery, not confirmed receipt. Check the inbox and spam folder to verify appearance and receipt. Failed and unknown results are displayed separately. A lost browser response can be checked with the same request ID without another send. A provider timeout may have sent the email, so check the inbox before an explicit resend. Attempts are not retried automatically.

SendGrid's sandbox validates payloads without delivery or credit consumption. It does not prove inbox delivery. References: [Mail Send](https://www.twilio.com/docs/sendgrid/api-reference/mail-send/mail-send), [Sandbox Mode](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/sandbox-mode).

## Local review on 4 October 2026

The local app uses the separate review database and keeps routine notifications/member sends paused. Admin test copies are enabled locally. A test attempt to the owner's admin address was rejected. Read-only provider checks confirmed a valid key with mail-send permission and a verified sender. A sandbox validation returned HTTP 401 with `Maximum credits exceeded`. Restore the SendGrid allowance before another inbox test. No provider plan or billing change was made.

Google sign-in was exercised in the browser with the owner's existing personal account. Account selection and return to `/welcome` passed after fixing router synchronization. Automated scratch-database tests cover first-time claims, wrong/unverified emails, and preservation of the invited member ID, balance, ledger rows, and saved IN RSVP. Other members' Google accounts were not accessed.
