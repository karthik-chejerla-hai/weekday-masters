# Manual verification

Run the explicit backend migrator before the server so transactions gain the correction link and reason fields. Sign in as an approved admin and open Money → Club assets.

1. Record a shuttle purchase for $50 / 12 shuttles. Open Purchase history and corrections and choose Correct purchase.
2. Enter $60 / 24 shuttles and a receipt-check reason. Review the $10 bank decrease and $10 stock increase, then save. Verify history retains the original marked reversed/corrected plus the replacement and reason. Member balances and surplus stay unchanged.
3. Correct the replacement again. Confirm each correction remains in history.
4. Repeat for a court-credit purchase. No shuttle count should be requested.
5. For an inaccurate bank or court balance, use Audit bank balance, choose the asset, and enter its verified current balance and reason. Review the existing audited adjustment and history.
6. Confirm a regular member cannot see purchase correction controls. Try a correction reducing stock below what has already been consumed; it must fail without any ledger writes.

Previously settled member charges are preserved; purchase corrections adjust current assets. Imports and opening balances require verified balance audits or their reviewed import workflow.
