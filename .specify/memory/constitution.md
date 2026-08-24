<!--
SYNC IMPACT REPORT
Version change: (none) → 1.0.0
Rationale: Initial ratification. No prior constitution existed; the template
placeholders are replaced with concrete, project-specific governance.

Modified principles: none (initial adoption)

Added sections:
  - Core Principles I–IX
  - Technology Stack and Constraints
  - Development Workflow and Quality Gates
  - Governance

Removed sections: none

Notes:
  - The resolved template provided five principle slots; this project defines
    nine. Heading hierarchy is preserved.
  - Principles I–IV and VIII–IX codify conventions already practised in the
    repository and documented in CLAUDE.md.
  - Principles V–VII are new, introduced ahead of the club ledger and session
    settlement work.

Follow-up TODOs: none
-->

# Rally Constitution

Rally is a badminton club management application: member registration, session
scheduling, RSVP and waitlist handling, notifications, and a club ledger that
tracks player balances and prepaid club assets.

## Core Principles

### I. Layered Backend

Request handling MUST be separated from business logic. Handlers parse and
validate requests and format responses; they MUST NOT contain business rules.
All business logic lives in the service layer. Services access the database
through GORM via the global `database.DB` handle; there is no dependency
injection container and none is to be introduced without an amendment.

All primary keys are UUIDs, generated in a `BeforeCreate` hook on the model.

**Rationale**: The boundary is what makes the business rules testable without an
HTTP layer, and keeps rule changes from leaking into transport concerns.

### II. Migrations Are Explicit and Never Run by the Server

Schema changes MUST be applied by the separate `cmd/migrate` binary. The server
MUST NOT run `AutoMigrate` or any other schema mutation at startup.

CI runs the migrator as an explicit step against `DATABASE_URL` before deploying;
`docker-compose` runs it as a one-shot service.

**Rationale**: Cloud Run starts revisions concurrently. A server that migrates on
boot races itself during a rollout.

### III. Middleware Is a Security Boundary, Bound at Registration

Gin binds the handler chain when a route is registered. A route registered on the
wrong group silently skips its checks and fails open.

Every route MUST be registered on the group matching its required access level:

| Group | Requirement |
| --- | --- |
| public | none |
| `RequireValidToken` | valid JWT; user row may not exist yet |
| `AuthMiddleware` | authenticated, user row exists |
| `RequireApproved` | `membership_status = approved` |
| `RequireAdmin` | `role = admin` |

Choosing a group is a security decision and MUST be reviewed as one. Approved-only
routes MUST be registered on the `approved` group, never on `protected`.

**Rationale**: The failure mode is silent and produces no error — only review
discipline catches it.

### IV. Sydney Time, Resolved and Stored

All user-facing times are Australia/Sydney. Times that participate in business
rules — session start and end, RSVP deadlines, billing periods — MUST be stored
as resolved timestamps, not reconstructed at read time from a bare date plus an
`HH:MM` string.

Code that computes across a date boundary MUST handle DST transitions explicitly;
a Sydney day is not always 24 hours.

**Rationale**: Deadline and duration arithmetic on unresolved local times is
correct until the first October and April, then silently is not.

### V. Money Is Integer Cents

Currency MUST be represented as signed integer cents. Floating-point types MUST
NOT be used for monetary values anywhere: models, service logic, JSON payloads,
or the database.

Per-unit costs that do not divide evenly MUST be derived at point of use from the
total and the quantity, never stored as a pre-rounded per-unit price. A $50 tube
of 12 shuttles is 416.66… cents per shuttle and MUST be carried as stock value
and unit count, with the unit cost derived on demand.

Splitting a cost across participants MUST use largest-remainder allocation, so
that the sum of allocated shares equals the total exactly. A split that loses or
invents cents is a defect, not a rounding artefact.

**Rationale**: This is real money between friends. Penny drift is both a
correctness bug and a trust problem.

### VI. The Ledger Is Append-Only

Ledger entries MUST NOT be updated or deleted once written. A correction is a new
reversing transaction that references the original.

Settlement is a one-way door: once a session is settled and its charges are
posted, the settlement MUST NOT be silently re-edited. Re-settling requires an
explicit reversal followed by a new settlement, and both remain visible in
history.

**Rationale**: The ledger's value is that it is a record. A mutable record of who
owes what is not a record.

### VII. Accounting Invariants Are Enforced and Tested

Two invariants MUST hold after every write that moves money:

1. **Balanced transactions** — the entries of a single transaction MUST sum to
   zero under the project's sign convention.
2. **Club position** — `bank + court credit + shuttle stock` MUST equal
   `Σ player balances + club surplus`.

These MUST be enforced in code, not assumed by convention, and MUST be covered by
tests that fail when the invariant is broken.

Money-moving writes MUST occur inside a database transaction with row locks
sufficient to prevent lost updates, following the pattern already established by
the RSVP capacity check (`SELECT ... FOR UPDATE` on the contended row). Where
concurrency is part of the rule, a concurrency test MUST accompany it, as the
existing session-capacity test does.

**Rationale**: Accounting errors compound silently and are discovered long after
the transaction that caused them.

### VIII. Tests Skip Without a Database; CI Enforces Coverage

Database-backed tests MUST skip, not fail, when `TEST_DATABASE_URL` is unset, so
that `go test ./...` stays green on a clean checkout. They MUST obtain their
database through the existing `requireDB(t)` harness, which truncates every table
before each test. `TEST_DATABASE_URL` MUST NEVER point at a database whose
contents matter.

CI runs the full suite against a PostgreSQL service container and enforces
coverage floors on both backend and frontend. Lowering a floor requires the same
justification as any other amendment to this document.

**Rationale**: A suite that fails without local infrastructure gets ignored; a
suite that destroys data gets run once.

### IX. Frontend Without a Global Store

The frontend MUST NOT introduce a global state library. User state comes from
`AuthContext`; component data comes from the `useApi` / `useApiMutation` hooks.

All backend calls MUST go through the singleton Axios client in
`services/api.ts`, which owns the bearer-token interceptor. Components MUST NOT
call `fetch` or construct their own HTTP clients.

**Rationale**: One interceptor is one place to get authentication right.

## Technology Stack and Constraints

**Backend** — Go 1.22, Gin, GORM, PostgreSQL 16. Module
`github.com/weekday-masters/backend`. Two binaries: `cmd/server` and
`cmd/migrate`.

**Frontend** — React 18, TypeScript, Vite, Tailwind CSS, PWA-enabled. Auth0 with
Google OAuth (PKCE).

**Deployment** — Backend to Cloud Run (australia-southeast1) via GCP Artifact
Registry; frontend to Firebase Hosting. Deploys run from GitHub Actions on push
to `main`. All secrets live in GitHub repository secrets.

**Authentication** — Auth0 JWT validated against JWKS. Identity claims are taken
from the token and, for first-time registration, from Auth0's `/userinfo`. Email
and identity MUST NEVER be read from the request body; only display fields
(`name`, `profile_picture`) may be.

**Notifications** — FCM push and SendGrid email, both optional and independently
initialised. Neither may become a hard startup dependency.

## Development Workflow and Quality Gates

Changes reach `main` through pull requests. Before merge:

- `go test ./...` passes, and the database-backed suite passes in CI against the
  service container.
- `npm run lint` and `npm run build` pass; frontend tests pass.
- Coverage floors are met on both backend and frontend.
- Any new route's group assignment has been reviewed against Principle III.
- Any change touching money has been reviewed against Principles V, VI, and VII.

Schema changes ship with the migration in the same pull request as the code that
depends on them.

Work follows the Spec Kit flow: specification, clarification, plan, tasks, then
implementation. Specifications and plans live under `specs/`.

## Governance

This constitution supersedes ad-hoc convention. Where this document and a habit
disagree, this document wins until amended.

**Amendment procedure** — Amendments are proposed as a pull request modifying
this file, stating the principle affected, the rationale, and the migration path
for any code that currently violates the amended rule. An amendment that
invalidates existing code MUST land with either the fix or a tracked plan for it.

**Versioning policy** — This document is versioned semantically:

- **MAJOR** — a principle is removed, or redefined in a backward-incompatible way.
- **MINOR** — a principle or section is added, or guidance materially expanded.
- **PATCH** — clarifications, wording, and non-semantic refinement.

**Compliance review** — Pull request review MUST verify compliance with the
principles above. Complexity that departs from a principle MUST be justified in
the pull request description, not merely in code comments. `CLAUDE.md` carries
the runtime development guidance that implements these principles day to day and
SHOULD be updated alongside amendments that change it.

**Version**: 1.0.0 | **Ratified**: 2026-08-24 | **Last Amended**: 2026-08-24
