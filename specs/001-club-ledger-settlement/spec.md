# Feature Specification: Club Ledger and Session Settlement

**Feature Branch**: `001-club-ledger-settlement`

**Created**: 2026-08-24

**Status**: Draft

**Input**: User description: "Club ledger and session settlement for Rally. Replace the club's use of Splitwise with a proper account and ledger per player, plus a way to record what each session cost and charge the players who were there."

## Overview

The club currently tracks who owes what in Splitwise. It is a poor fit: the free tier
limits how many entries can be recorded in a day, the split rules do not match how the
club actually shares costs, and — most importantly — it cannot represent the fact that
most of the club's money is not cash.

Club money sits in three places, only one of which is a bank balance:

- cash in the bank
- prepaid court-hire credit held at the venue, bought in fixed $100 blocks
- the value of shuttles bought in advance and sitting in the admin's bag

Looking at any one of these tells you nothing about whether the club is square with its
players. This feature replaces Splitwise with a ledger that tracks all three, alongside a
balance for every player, and a settlement flow that turns a night of badminton into an
accurate charge for each person who played.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Keep the books without Splitwise (Priority: P1)

Every player has an account. Players put money in by transferring it to the club, and the
admin records the top-up. Any member can open the app and see what everyone's balance is,
and drill into their own history to see every top-up and charge that produced it.

**Why this priority**: This alone replaces Splitwise. Even with no session costing, the
club can record money in and see who is in credit, which is the daily pain today.

**Independent Test**: Seed opening balances, record a handful of top-ups, and confirm that
every member sees the same balances and that each player's own history reconciles to their
balance.

**Acceptance Scenarios**:

1. **Given** a newly approved member with no history, **When** they open the money view,
   **Then** they see their own balance as zero and the balances of all other members.
2. **Given** a player transfers $50 to the club, **When** the admin records a $50 top-up
   against that player, **Then** the player's balance increases by exactly $50 and a
   corresponding entry appears in their history.
3. **Given** the club is going live, **When** the admin records opening balances carried
   over from Splitwise, **Then** each player's balance matches the figure carried over and
   the entries are marked as opening balances.
4. **Given** the admin records a top-up against the wrong player, **When** they reverse
   that transaction, **Then** both players' balances are correct and both the original
   entry and its reversal remain visible in history.
5. **Given** any recorded transaction, **When** anyone attempts to edit or delete it,
   **Then** the system refuses; only a reversing transaction can change the outcome.

---

### User Story 2 - Settle a night's play (Priority: P2)

After a session, the admin opens the settlement form. It lists everyone who said they were
coming, pre-ticked for the standard hours and — when play ran long — for the extra hour
too. The admin unticks whoever went home early, adds any walk-ins or guests, confirms the
rates, and settles. Each person's charge is worked out and applied to their account, and
the session's cost is drawn down from court credit and shuttle stock.

**Why this priority**: This is the other half of replacing Splitwise, and it is the part
Splitwise gets wrong. It depends on the accounts from User Story 1 existing.

**Independent Test**: Settle a session with a mix of full-night players, early leavers, a
guest, and a no-show; confirm each charge is correct, the charges sum exactly to the
session cost, and the club's assets are drawn down by the same total.

**Acceptance Scenarios**:

1. **Given** a session where everyone played the standard hours only, **When** the admin
   settles it, **Then** the cost of the standard hours is divided equally among them and
   the individual charges sum to exactly the session cost.
2. **Given** a session that ran an extra hour where only some players stayed, **When** the
   admin settles it, **Then** players who left are charged only for the standard hours,
   and the extra hour's cost is divided among only those who stayed.
3. **Given** a player who said they were coming but did not turn up, **When** the admin
   settles without changing their line, **Then** they are charged in full, because the
   court was booked for them.
4. **Given** a guest played as the admin's guest, **When** the admin settles, **Then** the
   guest counts as a head in the split, the guest's share is charged to the admin's
   account, and the guest is named in the session history.
5. **Given** a session cost that does not divide evenly among the players, **When** the
   admin settles, **Then** the individual charges still sum to exactly the session cost,
   with the odd cents distributed rather than absorbed or invented.
6. **Given** a settled session, **When** the admin attempts to settle it again, **Then**
   the system refuses; correcting it requires reversing the settlement first.
7. **Given** a settled session, **When** any member views session history, **Then** they
   see the date, times, duration, rates used, shuttles consumed, total cost, who played,
   which parts of the night each person was there for, and each person's charge.
8. **Given** club rates are changed after a session was settled, **When** anyone views
   that session's history, **Then** it still shows the rates that were in force when it
   was settled.
9. **Given** a participant is marked as comped, **When** the admin settles, **Then** every
   other participant is charged exactly what they would have been charged had the comped
   player paid, the comped player's line shows zero, and the waived share appears against
   the club's surplus.
10. **Given** the session would consume more shuttles than are recorded in stock, **When**
    the admin settles, **Then** settlement stops and offers to record the missing purchase
    without leaving the form, and completes once stock covers the session.

---

### User Story 3 - Know the club's true position (Priority: P3)

The admin needs to answer one question: is the club square with its players? That means
totalling the bank, the credit left at the venue, and the value of shuttles still in the
bag, and comparing that against what players have prepaid. The same view warns when the
court credit will not cover the next session and when shuttles are running low.

**Why this priority**: Valuable but not blocking. The underlying accounts exist from
User Story 1; this story is the view that makes them legible, plus the forms for recording
asset purchases.

**Independent Test**: Record a court-credit top-up and a shuttle purchase, settle a
session, and confirm the position view reconciles and that the low-credit warning fires
when remaining credit is less than the next session's cost.

**Acceptance Scenarios**:

1. **Given** the admin buys a $100 block of court credit, **When** they record it,
   **Then** the bank falls by $100 and court credit rises by $100, with no change to any
   player's balance.
2. **Given** the admin buys shuttles, **When** they record the purchase with a quantity
   and a price, **Then** shuttle stock rises by both that quantity and that value, and the
   club's total position is unchanged.
3. **Given** shuttles were previously bought at a different price, **When** a new purchase
   is recorded, **Then** the value of existing stock is not altered and the cost applied to
   future sessions reflects the blended cost of what is actually in the bag.
4. **Given** a session has been settled, **When** the admin views the position, **Then**
   the bank, court credit, and shuttle stock together equal what players have prepaid plus
   the club's surplus.
5. **Given** the credit remaining at the venue is less than the cost of the next scheduled
   session, **When** the admin views the position, **Then** they are warned to top up.

---

### User Story 4 - Nudge players who are running low (Priority: P4)

Players who are running out of credit get told, so the admin does not have to chase them.
The nudge arrives on the back of a settlement, when the charge that pushed them low is
fresh and the message can say what the night cost them.

**Why this priority**: A convenience on top of a working ledger. Nothing depends on it.

**Independent Test**: Settle a session that pushes players across each threshold and
confirm the right notification is sent to the right people, honouring their existing
notification preferences.

**Acceptance Scenarios**:

1. **Given** a session is settled and a participant's resulting balance is positive but
   below the low-balance threshold, **When** the settlement completes, **Then** they
   receive a low-balance notification stating what the session cost them and their new
   balance.
2. **Given** a session is settled and a participant's resulting balance is negative,
   **When** the settlement completes, **Then** they receive a negative-balance
   notification.
3. **Given** a player has turned off notifications of that kind, **When** a settlement
   would notify them, **Then** they are not notified.
4. **Given** a participant's resulting balance is comfortably positive, **When** the
   settlement completes, **Then** they are not notified.
5. **Given** a member whose balance is low but who did not play in the settled session,
   **When** the settlement completes, **Then** they are not notified.
6. **Given** a settlement notifies a participant, **When** the same settlement is
   processed, **Then** that participant receives no more than one balance reminder from
   it.

---

### Edge Cases

- **Settling with insufficient shuttle stock.** The recorded stock is fewer shuttles than
  the session consumed, because a purchase was never recorded. Settlement stops and offers
  to record the missing purchase without leaving the form, then proceeds. Stock is never
  driven negative, so the cost of a shuttle is always derivable from stock that exists.
- **Settling with insufficient court credit.** The venue account does not hold enough to
  cover the session. Credit is allowed to go negative and the position view flags it,
  because the play already happened and the ledger must record reality.
- **A comped player.** A player who was there but is not being charged. They still count as
  a head in the split, so every other participant pays exactly what they would have paid
  anyway, and the waived share is absorbed by the club's surplus. Comping is the club's
  decision and the club bears it.
- **Nobody is charged.** A session is settled with every line unticked — a washout, or a
  night the admin covered. The session's cost is drawn from club assets and absorbed by
  surplus; no player is charged.
- **A single player.** A session settled with one participant charges them the whole cost.
- **A session with no extra hour.** The extra band contributes nothing; only the standard
  hours are charged.
- **Reversing a settlement after players have since topped up.** Balances unwind correctly
  because reversal posts opposite entries rather than recalculating from a snapshot.
- **A player leaves the club in credit.** Their balance and history are retained; the admin
  settles up out of band and records a withdrawal.
- **A walk-in exceeds the session's stated capacity.** Settlement records what actually
  happened and does not enforce the RSVP capacity limit.
- **Reversing a settlement that consumed shuttles.** Both the value and the unit count
  return to stock.

## Requirements *(mandatory)*

### Functional Requirements

#### Accounts and ledger

- **FR-001**: The system MUST hold a distinct account for every player, and separate
  accounts for the club's bank, its prepaid court credit, its shuttle stock, and its
  surplus.
- **FR-002**: The system MUST keep the admin's player account separate from the club's
  accounts, so that the admin tops up their own balance like any other player.
- **FR-003**: Every movement of money MUST be recorded as a transaction whose entries
  balance across accounts.
- **FR-004**: The system MUST maintain, after every transaction, the identity that the
  bank plus court credit plus shuttle stock equals the sum of player balances plus the
  club's surplus.
- **FR-005**: Recorded ledger entries MUST NOT be editable or deletable. Corrections MUST
  be made by posting a reversing transaction that references the original.
- **FR-006**: The system MUST support these transaction kinds: player top-up, withdrawal
  or refund, court-credit purchase, shuttle purchase, session settlement, opening balance,
  and reversal.
- **FR-007**: Only admins MUST be able to create, reverse, or otherwise post transactions.
- **FR-008**: The system MUST allow opening balances to be recorded once per account when
  the club goes live, covering player balances, shuttle stock, and court credit.
- **FR-009**: Player balances MUST be permitted to go negative. No action in the app is
  blocked by a player's balance.

#### Money and precision

- **FR-010**: All monetary amounts MUST be exact to the cent. No calculation may lose or
  invent value.
- **FR-011**: Splitting a cost MUST distribute the remainder so that the individual charges
  sum to exactly the amount being split.
- **FR-012**: Per-shuttle cost MUST be derived at the moment of consumption from the value
  and quantity of stock on hand, never from a stored rounded per-unit price. A $50 tube of
  twelve is 416.66... cents per shuttle and MUST NOT be rounded before it is used.
- **FR-013**: Shuttle stock MUST be tracked as both a quantity and a value, so that
  purchases at different prices blend correctly and stock remaining can be reported.

#### Session costing and settlement

- **FR-014**: A session's cost MUST be composed of two bands: the standard hours, and an
  optional extra period. Each band has its own hourly court rate and its own shuttle
  consumption.
- **FR-015**: Each band's total cost MUST be divided equally among only the participants of
  that band. A participant of the standard hours who did not stay MUST NOT contribute to
  the extra period's cost.
- **FR-016**: The system MUST hold club-level defaults for the standard hours, the standard
  hourly rate, the extra hourly rate, and shuttles consumed per hour, and MUST allow each
  to be overridden when a session is settled.
- **FR-017**: The rates, hours, and consumption actually used MUST be recorded on the
  settlement, so that later changes to club defaults never alter a settled session.
- **FR-018**: The settlement form MUST default to charging everyone who indicated they were
  attending, for every band of the session.
- **FR-019**: The admin MUST be able to remove a participant from the extra band, remove
  them from the session entirely, add a participant who never indicated attendance, and add
  a named guest whose charge falls to a hosting member's account.
- **FR-020**: The charge lines produced by settlement MUST serve as the record of who
  played, including lines charged at zero.
- **FR-021**: A participant MUST be able to be marked as comped. A comped participant MUST
  still count as a head when dividing a band's cost, so that no other participant's charge
  is affected, and their waived share MUST be absorbed by the club's surplus.
- **FR-022**: Settlement MUST NOT reduce shuttle stock below zero. Where the session would
  consume more shuttles than are held, the system MUST stop and offer to record the missing
  purchase without leaving the settlement, then continue.
- **FR-023**: Settling a session MUST, in a single all-or-nothing operation, reduce court
  credit by the court cost, reduce shuttle stock by both the value and quantity consumed,
  reduce each charged participant's balance by their share, and post any waived shares to
  surplus.
- **FR-024**: A session MUST NOT be settled more than once. Correcting a settled session
  MUST require reversing the settlement first, and both the settlement and its reversal
  MUST remain visible.
- **FR-025**: Concurrent attempts to settle the same session MUST result in exactly one
  settlement.

#### Visibility

- **FR-026**: Any approved member MUST be able to see every player's balance.
- **FR-027**: Any approved member MUST be able to see the full breakdown of any settled
  session, including who played, which bands they were in, and what each person was
  charged.
- **FR-028**: Every member MUST be able to see their own transaction history, itemised, and
  reconciling to their current balance.
- **FR-029**: The club's asset position MUST be visible to admins only.
- **FR-030**: A member's current balance MUST be visible from anywhere in the app without
  navigating to the money section.

#### Reminders

- **FR-031**: The system MUST notify a player whose balance is positive but below the
  low-balance threshold, and separately notify a player whose balance is negative.
- **FR-032**: Both thresholds MUST be club settings.
- **FR-033**: Balance reminders MUST respect each member's existing notification
  preferences and delivery channels.
- **FR-034**: Balance reminders MUST be triggered by the settlement of a session, and MUST
  be sent only to participants of that session whose resulting balance is below a
  threshold. A player MUST receive at most one balance reminder per settlement, and
  reminders MUST NOT be sent on any other schedule.
- **FR-035**: A balance reminder MUST tell the player what the session cost them and what
  their balance is now.

#### Navigation

- **FR-036**: The app MUST provide a money section containing balances, the member's own
  history, and - for admins - the club's asset position.
- **FR-037**: The sessions area MUST separate upcoming sessions from past sessions.

### Key Entities

- **Account**: A place a balance accumulates. Either a player's account or one of the
  club's — bank, court credit, shuttle stock, or surplus. Shuttle stock additionally
  carries a quantity.
- **Transaction**: A single recorded financial event of a known kind, optionally referring
  to a session or to the transaction it reverses. Carries the description and who recorded
  it.
- **Ledger entry**: One account's part of a transaction — a signed amount, and for shuttle
  stock a signed quantity. Entries are written once and never altered.
- **Settlement**: The record of a session being costed. Holds the rates, hours, and shuttle
  consumption in force at the time, and links to the charge lines it produced.
- **Charge line**: One participant's part of a settlement — which bands they were in, what
  they were charged, and, when the line is a guest, the guest's name and the member whose
  account bears the charge.
- **Club settings**: The defaults used when settling — standard hours, standard and extra
  hourly rates, shuttles per hour, and the balance reminder thresholds.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The club stops using Splitwise entirely; every balance and charge lives in
  Rally.
- **SC-002**: The admin can settle a completed session in under two minutes when nobody
  left early, and under three minutes with early leavers and a guest.
- **SC-003**: For every settled session, the sum of individual charges equals the session's
  total cost exactly, to the cent, with no exceptions.
- **SC-004**: At any moment, the club's combined bank, court credit, and shuttle stock
  equals the total of player balances plus surplus.
- **SC-005**: A member can determine their current balance within five seconds of opening
  the app, without navigating to a separate screen.
- **SC-006**: Any member can reconstruct how their balance was arrived at from their own
  history, with no unexplained movements.
- **SC-007**: The admin no longer needs to chase players individually about topping up;
  players below threshold are notified automatically.
- **SC-008**: A mis-recorded transaction can be corrected without any prior entry being
  altered or removed.

## Assumptions

- The club plays on one court as a standing weekly booking; multi-court and multi-venue
  costing is out of scope. Rates are held as settings so this can change without code.
- Players pay the club by bank transfer out of band. There is no payment gateway; the admin
  records money received. Payment collection inside the app is out of scope.
- Guests do not have accounts and do not sign in. A guest is a name on a charge line, and
  the hosting member settles with them privately.
- Every member of the club can see every other member's balance. This matches what
  Splitwise already exposes and the club is comfortable with it.
- Balances may go negative. Nothing in the app — RSVP included — is gated on balance.
- Settlement records what actually happened and therefore does not enforce the session's
  RSVP capacity limit.
- Court credit may go negative if a top-up was not recorded before a session was settled.
  The position view flags it rather than blocking.
- Opening balances are recorded once, at go-live, after all members are onboarded.
- The existing notification system and each member's existing preferences are reused for
  balance reminders; no new delivery channel is introduced.
- Balance reminders are settlement-driven, so a member who stops playing is never reminded,
  however low their balance falls. This is accepted: someone who is not playing is not
  accruing charges, and the admin can see their balance on the balances screen. If members
  start drifting negative while inactive, a scheduled sweep can be added later.
- Sessions currently record a date and a start and end time separately, which is not a
  reliable basis for billing durations. Establishing a dependable session duration is a
  prerequisite of this feature.
- Currency is AUD throughout. Multi-currency is out of scope, and no tax handling is
  required.
