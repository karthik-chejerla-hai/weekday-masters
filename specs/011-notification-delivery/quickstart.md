# Validation Guide

This guide covers the six forward-only implementation releases for issue 81. Use synthetic members and a disposable database for local checks. Pull-request preview runs validate schema migration and deploy the stopped notification worker without sending member push alerts.

## Prerequisites

Use a disposable local PostgreSQL database, configured backend/frontend and synthetic approved members. Never point TEST_DATABASE_URL at production or an imported roster: the test harness truncates tables. Notification regression tests use a fake provider and controlled clock; real delivery is a separate explicit self-test. Do not enable club-wide delivery to validate a feature.

## Automated checks

```bash
cd backend
go test ./...
# Use a disposable local scratch database to exercise concurrency/restart tests:
TEST_DATABASE_URL='postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable' go test -race ./...
cd ../frontend
npm run lint
npm run build
npm test -- --run
```

Use the Playwright script/configuration in `frontend/package.json` for browser cases. Add outbox/device tables to truncation before running database tests.

## Independent demonstrations

1. **Accurate status**: Default-true preferences with no devices show No push devices; one/two active devices show correct count; status load failure is unknown, never false success.
2. **Navigation**: Exercise all category links in foreground, service-worker click and history, signed in/out. Charge opens `/sessions/:id/settlement`; OAuth returns there and foreign event access is blocked.
3. **Device controls**: Enable A/B; disable A; reload/sign out/in/open another tab. Only B remains active. Ordinary sign-out of B preserves its registration. Switch accounts on shared installation and retire the former account’s local registration. Migrate registrations with every combination of old flags and prove tokens/ownership remain intact and usable without a permission prompt; run migration twice and verify explicitly enabled/revoked device states are not reset.
4. **Durability**: Queue an existing message; interrupt worker before send, after provider acceptance and before outcome write. Restart safely. Two concurrent workers create one logical event; accepted devices are not intentionally retried; ambiguous acceptance is uncertain.
5. **RSVP**: Clock at start minus 96h/60h/48h, exactly on boundaries and across Sydney DST. None/IN/OUT/waitlisted cases; late creation; outage across stages; time edit; cancellation; stopped delivery; rollout skips past stages and never reopens an old closed window; all still-open custom windows become 48h.
6. **Charges**: Split with regular, extra-hour and guest shares. Check one event/member, exact aggregated cents and balance snapshot at negative/$0/$19.99/$20 boundaries. Zero-cost split still gives included members receipts. Rollback/retry/import creates no extra event. Top-up before delayed delivery does not rewrite the snapshot. Reversal suppresses pending receipts and creates one correction/member; re-settlement creates distinct receipts with settlement-specific destinations.
7. **Nudges**: Concurrent admins, repeated request ID, 24-hour cooldown, no device, pause and balance rising before dispatch. Outcome never claims delivery when only queued/history recorded. Automatic charge alerts do not consume manual cooldown.
8. **Announcements/cleanup**: Repeated publish/cancel creates one publication and recipient event set; removed members suppressed at send. Waitlist still promotes without notifications. Old history remains readable.

## Deployment rehearsal

Each pull request's Preview Deploy workflow provisions an isolated preview branch, runs the explicit migration, seeds synthetic fixtures, deploys the API with notification delivery stopped, and deploys a private worker with its own OIDC audience. The preview Hosting channel is separate from production. The workflow is not an invitation to send a real member push; use the fake provider for delivery assertions and keep the global stop in preview.

Before enabling delivery in any environment, verify anonymous/member calls cannot invoke the worker, only the configured scheduler identity can invoke it, and the one-minute schedule, retry/expiry behavior, stop controls and overdue/terminal health monitoring are active. Inspect logs for aggregate counts and error codes only; never log tokens or revocation proofs. Re-run migrations to verify idempotence and check old clients receive the forward-compatible update-required response where specified. Production activation and real-device self-tests are separate operational actions and are not part of routine PR validation.
