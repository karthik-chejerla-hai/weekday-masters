# Dev environment and reviewed production releases

## Outcome

Use `dev` consistently. A permanent dev site uses a separate Google Cloud/Firebase project and a new empty Neon project. No production members, ledger entries, device tokens, email keys or production database credentials enter dev.

Changes reaching main deploy to dev. A maintainer tests the dev site, then approves that workflow's prod job. Manual dev runs can select a branch or immutable commit; a commit outside main cannot reach prod. Merging a tested PR with squash creates a new commit, so main is tested in dev again before release.

## Required behavior

- Dev URL: https://rally-club-app-dev.web.app. Region: australia-southeast1. Use Rally's existing billing account.
- Separate runtime, deployer, scheduler, Firebase web app and push keys. No new long-lived service-account key.
- Use synthetic fixtures plus approved test accounts. Notification sends use only the dev Firebase project. Invitation email stays unavailable until separately configured.
- Dev always shows its environment and source commit. Installed dev and prod apps have distinct names and origins.
- Build and test the selected source commit. Pin the API image by digest. Prod promotes that tested API image; rebuild the frontend from the same source commit using prod's public environment settings.
- Require human GitHub environment approval before any prod credential access, database migration or deployment.
- Before release, verify the approved source is in main and is still the successful dev release. Preserve the tested source and API digest in a release manifest.
- A deployment failure cannot count as a successful dev release. Never silently fall back to production settings when dev configuration is absent.
- Use separate dev credentials for migrations and idempotent synthetic seeding. No production seeding and no automatic database rollback.
- Support the private notification worker once its source is present. Never enable queued delivery without that worker. The legacy main version and pending notification releases must both deploy correctly.
- Keep PRs, review, tests and branch history. No forced push, production deployment or PR merge during this setup task.
