# T007: early financial table-lock integration

Resumed: 2026-11-02. Branch: `codex/financial-lock-order`.
Base: merged main including #104, #105 and #106; refreshed through `be5507e`
(#107 guest RSVPs, #108 deployment changes and #109 session prompt cleanup).
This is a standalone change against main, with no unmerged PR dependency.

## Scope

All existing financial write transactions now acquire the compatible
`transactions` ROW EXCLUSIVE table intent before locking users, sessions,
original transactions or accounts:

- LedgerService.Post and the non-session generic reversal transaction.
- SettlementService.Settle and ReverseSettlement. Generic session reversals
  already dispatch to the latter and receive the same protection.
- CorrectAssetPurchase and CorrectAssetBalance.
- PostWithin repeats the table lock defensively before locking accounts. This
  does not replace early acquisition by orchestrations that prelock other rows.

The reviewed-operation runner already acquired this lock before user locks and
remains a control case in the regression. Ordinary writers' table locks remain
mutually compatible. The importer retains its existing stronger SHARE ROW
EXCLUSIVE lock, acquired before it writes financial application data.

## Caller audit

Every production PostWithin caller was checked. Settlement, reversal and asset
correction/audit orchestrations now take the early guard. Both Splitwise source
row posting and imported-asset confirmation execute under ImportSplitwiseWithOptions'
existing stronger table lock; no import guard or reviewed history protection is
weakened. Private inverse helpers are reached through guarded outer transactions.

Cash top-ups/withdrawals, court/shuttle purchases and opening-balance seeding
ultimately use LedgerService.Post. The synthetic and roster seed commands use
those public service paths; there is no additional seed PostWithin orchestration
holding row locks that needs a separate guard. This task does not change opening
balance policy or other business validations.

The new guest-RSVP paths merged during the pause do not post ledger entries.
Their payer/membership consistency remains relevant to T008–T010, rather than
being silently treated as completed by this financial lock-order change.

## Regression and validation

`TestFinancialWritesWaitForImportBeforeRowLocks` holds the importer's actual
strong table-lock mode, starts a writer, observes its pending RowExclusiveLock
in pg_locks, and probes application rows with FOR UPDATE NOWAIT. It uses actual
lock waits rather than guessed sleeps, then releases the gate and verifies the
financial operation succeeds and the club invariant remains balanced.

Before the change, eight existing financial paths failed because an account,
transaction or session row was already held while the writer waited for the
financial table lock. The reviewed runner control already passed.

After the change, all nine paths passed with race detection: ordinary posting,
caller-transaction posting, generic reversal, settlement, dedicated/generic
session reversal, purchase correction, balance audit and reviewed runner.

The full backend race suite passed on the original merged baseline before the
usage-limit interruption. It passed again on 2026-11-02 after integrating the newer main, including the
guest-RSVP/removal and existing import/ledger/settlement packages. Tests use the shared RequireDB
harness and a dedicated disposable PostgreSQL 16 database, not application data.
No schema, frontend, notification or production financial operation is added.
