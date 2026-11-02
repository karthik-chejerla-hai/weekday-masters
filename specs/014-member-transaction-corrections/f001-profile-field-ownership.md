# F001 — Profile field ownership

Implemented 2026-11-02 from main `e26869a`, after #110 and #111 merged.
F001 is complete in this branch; merge its reviewed, green PR before F002.
T008/T009 remain open because their other packet obligations are still pending.

## Behavior

SyncDisplayFields, UpdateProfile and UpdateMemberDetails now reread the user
under NO KEY UPDATE and issue targeted updates for supplied fields. Auth refresh
cannot restore stale identity, role, membership, nickname, phone or player status.
Email ownership is validated against the locked row, including a winning invite
claim. Existing role-change validation is retained; serialized role/status policy
and invite adoption remain F002.

Profile and player-account names commit or roll back together. The order is early
financial table intent → user NO KEY UPDATE → existing player account FOR UPDATE
→ targeted user update → reread → account-name update. The account lock precedes
any unique email update that could upgrade the user lock. Pending members without
accounts can edit cosmetic fields; approved members missing their required account
receive an actionable repair error before their user fields change. Profile edits
never create accounts through a second transaction.

The new-user verified ADMIN_EMAIL bootstrap now creates its player account in
the same transaction as its user. It bypasses ordinary approval, so without this
narrow integration fix the new guard would block its next login. Account-creation
failure rolls back the user too; pending registration still creates no account.
Invite adoption and other role/status transitions remain F002.

The shared HTTP fixture now creates accounts for approved members, matching the
normal approval/invitation lifecycle. No endpoint, frontend, schema, settlement
implementation or financial posting behavior changes.

## Executed verification

Used dedicated disposable PostgreSQL **16.15**, container `rally-f001-test-db`,
loopback port 55431 and database `rally_f001_test`; no application data was used.
The normal scratch harness supplied isolated package schemas and migrations.

- New `TestFoundationF001...` cases failed on the old implementation for stale
  fields, non-atomic rollback, claimed email ownership, missing-account handling
  and missing early import coordination.
- `go test -race ./internal/services -run '^TestFoundationF001' -count=1 -v`
  passed with no F001 skips.
- The real pre-F005 Settle path pauses at ChargeLine creation after holding its
  accounts. A concurrent invited-member email/nickname edit must retain compatible
  user locks while waiting, leave the user edit uncommitted, and let both writes
  finish. The test checks saved identity/account names, exact settlement cost,
  stock, stored charge, member balance and the club-position invariant.
- Negative controls confirmed that this test rejects the original separate-commit
  implementation and an atomic variant with the user UPDATE before the account
  lock. The latter reproduced `deadlock detected (SQLSTATE 40P01)` on PostgreSQL 16.
  Both temporary variants were restored before final verification.
- `go test -race ./... -count=1 -v` passed for all backend packages. Database tests
  ran; only the existing optional private-export reconciliation test skipped for
  lack of its private input. No tests require production data or notifications.
- Bootstrap registration, its next display refresh and rollback on injected
  account creation failure have dedicated failing-then-passing regressions.
- Financial import gating is tested for all three profile entry points with an
  observed lock wait and NOWAIT row probes. Worker/barrier cleanup is bounded.

Frontend checks run through normal PR CI; this packet changes no frontend source.
No schema or production migration is required. The next packet is F002 after merge.
