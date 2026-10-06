# Browser test strategy

## Goal and current gaps

Protect Rally's primary member and administrator workflows on desktop and mobile.
The existing Vitest suite covers components and arithmetic. Go handler/service tests
cover real PostgreSQL writes, authorization, concurrent RSVP capacity and accounting
invariants. The existing seven Playwright checks cover only anonymous pages; two
can pass with a broken app because they assert a body or conditionally inspect login.
CI does not currently execute Playwright.

## Layers and scope

| Area | Browser scenarios in this change | Lower-layer responsibility |
| --- | --- | --- |
| Public entry | Club/venue render, sign-in action, invitation account selection | Auth0 token verification, verified invite claiming |
| Access | Anonymous deep links; pending/removed member redirects; player blocked from admin routes; admin member preview | Server authorization on every endpoint |
| Sessions | Dashboard to detail, IN/OUT persistence, full-session waitlist response, deadline lock, cancellation reason/no RSVP, missing session recovery | Capacity locking, queue promotion, cancellation notifications |
| Profile | Save nickname/phone, reload, API failure then retry | Profile validation and persistence |
| Money | Member read-only tabs, ledger filter requests/error retry, assets error retry, admin top-up cents and refreshed balance, rejected write preserves form | Ledger invariants, exact splitting, duplicate settlement, reversals |
| Games | Four-player validation, tied-score rejection, save and reload, rejected save retry keeps request ID, future session entry blocked | Game versions, duplicate requests, comparison statistics |
| Layout | Same functional cases in desktop Chromium and mobile Chromium; visible navigation and no horizontal overflow | Visual design review and broader accessibility audit |

All listed browser scenarios must pass. No conditional assertions or blanket skips.
Keep existing unit/API coverage floors. Browser pass counts are not code coverage.

## Harness

Run a dedicated loopback Vite server on port 4173. A separate E2E Vite config
replaces only the Auth0 SDK. The real AuthProvider, route guards, React pages,
styles and Axios client run unchanged. Normal development and production builds
never import this replacement. Use fixed Sydney dates and synthetic identities.
Each test has an isolated browser context and its own in-memory API state.
Intercept HTTP at the browser boundary, assert write payloads and show resulting
state after refetch/reload. Unknown API requests and unexpected external requests
fail the test. Do not contact a deployed backend, Auth0, email or push providers.

These are browser integration tests, not full-stack end-to-end proof. Mocked
responses cannot validate Go behavior, real OAuth, or database persistence.
The existing PostgreSQL CI job remains the authority for those backend rules.
Live Google sign-in, actual push delivery, voice hardware/provider integration,
PWA offline behavior, Safari/Firefox, complete admin CRUD and settlement browser
flows remain separate follow-up coverage. No provider credentials are needed here.

## Execution and diagnostics

Use role/name/label locators and retrying expectations. Freeze Date, not timers.
Do not use sleeps. Run both projects on every pull request and main push in a
separate CI job. Retain HTML reports, failure screenshots and traces. Fail CI on
flaky retries and committed focused tests. Start a fresh server rather than
reusing an unknown local app. Type-check the harness separately from application
source. Run lint, production build, Vitest coverage and Playwright before commit.

From `frontend`:

```sh
npm ci
npx playwright install chromium
npm run test:e2e
npm run test:e2e -- --project=mobile-chromium
npm run test:e2e -- --ui
npx playwright show-report
```

No `.env`, backend, database or test account is required. On Linux CI, install
browser system libraries with `npx playwright install --with-deps chromium`.

## Validation results

- Playwright: 33 scenarios, 66 passes across desktop and mobile Chromium; no retries.
- E2E TypeScript check: passed.
- ESLint: passed with the existing `SessionDetail.tsx` effect-dependency warning.
- Production build: passed; test identity/token markers are absent from `dist`.
- Vitest: 43 files, 271 tests passed. Coverage floors passed (78.28% statements,
  76.75% branches, 71.61% functions, 80.33% lines).
- Go: `go test ./...` passed with Go 1.25.5. Database-dependent tests skip locally
  without `TEST_DATABASE_URL`; the existing CI PostgreSQL job runs those tests.
- Production fix found during test development: RSVP buttons now retain accessible
  names on mobile, where their visible text is hidden.

The first browser run found two harness issues: an omitted assistant-status
response and a locator trying to click a visually hidden switch input. Both were
corrected; the final full run passed. CI execution itself is not yet verified.
