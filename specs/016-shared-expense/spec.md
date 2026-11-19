# Shared expenses

Issue: #123, "Ability to split a non-session expense such as a meal or coffee".

An admin can record a shared expense: a bill that is not a badminton session, such as a meal or a coffee, split between approved members. A shared expense has a description, a date, a total, a payer, a split method and a list of participants. It has no link to a session and uses no hours, shuttles or court credit.

## Terms

- **Shared expense**: one non-session bill split between members. Ledger transaction kind `shared_expense`.
- **Payer**: who paid the venue. Either one approved member or the club bank.
- **Participant**: an approved member who owes a share of the bill.
- **Share**: the integer-cent amount one participant owes.
- **Split method**: `equal`, `exact` or `shares`.

## Ledger

A shared expense is one ledger transaction of kind `shared_expense`, posted by `LedgerService`. No new table is added. The transaction carries the description and the occurred-at date; the entries carry each share. The ledger holds only the resulting amounts. The submission itself (total, payer, split method and every participant, including a zero-net one) is kept in `money_operations.canonical_payload`, linked from the transaction by `money_operation_id`. That payload holds the inputs, not the computed shares, and is the source for any later migration to spec 014's tables, so it must be kept.

- **Member pays.** Each participant's player account is debited by their share. The payer's player account is credited with the total. Club assets do not change, so the club-position identity holds because the player movements sum to zero.
- **Club bank pays.** The bank is decreased by the total and each participant's player account is debited by their share. Assets and player balances fall by the same amount.

When the paying member is also a participant, their account gets a single net entry (total minus own share), not two entries. Example: a member pays $80 for four people split equally; the payer gets one entry of +$60 and the other three get −$20 each. A payer whose net is zero (sole participant) is refused as there is nothing to post.

A share may take a member's balance below zero, as session charges do.

## Split arithmetic

The arithmetic lives in `internal/services/money` and touches no database. All amounts are integer cents.

- **Equal**: the existing largest-remainder split across the participants.
- **Exact**: the admin gives an amount for each participant. Every amount must be greater than zero and the amounts must sum to the total exactly, otherwise the expense is refused.
- **Shares**: the admin gives a whole-number weight of at least 1 for each participant. Each share is the total in proportion to the weight, with largest-remainder allocation of the spare cents, so the shares always sum to the total exactly.

Remainder allocation must be deterministic for a given input so a preview and its confirmation agree.

## Service and validation

A new `SharedExpenseService` owns the rules and hands movements to `LedgerService`. It refuses the expense unless:

- the actor is an approved admin;
- the description is non-empty, at most 200 characters, and has no control characters;
- the total is greater than zero and at most $10,000, and large enough that every participant owes at least one cent;
- the date is not in the future (Australia/Sydney) and not more than 365 days ago, because a backdated entry re-orders every later running balance of its members;
- there are between 1 and 100 participants and each appears once;
- every participant is an approved member (removed and pending members are refused). An invited member who has not signed in yet is an approved, chargeable row, as for session charges, and is accepted;
- a member payer is an approved member;
- the split inputs match the chosen method and reconcile with the total;
- club assets are not pending confirmation, matching session expenses.

`Preview` returns each participant's share, the payer's net movement and a fingerprint of the canonical input. `Confirm` requires that fingerprint and refuses when it no longer matches, so the figures posted are the figures shown.

`Confirm` also requires a `request_id` chosen by the client and runs through the reviewed-operation runner from spec 014 (`runMoneyOperation`, action `create_shared_expense`). The runner reserves the ID, so repeating a submission returns the original transaction and charges nobody twice, and reusing an ID for different contents is refused with `request_conflict`. It SHARE-locks every member involved before the membership check, so a member cannot be removed between that check and the posting. The form keeps one ID per reviewed bill, keyed by the bill's contents: it reuses the ID when a save is retried, even after an Edit that changed nothing, and takes a new one only when the contents differ. A retry that arrives after another admin reversed the expense still returns the original result; the form does not detect that case. Nothing guards against two different submissions of the same bill: that is a legitimate case (two identical coffees on one day).

A bank-paid expense may take the bank below zero. This matches withdrawals and spec 014 FR-016, which introduces no non-negative bank rule.

## API

Both routes sit on the `/api/admin` group.

- `POST /api/admin/expenses/preview`
- `POST /api/admin/expenses`

## Frontend

Admins see a "Record expense" button on `/money`. It opens a form with description, date (default today), total, payer (a member or the club bank), split method and the member list. Choosing a member as payer selects them as a participant by default; the admin can clear them. Exact and shares methods show an amount or weight field beside each selected participant. A preview panel shows every share before the admin confirms, and any change to the form discards the preview.

In the ledger lists, a `shared_expense` row shows an icon chosen from its description: a cup for coffee words (such as "coffee", "cafe"), cutlery for meal words (such as "meal", "dinner", "lunch", "breakfast", "brunch"), and a receipt otherwise. Matching ignores case. No category is stored.

The ledger history query already derives a `category` for every row, and `LedgerList` already maps `food` to a cutlery icon and `other` to a receipt. The rule therefore lives in that query, beside the existing Splitwise description rules, not in a new frontend function: coffee words give a new `coffee` category, meal words give `food`, anything else gives `other`. The frontend only gains the `coffee` entry in its category map.

## Corrections

An admin corrects a shared expense by reversing its transaction with the existing reversal path and recording a new one. There is no edit in place.

The reversal path existed only as an API (`POST /api/admin/transactions/:id/reverse`); no screen offered it for a general transaction. Two screens now do, for shared expenses only:

- the confirmation screen carries a "Reverse this expense" action for the expense just recorded;
- in the ledger, an admin sees a "Reverse" action on every shared expense row that has not been reversed, however old. It asks first, because it undoes the whole expense for every member and notifies them. Ledger rows carry their `transaction_id` for this.

Other transaction kinds get no reverse action in the ledger. Top-ups, withdrawals, settlements and asset purchases have their own correction rules (spec 014 FR-025), and a general undo button would bypass them. A reversal suppresses any push or email of the original expense that has not been sent yet. The original notice stays in each member's notification history, where it cannot be withdrawn, so the reversal also sends every affected member a "Shared expense reversed" notice with their balance.

## Notifications

Each participant receives a money notification with their share and new balance. A paying member receives a credit notification. These use a new `shared_expense` notification type, because the existing charge notification names a session and opens its settlement; a shared expense notification opens `/money`. A payer who is also a participant receives one notification for the net amount.

## Validation

Browser tests (`frontend/e2e/shared-expense.spec.ts`) cover the admin flow on desktop and mobile. Tests cover: all three split methods summing to the total at every headcount; both payer types; the net single entry for a participating payer; the club-position identity; each refusal above; reversal of a shared expense; the fingerprint check; the admin form and preview; and the icon function.

## Relation to spec 014

`specs/014-member-transaction-corrections` already specifies shared expenses, and its entry screen (its PR 4) was not built. This change delivers a smaller version and differs from it on purpose in these places. Each is a decision taken for issue #123 and should be reconciled before 014's remaining work starts:

- **Notifications.** 014 FR-026 says shared expenses send no push or email. This change sends them. The club owner confirmed this on 2026-11-19 with both specifications in view, so FR-026 is superseded for shared expenses.
- **Storage.** 014 adds `shared_expense_versions` and `shared_expense_allocations`. This change stores only the ledger transaction, so paid and owed amounts are not kept separately and a zero-net member has no row.
- **Payers.** 014 FR-003 allows several member payers. This change allows one.
- **Category.** 014 FR-002 stores a Food/Other category. This change derives the icon from the description.
- **Correction.** 014 FR-010 to FR-012 require an in-place correction and a reviewed reversal with a reason. This change offers a confirmed reversal without a typed reason, and no in-place correction.
- **History.** 014 FR-020 and FR-021 group an expense's rows and add a Shared expenses filter. This change shows ordinary ledger rows.
- **Split methods.** This change adds weighted shares, which 014 does not have.
- **Member view.** 014 FR-001 lets every approved member read an expense breakdown. Here a member sees only their own ledger row.
- **Review.** 014 FR-006 shows Paid, Share, Change and Current to New balance for each member, and states that recording moves no money. The preview here shows shares and the payer's credit only. Resulting debt is not shown (FR-016), and the fingerprint binds the shares, not the resulting balances (FR-018).
- **History detail.** 014 FR-019 keeps actor, reason and recording time with links between records. Here history shows the ledger rows and a Reversed flag.
- **Reversal date.** 014 FR-022 dates the inverse at the original occurrence. The existing reversal path used here dates it now, so a backdated expense that is reversed stays in the running balances between the two dates.
- **Reversal record.** A reversal made here has no reason and no `money_operations` row, both of which 014 FR-012 requires.

It does follow 014 for FR-001 (admin only), FR-005 (movements), FR-007 (approved members), FR-016 (identity, debt allowed), FR-017 (safe retry) and FR-023 (excluded from badminton spend).

## Outside this change

Guests, a link to a session, edit in place, assistant (voice or text) support, a stored category, and recording by non-admin members.
