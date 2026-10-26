# Money history and frontend behavior

## Read contract

Extend `/api/accounts/activity` with `type=all|sessions|topups|shared_expenses`;
keep existing `scope=mine|all`, limit and offset behavior. The caller remains the default
personal selection; existing all-member visibility requires approved membership.
The existing `type=topup` remains an alias for `type=topups`. No new arbitrary
user-ID selector is introduced; personal scope remains the authenticated caller. `All` preserves
imports, withdrawals, opening balances and miscellaneous existing entries.

Retain the `{items,total}` envelope and existing `entry`/`game` fields. Add nullable
`shared_expense` as a third activity shape. Exactly one of entry/game/shared-expense
is present. An expense version is one expandable activity, including its entire
allocation set. Its exact inverse is a separate linked reversal activity, not an
active bill; the replacement is a new version/activity. Native reversal activities
likewise display as inverse events with whole-settlement detail, not unrelated
member rows. Existing imported regular/extra-hour grouping stays unchanged.

Add common fields: `transaction_id` (native events), `recorded_at`, `family`,
`event_kind` (`original|reversal|replacement`), `status` (`active|reversed|corrected`),
`correction_reason`, `actor_name`, `reverses_transaction_id`,
`corrects_transaction_id`, `reversal_transaction_id`, `replacement_transaction_id`,
`current_transaction_id`, and `allowed_actions`. Inactive originals expose no
mutation actions; reversals expose none. Allowed actions depend on current actor
and supported type, but service enforcement remains authoritative. Approved members
can follow detail links even if not admins. Preserve original source/category
fields for imports and raw entries. Transaction IDs and recording timestamps
currently hidden from raw JSON become additive public money fields.

Shared detail: version/root IDs, title/description, category, effective Sydney date,
bill, funding, optional session, split mode, payer summary, participant count,
caller's gross share/net change and full allocations. Each allocation has user ID,
display name, inactive flag, participates, Paid, Share, Change, Balance after.
Payer-only people remain visible. Inverse detail retains original gross values with
an explicit Undo label and inverted net changes; don't present it as new payment.
A full read includes inactive original members with the standard display-name rule.

## Ordering and running balances

Implement **one** order comparator in SQL/read builders. Ascending effective balance
order is the following tuple (descending page order is its exact reverse):

1. transaction `occurred_at`;
2. recording-group timestamp: MoneyOperation.created_at for new reviewed rows,
   otherwise the legacy ledger entry's created_at;
3. group class: legacy 0, reviewed operation 1;
4. legacy entry UUID (zero UUID for reviewed operations);
5. operation sequence (0 for legacy);
6. operation step (0 inverse/standalone, 1 replacement);
7. ledger entry UUID.

This preserves relative ordering between all legacy entries exactly, including
old asset correction rows. Reviewed inverse/replacement with the same effective
date share their operation group; sequence resolves same-recording-time operation
ties, and step guarantees inverse before replacement. Transaction/operation IDs
are stable, and sequences need not be gapless. Use actual wall-clock recorded time
for new groups, not a clock value frozen before waiting on target locks. Reserve
sequence early; stamp the group's recording time after its target/account locks
are acquired. User-visible recording times remain actual timestamps.

Aggregate a transaction's same-account legs before computing the Balance after
shown for that event. A grouped event's anchor is the maximum entry order among
its transactions; each expanded member balance is their full-history balance at
that anchor. Zero-net allocations have canonical zero-valued legs and remain
included through allocation membership, regardless of whether net is zero.
For legacy comped sessions without a member leg, derive balance at the event anchor.
Raw entry API still exposes individual entries and their full-history running values.

Read activity in one REPEATABLE READ snapshot: build full effective account
history, compute balances using this tuple, group complete logical events, apply
personal/type selection, count groups, then paginate groups. Don't apply a SQL
member WHERE before collecting full detail. Personal inclusion for shared events
comes from allocations (payer OR participant), including zero-net allocations and
their inverse events. Never split members of a group across pages.

A reversal's filter family follows its original type. Category may still display
Reversal; positive payer credits are never classified as top-ups merely by sign.
Imported classification keeps its existing retained source facts. Session family
includes imported session groups. Shared filter includes native shared events and
their inverses; imported miscellaneous food remains available in All with its
existing classification, without inventing a native shared version.

Backdated corrections change effective historical running balances. Detail explains:
“Balances are recalculated using effective dates. Recorded on shows when this
change was saved.” No historical screen-snapshot claim. Session spend uses existing
Sydney play-date logic and only live replacement charges; shared events remain
excluded even with a linked session. Refresh existing spend queries; don't replace
the gross-charge logic with net credits.

## Frontend interaction contract

Money → Balances adds Record shared expense beside Record top-up. This work adds
an expense editor/review, not a new standalone page redesign. Use existing money
styles, cents parser, member selectors, disclosures and error handling.

Top-up/session/shared details offer supported Correct/Reverse actions to approved
admins. Correction opens saved inputs; original inactive users remain selectable
and clearly labeled, while new selections require approved users. Disambiguate
duplicate display names with an existing secondary identity cue in admin selection;
never change the application-wide display-name rule.

Keep editor and review separate. Review shows old/proposed gross values and exact
net Current → New balances for all affected users and assets, including zero deltas
and comped surplus. Reason required for correction/reversal. Confirmation is disabled
while preview is pending, after input changes, or when no valid review exists.
Cancelling simply closes review without a write. Optional session participant
suggestion requires an explicit button; warn before replacing an entered selection.

Keep a submission UUID and the **exact** confirmed payload across network failures.
Disable duplicate clicks locally, but rely on durable server replay. Unknown response
→ retry same payload first. `preview_changed` → retain inputs, fetch fresh review,
require explicit confirmation again. Version conflict → retain inputs, show current
record link; never silently retarget an old operation. Request conflict → retain
inputs, explain that this submission was already used; create new identity only for
a deliberate new operation. Forbidden → stop writes, retain useful error information.

On accepted first/replayed result, emit the existing balance-change event and
invalidate/refetch member balances, club position, active history/detail and affected
spend summaries. A replay's stored review is historical, not today's live balance.
No new global store, HTTP client, automated email or push behavior is introduced.

Mobile acceptance: editor fields, participants, full review and confirm controls fit
360px width without horizontal page scrolling. Expanded breakdown may use stacked
rows. Existing member history is readable by non-admins; action buttons remain
admin-only. Test keyboard labels/focus and review return without losing edits.
