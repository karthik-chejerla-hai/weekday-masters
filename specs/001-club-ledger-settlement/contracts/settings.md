# Contract: Club Settings

Conventions in [README.md](./README.md).

Settings extend the existing `Club` model and reuse its existing endpoints rather than
introducing a parallel settings resource. Fewer endpoints, and the club record is already
where venue and name live.

---

## `GET /api/club` — group `public` (existing, extended)

The public club endpoint gains no money fields. Rates are not public.

## `GET /api/admin/club` — group `admin` (existing, extended)

Returns the existing club fields plus:

```json
{
  "base_hours": 2,
  "base_rate_cents": 3000,
  "extra_rate_cents": 2300,
  "shuttles_per_hour": 5,
  "low_balance_threshold_cents": 2000
}
```

## `PUT /api/admin/club` — group `admin` (existing, extended)

Accepts partial updates of the same fields.

**Rules**

- Rates and hours must be non-negative. `base_hours` must be greater than zero.
- Changing a setting affects **future settlements only**. Every settlement snapshots the
  values it used (FR-017), so a rate change never rewrites a settled session.
- These are defaults. Any of them can be overridden for a single session on the settlement
  form without changing the club record (FR-016).

**Why these live on `Club`** — the club is a single seeded row and already carries
club-wide configuration. A separate settings table would add a join and a second seeding
path for no benefit at this scale.

---

## Notification preferences (existing, extended)

Balance reminders reuse the existing notification preference model rather than adding
channels. Two new notification types join the existing set:

| Type | Fires |
|---|---|
| `balance_low` | On settlement, when a participant's resulting balance is positive but below `low_balance_threshold_cents` |
| `balance_negative` | On settlement, when a participant's resulting balance is negative |

Both respect the member's existing push and email toggles (FR-033). `UserNotificationPreferences`
gains `push_balance_alerts` and `email_balance_alerts`, defaulting to `true`, and
`IsPushEnabledForType` / `IsEmailEnabledForType` gain the two cases.
