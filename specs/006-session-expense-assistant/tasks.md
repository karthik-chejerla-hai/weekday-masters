# Tasks: Session expense assistant

## Setup
- [x] T001 Record owner decisions, research and design in specs/006-session-expense-assistant/.
- [x] T002 Prepare isolated local/test databases and runner in scripts/local-issue40.sh.

## Foundation
- [x] T003 Add provider interfaces, Groq adapter and HTTP tests in backend/internal/assistant/.

## US1: Record a session
Independent check: preview and confirm actual use through the form without a key.
- [x] T004 [US1] Add exact-share, stale-preview, stock and concurrency tests in backend/internal/services/expense_service_test.go.
- [x] T005 [US1] Implement actual-count settlement, expense previews and confirmation in backend/internal/services/expense_service.go and settlement_service.go.
- [x] T006 [US1] Add approved/admin route registration and tests in backend/internal/handlers/expenses.go and expenses_test.go.
- [x] T007 [US1] Add form and reusable expense review in frontend/src/pages/AdminExpense.tsx and frontend/src/components/assistant/ExpenseReview.tsx.
- [x] T008 [US1] Show every outstanding session above Next game in frontend/src/pages/Dashboard.tsx with role tests.

## US2: Voice and text
Independent check: scripted provider tests, recording lifecycle tests and local trial.
- [x] T009 [US2] Add bounded tool loop and permissions tests in backend/internal/services/assistant_service_test.go.
- [x] T010 [US2] Implement assistant tools and handlers in backend/internal/services/assistant_service.go and backend/internal/handlers/assistant.go.
- [x] T011 [US2] Add voice capture, transcript review and chat in frontend/src/components/assistant/.
- [x] T012 [US2] Connect assistant configuration and routes in backend/cmd/server/main.go, frontend/src/services/api.ts and frontend/src/App.tsx.

## US3: Club questions
Independent check: approved member can read club data and cannot prepare expenses.
- [x] T013 [US3] Add session, member and club-position tools in backend/internal/services/assistant_service.go with database tests.
- [x] T014 [US3] Add assistant page/navigation in frontend/src/pages/Assistant.tsx and frontend/src/components/layout/.

## Verification and local delivery
- [x] T015 Verify idempotent seed and print the past session in backend/internal/seed/ and backend/cmd/seed/main.go.
- [x] T016 Update backend/internal/handlers/openapi.yaml, AGENTS.md and specs/006-session-expense-assistant/quickstart.md.
- [x] T017 Run backend race tests with scratch DB, frontend tests/lint/build and local UI/API smoke checks. Record evidence in specs/006-session-expense-assistant/validation.md.

## Owner-requested attendance scenarios
- [x] T018 Add all four 5/6-RSVP scenarios after a previous expense and a shuttle purchase, checking player balances, court credit, stock count/value, history, duplicate refusal and reversal.
- [x] T019 Support extra_participant_ids in the expense service, HTTP contract and assistant tool. Allocate actual shuttle value by hours and test cent rounding and stale attendance previews.
- [x] T020 Add extra-hour selection to the form, retain assistant drafts, and show early departure in previews and settlement history. Verify with component tests.
- [x] T021 Run the complete backend race suite and frontend coverage, lint and build. Restart the isolated local app.

## Dependencies and execution
T001 precedes implementation. Provider tests and isolated DB setup are independent.
US1 supplies previews for US2; US3 reuses US2's tool loop. Complete every story
before final verification. Backend and frontend checks can run independently.
No commits or pushes. No deployment. No live Groq call until the owner adds a key.
