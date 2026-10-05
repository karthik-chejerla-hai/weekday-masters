# Session cancellation (Issue 45)

Admins can cancel one upcoming session with an optional reason. A cancellation retains the session and RSVPs, closes RSVP and settlement actions, and creates one announcement and one notification-history record for every approved member, including members with no RSVP. Pending, rejected and removed members are excluded.

The announcement contains the cancelled session date in Australia/Sydney, the reason (or "No reason provided."), and the earliest non-cancelled scheduled session after that session (or "No next session is scheduled."). Closed RSVP sessions still count as scheduled. Other occurrences of a recurring session remain unchanged.

Clarifications: Use the existing app, email and push channels. Email and push follow announcement preferences and both notification stops. In-app records remain visible when outbound delivery is disabled. No external group integration is added. Finished or settled sessions cannot be cancelled. Repeated requests return the original cancellation without changing its reason or creating another announcement. Provider delivery remains best effort, consistent with existing announcements.

The admin reviews the date and optional reason before confirming. Errors remain visible and preserve the reason for retry. Cancellation remains visible after reload. Non-admin cancellation requests must fail without changing data.
