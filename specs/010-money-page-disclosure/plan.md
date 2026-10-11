# Money page disclosure

## Specification and scope
Issues #59, #62 and #63 reduce repeated navigation and unused forms. Keep all money arithmetic, permissions, validation, ledger writes and warning thresholds unchanged. Member RSVP pills remain unchanged.

## Decisions
- Keep the header balance and profile links and YTD Analytics link. Remove only duplicate displays.
- Open member top-ups and asset purchase forms through named admin actions. Closing must not submit. Return focus to the opener. Keep failures visible and refresh balances after success.
- Keep asset totals, shuttle counts, low-resource warnings and reconciliation failures visible. Put routine reconciliation and dated metadata in accessible details. Preserve correction review and history.

## Execution
1. #59: remove duplicate navigation displays and heading balance fetch; verify narrow headers and refresh.
2. #62: add an accessible form disclosure with focus return, cancellation and successful-post feedback.
3. #63: use form disclosures for purchases; disclose routine asset details; keep correction controls available.

## Verification
Run lint, build, frontend coverage, browser type checking, and desktop/mobile browser tests. Use mocked financial requests only. Capture screenshots for each PR. Commit and push each branch, then link the PRs with gh stack. Do not merge or deploy production.
