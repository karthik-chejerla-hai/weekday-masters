# Notification Contracts (proposed)

All member setup/history routes require authentication and approved membership. The revoke-only proof route is an authenticated exception described below. Admin diagnostics/actions additionally require approved-admin status. Preserve existing API envelope conventions; update OpenAPI and singleton frontend client together.

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

## Release 2 concrete enrollment protocol

Existing `user_push_tokens` rows retain their ID, token and owner. Shared `database.Migrate` adds nullable `installation_id` (unique), `generation` (positive int64, default 1), hidden SHA-256 `revocation_hash`, `revoked_at` and `invalidated_at`. No opt-out migration or business backfill runs. Repeated migrations preserve lifecycle state. Active send/status counts exclude revoked and invalidated rows; old flags are inert.

`GET /api/users/me/push-devices` returns an array of `{id, installation_id, generation, device_name, state}`. State is `active`, `revoked` or `invalidated`; legacy installations are null. Tokens and revocation proofs are never returned.

`POST /api/users/me/push-devices/enroll` accepts `{installation_id: UUID, token: string, revocation_secret: 64 hex characters, generation: integer, device_name?: string}`. Generation zero means a new installation. Existing installations require the caller owner, device proof and current generation before any state change. Explicit enrollment alone can revive revocation; token rotations/re-enrollment increment the counter. Identical active-address retries do not change state. Globally unique tokens/installations cannot be reassigned to another account.

`POST /api/users/me/push-devices/adopt` uses the same input, automatically adding metadata only to an existing same-owner active legacy token. It never creates an absent enrollment or revives a tombstone. A browser with existing permission can adopt without a permission prompt. Unvisited legacy tokens remain deliverable.

`PUT /api/users/me/push-devices/:id/registration` accepts `{installation_id, token, generation}`. The caller must own the row/installation, revocation must be absent, and token changes require the current generation. Invalidated addresses may be refreshed; explicit revocation cannot. Identical active-address retry is idempotent. `DELETE /api/users/me/push-devices/:id` accepts `{generation}` and revokes only the caller’s installation; the counter advances, while an already-revoked retry succeeds without further mutation.

`POST /api/push-devices/revoke-proof` accepts `{installation_id, revocation_secret, generation}` and returns 204 only on valid proof or its already-revoked retry. Authentication is required; approval is deliberately not required so a pending/removed new account can retire the former browser enrollment after its JWT is unavailable. This endpoint can only revoke; it reads/returns no ownership or registration data and cannot enroll, refresh or transfer a device. Incorrect proof and unknown IDs both return 404. Stale generations return 409; retry may not revoke a later re-enrollment.

Other-member/missing owned IDs return 404. Invalid request shape returns 400; stale generation/state/proof on enrollment returns 409; unapproved setup/history returns 403; backend failures return 500. Old preference PUT and push-token POST/DELETE return `409 {code: "update_required", message: "Refresh Rally to manage notifications on this device."}` without mutation; old reads remain available as inert compatibility data.

The browser persists a random installation ID and 256-bit revoke secret, serializes operations with Web Locks where supported, and listens for cross-tab storage changes. Ordinary logout preserves the record. Account switching first revokes the former enrollment using the device proof, then deletes the Firebase address and generates a new installation/proof. A new account must explicitly enable; old rows are not transferred. Failed cleanup retains retry state and blocks sign-in completion rather than reporting success. Old tabs cannot automatically reclaim a different account’s installation. Explicit disable persists independently of browser permission and cannot be undone by startup restoration or old clients.

### Release 2 reviewed recovery behavior

Before any enrollment/adoption request, persist that an attempt may have reached the server. Before any cleanup mutation, persist a unique cleanup-intent ID. A lost enrollment/adoption response must not make the browser assume that no registration exists. Cleanup uses its installation proof even when the local device ID is unknown.

The proof revoke request permits generation zero as a recovery probe. Only after validating the 256-bit proof, a stale counter returns `409 {code: "device_generation_changed", details: {current_generation: integer}}`. Wrong proof/unknown installation returns 404 without details. Current-generation recovery retries are bounded to three calls per cleanup invocation and require the same persisted explicit cleanup intent throughout. A newer action that clears/replaces the intent fences delayed responses; it cannot automatically revoke a later enrollment. Retain identity and cleanup intent on failure, and replace browser ownership only after server retirement plus Firebase cleanup succeed.

`POST /api/push-devices/legacy-revoke` is an authenticated revoke-only exception for the first post-rollout login/account switch. It accepts the browser's existing FCM `{token}` as proof of address possession, and can mutate only a row whose `installation_id IS NULL`. Same-owner active legacy rows remain untouched for automatic adoption. Foreign legacy rows become revoked with their original ID, token and owner retained; retries are idempotent. Adopted v2 rows are never changed by this endpoint. Its sole response field is `{cleanup_required: boolean}`; it returns no identity, registration metadata or addresses. A true result requires Firebase cleanup before sign-in completes. Persist the legacy token cleanup intent before the request so an uncertain/lost response can be retried. Pending/removed new accounts may call this route to prevent former-account alerts on their browser; approval remains required for enrollment, refresh and device reads.
