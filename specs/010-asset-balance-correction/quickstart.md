# Using balance corrections

Deploy the feature through the normal pipeline, which runs `backend/cmd/migrate`
to create `balance_corrections` before deploying the backend.

As an approved admin:

1. Open **Money → Club assets → Correct an asset balance**.
2. Select **Court credit** and enter the verified balance, e.g. `113.00`.
3. Enter why the balance is being corrected and the source you checked.
4. Select **Review correction**. If the recorded balance is A$53, the review
   shows A$113 and an A$60 increase to court credit and surplus.
5. Select **Save correction**. Review the saved audit record below the form.

The recorded bank balance, member balances and shuttle inventory are unchanged
by a court-credit correction. This is a reconciliation against club surplus.
Use **Top up court credit** to record an actual bank payment to the venue.

If the balance changed after review, refresh and review it again. If a save
response is lost, retry the same review: its request ID prevents double posting.
To correct a mistaken correction, enter the newly verified balance with a new
reason. Both corrections remain in history.
