# Luna implementation handoff

Check out branch `codex/notification-delivery` in a dedicated checkout/worktree of `karthik-chejerla-hai/weekday-masters`. Use that checkout’s actual path on your machine. This branch starts from remote-default `origin/main` at `c6ae365`. The original local worktree is `/Users/karthikchejerla/.codex/worktrees/notification-delivery/weekday-masters`; other machines need no such path. Keep notification changes separate from unrelated work.

## Objective for this run

Implement **release 1 only**: accurate member push-registration status and actionable notification navigation/history. Complete setup task T001 insofar as it defines release 1's cutover, then T002–T011 in `specs/011-notification-delivery/tasks.md`. Deliver functioning code with relevant validation; go beyond a plan or explanation. Later releases have their own work and should remain unchecked.

## Read first

1. Repository `AGENTS.md` and `.specify/memory/constitution.md`.
2. `specs/011-notification-delivery/spec.md` and `release-plan.md`.
3. `plan.md`, `contracts/notifications.md`, and the release 1 section of `tasks.md`.
4. `review-followups.md`, especially the release 1 prerequisites.

The HTML overview is `specs/011-notification-delivery/release-plan.html`. The specification, implementation plan, release artifacts and handoff are committed on this branch. Although the repository normally ignores `plan.md`, it is explicitly included for this portable handoff at the owner’s request. `.specify/feature.json` remains local/ignored. On another machine, set `SPECIFY_FEATURE_DIRECTORY=specs/011-notification-delivery` for Spec Kit commands, or create that local feature pointer.

## Decisions already confirmed — do not ask again

- Preserve every existing push registration/token and its ownership. No mandatory re-enrollment, opt-out migration or permission prompt at rollout. Old master/category preferences are retired at release 2; release 1 keeps current delivery unchanged.
- New member controls are device setup only; all four categories are included as their triggers ship: RSVP reminders, Session charges, Balance alerts, Admin announcements.
- Ordinary sign-out retains push registration. Explicit disable persists across reload/tabs/sign-in; switching accounts retires the former account's registration on that browser.
- RSVP reminders will be at 96 and 60 elapsed hours before play for members with no RSVP. IN, OUT and waitlisted all count as responses. Deadline is 48 elapsed hours before play. Normalize every still-open upcoming window, including custom deadlines; never reopen already-closed windows.
- One charge notification combines the committed session charge and resulting balance warning. Negative below 0 cents, low from 0 through 1999 cents, no warning at 2000 cents or above. Manual nudges use the same rule, with the existing 24-hour manual cooldown.
- Split reversal generates a Session charges correction, atomically with reversal, and suppresses pending original receipts. Re-settlement produces a distinct receipt. These changes belong to release 5.
- Remove waitlist notification emission in release 2, retaining waitlist promotion. Cancellations remain announcements. Automatic email/SMS is outside scope; explicit invitation email remains separate.
- All times and display use Australia/Sydney. Money stays integer cents and LedgerService remains the only ledger writer.

## Release 1 boundaries and acceptance

- Admin member summaries use batched registered-device counts, showing Push set up/No push devices. Missing/error state is unknown, not setup success. Token presence describes registration, never confirms current permission or delivery. Existing flags must not fabricate a device count.
- Foreground notifications, background worker clicks and in-app history use consistent allowlisted destinations. Session charges open `/sessions/:id/settlement`; RSVP opens its session; manual nudges open `/money`; announcement opens its own recipient history detail; cancellation opens the cancelled session.
- Add a usable entry point to own notification history and own-event detail. Register routes on the approved group, respecting ownership. Keep existing API field naming/envelopes where possible; describe exact additive contracts before implementing the client.
- Preserve safe local return destinations through Auth0 login without breaking `/welcome`, approval redirects or admin boundaries. Reject external/protocol-relative/malformed return paths. Add useful missing-resource/error handling.
- Session-charge receipts do not exist until release 5: prepare their route/payload handling, but do not create charges or change settlement in this run.
- Preserve all existing token records, delivery timing, preference behavior, notification stops and invitations during release 1. Do not create the worker/outbox or perform business data backfills yet.

## Execution and validation

Resolve release 1's exact contracts and error/navigation cases from review-followups.md before coding. Record the scoped decisions/tests in the docs and report readiness honestly: later-release design gaps do not justify claiming the whole checklist is complete, and do not block this independent release once its own prerequisites are resolved. Use an implementation scope limited to T001–T011 rather than a workflow that executes the entire backlog.

Implement in the existing handler -> service -> DB structure. Frontend requests use the singleton Axios client. Keep authentication/identity claims authoritative. Prefer focused regression tests proving actual behavior rather than implementation-mirroring tests. Verify database behavior with a disposable scratch database and the existing harness; never use production, an imported roster or a remote DATABASE_URL for tests. Tests without TEST_DATABASE_URL skip database cases; disclose that limitation if infrastructure is unavailable.

Run applicable backend tests (`go test ./...`, database-backed cases with scratch TEST_DATABASE_URL when available), frontend lint/build/Vitest and targeted Playwright notification navigation/member-status tests. Do not deploy or send member notifications while implementing. Retain operational notification stops. Update completed task checkboxes only after implementation and validation.

Do not launch other chats or send messages to other chats as part of this handoff. Parallel agents are unnecessary for this first run. Separate feature worktrees for releases 4–6 can be considered after release 3 lands with stable contracts.

At completion, report changed behavior, test results/limitations, completed task IDs and any unresolved release 1 blocker. Show the final diff. Leave later releases untouched. A new PR or deployment is a separate action; this run's deliverable is release 1 code and validation in this worktree.
