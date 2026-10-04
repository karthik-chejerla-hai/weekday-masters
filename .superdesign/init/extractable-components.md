# Extractable Components

## AppShell
- Source: `frontend/src/components/layout/Layout.tsx`
- Category: layout
- Description: Shared authenticated frame with sticky header, central content, and mobile bottom navigation.
- Extractable props: none
- Hardcoded: layout widths, page gutters, responsive navigation positions

## Header
- Source: `frontend/src/components/layout/Header.tsx`
- Category: layout
- Description: Brand, member balance, profile identity, admin entry point, and sign out.
- Extractable props: `isAdmin`, `balanceState`, `balance`, `displayName`
- Hardcoded: Rally mark, destination routes, icon choice, all CSS

## Navigation
- Source: `frontend/src/components/layout/Navigation.tsx`
- Category: layout
- Description: Mobile bottom navigation for Home, Sessions, Money, Profile, and optional Admin.
- Extractable props: `activeItem`, `isAdmin`
- Hardcoded: route labels, route destinations, Lucide icons, CSS

## SessionCard
- Source: `frontend/src/components/sessions/SessionCard.tsx`
- Category: basic
- Description: Session date/time/location, RSVP state, confirmed players, and capacity.
- Extractable props: `rsvpStatus`, `isFull`, `confirmedCount`, `maxPlayers`
- Hardcoded: field labels, status iconography, card styling

## BalanceChip
- Source: `frontend/src/components/money/BalanceChip.tsx`
- Category: basic
- Description: Compact colour-coded member balance.
- Extractable props: `state`, `amount`, `compact`
- Hardcoded: tone mapping, currency typography

## Avatar
- Source: `frontend/src/components/ui/Avatar.tsx`
- Category: basic
- Description: Profile image or member initials.
- Extractable props: `name`, `src`, `size`
- Hardcoded: fallback style

