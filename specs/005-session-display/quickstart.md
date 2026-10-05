# Review and rollout

The local review app runs at http://localhost:5173. It has four upcoming test sessions so the two-session member limit and full admin schedule can be compared. Club Settings is configured with venue BadmintonWorx Norwest, Court 8 and 12-hour time. Local notification delivery is disabled.

Review Sessions, the dashboard's Future sessions section, and Admin > Club Settings. Save 24-hour time, revisit a session, then restore 12-hour time. Both values persist across reloads. A blank court number clears it. Capacity continues to use the session's court count, not its court number.

This change adds two Club columns through the existing explicit migration command. A later production deployment must run the migration before starting the updated backend. Existing clubs start with court number unassigned and 24-hour display. After deployment, set Court 8 and the reviewed time format through Admin > Club Settings. The new settings do not change stored session times, capacity, RSVP deadlines, imported history or financial records.

Verification: backend database tests with race detection, frontend tests, lint and production build passed. The UI was checked on desktop and at 320px and 390px widths. The venue and court number remain readable without horizontal scrolling. The admin list retains all four review sessions. Session details and the admin list follow both saved time formats.
