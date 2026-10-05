# Tasks: Doubles game scores

## Phase 1: Setup
- [x] T001 Record scope, clarifications and design in specs/008-game-scores/.

## Phase 2: Foundation
- [x] T002 Add game models and constraints in backend/internal/models/game.go; register migrations in backend/internal/database/db.go and reset tables in backend/internal/testsupport/db.go.

## Phase 3: Record a game
Independent test: record a reviewed result, retry once, and reload one game.
- [x] T003 [US1] Test validation and concurrent retry behavior in backend/internal/services/game_service_test.go.
- [x] T004 [US1] Implement previews, create and reads in backend/internal/services/game_service.go.
- [x] T005 [US1] Add approved handlers and HTTP tests in backend/internal/handlers/games.go and games_test.go; wire backend/cmd/server/main.go.
- [x] T006 [US1] Test and add the read-only game tool in backend/internal/services/assistant_service.go and assistant_service_test.go.
- [x] T007 [US1] Add typed API, manual/voice review and session score page in frontend/src/services/api.ts, types/index.ts, components/games/ and pages/SessionGames.tsx; test entry and save behavior.

## Phase 4: Correct the record
Independent test: correct and void a game, retaining all saved versions.
- [x] T008 [US2] Test correction rights, stale edits and audit in backend/internal/services/game_service_test.go; implement row locking and revisions in game_service.go.
- [x] T009 [US2] Add correction, void and history controls with tests in frontend/src/components/games/GameCard.tsx and GameCard.test.tsx.
- [x] T010 [US2] Preserve scored sessions in backend/internal/services/session_service.go and test deletion behavior.

## Phase 5: Compare opponents
Independent test: player and exact-team totals agree with games under both side orders.
- [x] T011 [US3] Test and implement SQL comparison and historical players in backend/internal/services/game_service.go and game_service_test.go.
- [x] T012 [US3] Add tested comparison UI in frontend/src/pages/HeadToHead.tsx and HeadToHead.test.tsx.

## Phase 6: Verification
- [x] T013 Add routes and discovery links in frontend/src/App.tsx, pages/SessionDetail.tsx and components/sessions/PastSessionCard.tsx; update backend/openapi/openapi.yaml and AGENTS.md.
- [x] T014 Run backend database/race tests and frontend coverage, lint and build; record results in specs/008-game-scores/quickstart.md.
- [x] T015 Review changes, commit and push feat/issue-39-game-scores.

## Dependencies and execution
Foundation precedes all stories. Record a game is the first usable increment. Corrections and comparisons depend on stored games. Verification follows all stories. Backend and frontend tests are independent checks and can run concurrently. Within a story, service tests precede service code and UI tests precede UI code. Complete all three stories for issue #39.
