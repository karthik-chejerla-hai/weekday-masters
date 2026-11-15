# Tasks

- [x] Extend native history response and OpenAPI documentation with stored session status.
- [x] Render cancellation and suppress settlement/game actions.
- [x] Cover native state propagation and cancelled/played/settled/imported cards.
- [x] Frontend build, lint (one existing hook warning), and all 533 tests pass.
- [x] Full backend race suite on disposable PostgreSQL 16 (including reminder/dispatcher/worker regressions).

The specific production record from 10 November was not accessed or changed. It will render as cancelled if its stored status is cancelled.
