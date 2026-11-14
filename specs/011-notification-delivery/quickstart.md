# Validation Guide

These scenarios validate future implementation; no notification code was changed by this planning task.

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

Use the repository’s Playwright script/configuration for new E2E cases; inspect `frontend/package.json` rather than invent a runner command. Add outbox/device tables to truncation before running database tests.

## Independent demonstrations

1. **Accurate status**: Default-true preferences with no devices show No push devices; one/two active devices show correct count; status load failure is unknown, never false success.
2. **Navigation**: Exercise all category links in foreground, service-worker click and history, signed in/out. Charge opens `/sessions/:id/settlement`; OAuth returns there and foreign event access is blocked.
3. **Device controls**: Enable A/B; disable A; reload/sign out/in/open another tab. Only B remains active. Ordinary sign-out of B preserves its registration. Switch accounts on shared installation and retire the former account’s local registration. Migrate registrations with every combination of old flags and prove tokens/ownership remain intact and usable without a permission prompt; run migration twice and verify explicitly enabled/revoked device states are not reset.
4. **Durability**: Queue an existing message; interrupt worker before send, after provider acceptance and before outcome write. Restart safely. Two concurrent workers create one logical event; accepted devices are not intentionally retried; ambiguous acceptance is uncertain.
5. **RSVP**: Clock at start minus 96h/60h and the configured deadline, exactly on boundaries and across Sydney DST. None/IN/OUT/waitlisted cases; late creation; outage across stages; time edit; cancellation; stopped delivery; rollout skips past stages without changing deadlines; the initial default remains 50h; custom and bulk deadline changes update future reminder usefulness without replaying past-due stages.
6. **Charges**: Split with regular, extra-hour and guest shares. Check one event/member, exact aggregated cents and balance snapshot at negative/$0/$19.99/$20 boundaries. Zero-cost split still gives included members receipts. Rollback/retry/import creates no extra event. Top-up before delayed delivery does not rewrite the snapshot. Reversal suppresses pending receipts and creates one correction/member; re-settlement creates distinct receipts with settlement-specific destinations.
7. **Nudges**: Concurrent admins, repeated request ID, 24-hour cooldown, no device, pause and balance rising before dispatch. Outcome never claims delivery when only queued/history recorded. Automatic charge alerts do not consume manual cooldown.
8. **Announcements/cleanup**: Repeated publish/cancel creates one publication and recipient event set; removed members suppressed at send. Waitlist still promotes without notifications. Old history remains readable.

## Deployment rehearsal

In a synthetic preview environment, verify private worker authentication (anonymous/member calls fail), least-privilege scheduler invocation, minute cadence, stop controls, transient retries, backlog expiry and logs without tokens. Rehearse explicit migration and old/new client compatibility before production rollout. Configure an independent alert for failed scheduler runs, overdue outbox work or terminal/uncertain provider attempts; the worker cannot be its own sole outage monitor.
