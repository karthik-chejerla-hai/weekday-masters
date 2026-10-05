# Implementation Plan: Doubles game scores

**Branch**: `feat/issue-39-game-scores` | **Date**: 2026-10-05 | **Spec**: [spec.md](spec.md)

## Summary

Add session-linked doubles games, a prominent voice preview with manual fallback, versioned corrections and voids, and player/exact-team comparisons. Reuse the assistant provider and recorder. Every write uses GameService, never an assistant tool.

## Technical Context

**Language/Version**: Go 1.22 and TypeScript/React 18.
**Primary Dependencies**: Existing Gin, GORM, PostgreSQL, Axios, Vite and Groq provider boundary. No new packages.
**Storage**: PostgreSQL game_results and game_revisions.
**Testing**: Existing Go scratch database harness, race detector, Vitest and Testing Library.
**Target Platform**: Mobile and desktop web.
**Project Type**: Web application.
**Performance Goals**: Database-paged game history (50 per page); statistics aggregate matching rows in SQL.
**Constraints**: Four distinct member UUIDs, integer scores 0–99, explicit Save, immutable revisions, optimistic edit versions, create idempotency, approved access.
**Scale/Scope**: One club; scheduled sessions only; two score pages plus assistant review.

## Constitution Check

Pre-research and post-design: PASS. Handlers decode; GameService owns rules. UUID hooks and explicit migrator register new tables. RegisterRoutes includes RequireApproved in server and tests. Compare starts_at; display Sydney session dates. No money changes. Tests use scratch schemas and skip without TEST_DATABASE_URL. Components use the shared Axios client and local state/hooks. No new global store or dependency.

## Project Structure

- backend/internal/models/game.go: result and revision models.
- backend/internal/services/game_service.go: validation, writes, audit, session list and comparisons.
- backend/internal/handlers/games.go: approved routes and bounded decoding.
- backend/internal/services/assistant_service.go: read-only prepare_game tool.
- backend/internal/database/db.go and internal/testsupport/db.go: migration and test reset.
- frontend/src/components/games/: entry/review and result cards.
- frontend/src/pages/SessionGames.tsx and HeadToHead.tsx: session entry and comparisons.
- frontend/src/services/api.ts and types/index.ts: typed contract.
- Tests alongside services, handlers, components and pages.

## Implementation Strategy

Build and test the service first, wire approved HTTP routes, then add the manual interface and assistant preview, then comparisons. Review all routes and new storage rules. Run full backend database/race tests and frontend coverage, lint and build before commit and push.
