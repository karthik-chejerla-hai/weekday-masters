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

A shared expense is one ledger transaction of kind `shared_expense`, posted by `LedgerService`. No new table is added. The transaction carries the description and the occurred-at date; the entries carry each share. The split method itself is not stored, only the resulting amounts.

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
- the description is non-empty and at most 200 characters;
- the total is greater than zero and at most $10,000, and large enough that every participant owes at least one cent;
- the date is not in the future (Australia/Sydney);
- there are between 1 and 100 participants and each appears once;
- every participant is an approved member (removed, pending and invited-unclaimed members are refused);
- a member payer is an approved member;
- the split inputs match the chosen method and reconcile with the total;
- club assets are not pending confirmation, matching session expenses.

`Preview` returns each participant's share, the payer's net movement and a fingerprint of the canonical input. `Confirm` requires that fingerprint and refuses when it no longer matches, so the figures posted are the figures shown.

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

The reversal path existed only as an API (`POST /api/admin/transactions/:id/reverse`); no screen offered it for a general transaction. The confirmation screen therefore carries a "Reverse this expense" action for the expense just recorded. Reversing an older shared expense still needs the API, because ledger rows do not expose their transaction ID. A reversal sends no notification, as with top-ups.

## Notifications

Each participant receives a money notification with their share and new balance. A paying member receives a credit notification. These use a new `shared_expense` notification type, because the existing charge notification names a session and opens its settlement; a shared expense notification opens `/money`. A payer who is also a participant receives one notification for the net amount.

## Validation

Browser tests (`frontend/e2e/shared-expense.spec.ts`) cover the admin flow on desktop and mobile. Tests cover: all three split methods summing to the total at every headcount; both payer types; the net single entry for a participating payer; the club-position identity; each refusal above; reversal of a shared expense; the fingerprint check; the admin form and preview; and the icon function.

## Outside this change

Guests, a link to a session, edit in place, assistant (voice or text) support, a stored category, and recording by non-admin members.
