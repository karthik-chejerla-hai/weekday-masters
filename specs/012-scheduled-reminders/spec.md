# Feature Specification: Scheduled Reminder Execution

**Feature Branch**: `012-scheduled-reminders`

**Created**: 2026-10-25

**Status**: Draft

**Input**: User description: "Explore solutions for GitHub issue 99: run scheduled reminders independently of the API on request-based Cloud Run."

## User Scenarios & Testing

### User Story 1 - Reminders run while the API is idle (Priority: P1)

As a Rally member who has opted into reminders, I want due session and RSVP deadline reminders to be checked and sent even when nobody is using the app, so reminder delivery does not depend on an API instance staying active.

**Why this priority**: This is the requested outcome and preserves useful reminders while the API scales to zero.

**Independent Test**: With the API idle or scaled to zero, trigger one scheduled run against a disposable database and synthetic recipients; verify eligible due reminders are recorded and submitted to a test provider.

**Acceptance Scenarios**:

1. **Given** a due reminder for an eligible synthetic member and no active API instance, **When** the scheduled run executes, **Then** the reminder is recorded and delivered through the test provider.
2. **Given** no reminder is due, **When** the scheduled run executes, **Then** it completes successfully without creating reminder history.
3. **Given** an execution cannot reach the database or notification provider, **When** it runs, **Then** the failure is visible in operational logs and a later scheduled execution can retry safely within the applicable reminder's useful window.
4. **Given** one or more scheduled runs were missed, **When** a later run executes, **Then** it catches up reminders that are still useful under the current policy and does not send reminders after the session starts or the RSVP deadline passes.

### User Story 2 - Repeated runs are safe (Priority: P1)

As a member, I want a reminder to arrive at most once for the same reminder occasion, even if a scheduled run is retried, repeated, or overlaps another run.

**Why this priority**: External schedulers retry and can overlap work; duplicate alerts would undermine trust.

**Independent Test**: Run the same due reminder repeatedly and concurrently with synthetic recipients, then verify each intended reminder identity has no more than one history record or provider submission.

**Acceptance Scenarios**:

1. **Given** an eligible reminder was already processed, **When** a retry checks the same reminder occasion, **Then** no duplicate reminder history or intentional repeat submission is created.
2. **Given** two executions overlap for the same reminder occasion, **When** both attempt delivery, **Then** at most one records and submits that reminder.
3. **Given** two distinct reminder kinds are due for one session, **When** the scheduled execution runs, **Then** each kind remains independently deliverable once.

### User Story 3 - Club controls and existing reminder rules remain effective (Priority: P1)

As a club member or administrator, I want scheduled reminders to follow current reminder timing, Sydney time, notification preferences, and operational pauses, so moving the execution mechanism does not change who is notified or when.

**Why this priority**: The change is about execution reliability; existing notification policy and safety controls must continue to apply.

**Independent Test**: Use synthetic members and sessions to exercise due-time boundaries, daylight saving transitions, preference opt-outs, membership status, deployment notification stop, and club pause.

**Acceptance Scenarios**:

1. **Given** a session reminder or RSVP deadline reminder is due under the current policy, **When** the scheduled run executes, **Then** it uses the current 24-hour, 12-hour, and 6-hour rules and interprets session times in Australia/Sydney, with exact-boundary behavior following issue #96's corrected contract.
2. **Given** a member is unapproved, has opted out of that notification type, has no device, or notifications are paused globally or for the club, **When** the run evaluates that reminder, **Then** it follows existing notification history semantics, sends no push, and does not consume the delivery identity in a way that blocks a later eligible send.
3. **Given** a due instant is exactly on a reminder boundary or crosses a Sydney daylight saving transition, **When** the run evaluates it, **Then** the reminder is handled once at the intended boundary without drifting by an hour or being missed.
4. **Given** a member has registered push devices, **When** the execution mechanism is changed, **Then** existing registrations remain associated with that member and can receive eligible reminders.

### User Story 4 - Operators can safely manage the scheduled path (Priority: P2)

As an operator, I want to configure, observe, and roll back scheduled reminder execution without leaving an unauthenticated notification trigger or two active senders.

**Why this priority**: A safe operational path is needed to enable reminders without always-on compute or accidental public access.

**Independent Test**: Review the deployed configuration and logs, reject unauthenticated trigger attempts, verify only one execution path is active, and rehearse rollback without sending to real members.

**Acceptance Scenarios**:

1. **Given** a request to trigger reminder execution without valid service authentication, **When** it reaches the trigger, **Then** it is rejected and sends no notifications.
2. **Given** a request has a valid identity but lacks the exact invocation permission for this trigger, **When** it reaches the trigger, **Then** it is rejected before database or notification work begins.
3. **Given** the replacement schedule is enabled, **When** API instances start or receive requests, **Then** the previous in-process timer does not send reminders.
4. **Given** multiple old and new API revisions are active during cutover, **When** the external schedule is enabled, **Then** no API revision can run the old timer and there is only one sender path.
5. **Given** an operator follows the documented rollback, **When** the replacement schedule is disabled, **Then** the system returns to a safe paused state with no overlapping reminder senders; resuming any sender requires a separate explicit operation.
6. **Given** an operator reviews the setup guide, **When** they configure or roll back the scheduled execution, **Then** the required configuration, authentication, deployment steps, logs, and expected costs/free allowance are documented.

### Edge Cases

- A scheduled execution is delayed, retried, or runs concurrently with another execution.
- The API is scaled to zero, unavailable, or running a revision during deployment.
- The database or notification provider is temporarily unavailable.
- A reminder is exactly at its time-window boundary, including Sydney daylight saving transitions.
- Club pause, deployment stop, member status, or preference changes between candidate selection and delivery.
- A member has no registered devices; the existing history and provider acceptance semantics remain truthful.
- The old in-process timer and new scheduled path could overlap during rollout or rollback.
- A trigger request is missing authentication or uses an identity without invocation permission.
- A session start or RSVP deadline is edited after an identity was created, or an RSVP changes before delivery.
- A provider accepts a request but the worker does not receive or persist the response.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST execute session and RSVP deadline reminder checks on an external schedule that continues to run when the API is idle or scaled to zero.
- **FR-002**: Scheduled execution MUST be private and authenticated; an unauthenticated request MUST NOT be able to cause reminder evaluation or notification sending.
- **FR-003**: The system MUST preserve the current reminder kinds, timing rules, Sydney time handling, recipient eligibility, preferences, deployment notification stop, and club pause behavior until a separately approved notification policy change replaces them. For exact boundaries and duplicate prevention, the corrected shared behavior in issue #96 takes precedence over defects in the current implementation.
- **FR-004**: Repeated, retried, and concurrent execution MUST use the shared persisted delivery identity and outcome contract from issue #96 and remain compatible with issue #81. They MUST NOT create duplicate reminder history or resubmit a delivery already known to have been accepted. A provider outcome that is uncertain MUST be recorded as uncertain and MUST NOT be automatically resubmitted unless the provider guarantees idempotency for that request; this may leave a reminder undelivered. The system MUST NOT claim exactly-once device receipt.
- **FR-005**: The system MUST consider reminders due since the last covered execution, recover missed runs while the session has not started or the RSVP deadline has not passed, and stop sending each reminder kind once its relevant session start or deadline has passed.
- **FR-006**: Suppressed reminders MUST follow current history semantics but MUST NOT consume a delivery identity that prevents a later eligible delivery for the same reminder occasion.
- **FR-007**: When the replacement schedule is enabled, the existing in-process timer MUST be disabled across all API revisions before the first scheduled run can send. Rollback MUST first disable the external schedule and leave reminders paused unless an operator explicitly enables one sender.
- **FR-008**: The execution change MUST preserve existing push-device registrations and ownership.
- **FR-009**: Operators MUST be able to identify each run and its success or failure, including counts of evaluated, suppressed, claimed, sent, and uncertain outcomes, from operational logs or equivalent monitoring. Failed executions MUST return a failure signal so the scheduler can apply its configured retry behavior.
- **FR-010**: The project MUST document required configuration, authentication and permissions, deployment, rollback, operational visibility, and expected scheduling cost/free allowance.
- **FR-011**: Verification MUST use synthetic recipients and test providers for idle execution, retries, concurrency, exact time boundaries, Sydney daylight saving behavior, suppression, and uncertain provider outcomes. Verification MUST NOT send notifications to real members.
- **FR-012**: Scheduled execution MUST NOT require continuously running or always-on compute.
- **FR-013**: This feature's implementation and local verification MUST NOT deploy production infrastructure or send live member notifications. Any later non-production or production rollout verification requires separate authorization and uses synthetic recipients/providers.

### Key Entities

- **Scheduled execution**: One externally initiated run that evaluates due session and RSVP deadline reminders and completes without requiring an API request from a member.
- **Reminder occasion**: A specific reminder kind for a specific recipient and session at its applicable scheduled instant; repeated execution must recognize it as the same occasion.
- **Notification delivery controls**: Existing club-wide, deployment-wide, membership, preference, and device-registration state that governs whether and where a reminder may be delivered.

## Success Criteria

### Measurable Outcomes

- **SC-001**: In synthetic verification, every eligible due reminder is processed during a scheduled run while the API has zero active instances.
- **SC-002**: Across repeated and concurrent synthetic runs, each reminder occasion produces no more than one history record and no more than one provider submission, including when the provider outcome is uncertain; uncertain outcomes are reported distinctly and are not automatically resubmitted.
- **SC-003**: Synthetic suppression cases produce zero provider submissions, and do not block a later eligible delivery for the same reminder occasion.
- **SC-004**: Boundary cases, including exact due instants and Sydney daylight saving transitions, produce no missed or duplicated synthetic reminders.
- **SC-005**: In implementation review, configuration and automated checks confirm that the trigger rejects unauthenticated or unauthorized identities before reminder work begins, and the old timer is disabled whenever the replacement schedule is enabled. A separately authorized rollout review verifies this across active revisions before enabling sends.
- **SC-006**: An operator can follow the documentation to configure or roll back the schedule without enabling always-on compute or sending a real member notification.

## Assumptions

- The existing reminder policy remains 24-hour and 12-hour session reminders plus 6-hour RSVP deadline alerts for this feature; issue #81 may change policy in its later notification redesign release.
- Issue #96 establishes or updates durable deduplication and boundary handling; this feature consumes that shared behavior rather than creating a second identity scheme.
- The hourly schedule covers missed executions until each reminder's useful cutoff: session reminders expire when the session starts and deadline alerts expire when the RSVP deadline passes. Issue #96 and planning define the persisted coverage cursor/identity behavior so retries and overlapping runs share a consistent interval.
- Scheduled execution may be temporarily unavailable during development and rollout; restoring reminders is not a prerequisite for changing the API's billing mode.
- The current notification service remains the authority for policy checks and push delivery, while device registrations are preserved.
- The preferred operational shape is an on-demand scheduled worker. The detailed implementation choice is for planning, provided it satisfies private authentication, zero-scale API independence, and no always-on compute.
- An external push provider cannot prove that a device displayed a notification. If provider acceptance is uncertain after a request, the durable delivery contract favors duplicate avoidance: report the uncertain outcome and do not automatically resubmit it. This can lose a reminder in a crash window; planning must ensure this does not conflict with issue #81's per-device retry contract.
- Cutover deliberately permits a paused interval: disable the old sender across revisions, verify it is inactive, then enable the external schedule. Rollback disables the external schedule first and does not automatically revive old timers.
- Infrastructure deployment and live notification sends require separate authorization and are outside this specification.
