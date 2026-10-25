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

R1 implemented and locally validated; independent integrated review found no P1/P2 issues. All 122 desktop/mobile browser tests pass after updating old assertions to the new device/return-path contracts. Backend race tests with real scratch DB and 72.0% coverage pass; frontend 351 unit tests and 81.47% line coverage, lint, build and E2E typecheck pass. The final test-only changes are being confirmed by the independent reviewer before push. R2 device/waitlist and R3 independent library/infra work are in progress in separate worktrees; none pushed. No application notification sends or production database operations are part of local verification.
