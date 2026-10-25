# Feature Specification: Clear and Reliable Notifications

**Feature Branch**: `codex/notification-delivery`; planning artifacts only

**Created**: 2026-10-13

**Status**: Revised draft; existing registrations preserved, forward-only policy confirmed; sign-out, deadline normalization and reversal behavior confirmed

**Input**: Replace misleading member notification status; simplify controls to device setup only; send RSVP reminders at 96 and 60 hours before play with a 48-hour deadline; notify session charges and resulting low/negative balance in one message; retain manual balance nudges and announcements; remove waitlist notifications; prepare separately deliverable work.

## Confirmed Decisions

- Members enable or disable push on a device. There is no account-wide or per-category switch in the new experience; all four categories are included when a device is enrolled.
- RSVP closes exactly 48 elapsed hours before the resolved session start. Reminders are due 96 and 60 elapsed hours before that start, rather than calendar-day offsets.
- The preferred user-facing name is **Session charges**. One notification combines a session charge with any applicable balance warning.
- A balance below $0 is negative; a balance from $0 inclusive to below $20 is low; exactly $20 receives no balance warning.
- Push is the only delivery channel in this feature. Invitations remain a separate, explicitly initiated email workflow.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Understand and control push setup (Priority: P1)

A member sees whether this device is set up for push and can enable or disable it. An admin sees whether a member has any registered push devices, rather than mistaking saved preference defaults for device setup.

**Why this priority**: Corrects the misleading status and removes unnecessary choices.

**Independent Test**: With a member whose old preferences are true and who has no devices, the admin sees “No push devices.” Enabling a supported device changes setup status; disabling it remains effective across reload, sign-in and another open tab.

**Acceptance Scenarios**:

1. **Given** no registered devices, **When** an admin views the member, **Then** the member is not labelled as having push set up, regardless of legacy flags.
2. **Given** one or more enrolled devices, **When** an admin views the member, **Then** the status is “Push set up” with a device count, without claiming current browser permission or successful delivery.
3. **Given** a supported browser, **When** the member explicitly enables this device and registration succeeds, **Then** this device is enrolled for all four categories and no category switches appear.
4. **Given** two enrolled devices, **When** the member disables one, **Then** the other remains enrolled and the disabled device is not silently re-enrolled.
5. **Given** denied permission, unsupported push or failed registration, **When** setup is inspected, **Then** the member sees the applicable state and a recovery action, never a success state based on a flag alone.
6. **Given** an existing device registration, **When** the new policy is released, **Then** its token remains registered and becomes eligible for all four categories without user action, regardless of old account/category flags.

### User Story 2 - Open the relevant notification destination (Priority: P1)

A member clicks a notification and reaches the relevant session, split, balance or announcement, whether Rally is open or closed.

**Why this priority**: Messages are actionable only if their destination works.

**Independent Test**: Open each notification category from both a visible Rally tab and a background system notification; repeat while signed out.

**Acceptance Scenarios**:

1. **Given** a session-charge notification, **When** clicked, **Then** the member reaches that session’s existing “See the split” page directly.
2. **Given** an RSVP reminder, **When** clicked, **Then** the relevant session opens with its current RSVP controls.
3. **Given** a manual balance nudge or announcement, **When** clicked, **Then** the member’s money page or the exact announcement opens respectively.
4. **Given** a signed-out member, **When** a protected destination is opened, **Then** sign-in returns them to that destination after membership checks.
5. **Given** a missing or unavailable destination, **When** clicked, **Then** an accessible explanation and a relevant fallback appear; an unrelated page is not silently substituted.

### User Story 3 - Receive RSVP reminders at useful times (Priority: P1)

An approved member who has not responded receives reminders at 96 and 60 hours before a session. They stop once the member responds.

**Why this priority**: The admin needs responses before planning and the deadline.

**Independent Test**: Move a controlled clock across both due times and the deadline; exercise no response, IN, OUT, waitlisted, cancellation and a changed start time.

**Acceptance Scenarios**:

1. **Given** an open upcoming session and no RSVP, **When** the 96-hour point is reached, **Then** one first reminder becomes due.
2. **Given** the member still has no RSVP, **When** the 60-hour point is reached, **Then** one second reminder becomes due.
3. **Given** any RSVP status including OUT or waitlisted, **When** either reminder is evaluated, **Then** no reminder is sent.
4. **Given** a response after a reminder was queued, **When** delivery is about to occur, **Then** the reminder is suppressed.
5. **Given** the 48-hour deadline, **When** it is reached, **Then** existing deadline restrictions apply from that instant, retaining existing admin exceptions and late-RSVP rules.
6. **Given** cancellation, removal, a changed session time or an expired reminder, **When** delivery is attempted, **Then** stale work is suppressed or rescheduled as appropriate.
7. **Given** repeated or concurrent scheduled checks, **When** the same reminder is found, **Then** only one logical reminder exists per member, session and stage.

### User Story 4 - Understand session charges and balance (Priority: P1)

Every charged member receives their total session charge. A balance warning, when applicable, is included in that same notification and opens the split.

**Why this priority**: A committed charge should be visible and understandable without duplicate alerts.

**Independent Test**: Split a session with standard/extra-hour participants and hosted guests; check exact charges and post-split balances across negative, zero, $19.99 and $20.00 cases.

**Acceptance Scenarios**:

1. **Given** a successfully committed split, **When** notifications are prepared, **Then** each charged member receives one logical notification of their total charge, including all their charge bands and hosted guests.
2. **Given** a post-split balance below $0, **When** the message is composed, **Then** it gives the charge, balance and negative-balance top-up wording.
3. **Given** a balance from $0 through $19.99, **When** composed, **Then** it gives the charge, balance and low-balance wording; at $20.00 or above there is no warning.
4. **Given** a failed split, **When** the operation rolls back, **Then** no charge notification exists.
5. **Given** repeated confirmation requests or a restart immediately after commit, **When** work resumes, **Then** no charge or logical notification is duplicated or lost.
6. **Given** a member who did not participate in the split, **When** it completes, **Then** they receive no charge/balance alert from that split.
7. **Given** delivery delayed by another account movement, **When** the message is eventually sent, **Then** it identifies the balance as the balance after this split, rather than claiming it is the current balance.

8. **Given** a split is reversed, **When** the reversal commits, **Then** included members receive one correction event, any pending original receipts are suppressed, and re-settlement creates a new distinct receipt.

### User Story 5 - Send a useful manual balance nudge (Priority: P2)

An admin can nudge an approved member below the same threshold using the same balance classification and wording used after a split.

**Why this priority**: Reuses one understandable rule for automatic and manual reminders.

**Independent Test**: Nudge negative, zero, low and $20-or-higher balances; repeat concurrently and inside the existing 24-hour manual-nudge cooldown.

**Acceptance Scenarios**:

1. **Given** a balance below $20, **When** an approved admin explicitly nudges the member, **Then** one balance notification is created using the balance at the action and linking to the member’s money page.
2. **Given** a balance of $20 or more, **When** nudged, **Then** the request explains that no nudge is needed.
3. **Given** repeated clicks or another admin’s simultaneous nudge, **When** processed, **Then** the existing 24-hour manual-nudge cooldown prevents another logical nudge; automatic charge messages do not consume that cooldown.
4. **Given** no push devices or a notification pause, **When** the admin reviews the action/result, **Then** it does not claim a push was delivered.

### User Story 6 - Publish announcements without notification clutter (Priority: P2)

An admin publishes an ad hoc announcement to approved members. Session cancellations continue to produce announcements. Waitlist promotions still happen but no longer produce notifications.

**Why this priority**: Retains essential club communication while reducing categories.

**Independent Test**: Publish and retry an announcement; cancel a session; promote a waitlisted member and check that no new waitlist notification exists.

**Acceptance Scenarios**:

1. **Given** an approved admin’s announcement, **When** published, **Then** every approved member in the publication audience receives one logical announcement with a link to that announcement.
2. **Given** an unapproved/removed member, **When** announcement delivery is evaluated, **Then** they do not receive it.
3. **Given** a valid cancellation, **When** committed, **Then** the existing cancellation announcement remains available and uses the shared delivery mechanism.
4. **Given** a waitlist promotion, **When** a place becomes free, **Then** existing promotion behavior remains but no new waitlist notification is created or sent.
5. **Given** historical waitlist notifications, **When** history is viewed, **Then** existing history is retained.

### Edge Cases

- A member can have no devices, several devices, revoked devices or invalid provider registrations; device counts must describe active registrations only.
- Disable/reload and multiple tabs must not cause silent enrollment. Account switching must prevent subsequent alerts to that browser for the former account. Ordinary sign-out retains registration; explicit disable revokes it. Switching accounts retires the former account’s registration on that browser.
- Existing registrations are preserved regardless of legacy master/category flags. No preference-based device revocation or mandatory re-enrollment occurs at rollout.
- Durations are elapsed hours from the resolved start; Sydney daylight saving must not shift the 96/60/48-hour instants.
- On rollout, already-closed RSVP windows are not reopened. All still-open future windows adopt 48 hours, including custom deadlines; historical and cancelled sessions retain their deadlines.
- A session created late or an outage spanning both reminders must not cause a burst of both stages. Only the latest still-useful stage is eligible.
- A member who enables a device later receives future alerts; old suppressed work is not replayed as a backlog.
- A provider may accept a message immediately before a worker crashes. Delivery is best-effort with duplicate mitigation, not a guarantee that the member saw it exactly once.
- A deployment stop or club pause blocks delivery. Reminder work expires; paused work is not replayed automatically as a burst on unpause.
- A charge notification must not be produced for historical imports or already-settled sessions at rollout.
- A zero-cost split still produces one receipt for each included member, with any balance warning based on the post-split position.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Member status MUST distinguish push setup from permission, preferences and delivery; no registered device means no push setup.
- **FR-002**: Members MUST have device enrollment/removal controls only, covering all four categories. Removal MUST persist and must not remove other devices.
- **FR-003**: Rollout MUST preserve existing device registrations without re-enrollment, disregard legacy account/category opt-outs, and preserve membership checks, operational notification stops, private token handling and explicitly initiated invitation behavior.
- **FR-004**: RSVP reminders MUST target approved members with no response at 96 and 60 elapsed hours before the session start; eligibility MUST be checked again immediately before delivery.
- **FR-005**: The RSVP deadline MUST become 48 elapsed hours before the session start for new sessions and all still-open future windows, including custom deadlines, without reopening closed windows or modifying history.
- **FR-006**: Session-charge messages MUST summarize committed total charges and balances at that split, and MUST open the split directly.
- **FR-007**: Balance classification MUST use negative below zero, low from zero to below 2000 cents, and no warning at or above 2000 cents. Automatic alerts MUST be combined with the charge message.
- **FR-008**: Manual nudges MUST reuse balance rules and wording, retain the 24-hour manual cooldown, and open the member’s money page.
- **FR-009**: Admin announcements MUST retain a publication audience of approved members and an exact destination; cancellations MUST remain announcements.
- **FR-010**: New waitlist notifications and old attendance/deadline reminder schedules MUST be retired without changing waitlist promotion or deleting history.
- **FR-011**: Notification intent MUST survive application restarts and MUST be committed atomically with its originating action where that action writes data. Provider failures MUST NOT undo posted money.
- **FR-012**: Repeated/concurrent triggers MUST produce one logical notification per recipient and originating action/stage; transient delivery failures MUST receive bounded retries, excluding already accepted devices.
- **FR-013**: Members MUST be able to read their own notification history even without push setup; history creation and delivery are separate outcomes.
- **FR-014**: Foreground, background and history clicks MUST use the same destination rules and preserve authorized destinations through sign-in.
- **FR-015**: Admin feedback MUST distinguish queued, suppressed, failed/uncertain and provider-accepted work; no status MUST claim confirmed device or inbox delivery.
- **FR-016**: Adding a future channel MUST not require changing business triggers. Contact details alone MUST not imply delivery consent or availability; future channels are outside this release.
- **FR-017**: Automated regression coverage MUST verify scheduling, durable delivery/concurrency, setup removal, rollout, routing and unchanged settlement/waitlist behavior.

### Key Entities

- **Device enrollment**: One member’s explicitly enrolled browser/device, its delivery address and registration lifecycle. Revoked devices do not count as set up.
- **Notification event**: A recipient’s category, originating action, immutable event facts, destination and unique logical identity.
- **Delivery attempt**: A device/channel-specific attempt with retry time, outcome and provider acceptance; separate from whether the member read the event.
- **Reminder stage**: First or second reminder for a session, calculated from its current resolved start and invalidated by relevant changes.
- **Announcement**: Existing admin publication, content and publication audience; cancellation is a subtype/source.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All no-device and legacy-preference fixtures show no push setup; no setup label claims confirmed delivery.
- **SC-002**: Members see zero category/master switches, can enroll a supported device in one explicit action, and can disable that device without affecting another.
- **SC-003**: In healthy operation, eligible reminder work is processed within five minutes after each due instant, never before it; responded, closed and cancelled cases receive none.
- **SC-004**: Every committed test split creates one logical receipt per included member, with exact committed amounts; rollback and duplicate-confirmation cases create none or no additional receipt respectively.
- **SC-005**: Every category reaches its specified destination from foreground, background and history clicks, including after sign-in.
- **SC-006**: Repeated/concurrent trigger tests create zero duplicate logical events; restart tests leave zero committed actions without their associated notification intent.
- **SC-007**: No new waitlist or retired attendance-reminder notification is created after its cutover.

## Assumptions

- “Not RSVP’d” means no RSVP row: IN, OUT and waitlisted all count as responses. Existing late-response/admin exceptions are retained.
- Cancellations remain important club announcements; they do not add a fifth notification category.
- $20 is the fixed notification threshold in this feature, shared by automatic and manual paths. Existing threshold reporting must use the same value so the UI cannot contradict the rule.
- All existing device registrations carry forward unchanged regardless of saved account/category flags. New installation metadata attaches automatically when the browser returns; it never requires a new permission prompt. Old preference behavior is retired; schema additions are forward-only and repeat-safe.
- Notification history remains available without devices. Paused/disabled operation records event history and suppressed delivery intent; it does not send, and does not replay suppressed work automatically.
- Ordinary sign-out retains push registration. Only explicit disable, account switching on the same browser, or a provider-confirmed invalid registration retires a device.
- Reversing a split creates one correction event per included member under Session charges, atomically with the reversal. Pending receipts for the reversed split are suppressed; historical receipts are retained and their destinations identify the original settlement.
- Reminder usefulness expires at the next stage or the 48-hour deadline. Rollout marks stages already due as skipped; later stages remain eligible.
- New session forms use the 48-hour deadline without a custom-deadline control. All still-open upcoming windows adopt the new default, including previously custom deadlines; already-closed windows stay closed. A later explicit change to an open session’s date/start converts it to the new default with a visible preview.
- Other event delivery expires 24 hours after creation. Manual nudges re-check current eligibility before delivery; their message states when the quoted balance was checked.
- Proposed operational defaults and migration rules are documented in the plan for review; the three primary product choices above were explicitly confirmed.
- No application changes, production messages or cloud-resource changes are authorized by this planning work.
