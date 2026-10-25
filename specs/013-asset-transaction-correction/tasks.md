# Tasks

- [x] Add immutable purchase correction service and transaction links.
- [x] Add approved-admin history and correction HTTP routes.
- [x] Add asset purchase history, edit and review controls.
- [x] Verify money, concurrency, access and UI behavior; run build/lint/tests.

Validation completed:
- Full backend `go test -race ./...` against a dedicated PostgreSQL scratch database.
- 401 frontend unit/component tests; production build and lint (one pre-existing SessionDetail hook warning).
- Four Playwright checks across desktop/mobile Chromium; E2E TypeScript check.
