# Tasks

- [x] Prioritize requests, compact navigation and empty status, align page heading.
- [x] Collapse settings/announcement forms behind explicit named controls.
- [x] Preserve behavior and update existing unit/browser workflows to open controls.
- [x] All 528 frontend tests, build and lint pass (one existing SessionDetail hook warning).
- [x] Orca browser: desktop and 390px mobile, no horizontal overflow; focused settings summary opens/closes with Enter. Synthetic fixture only.

All 14 existing announcement/deadline browser scenarios pass across desktop and mobile Chromium, including uncertain-publication retries and stale deadline protection.

## Adversarial review correction

- [x] Keep pending status and late success/error feedback visible outside collapsed settings/announcement disclosures. Four deferred-response regressions fail on the original change and pass after correction.

- [x] Second independent review: collapsed club summary must display the last confirmed name/venue, not unsaved or failed drafts. Preserve a saved summary snapshot, update only on successful load/save, and cover draft collapse, failed save and confirmed response values.
