# Admin asset balance corrections

## Request

Allow an admin to correct the court-credit balance in the app and retain an
audit record. The motivating discrepancy is A$53 recorded versus A$113 verified.

## Scope and decisions

- Approved admins can reconcile court credit or bank to a verified non-negative
  balance. Member balances and shuttle inventory use their existing flows.
- The selected asset and surplus move by the same difference. A reconciliation
  records no payment between bank and venue and changes no member balance.
- A reason is required. The app shows before, target and adjustment before Save.
- Record actor ID and display-name snapshot, timestamp, reason, account, before,
  after, adjustment and linked ledger transaction. Keep all previous entries.
- A correction can be corrected through another reviewed correction. Individual
  reversal of a balance-correction snapshot is rejected.
- Reject stale reviewed balances and unconfirmed imported assets. Repeated
  requests with the same ID and identical inputs return the existing audit record.
- Only approved admins can read the paged correction history.

## Acceptance criteria

1. With court credit at 5300 cents, a reviewed target of 11300 appends +6000
   court credit and +6000 surplus. Bank, members and shuttles retain their values.
2. History shows the actor, Sydney time, reason, 5300 → 11300 and transaction ID.
3. Reducing a balance uses negative asset and surplus adjustments.
4. A stale balance returns 409 without writing. An identical retry posts once.
5. Invalid targets, empty reasons and unauthorized requests cannot post.
6. A failure writing either ledger or audit rolls back the entire transaction.
