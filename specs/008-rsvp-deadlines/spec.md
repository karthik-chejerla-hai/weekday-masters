# Configurable RSVP deadlines

Issue: https://github.com/karthik-chejerla-hai/weekday-masters/issues/94

Admins need to control RSVP deadlines for existing sessions and set a club default based on hours before the session starts.

Acceptance criteria:
- An admin can view and change an existing session's RSVP deadline.
- Club settings include the default number of hours before the session starts when RSVPs close.
- New sessions use this default unless an admin supplies a custom deadline.
- Changing the default updates every future session's RSVP deadline, including custom deadlines.
- Past sessions retain their stored deadlines.
- Calculations use each session's actual start time and handle Sydney daylight saving changes.
- Existing RSVP and admin access rules remain enforced.
- Add tests for session edits, validation, recurring sessions, and bulk deadline updates.

Link the implementation PR with a closing reference so GitHub closes this issue when the PR merges.

Implementation rules:
- The initial default is 72 elapsed hours before the resolved session start.
- Admins can select whole hours from 0 to 8760. Zero closes RSVPs at the start.
- Introducing the setting does not rewrite existing deadlines.
- Saving an unchanged default preserves session-specific changes.
- A changed default replaces all deadlines with a start time after the save time.
- Started sessions keep their deadlines. Future cancelled and closed sessions receive the new deadline.
- An explicit deadline must be in the future and at or before the session start.
- Default deadlines can already be past for short-notice sessions.
- All date and time inputs refer to Sydney time, independent of the browser timezone.
- New default recurring deadlines use elapsed hours. Custom recurring deadlines keep their day and clock offset.
- Editing one session does not change the original recurring rule.
- Default changes also clear recurring custom rules so later generated sessions use the new default.
