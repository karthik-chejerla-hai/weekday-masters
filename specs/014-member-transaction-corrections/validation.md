# Design validation — 2026-10-26

Scope: documentation design and task generation after PR #103 merged as `9b83151`.
No runtime implementation, migration or financial mutation was performed.

## Checks executed

- `git diff --check` — passed.
- `SPECIFY_FEATURE_DIRECTORY=specs/014-member-transaction-corrections .specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks`
  — passed; discovered research.md, data-model.md, contracts/, quickstart.md and tasks.md.
- Local artifact audit — all relative Markdown links resolve, no unresolved plan
  placeholders, T001–T060 unique/sequential with implementation paths. T001 design
  complete; 59 execution tasks unbuilt. Story counts: US1 10, US2 12, US3 9,
  US4 6, US5 5; foundation 13, setup 1 and final verification 4.
- FR-001–FR-026 mapped to tasks; I–IX constitution check passed before and after design.
- `.specify/extensions.yml` absent: plan/tasks pre/post hooks skipped as specified.

## Source-based research checks and resolutions

Two planning research agents inspected existing financial/history/membership code
and the drafted design. They identified the following issues, resolved in the
artifacts before completion:

1. Operation reservation's target FK can acquire KEY SHARE before original
   FOR UPDATE, causing competing-request upgrade deadlocks. Explicitly deferred
   target FK and synchronized dual-reservation regression scenario added.
2. Paid/owed alone cannot reconstruct zero-share participation. Immutable
   participates field, validation and one-cent/three-participant case added.
3. Current member removal isn't transactional and can miss new RSVPs. Required
   user-before-session locks, tx-aware removal, RSVP creation cooperation and
   targeted profile/role writes added to foundation tasks.
4. Import table-first vs posting account-first locks can invert. All cooperating
   financial entry points now have an early compatible table intent protocol.
5. UUID order cannot guarantee inverse before replacement at equal timestamps.
   Common legacy-preserving operation group/sequence/step comparator specified.
6. Existing costing consumes current stock and split identity includes line index.
   Frozen valuation/allocation seam, stable line keys, key-only no-op and exact
   legacy reconciliation/refusal defined.

Final history research check found no blocking source/spec contradictions.
PR 2 is intentionally an API foundation; new reversal UI exposure waits for PR 3's
history/detail integration. This is design review, not executable proof of those
future money operations. Runtime checks in quickstart.md remain implementation gates.
