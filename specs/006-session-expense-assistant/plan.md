# Implementation Plan: Session expense assistant

**Branch**: `feat/issue-40` | **Date**: 2026-10-05 | **Spec**: [spec.md](spec.md)

## Summary

Add a reusable voice/text assistant backed by Groq. Its tools can read club data
and prepare expense previews with separate standard and extra-hour groups. A separate admin request confirms an
expense. Add an actual shuttle count to settlement costing while retaining the
existing advanced form and historical band settlements.

## Technical Context

Go 1.22, Gin, GORM, PostgreSQL 16, React 18, TypeScript, Axios and MediaRecorder.
Use net/http for Groq. Provider-neutral Transcriber and Planner interfaces isolate
wire formats. Bounded calls, response sizes and recording duration limit resource
use. No provider calls without a key; no recordings or conversations persisted.
Tests use httptest providers, the existing scratch DB harness and Vitest.

## Constitution Check

All nine principles pass before and after design. Handlers parse; services own
rules. Additive model fields migrate through cmd/migrate. Register new routes with
shared approved/admin middleware. Sydney timestamps drive session selection.
Money remains integer cents; LedgerService is the only writer. Account locks,
preview fingerprints and duplicate settlement guards protect confirmation.
Frontend state stays local and all server calls use services/api.ts.

## Project Structure

- backend/internal/assistant/: provider interfaces and Groq adapter
- backend/internal/services/assistant_service.go: tools and bounded conversation loop
- backend/internal/services/expense_service.go: canonical expense preview/input
- backend/internal/services/settlement_service.go: actual use, hourly value allocation, group split, stale-preview check
- backend/internal/handlers/assistant.go and expenses.go: route contracts
- frontend/src/components/assistant/: recording, conversation, expense review
- frontend/src/pages/Assistant.tsx and AdminExpense.tsx: entry points
- frontend/src/pages/Dashboard.tsx: all unsettled past sessions
- scripts/local-issue40.sh: isolated DB, migration, seed and local server startup

## Verification

Test actual usage including zero, exact shares, stock/court movements, stale
previews, duplicate confirmations, concurrent settlements and reversals. Test
all four owner-requested RSVP scenarios after an earlier expense and purchase.
Check early departures through the form, assistant tool and HTTP route. Test
approved/admin routes, malformed tools, bounded execution, missing key, quotas,
provider errors, microphone cleanup and out-of-order previews. Run backend tests
with a dedicated scratch database, frontend tests, lint and build. Start a seeded
local environment. The owner supplies the real Groq key for the final live trial.
