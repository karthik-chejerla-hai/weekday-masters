# T008–T014 refinement validation

Date: 2026-11-02. Source baseline: main `be5507e`.
Scope: planning documents only; no runtime implementation or financial operations.

## Source audit used

- user_service.go: stale whole-row profile/auth refresh writes; role updates and
  global last-admin count; removal's NO KEY UPDATE and guest refusal; separate
  RSVP cancellation and global balance read; nil canceller behavior.
- rsvp_service.go/guest_rsvp_service.go: member versus guest locking, promotion
  within tx, post-commit public notifier behavior and current guest capacity rules.
- settlement_service.go/expense_service.go: actor/participant discovery, global
  default reads, current actual/legacy behavior and account lock order.
- member_money_service.go and handlers/member_money.go: existing runner,
  canonicalization, DTOs and strict envelope decoder to reuse.
- server route registration and frontend API reverse wrappers: read/write access
  groups and the coordinated per-type legacy cutover; no current UI callers.

## Refinement decisions

Retain the 60 historical T identifiers as milestones. Split the T008–T014 execution
scope into 14 F packets, counted separately, each with files, dependencies, acceptance
cases, verification and exclusions. Tests and implementation ship in the same PR.
The old single-foundation-PR delivery instruction is superseded. Packet F001 starts
only after the independent T007 PR #110 and this planning refinement merge.

The source audit preserves existing guest-payer safety rather than claiming it
must be rebuilt. Initial settlement and normal RSVP locking land before the final
atomic-removal slice. Profile/account-name and invite-claim writes acquire table
intent, compatible user NO KEY UPDATE and the player account before any
key-changing user UPDATE; table intent alone does not prevent ordinary-writer
deadlocks. New reviewed top-up/session routes ship with
their matching legacy cutovers and typed client requests; no interim bypass.

## PR #111 P2 correction

The review identified an unsafe F001 intermediate release: changing unique email
can upgrade the user lock while the transaction waits for an account held by
settlement, whose ChargeLine foreign-key check then waits for that user. The
reviewer's minimal SQL reproduction used PostgreSQL 14.24, not the full app or
PostgreSQL 16. F001 now requires account locking before user updates and a bounded
email-plus-nickname-versus-settlement regression on PostgreSQL 16 before F005.
F002 applies the same rule and regression to invite claiming's unique auth0_id
update. The shared locking contract records this ordering. Packet dependencies
and completion markers remain unchanged. These runtime regressions are delivery
requirements for the future implementation; they were not executed by this
documentation-only correction.

## Documentation checks

- Spec Kit setup loaded spec, local ignored plan, model/research/contracts and
  task template. `.specify/extensions.yml` absent; task pre/post hooks skipped.
- All existing T001–T060 IDs and completion markers retained. Only T008–T014 task
  descriptions and the foundation delivery explanation changed.
- T015–T060 descriptions retained verbatim; other feature implementation remains
  outside this refinement.
- F001–F014 unique, ordered and unchecked; dependency graph is acyclic and every
  T008–T014 milestone has an explicit packet completion mapping.
- Artifact links/explicit anchors and checklist syntax checked. `git diff --check`
  passed. No application suite run: no application source changed.

These checks validate the plan, not the future implementation. Every packet still
requires its failing-then-passing runtime regression and full applicable CI gate.
