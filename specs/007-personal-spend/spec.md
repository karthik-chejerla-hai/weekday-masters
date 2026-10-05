# Personal badminton spend

## Request

Approved members can see their own badminton spend year to date and all time.
A chip beside the header balance opens a fourth Money tab, Analytics.

## Defined behaviour

- Spend is the member's recorded session charges, including guest charges on
  their account. It is not deposits, withdrawals, opening balances or club costs.
- Native settlements use charge lines. Reviewed imported sessions use gross
  `charge_cents`, never net balance movements or amounts paid for other players.
- Reversed charges are excluded from their original play period. Replacement
  settlements count once. Pending/unsettled sessions do not count.
- Calendar periods use Australia/Sydney. Native games use their resolved start
  date (legacy rows fall back to the stored session date). Imports use retained
  play dates, including the import's recorded-date fallback when applicable.
- Year to date includes January 1 through today. All time covers available
  recorded history through today. Future play dates do not count.
- Analytics shows both totals, the current year's monthly spend and session
  counts, and the first recorded session date. All amounts are integer cents
  until display formatting.
- Each monthly session count includes the member's own attendance once per
  native session, including comped attendance. Guest-only payments do not count
  as attendance. Guest charges still contribute to spend.
- Imported positive gross shares count once per import and play date, combining
  regular and extra-hour records. A zero share does not establish attendance.
- Header balance and YTD spend use joined label/value badges. Labels stay visible
  on narrow screens; the badges stack when needed.
- The endpoint always uses the signed-in approved member. No query parameter
  can select another member or the whole club.
- Loading and failures never appear as zero spend. Empty history shows zero
  with an explanation. The header link opens Analytics directly.

## Acceptance

1. A player who paid a whole imported game sees only their gross charge share.
2. Deposits and the other players' charges do not increase personal spend.
3. Reversals remove charges and a replacement appears once.
4. Sydney year boundaries and play dates determine the totals.
5. Analytics and the chip work for members and admins on narrow and wide screens.
6. Paying for a guest does not increase the member's monthly session count.
7. Imported extra-hour records do not count as another session. Separate native
   sessions on the same date each count once.

## Clarifications

Use account-borne session charges, including guests, as personal spend. Explain
this definition in the UI. All time means the history retained in Rally, not
unrecorded purchases or badminton outside this club.
