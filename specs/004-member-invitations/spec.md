# Member invitation emails

Created 2026-10-04. Work remains on the existing branch.

An admin can review an invitation for an approved member who has not signed in, send a test copy to their own signed-in address, and send or resend the invitation to that member. Adding a member still sends nothing automatically.

The preview and sent email use the same escaped HTML and plain-text content. They identify the recipient's saved email and explain that Google sign-in retains their membership, balance, history, and recorded RSVPs. A public welcome page supplies the sign-in action and help for a different signed-in Google account. The link contains no email address, authentication token, or grant of access.

Member invitation sends obey both existing notification stops. Tests use a separate, default-off `INVITATION_TEST_EMAILS_ENABLED` control and can send only to the authenticated admin's saved email. Preview works without an email provider. No member invitations or real test messages are sent during implementation without the owner's instruction.

Keep a durable record of each attempt, its recipient, actor, test flag, and result. Provider acceptance is not proof of inbox delivery. Prevent duplicate submission of the same request and closely spaced sends. An uncertain network result must not appear as a successful send. Tests do not mark the member invited.

Acceptance: admin-only endpoints, member and test recipient restrictions, paused sends, truthful errors/status, HTML escaping, valid application links, repeat sends, double submission, and first-sign-in retention of member ID, balance, ledger and RSVP. Preview and welcome page work on narrow screens. Existing app flows continue to pass their tests.
