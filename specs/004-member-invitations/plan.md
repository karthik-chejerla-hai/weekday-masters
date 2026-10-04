# Plan

Add an invitation service and handler, with the same approved/admin route registration shared by server and handler tests. A small invitation-delivery table records attempts without changing member or ledger records. Register it with the explicit migrator and scratch test reset.

Render a Go HTML template once for preview and send. Reuse the installed SendGrid client, with request timeout and click tracking disabled. Reserve attempts in a database transaction before the external call; persist the result separately. Repeated request IDs return the saved attempt. A per-recipient cooldown prevents double clicks with different request IDs. Network errors remain uncertain; never retry automatically.

Add an invitation review panel to member management, showing recipient, preview, test destination, pause/configuration status, last real send, and actions. Change the misleading Invited tab label to Not signed in. Add a public welcome route using existing Google authentication. Surface sign-in sync failures and allow retry. Identity still comes only from verified Auth0 claims.

No real sends or production deployment occur as part of code implementation. Use the seeded local review app for visual checks and a separate scratch database plus fake mail sender for tests. Actual inbox receipt and a recipient's own Google login require a later live test.

Constitution: service/handler boundary retained, explicit migration only, approved admin routes, Sydney display dates, no money writes or global frontend store, existing scratch harness used.
