# Contract: Session Settlement and History

Conventions in [README.md](./README.md).

---

## `GET /api/admin/sessions/:id/settlement/preview` — group `admin`

Backs the settlement form. Returns the participant list pre-populated from RSVPs and the
costing that *would* result, without writing anything. Safe to call repeatedly as the admin
adjusts the form — the frontend re-previews on each change so the numbers shown are always
the numbers that will be posted.

**Query/body** — all optional; omitted values fall back to club settings (FR-016).

```json
{
  "base_hours": 2,
  "base_rate_cents": 3000,
  "extra_hours": 1,
  "extra_rate_cents": 2300,
  "shuttles_per_hour": 5,
  "lines": [
    { "user_id": "…", "in_base": true, "in_extra": true,  "comped": false },
    { "user_id": "…", "in_base": true, "in_extra": false, "comped": false },
    { "user_id": "…", "in_base": true, "in_extra": true,  "comped": false, "guest_name": "Sanjay" }
  ]
}
```

Omitting `lines` entirely returns the default: everyone who RSVP'd `in`, in every band of
the session (FR-018).

**Response**

```json
{
  "bands": {
    "base":  { "hours": 2, "court_cents": 6000, "shuttle_units": 10, "shuttle_cents": 4167, "total_cents": 10167, "heads": 6 },
    "extra": { "hours": 1, "court_cents": 2300, "shuttle_units": 5,  "shuttle_cents": 2083, "total_cents": 4383,  "heads": 4 }
  },
  "totals": { "court_cents": 8300, "shuttle_cents": 6250, "shuttle_units": 15, "charged_cents": 14550, "surplus_cents": 0 },
  "lines": [
    { "user_id": "…", "name": "Karthik", "in_base": true, "in_extra": true, "comped": false, "amount_cents": 2791 },
    { "user_id": "…", "name": "Karthik", "guest_name": "Sanjay", "in_base": true, "in_extra": false, "comped": false, "amount_cents": 1694 }
  ],
  "stock_after": { "units": 9, "amount_cents": 3750 }
}
```

`charged_cents` always equals `court_cents + shuttle_cents` less anything comped, so the
admin can see the split is exact before committing (I3).

**Errors** — `shuttle_stock_short` (422) fires here too, so the form can prompt for the
missing purchase before the admin commits rather than after they press settle.

---

## `POST /api/admin/sessions/:id/settle` — group `admin`

Same request body as the preview. Posts the settlement atomically (FR-021, FR-023).

Returns the created settlement, its charge lines, and the transaction. Then, as a
consequence rather than a separate call, sends balance reminders to participants whose
resulting balance is below threshold (FR-034).

**Errors**

| Code | HTTP | When |
|---|---|---|
| `session_already_settled` | 409 | A live settlement exists. Reverse it first (FR-024). |
| `shuttle_stock_short` | 422 | Stock is short. Nothing is written. |
| `not_settleable` | 422 | The session is cancelled, or has not finished yet. |

Settling twice concurrently yields exactly one settlement; the loser gets `409`.

To correct a settled session: `POST /api/admin/transactions/:id/reverse` against the
settlement's transaction, then settle again. Both remain visible in history.

---

## `GET /api/sessions/history?limit=&offset=` — group `approved`

Past sessions, newest first (FR-037). Uses the resolved `ends_at` timestamp, so a session
that finished three hours ago is correctly in the past (research.md R10).

```json
{
  "items": [
    {
      "session_id": "…",
      "title": "Tuesday Social",
      "starts_at": "2026-08-25T20:00:00+10:00",
      "ends_at": "2026-08-25T23:00:00+10:00",
      "settled": true,
      "total_cents": 14550,
      "player_count": 6
    }
  ],
  "total": 41
}
```

`settled: false` appears for a finished session nobody has costed yet. The frontend shows it
as awaiting settlement rather than hiding it.

---

## `GET /api/sessions/:id/settlement` — group `approved`

The full breakdown of a settled session, visible to every approved member (FR-027, FR-025).

```json
{
  "session": { "id": "…", "title": "Tuesday Social", "starts_at": "…", "ends_at": "…" },
  "rates": { "base_hours": 2, "base_rate_cents": 3000, "extra_hours": 1, "extra_rate_cents": 2300, "shuttles_per_hour": 5 },
  "bands": { "base": { "…": "…" }, "extra": { "…": "…" } },
  "totals": { "court_cents": 8300, "shuttle_cents": 6250, "shuttle_units": 15, "charged_cents": 14550 },
  "lines": [
    { "user_id": "…", "name": "Karthik", "in_base": true, "in_extra": true, "comped": false, "amount_cents": 2791 },
    { "user_id": "…", "name": "Priya", "in_base": true, "in_extra": false, "comped": true, "amount_cents": 0 }
  ],
  "settled_at": "…",
  "reversed_at": null
}
```

`rates` are the snapshot from the settlement, not current club settings — so a session
viewed a year later still shows what it actually cost (FR-017).
