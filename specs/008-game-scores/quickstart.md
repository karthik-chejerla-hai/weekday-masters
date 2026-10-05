# Validation Guide

Use a scratch loopback PostgreSQL database. Never use production or a database containing valued data for TEST_DATABASE_URL.

1. Start PostgreSQL 16 on a free local port. Set TEST_DATABASE_URL for backend tests.
2. Run `cd backend && go test -race ./...`. New games and revisions are migrated by the test harness and cleared between tests.
3. Run `cd frontend && npm ci && npm run test:coverage && npm run lint && npm run build`.
4. For an interactive check, migrate and seed a local database, then start the API and frontend using the repository local setup. Use an approved local test identity. GROQ_API_KEY enables voice; manual entry works without it.
5. Open a started session, select Game scores, enter four players and 21–17, save, and reload. Record a second identical game. Both appear.
6. Speak four names and a score. Review the preview. No game exists until Save. Check ambiguous names through a follow-up. Deny microphone permission and use the form.
7. As recorder, correct a score and inspect history, then void it. As a different ordinary member, confirm correction is unavailable and the API rejects it.
8. Compare two opposing players, then exact teams in either partner order. Check totals after corrections and voids. Removed members retain history. Imported historical session cards do not offer score entry.

Automated provider tests use fakes; live speech quality requires an enabled provider and microphone.

## Validation completed on 2026-10-05

- Full backend suite passed with `go test -race -coverprofile=... -covermode=atomic ./...` against a dedicated PostgreSQL 16 database on loopback port 5436. Total statement coverage: 71.2%, above the 60% floor.
- Full frontend coverage run passed: 257 tests, 77.52% statements, 76.43% branches, 69.96% functions and 79.36% lines. A subsequent duplicate-name selector regression test and focused game/assistant/session suites also passed.
- Frontend build passed. Lint passed with one pre-existing exhaustive-deps warning in SessionDetail.tsx. The existing bundle-size warning remains.
- Chromium at 390×844 verified synthetic-data score review, explicit Save, refreshed history and opponent comparison. No browser errors or horizontal overflow. Review fits in 544px height.
- Browser visitor checks passed for `/games` and `/sessions/example/games`.
- OpenAPI YAML parses; whitespace checks pass.
- Automated speech tests use the provider fake. Live microphone transcription and live Groq name recognition were not exercised.
