# Notification Contracts (proposed)

All member routes require authentication and approved membership. Admin diagnostics/actions additionally require approved-admin status. Preserve existing API envelope conventions; update OpenAPI and singleton frontend client together.

## Push setup

- `GET /api/users/me/push-devices`: own device metadata, active/revoked setup state and safe status; never raw tokens in responses.
- `POST /api/users/me/push-devices/enroll`: explicit enrollment of installation ID, delivery token and device name. Idempotent for same member/installation; explains all four categories in the UI before invocation.
- `PUT /api/users/me/push-devices/:device_id/registration`: refresh address only for the caller’s already-active enrollment. Revoked/absent enrollment returns conflict; cannot silently enroll.
- `DELETE /api/users/me/push-devices/:device_id`: idempotent revoke of caller-owned device only. Other-member IDs do not disclose ownership. Local client coordinates tabs and cleans up the Firebase registration.
- Existing admin member-push endpoint retains metadata and adds derived active_device_count/setup status. Member listing uses a batched summary, not one detail request per row.
- At device-policy cutover, obsolete preference mutations and ambiguous tokenless unregister calls return an update-required conflict without changing registrations. No backward preference adapter is built. Existing tokens are retained and automatically associated with installation metadata by the new client, including tokens whose old preference flags were false. Any subsequent explicit revocation cannot be revived by obsolete refresh calls.

## Event and click payload

Notification response/push data includes notification_id, category, destination_path and relevant typed source IDs. Currency is integer cents in API/history facts; FCM string data serializes integer decimal strings. No tokens, email or mobile contacts in public payloads.

Allowed destinations:

- RSVP reminder: `/sessions/{session_uuid}`.
- Session charge, including balance warning: `/sessions/{session_uuid}/settlement`.
- Manual balance nudge: `/money`.
- Admin announcement: `/notifications/{notification_uuid}` (new detail view of the recipient’s history record).
- Cancellation announcement: `/sessions/{session_uuid}`.

Reject external origins, protocol-relative URLs, malformed identifiers and unexpected paths. Old payloads use a type/source-aware fallback. Foreground/history/service worker/provider link share these rules. Unauthenticated navigation saves an allowlisted local return path and restores it after OAuth, retaining existing `/welcome` invitation behavior. Authorization is rechecked at destination.

## History

Reuse existing paged history/read endpoints; add authorized own-event detail retrieval if needed for `/notifications/:id`. Events exist even when delivery is suppressed for no device or stops. Reading and provider acceptance remain independent. Foreign event IDs cannot expose another member’s charge, balance or announcement history.

## Admin actions

- Existing balance-nudge endpoint adds request_id and returns logical notification_id, balance facts, cooldown and truthful delivery_status (queued/suppressed/accepted/failed/uncertain). Repeated request_id returns original logical result.
- Existing announcement endpoint adds request_id and returns publication ID plus queued/suppressed audience counts. Audience/fan-out and publication are one transaction; retries do not republish.
- Reversals create Session charges correction events atomically with ledger reversal, suppress pending receipts for the reversed settlement and retain historical event facts. Charge links must identify their originating settlement even after re-settlement.
- Existing settlement response continues to represent committed money. Delivery failure cannot change settlement success. Notification intent is committed in the settlement transaction, not initiated by a new client call.
- Threshold reporting returns 2000 cents consistently through existing club/ledger endpoints; no new threshold editor is introduced.

## Private worker

`POST /internal/notifications/run` on a separately deployed private Cloud Run service. Cloud Scheduler identity has only invoke permission on this worker; no public access and no member/admin JWT substitution. Bounded scan/dispatch result reports scanned, claimed, accepted, suppressed, retried, uncertain and failed counts without raw tokens/financial payloads in logs. Repeated/concurrent invocation is safe. Deployment/club stops retain their authority. Explicit diagnostic self-test exception remains limited to the signed-in admin and does not bypass normal delivery policy.
