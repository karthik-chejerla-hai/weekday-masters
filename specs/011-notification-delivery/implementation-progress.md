# Implementation execution record

Owner request on 2026-10-13 supersedes the earlier R1-only handoff: implement all six releases of issue81 with subagents and separate worktrees, one PR per release in one gh stack; review every release and address all P1/P2 findings before pushing implementation; CI must pass on all PRs. Use GPT 6 Luna for routine test/CI work. No merge to main is requested.

The requirements checklist has one open technical-readiness item. The owner explicitly requested commencement after the review and subsequent product decisions. Resolve each affected release's contracts before its implementation and preserve the checklist's honest status; no blanket completion claim.

## Integration sequence

- R1 branch codex/notifications-r1-status-links; worktree notification-release-1. Status and navigation use separate part worktrees/commits, integrated and reviewed together.
- R2 branch codex/notifications-r2-device-setup, based on reviewed R1.
- R3 branch codex/notifications-r3-durable-delivery, based on reviewed R2.
- R4 branch codex/notifications-r4-rsvp-reminders, based on R3.
- R5 branch codex/notifications-r5-session-charges, based on R4 (independent trigger implementation may be authored against R3, then integrated).
- R6 branch codex/notifications-r6-announcements, based on R5 (independent module implementation may be authored against R3, then integrated).

Final gh stack is main -> R1 -> R2 -> R3 -> R4 -> R5 -> R6. No planning-only seventh PR. R1 includes the original planning commit. Isolate shared-file integration and migrations; never simultaneously test branches against the same truncating database/schema.

## Validation

Disposable PostgreSQL16 container rally-notification-test bound to 127.0.0.1:55481; synthetic database badminton_club_test. Test-only credentials are held in the orchestration session, not committed. Baseline actual database-backed go test ./..., frontend lint/build and 329 Vitest cases passed. Lint has the pre-existing SessionDetail effect warning; no new warning is acceptable. Each release gets scoped/full appropriate tests and independent diff review before implementation is pushed; cheap-model helper monitors CI afterward.

## Review gates

Each release review compares only that release against its parent branch, including pending edits. Report P1/P2 findings, fix and rerun relevant tests/review until none remain. Do not push new implementation until this gate passes. Keep a release-specific review/validation entry below, then attach each created PR to this chat and link the six PRs using gh stack.

## Status

R1 is [PR #86](https://github.com/karthik-chejerla-hai/weekday-masters/pull/86), reviewed with no P1/P2 findings and all required/preview CI green. Local verification passed backend database race tests (72.0% coverage), frontend 351 unit tests (81.47% lines), lint/build/typecheck and all 122 desktop/mobile browser tests.

R2 is [PR #88](https://github.com/karthik-chejerla-hai/weekday-masters/pull/88), stacked on R1 and independently reviewed with no P1/P2 findings. Existing registrations, IDs and owners remain intact regardless of old flags; ordinary logout keeps registration. Explicit device setup, proof-based account-switch cleanup, invalidated legacy repair and cross-tab operation fencing replace notification preferences. Waitlist emission is removed while promotion/locking remains. Local validation: backend service/handler race suites, 381 frontend unit tests, device/browser regression cases, lint, build and typecheck. All PR CI and preview checks pass.

R3 is locally implemented and independently reviewed with no P1/P2 findings. Durable event/outbox transactions, fenced per-device attempts, bounded retries and remaining-expiry FCM TTL replace immediate sends. Provider outages preserve pending intent; definite stale-token rejection can retry a healthy same-owner rotation. Deployment stops capture suppression. Legacy timing stays until the persisted R4 marker, with shared transaction/dispatch fencing. Private OIDC worker plus independent scheduled backlog health checks, deployment protocol tests, fixed $20 reporting and queued/suppressed nudge feedback are included. Production queue activation fails closed without worker provisioning; stopped preparation is supported. Terminal failed/partial/uncertain outcomes emit safe ERROR aggregate counters. Operators must configure identities, Scheduler API/IAM and alert routing before activation. No cloud provisioning was performed locally. Backend scratch-DB race/CI coverage (70.7%), frontend 358 tests (81.71% lines), lint/build and mocked deployment protocol checks pass. Final stacked CI remains to run.

R3 is [PR #89](https://github.com/karthik-chejerla-hai/weekday-masters/pull/89), stacked on R2. Independent review found no P1/P2; backend, frontend, browser and preview backend checks pass, with preview frontend still running at this update.

R4 is [PR #90](https://github.com/karthik-chejerla-hai/weekday-masters/pull/90), stacked on R3. It normalizes all still-open future RSVP deadlines to 48 elapsed hours, preserves closed windows, creates 96h/60h reminders only for members with no RSVP, and retires the legacy reminder scheduler. Independent review caught and cleared three P2s: lost status-close mapping, deletion blocked by reminder-stage rows, and safe handling of in-flight/partially accepted reminders during deletion. A follow-up audit removed accidentally retained rebase markers and obsolete timing variables from `.env.example` and both deploy workflows; the follow-up review is clear. Local verification passes the backend suites, frontend 376 tests, lint and production build. The production build reports the existing large-chunk advisory. Stacked CI is running.

