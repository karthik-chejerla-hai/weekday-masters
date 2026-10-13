# Research and Decisions

Date: 2026-10-13. Repository inspection only; production infrastructure and actual member device state were not queried.

## Reuse existing services, separate facts from delivery

**Decision**: Keep existing Notification, Announcement and service boundaries; add durable work and attempt records.
**Rationale**: `notification_service.go` already stores history, has stop policy and removes unregistered FCM tokens. `settlement_service.go` already sends low-balance alerts, but after commit without durable notification intent. Its current alert lacks a session destination and is only generated below the threshold. Expense confirmation delegates to the same settlement service.
**Alternatives considered**: Full rewrite, external event bus, synchronous provider sends inside money transactions. None is necessary for this club.

## Registration is evidence of setup, not proof of arrival

**Decision**: Member summary uses active device count. Local browser setup also checks permission. Delivery records say provider accepted, failed, suppressed or uncertain.
**Rationale**: FCM registrations can become stale; the server cannot infer current browser permission merely from a saved token. Existing code already removes `IsRegistrationTokenNotRegistered` failures.
**Primary reference**: [Firebase registration management](https://firebase.google.com/docs/cloud-messaging/manage-tokens).
**Alternatives considered**: Use default-true preference as readiness, label any token as confirmed delivery. Both mislead.

## Device removal must be durable and scoped

**Decision**: Stable enrollment identity, scoped revocation, separate explicit enrollment from automatic refresh.
**Rationale**: `frontend/src/services/notifications.ts` disables without supplying a token; backend `UnregisterPushToken` then removes all user tokens. `PushNotifications.tsx` and `NotificationSettings.tsx` automatically re-register a permitted browser. Removing the account flag without replacing this lifecycle would undo disable.
**Alternatives considered**: Delete token only, keep a renamed account master flag. Neither meets per-device controls and persistent disable.

## External scheduled invocation and PostgreSQL outbox

**Decision**: Cloud Scheduler every minute invokes a private worker service using OIDC. Worker evaluates reminders and drains PostgreSQL rows.
**Rationale**: Current hourly cron runs inside each API process; deploy code does not establish background execution guarantees. Cloud Scheduler supports authenticated Cloud Run invocation and at-least-once execution, requiring idempotent claims. Private service avoids relying on Cloud Run IAM to protect only one route on the existing publicly invokable API.
**Primary references**: [Scheduled Cloud Run services](https://docs.cloud.google.com/run/docs/triggering/using-scheduler), [Cloud Scheduler execution semantics](https://docs.cloud.google.com/scheduler/docs/overview).
**Alternatives considered**: Always-running process cron, one Cloud Task per member/stage, another message broker. A database scanner handles session edits and member eligibility without coordinating individual remote task reschedules.

## Consistent click contract and sign-in return

**Decision**: Store an allowlisted destination and event ID, reuse it for every click surface; add safe Auth0 return restoration.
**Rationale**: Existing foreground/background code routes generic session IDs to `/sessions/:id`; existing split page is `/sessions/:id/settlement`. Backend FCM links also construct generic destinations. `App.tsx` redirects unauthenticated visitors without a destination; login/redirect handling currently specializes in `/welcome`.
**Primary reference**: [FCM web message and click handling](https://firebase.google.com/docs/cloud-messaging/web/receive-messages).
**Alternatives considered**: Fix only service worker, trust arbitrary payload URLs. The former leaves other surfaces inconsistent; the latter permits unsafe redirects.

## Confirmed product simplifications and rollout defaults

**Decision**: Device-only controls; all four categories; deadline 48h; combine charge/balance push. Preserve all existing registrations without re-enrollment and ignore old opt-outs, as explicitly requested on 2026-10-13. Keep existing waitlist promotion without its notification. All still-open upcoming deadlines normalize to 48h; sign-out retains registration; split reversals generate correction events. These choices were explicitly confirmed on 2026-10-13.
**Rationale**: User explicitly selected the primary product choices. The user explicitly requests forward-only policy with existing tokens retained; no legacy preference adapter or opt-out migration is needed. Prevent backlog sends and preserve already-closed windows.
**Alternatives considered**: Four category switches, 60h reminder after the current deadline, two post-split pushes. User declined them.

## Delivery guarantees

**Decision**: One logical event with bounded per-device retries; record ambiguous provider outcomes as uncertain and do not auto-resend them.
**Rationale**: A crash after FCM submission and before persistence makes exactly-once external delivery unknowable. Database uniqueness covers event generation, not human viewing.
**Alternatives considered**: Mark every timeout failed and resend, claim exactly-once delivery. The former can duplicate alerts; the latter is not supportable.
