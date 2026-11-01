# Executable foundation packets: T008–T014

Refined 2026-11-02 against main `be5507e` (#104–#109). This is planning only.
T007 is implemented in [PR #110](https://github.com/karthik-chejerla-hai/weekday-masters/pull/110)
and must merge before code work on these packets begins. No packet below is complete.

T008–T014 remain stable tracking milestones for historical requirement references.
F001–F014 are the smaller execution units. They are not added to the 60-milestone
progress denominator. Each F packet ships its implementation **and** regression
tests in one independently safe PR. No tests-only red PR is a deliverable.

## Existing code to preserve

- #105/#106 already supply MoneyOperation storage, deferred FK/completion guards,
  canonical JSON, strict envelope decoding, review DTOs and runMoneyOperation.
  Extend those seams; do not create another writer, request table or retry engine.
- #107 already locks guest payers FOR SHARE before session, includes guests in
  session capacity/default legacy charge lines, and refuses removal of an IN guest's
  payer in a non-cancelled unsettled session. Preserve those rules and tests.
- RemoveMember currently holds NO KEY UPDATE, but its balance lookup and RSVP
  cancellation use global DB/separate transactions. Its transaction wrapper alone
  does not make removal atomic. Missing notifier wiring currently skips RSVP cleanup.
- Normal member RSVP creation currently takes only the session lock. Profile/auth
  refresh and role/status methods still use whole-row Save. UpdateUserRole lacks
  the last-admin guard; the other guard's count uses global DB without serialization.
- Initial settlement still discovers/validates members after its session lock.
  New reversal services must freeze users before session/original/account locks.
- Frontend reverseTransaction/reverseSettlement wrappers exist but have no current
  UI callers. Reviewed route releases must update their typed request contracts.

## Rules for every handoff

Read spec.md, the relevant contracts/ section, this packet and its listed source
files. Change only listed modules and their direct test fixtures; stop and report
if a new product decision, schema change or unlisted cross-module change is needed.
Source shorthand below: bare Go service filenames live in backend/internal/services/;
handlers/ means backend/internal/handlers/; server/main.go means
backend/cmd/server/main.go; frontend paths are under frontend/src/ unless stated.
Recheck the packet baseline against main before coding if additional PRs have merged.
Within a packet, write/run a failing regression first, implement, then run the
specific check and the existing affected regressions. A filter matching no tests,
or a skipped DB test, is not evidence of execution.

Go regressions introduced here use `TestFoundationFxxx...` prefixes so verification
is unambiguous. From backend/, with TEST_DATABASE_URL explicitly targeting a new
scratch PostgreSQL 16 database, run:

```bash
go test -race ./internal/services -run '^TestFoundationF001' -count=1 -v
```

Substitute the packet ID; HTTP packets use ./internal/handlers. Check output names
and absence of SKIP, then run `go test -race ./...` before merge. The harness must
still skip safely without TEST_DATABASE_URL. Use database barriers/observed lock
waits and bounded timeouts, not sleeps as the only concurrency synchronization.
No production data, migration execution or notification sends are needed.

Every PR passes CI, is reviewed, then merges before the next dependent branch is
created from main. F labels below identify planned delivery units, not GitHub PR
numbers. This planning PR contains no application implementation.

## Execution checklist and dependencies

- [ ] F001 Prevent profile refreshes from restoring stale role/status in backend/internal/services/user_service.go and user_service_test.go.
- [ ] F002 Serialize last-admin and membership transitions in backend/internal/services/user_service.go and user_service_test.go.
- [ ] F003 Freeze the target member before normal RSVP creation/update in backend/internal/services/rsvp_service.go and rsvp_service_test.go.
- [ ] F004 Extract caller-transaction RSVP deletion/promotion effects in backend/internal/services/rsvp_service.go and rsvp_service_test.go.
- [ ] F005 Freeze initial-settlement membership before session locks in backend/internal/services/settlement_service.go and settlement_service_test.go.
- [ ] F006 Make member removal and its RSVP/notification effects atomic in backend/internal/services/user_service.go, rsvp_service.go and their tests.
- [ ] F007 Extract exact-inverse/settlement-undo helpers without changing public behavior in backend/internal/services/ledger_service.go, settlement_service.go and reversal_safety_test.go.
- [ ] F008 Implement reviewed top-up reversal services in backend/internal/services/member_money_service.go and member_money_service_test.go.
- [ ] F009 Implement reviewed native-session reversal services in backend/internal/services/member_money_service.go, settlement_service.go and their tests.
- [ ] F010 Add approved-member transaction detail in backend/internal/services/member_money_service.go, backend/internal/handlers/member_money.go and their tests.
- [ ] F011 Release reviewed top-up HTTP reversal and matching legacy/API-client cutover in backend/internal/handlers/member_money.go, ledger.go, openapi.yaml, backend/cmd/server/main.go and frontend/src/services/api.ts.
- [ ] F012 Release reviewed session HTTP reversal and both matching legacy/API-client cutovers in backend/internal/handlers/member_money.go, ledger.go, settlement.go, openapi.yaml, backend/cmd/server/main.go and frontend/src/services/api.ts.
- [ ] F013 Prove mixed-route authorization, rollback and replay compatibility in backend/internal/handlers/member_money_test.go and backend/internal/services/reversal_safety_test.go.
- [ ] F014 Add the exact-payload retry/review hook in frontend/src/components/money/useMoneyOperation.ts and useMoneyOperation.test.tsx.

Default order is F001→F002→F003→F004→F005→F006→F007→F008→F009→F010→F011→F012→F013→F014.
Actual dependencies: F001 needs merged #110; F002/F003/F004 can follow F001 without
depending on one another; F005 needs F002/F003; F006 needs F001–F005; F007 needs
F006; F008 needs F007; F009 needs F008; F010 needs F008/F009; F011 needs F010;
F012 needs F011; F013 needs F012; F014 needs F011/F012. Default serial delivery
avoids overlapping edits to shared service files. Independent fixture/UI-test
preparation can happen in parallel; merge gates and shared-file ownership remain.

### Stable milestone completion map

T008: acceptance tests in F001–F006. T009: F001/F002/F006 implementation.
T010: F003/F004/F005. T011: F007/F008/F009. T012: F010/F011/F012.
T013: F011/F012/F013 HTTP tests and compatibility gate. T014: F011/F012 typed
API methods plus F014. Mark a T milestone complete only when every mapped
obligation has shipped and passed its checks. Do not count a packet twice.

<a id="f001"></a>
## F001 — Profile field ownership

**Outcome:** cosmetic/profile writes cannot overwrite a newer role or membership
status. **Depends:** merged #110. **Owner:** Luna implementation is reasonable;
Sol reviews field ownership and race tests.

**Files:** user_service.go: UpdateProfile, SyncDisplayFields, non-role portions of
UpdateMemberDetails and syncAccountName; user_service_test.go. Preserve verified
identity handling and all current nickname/name/account-name rules.

**Acceptance:** load a stale user, change its role/status in another transaction,
then call SyncDisplayFields: the new name/picture saves but the newer role/status
survive. Exercise stale admin/self-profile editing too. Omitted fields remain
unchanged. An admin email edit racing invite claiming must re-read the locked
row and refuse changing provider-owned email after the claim wins. A failed
account-name update rolls back a nickname/name edit so screen
and account names remain consistent. Use targeted updates, validate field ownership from the current locked row, and
re-read the returned user. The name/account synchronization
transaction takes the existing financial table intent before any user/account
write, so making these edits atomic cannot introduce an import/user/account cycle.

**Exclude:** changing role/status policy or Auth0 invite adoption; those are F002.
No role/status transitions, RSVP changes or API shape changes here. **Check:** service command with
F001 prefix, plus existing profile/member display-name tests.

<a id="f002"></a>
## F002 — Serialized role/status changes and last-admin safety

**Outcome:** no supported role/status write can remove the final approved admin
or restore stale membership. **Depends:** F001. **Owner:** Sol leads; Luna may add
specified fixtures. **Files:** user_service.go/user_service_test.go; ledger_service.go only for a
caller-tx account-ensure helper; relevant Auth0 adoption/approval tests in those
existing suites. Account creation associated with a status/claim transaction
uses the caller tx so rollback cannot publish part of that membership change.

Use one named transaction-scoped advisory guard for the last-admin predicate:
reserve key 724359682 (the source audit found only import key 724359681).
All cooperating writers use this same constant; do not select a per-user key.
Every path that may demote/remove an approved admin takes financial table intent,
then that guard, then user NO KEY UPDATE; count other approved admins through tx.
UpdateUserRole, role edits and RemoveMember must cooperate in this same release.
Approval/rejection/reinstatement/adoption re-read rows and update intended fields
only; preserve existing verified-email/admin-email promotion behavior and account
creation. Do not let invite adoption Save a stale whole user.

**Acceptance:** two different approved admins concurrently demoted/removed through
different entry points leave at least one; exactly one losing mutation is refused.
Single-admin demotion is refused by UpdateUserRole too. A stale join decision
cannot replace a newer approved/removed state. Preserve concurrent invite claiming
and original ledger/account identity. No RSVP atomicity refactor yet.
**Check:** F002 service tests plus existing registration/invitation/membership tests.

<a id="f003"></a>
## F003 — Normal RSVP target membership lock

**Outcome:** normal/admin member RSVP writes cannot introduce attendance after
removal wins. **Depends:** F001. **Owner:** Luna may implement the prescribed
lock change; Sol reviews concurrency. **Files:** rsvp_service.go/rsvp_service_test.go,
guest_payer_removal_test.go only if extending existing race fixtures.

CreateOrUpdateRSVP locks its target user FOR SHARE and checks current approval
before session FOR UPDATE. Reuse/generalize the existing guest-payer lock wrapper;
keep messages appropriate to member versus guest. Removal's deletion path must
remain callable without acquiring a late user lock. Keep capacity/admin bypass,
deadlines, late flags, waitlist order and guest counting unchanged.

**Acceptance:** hold the target removal lock, queue a new member RSVP on a different
session, mark removed and commit: the writer refuses and creates no row. Reverse
ordering: a committed RSVP is visible for subsequent removal cleanup. Existing
TestConcurrentGuestSaveAndPayerRemoval and guest/member-capacity tests still pass.
**Exclude:** actor/interface redesign or new RSVP product rules. **Check:** F003
and existing TestGuest/TestConcurrentGuest tests.

<a id="f004"></a>
## F004 — Transaction-scoped RSVP release and deferred effects

**Outcome:** an outer service can delete/promote using its own tx without sending
a notification before commit. **Depends:** F001. **Owner:** Luna may implement from
the explicit behavior; Sol reviews transaction enlistment.

**Files:** rsvp_service.go/rsvp_service_test.go and the existing RSVPCanceller
interface in user_service.go only as required to expose the transaction capability.
Extract deletion/promotion work into a helper accepting caller tx and an already
locked session; return promotion effects (member IDs plus session context). Public
DeleteRSVP keeps its existing behavior through the helper and dispatches effects
only after successful commit. The outer caller must have a way to deliver returned
effects after its own commit. Never fall back to public DeleteRSVP from inside tx.

**Acceptance:** force outer rollback after deletion/promotion: original RSVPs remain
and notifier records zero calls. Commit: one promotion and one post-commit call.
Cancelled sessions, deadline checks, guest capacity and waitlist order retain their
existing meaning. **Exclude:** member removal rewiring until F006; no silent
change to guest deletion. **Check:** F004 and existing RSVP/guest deletion suites.

<a id="f005"></a>
## F005 — Initial settlement membership snapshot

**Outcome:** initial settlement acquires users before session/accounts and charges
only a current, approved selected roster. **Depends:** F002/F003. **Owner:** Sol.
**Files:** settlement_service.go, expense_service.go where it delegates input,
settlement_service_test.go and guest_rsvp_service_test.go/fixtures as required.

For explicit lines, the user set is known. For default attendance, discover member
RSVPs and guest payers before row locks. Begin/restart the whole transaction with
financial table intent, sorted actor/user SHARE locks, then session lock. Re-read
default attendance through tx. A newly discovered user causes rollback and retry
from discovery, at most three attempts; exhaustion returns 409 preview_changed
with a review-again message. Never acquire late user locks under a session.

**Acceptance:** membership removal/status change before acquisition is refused;
status changes queued after locks wait until commit. A guest/member added between
discovery and session locking is either included after restart or causes clean
refresh refusal. No missing/extra charge or partial stock change. Confirm current
actor approval/admin role under lock, matching existing HTTP access; fix synthetic
actor fixtures to be real admins rather than weaken this check. Preserve valid
legacy guests/comping, actual-count restrictions, saved costing and notifications.
**Exclude:** correction costing or RSVP mutation. **Check:** F005 plus existing
settlement/expense/guest default-line and stock-concurrency suites.

<a id="f006"></a>
## F006 — Atomic member removal

**Outcome:** removal, zero-balance decision, RSVP release/promotions and status
change commit together. **Depends:** F001–F005. **Owner:** Sol.

**Files:** user_service.go, rsvp_service.go, user_service_test.go,
rsvp_service_test.go and guest_payer_removal_test.go. Follow table intent → F002
admin guard → target NO KEY UPDATE → future sessions in UUID order → player
account lock. Read the balance through tx while holding its account lock; preserve
self-removal/last-admin/guest-obligation checks. Reuse F004 helpers for RSVP writes.
No global-DB reads or separately committed cancellation on this path.

**Acceptance:** inject failure on the second session release or final user update:
all RSVPs/status/balance remain and notifications are zero. Successful multi-session
removal releases all places and delivers each promotion only after outer commit.
No configured notifier must suppress only delivery, never database cleanup. Test
normal/guest RSVP creation and initial settlement races using F003/F005 locks.
Exact future reviewed reversal of an inactive original is still allowed; removal
is not a rule that historical balances can never change later. Guest payer refusal
must leave every RSVP untouched. **Check:** F006 plus all existing removal/guest
payer tests. **Exclude:** deleting users, ledger entries or financial adjustments.

<a id="f007"></a>
## F007 — Exact inverse and settlement-undo seam

**Outcome:** caller-tx inverse posting can carry operation ID/step and an explicit
effective timestamp, and settlement undo can enlist in that same tx.
**Depends:** F006. **Owner:** Sol. **Files:** ledger_service.go,
settlement_service.go, reversal_safety_test.go and ledger_service_test.go.

Extract helpers around the existing exact-entry negation and settlement metadata
update. Caller retains the early table/session/original/account lock contract.
Keep LedgerService the sole entry writer. Existing public legacy methods delegate
with their existing timestamp/response/reason behavior; the later reviewed path
will explicitly supply original occurred_at. No new endpoint/caller policy here.

**Acceptance:** exact cents/units/comped-surplus restoration, immutable originals,
status-write failure rollback and mixed-endpoint races still pass. A direct reviewed
helper test posts operation step 0 and exact original timestamp in the same tx;
rollback leaves neither inverse nor metadata. Keep imported/audit/reversal guards.
**Check:** F007 plus existing TestSessionReversal/TestReverse tests.

<a id="f008"></a>
## F008 — Reviewed top-up reversal service

**Outcome:** read-only preview and durable confirmation undo one exact current
native top-up. **Depends:** F007. **Owner:** Sol.
**Files:** member_money_service.go/member_money_service_test.go; ledger_service.go
only for the F007 helper call. Add a public MemberMoneyService facade for handlers;
keep the existing runner and its callback/request types internal.

Preview takes a read-only repeatable-read snapshot. Verify positive two-leg native
bank/member shape, empty proposed object, reason and exact original identity. Return
the common review DTO with deposit labels, null bill fields and member/bank before,
delta and after values. Include current memberships, touched account entry counts,
original/live state and pending-asset status in the v1 fingerprint.

Confirm resolves immutable routing only before runMoneyOperation; do not reject a
successful replay based on current inverse/version state before the runner reads
its stored outcome. Inside callback lock original and the entire touched account
set in UUID order, re-read state, compare the fingerprint, use F007 exact inverse
at original occurred_at, and store the confirmation DTO atomically.

**Acceptance:** spent credit may become debt; inactive original member is reversible
with fresh review; changed relevant membership/balance/count invalidates review;
untouched members do not. Same ID/contents replays after later changes; different
contents/actor conflict; revoked admin gets no response. Unsupported/malformed
originals and duplicate/different-ID reversal write nothing. Response includes
operation/original/inverse IDs and accepted review. No email/push or transfer.
**Check:** F008 and existing money protocol/ledger tests. **Exclude:** top-up edits,
sessions, HTTP routes and new UI.

<a id="f009"></a>
## F009 — Reviewed native-session reversal service

**Outcome:** the same service reverses one exact settlement version, including
its status and stock, without touching a newer replacement. **Depends:** F008.
**Owner:** Sol. **Files:** member_money_service.go/member_money_service_test.go,
settlement_service.go and reversal_safety_test.go.

Use immutable transaction→settlement→session routing before the runner. Supply all
original charge-line users/guest hosts, including comped zero-leg members, for user
locks. Inside callback lock session, exact settlement, original transaction and
union of original/reviewed accounts. The reversal is an exact inverse, never new
costing. Preview shows all member, court, stock-unit/value and surplus effects.

**Acceptance:** original timestamp retained exactly; inactive original players and
hosts are retained; rate/price changes never recost undo. Status-update failure
rolls back money and reservation. Generic versus dedicated retries produce one
accepted operation or a version conflict, never double undo. After resettlement,
an old target cannot reverse the new version. If a historical zero-charge user lacks an account needed for the displayed
balance, refuse with legacy_correction_unsupported and an actionable membership
repair message; preview never creates an account or assumes an unlocked zero.
No correction-only date/finished restriction is added to exact undo; native association/live/import safeguards
still apply. Successful identical replay bypasses changed live status. No new
notifications. **Check:** F009 and complete existing reversal-safety suite.
**Exclude:** settlement replacement/attendance editing and route changes.

<a id="f010"></a>
## F010 — Approved-member transaction detail

**Outcome:** one read endpoint returns native identity, immediate inverse/
replacement and current descendant, supported snapshot and allowed actions.
**Depends:** F008/F009. **Owner:** Luna can implement to contract; Sol reviews
financial visibility. **Files:** member_money_service.go, handlers/member_money.go,
member_money_test.go in both packages, server/main.go and handlers/openapi.yaml.

Implement GET /api/transactions/:id, registered with RequireApproved in the same
handler registration used by server and harness. Approved members see existing
money visibility; only current approved admins receive supported live actions.
Unsupported/import/audit/reversal kinds return readable allowed metadata and no
mutation action, not an invented native snapshot. Expose no canonical operation
payload/response blob, Auth0 subject, email or push-token data.

**Acceptance:** 401 absent identity, 403 pending/removed member, 404 missing transaction
and 400 malformed UUID; correct multi-hop links,
actors/reasons/recording and effective dates; inactive display names retained.
Never propose actions on old/inverse versions. Test route registration in the
actual harness, not a hand-built weaker router. **Check:** F010 service/HTTP tests.
**Exclude:** grouped history/order/filter work in T018/T052 and all new write routes.

<a id="f011"></a>
## F011 — Top-up reviewed-route release and legacy cutover

**Outcome:** top-up reviewed HTTP reversal ships without an unreviewed top-up
bypass. **Depends:** F010. **Owner:** Luna may wire exact contracts; Sol reviews
route groups and the cutover. **Files:** handlers/member_money.go, ledger.go,
member_money_test.go, openapi.yaml, server/main.go; frontend/src/types/index.ts,
services/api.ts and services/member-money-api.test.ts (new).

Register /admin/transactions/:id/reversal/{preview,confirm} with approved-admin
checks. Dispatch native top-ups to F008. Reject unknown fields, duplicate keys,
nonempty proposed, invalid IDs/reasons/fingerprint using existing strict helpers.
In this same PR, legacy /admin/transactions/:id/reverse for top-ups accepts the
reviewed envelope and delegates; missing review returns 409 preview_required and
zero writes. Canonical/duplicate-key parsing runs first; on supported legacy
routes an absent submission ID, fingerprint or reason returns preview_required
before interpreting the old description-only body. A supplied reviewed envelope
is then subject to strict unknown-field and semantic validation. Other legacy kinds retain their existing protections/behavior.

Add typed preview/confirmation/envelope DTOs and API methods. The currently-unused
reverseTransaction wrapper requires a reviewed envelope for supported member
operations; no automatic preview+confirm or reason/fingerprint fabrication.
Update OpenAPI responses and harness registration in this PR.
**Acceptance:** access/input/stale/version/duplicate tests hit both HTTP paths;
same-ID replay returns same 201 DTO. Invalid input makes zero entry/operation rows.
Native-session legacy undo stays unchanged until F012; the new reviewed route
reports that type unavailable until F012. **Check:** F011 HTTP/service tests, API
request-shape test and frontend lint/build. **Exclude:** new action UI or corrections.

<a id="f012"></a>
## F012 — Session reviewed-route release and both legacy cutovers

**Outcome:** all three native-session reversal entry paths share F009 and cannot
bypass review. **Depends:** F011. **Owner:** Luna may wire; Sol reviews exact-version
routing. **Files:** F011 handler/server/OpenAPI/API files plus settlement.go and
settlement_test.go; frontend services/member-money-api.test.ts.

Extend the reviewed transaction route to native session kind. In this same release,
both /admin/transactions/:id/reverse for session charges and
/admin/settlements/:id/reverse require the reviewed envelope, resolve the **exact**
original transaction, and delegate. Immutable routing is allowed before replay;
live-version/freshness refusal must stay inside the runner's new-operation branch.
Update reverseSettlement's typed API request/response. HTTP route aliases are
not part of submission identity: the same envelope and underlying transaction
must hash identically through generic and dedicated routes. Preserve generic asset,
import/audit and reversal-of-reversal policy; no blanket rewrite of other kinds.

**Acceptance:** missing review on either legacy session path yields 409 and no
money/status change; approved-admin checks match the new path; all three paths
restore exact same state and original timestamp. A successful same-ID retry after
resettlement returns its old accepted DTO; a new ID against the old version cannot
undo the replacement. **Check:** F012 HTTP tests plus all reversal safety/API shape
checks and frontend lint/build. **Exclude:** session correction editor/costing.

<a id="f013"></a>
## F013 — Cross-route foundation compatibility gate

**Outcome:** prove the integrated route matrix and close T013 without introducing
new feature behavior. **Depends:** F012. **Owner:** Sol designs/asserts adversarial
cases; Luna may fill fixed matrix fixtures. **Files:** handlers/member_money_test.go,
services/reversal_safety_test.go, services/financial_lock_order_test.go and
quickstart.md validation results only.

Cover top-up new+legacy and session new+generic+dedicated paths using real handler
registration. Cross route same-ID/same-target retries must replay once; concurrent
different-ID changes must yield one success. Include pending/removed admin,
post-success demotion, strict raw duplicate-key requests, unsupported originals,
changed reviewed counts/balances, status-write failure and exact effective dates.
Record transaction/entry/operation/live-settlement counts before/after rejection.
Extend the existing T007 table-gate cases with real reviewed top-up and session
confirmation services, retaining the pg_locks/NOWAIT assertion. Preserve existing
asset correction/import protections and notification silence.

**Acceptance:** every matrix entry has an explicit expected status/code and DB
outcome; no generic success assertion replaces accounting/state checks. Full
backend race suite passes, with no DB skip or weakened old tests. **Check:** F013
HTTP/service filters then go test -race ./.... **Exclude:** UI tests deferred to
actual UI delivery, unrelated security cleanup or rewriting existing import data.

<a id="f014"></a>
## F014 — Exact-payload frontend review/retry state

**Outcome:** a tested hook preserves the submitted request through uncertainty
and never confirms a changed review silently. **Depends:** F011/F012.
**Owner:** Luna is appropriate with Sol review. **Files:** components/money/
useMoneyOperation.ts, useMoneyOperation.test.tsx, services/api.ts/types/index.ts
only if needed for the already agreed DTOs. No new screen yet.

Use the singleton API client. Keep a submission UUID and deep snapshot of the
exact confirmed envelope. Network uncertainty retries that snapshot/ID; editing
visible inputs cannot mutate it. A definitive preview_changed keeps entered data,
loads a new review only on request and requires explicit confirmation again.
request_conflict never invents a fresh ID and auto-posts; 403 stops submission.
Cancel/reset before confirmation makes no write. Ignore stale preview responses
when newer inputs/reviews exist; disable confirmation while preview is invalid.

**Acceptance:** deferred-promise tests cover out-of-order previews, double clicks,
lost response followed by edited inputs, exact retry, explicit renewed review,
conflict/forbidden/cancel and success. Accepted first/replayed results emit the
existing rally:balances-changed event once for that completed user submission;
live consumers refetch rather than treating stored accepted balances as current.
No new global store or direct fetch. **Check:** npm test -- useMoneyOperation.test.tsx,
then frontend tests/lint/build. **Exclude:** top-up form, generic UI redesign,
shared/session editors and automatic background retries.

## Final foundation exit and first implementation

After F014, every T008–T014 mapping above must be satisfied; run the existing
scratch race, frontend test/lint/build and relevant route regression gates.
This does not complete T015 onward. Start **F001 only after PR #110 and this
planning change merge**, on a fresh main branch. Evaluate actual review/repair
effort before assigning additional complex packets to a cheaper model.
