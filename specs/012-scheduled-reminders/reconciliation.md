# Scheduled-reminder draft and notification release overlap

Recorded 14 November 2026. The original `spec.md` and requirements checklist were
copied byte-for-byte from the untracked files in
`/Users/karthikchejerla/git/weekday-masters/specs/012-scheduled-reminders/`.
The originals remain untouched. A passed specification-quality checklist is not
evidence that implementation or deployment is complete.

## Implementation to reuse

Issue #99 asks for reminder execution independent of an idle/scaled-to-zero API.
Notification R3 (#89) already contains the intended private scheduled worker,
durable outbox and fenced per-device dispatcher. R4 (#90) adds persistent RSVP
reminder scanning and replaces the legacy scheduler. They are open PRs, not
shipped acceptance evidence. Reconcile and finish those releases rather than
create a second worker, outbox or delivery identity.

- FR-001/002/008/009/010/012: map to R3 worker invocation, device retention,
  monitoring and operational guide. Inspect `.github/scripts/notification-worker.sh`
  and its deployment-protocol tests; verify actual IAM/Scheduler/readiness in a
  separately recorded operational step.
- FR-004: map to R3 outbox identity/lease and acceptance/uncertainty tests, including
  `TestDeliveryOutboxConcurrentAtomicIdentityAndRollback`,
  `TestDeliveryRecoverUnsentAndFenceInflightUncertainty` and
  `TestDeliveryAcceptancePersistenceFailureRecoversUncertain` in
  `backend/internal/services/notification_dispatcher_test.go` on #89.
- FR-005/007/011: map to R4 scanner/cutover and R3 fake-provider/private-worker
  checks. R4 tests include `TestRSVPScannerExactBoundariesAndEveryResponseStopsReminders`,
  `TestRSVPScannerOutageAndLateCreationPublishOnlyLatestUsefulStage`,
  `TestRSVPScannerRolloutCutoffAtomicAndRepeatSafe` and
  `TestRSVPScannerDSTAndCancellationClosure`.
- Issues #96 (deduplication) and #97 (partial acceptance) overlap these same
  tests/services. Link their exact acceptance criteria to executed tests before
  closing issues; shared implementation alone is not proof of completion.

## Policy differences to resolve explicitly

1. **Timing:** the draft preserves legacy 24h/12h session reminders plus a 6h
   deadline alert. R4 instead implements 96h/60h RSVP reminders for members with
   no response, bounded by stored deadlines. Current main's initial deadline
   default is 50 elapsed hours. Do not implement both timing systems. Record the
   accepted issue #81/R4 policy as a superseding decision if that is the intended
   scope, while retaining the original draft as historical context.
2. **Pause/resume:** draft FR-006 says suppression must not consume an identity
   that prevents later eligible delivery. R3/R4 explicitly preserve suppressed
   origin/publication without replay after a pause lifts (for example
   `TestDeliverySuppressedOriginNeverReplaysAfterPauseLift` and
   `TestRSVPScannerPausedPublicationDoesNotReplayOnResume`). This is a product
   behavior difference, not a deduplication implementation detail. Do not silently
   mark both contracts satisfied.
3. **Catch-up:** the draft describes useful-window catch-up generally; R4 uses
   rollout cutoff and latest-useful-stage behavior. Check each missed-run and
   newly-created-session scenario against those semantics.
4. **Uncertain acceptance:** preserve the no-automatic-resubmission rule for an
   ambiguous provider attempt. Known failures eligible for a bounded retry are
   distinct from uncertain outcomes. Verify per-device scope in #97 assertions.

These differences need a recorded scope decision when #99 is reconciled with
R3/R4. They do not block R2 device-setup integration, which adds no new reminder
timing or catch-up policy. This document does not change either policy.

## Completion evidence still required

- [ ] Map the four policy differences above to an explicit agreed contract.
- [ ] Run relevant tests on the rebased R3/R4 release heads with a disposable DB.
- [ ] Prove worker authorization and duplicate/uncertain/partial outcomes with
  synthetic recipients and fake providers.
- [ ] Validate migration repeatability, old/new sender cutover and safe rollback.
- [ ] Record operational prerequisites and separately authorized rollout results.
- [ ] Only then reconcile/close #99/#96/#97 or document the precise remainder.

No production infrastructure, real member data or live notification sends were
used for this mapping. Test names identify existing acceptance candidates, not
tests executed during this documentation step.
