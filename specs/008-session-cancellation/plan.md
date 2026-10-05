# Implementation plan

1. Reuse the session status, cancellation reason, announcement and notification models. No schema change is required.
2. Lock the session row and atomically persist the cancellation, announcement and per-member history. Deliver the committed notifications through the existing service after commit. Prevent update/delete shortcuts from silently cancelling or reopening a session.
3. Retain the current admin cancellation flow. Improve confirmation, error reporting and cancelled-session visibility. Show the reason on session details.
4. Test authorization, audience, date selection, optional reasons, duplicate and concurrent requests, rollback, outbound stops and preferences, and the UI success/error paths. Run backend race tests against a disposable database plus frontend coverage, lint and build.
