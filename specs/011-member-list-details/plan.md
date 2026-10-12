# Compact member details (#65)

Keep display name, avatar, admin and membership status in each closed row. Use native keyboard-accessible details for identity, contact fields, editing, invitations, push settings and explicit Remove member text. Preserve removal safeguards, restoration, search and the three tabs. Keep existing API calls and permissions unchanged.

Build independently from origin/main. Test closed/open details, keyboard access, long names, missing contacts, editing, removal refusal and restoration on desktop and mobile. Capture screenshots, run frontend checks, and open a separate PR against main.

Show the saved account push setting in each closed row: bell for enabled, crossed bell for disabled, and a distinct unknown/loading state. Keep delivery/device details collapsed. Refresh the icon when the push detail panel refreshes. Use the existing admin-only status endpoint; do not infer browser permission or successful delivery.
