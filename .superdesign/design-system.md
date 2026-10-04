# Rally Mobile-First Design System

## Product and jobs

Rally helps a community badminton club understand what is happening next, RSVP quickly, see who is playing, manage personal balances, and complete club administration. The interface should reduce explanation: every page leads with its purpose, important state, and one obvious next action. Capabilities and information architecture must remain unchanged.

## Direction

Quiet, friendly, and operational. Keep the existing cyan primary and amber accent, but use colour selectively for action and status rather than filling large areas. Prefer warm white surfaces, slate text, thin borders, and minimal shadows. No neon, purple, glassmorphism, oversized gradients, decorative illustration, or flashy animation.

## Colour

- App background: slate-50; primary content surfaces: white; subtle section surfaces: slate-100/60.
- Text: slate-950 headings, slate-700 body, slate-500 supporting copy.
- Primary action: cyan-700 (`#0e7490`), hover cyan-800; primary tint cyan-50.
- Accent/attention: amber-500 with amber-50 tint; reserve it for deadlines, low balance, and highlights.
- Positive: emerald-700 on emerald-50. Destructive: red-700 on red-50.
- Preserve WCAG AA contrast. Never encode RSVP or money state by colour alone.

## Typography and language

Use the current system font stack. Mobile page title: 1.5rem/1.9rem semibold. Section title: 1rem/1.4rem semibold. Body: 0.9375rem/1.45rem. Supporting text: 0.8125rem/1.25rem. Use plain labels such as “Your next game”, “8 of 10 spots”, “RSVP closes Monday”. Avoid vague headings and repeated welcome copy.

## Shape and spacing

Use a 4px base spacing grid. Mobile gutters 16px, tablet/desktop 24px. Interactive targets at least 44px tall. Cards use 14–16px radius, 1px slate-200 border, and at most a very soft shadow. Buttons and inputs use 12px radius. Pills are reserved for compact status only. Sections should be visually grouped with 24–32px gaps.

## Responsive shell

- Mobile first: compact sticky top bar and bottom navigation, safe-area padding, uncluttered single-column page.
- Use a maximum of four primary bottom-navigation destinations. Keep Admin and sign-out in profile/overflow rather than shrinking five mobile destinations.
- Desktop: persistent left rail with labelled navigation, member identity and balance; page content max-width around 64rem. Preserve the same destination model as mobile.
- The page title and brief explanatory line belong inside the page, not in the global header.

## Components

- Buttons: solid cyan primary, white/outlined secondary, quiet text action, red destructive. Labels should say the outcome.
- Cards: elevate current/next action; secondary content should be flatter or grouped in lists.
- Tabs: segmented control on mobile when switching peer views; underline tabs only for dense admin tables.
- Session cards: show date block, session time/location, capacity, RSVP deadline, and the current member’s RSVP action in that reading order.
- Balance: always include a semantic label or accessible name, with amount in tabular numerals.
- Empty/error/loading states: name what is missing and what the user can do. Keep layouts stable while loading.
- Forms: labels above fields, helper copy only where decisions are non-obvious, validation near the field, primary action sticky on long mobile workflows when appropriate.

## Motion

Keep motion utilitarian: 150–200ms colour and transform transitions, respect reduced motion, no looping decoration.

## Target dashboard structure

On mobile, start with a small greeting and “Your next game” card. Make RSVP state and deadline understandable at a glance. Follow with quick destinations for all sessions and money, then any cancellation alerts, then later sessions. Move aggregate club counts below actionable content. On desktop, the same content can form a two-column dashboard within the persistent shell.

