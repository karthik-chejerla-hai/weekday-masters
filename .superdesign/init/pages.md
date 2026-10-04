# Key Page Dependency Trees

## `/dashboard`

Entry: `frontend/src/pages/Dashboard.tsx`

- `frontend/src/components/sessions/SessionCard.tsx`
  - `frontend/src/components/rsvp/RSVPButton.tsx`
  - `frontend/src/components/ui/Avatar.tsx`
  - `frontend/src/components/ui/Badge.tsx`
- `frontend/src/components/layout/Layout.tsx`
  - `frontend/src/components/layout/Header.tsx`
    - `frontend/src/components/money/BalanceChip.tsx`
    - `frontend/src/components/ui/Avatar.tsx`
  - `frontend/src/components/layout/Navigation.tsx`

## `/sessions`

Entry: `frontend/src/pages/Sessions.tsx`

- `frontend/src/components/sessions/SessionCard.tsx`
  - `frontend/src/components/rsvp/RSVPButton.tsx`
  - `frontend/src/components/ui/Avatar.tsx`
  - `frontend/src/components/ui/Badge.tsx`
- `frontend/src/components/sessions/PastSessionCard.tsx`
- shared layout tree from `/dashboard`

## `/sessions/:id`

Entry: `frontend/src/pages/SessionDetail.tsx`

- `frontend/src/components/rsvp/RSVPButton.tsx`
- `frontend/src/components/rsvp/PlayerList.tsx`
  - `frontend/src/components/ui/Avatar.tsx`
  - `frontend/src/components/ui/Badge.tsx`
- shared layout tree from `/dashboard`

## `/money`

Entry: `frontend/src/pages/Money.tsx`

- `frontend/src/components/money/BalanceChip.tsx`
- `frontend/src/components/money/BalancesList.tsx`
- `frontend/src/components/money/LedgerList.tsx`
- `frontend/src/components/money/TopupForm.tsx`
- `frontend/src/components/money/PositionPanel.tsx`
- `frontend/src/components/money/AssetPurchaseForms.tsx`
- shared layout tree from `/dashboard`

## `/profile`

Entry: `frontend/src/pages/Profile.tsx`

- `frontend/src/components/ui/Avatar.tsx`
- `frontend/src/components/ui/Badge.tsx`
- `frontend/src/components/notifications/NotificationSettings.tsx`
- shared layout tree from `/dashboard`

## `/admin`

Entry: `frontend/src/pages/Admin.tsx`

- shared layout tree from `/dashboard`

## `/admin/sessions`

Entry: `frontend/src/pages/AdminSessions.tsx`

- shared layout tree from `/dashboard`

## `/admin/members`

Entry: `frontend/src/pages/AdminMembers.tsx`

- `frontend/src/components/ui/Avatar.tsx`
- `frontend/src/components/ui/Badge.tsx`
- shared layout tree from `/dashboard`

## `/`

Entry: `frontend/src/pages/Home.tsx` (standalone public layout)

