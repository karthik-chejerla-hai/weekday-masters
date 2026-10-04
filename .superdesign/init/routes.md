# Route Map

Router: React Router configured in `frontend/src/App.tsx`. Authenticated screens share `Layout` (`Header`, page content, mobile `Navigation`).

| URL | Page | Access | Summary |
| --- | --- | --- | --- |
| `/` | `frontend/src/pages/Home.tsx` | Public | Club sign-in landing page |
| `/pending` | `PendingApproval.tsx` | Authenticated | Membership review state |
| `/dashboard` | `Dashboard.tsx` | Approved | Greeting, cancellations, upcoming sessions, quick totals |
| `/sessions` | `Sessions.tsx` | Approved | Upcoming and history tabs |
| `/sessions/:id` | `SessionDetail.tsx` | Approved | RSVP and player list |
| `/sessions/:id/settlement` | `SessionSettlement.tsx` | Approved | Read-only member settlement view |
| `/money` | `Money.tsx` | Approved | Balances, personal ledger, admin assets |
| `/profile` | `Profile.tsx` | Authenticated | Member profile and notification settings |
| `/admin` | `Admin.tsx` | Admin | Join requests, club settings, announcements |
| `/admin/sessions` | `AdminSessions.tsx` | Admin | Session scheduling and management |
| `/admin/sessions/:id/settle` | `AdminSettlement.tsx` | Admin | Preview and post settlement |
| `/admin/members` | `AdminMembers.tsx` | Admin | Invite, edit, remove, reinstate members |

Full router source: `frontend/src/App.tsx`.

