# HTTP contracts

All endpoints require an authenticated approved member unless marked admin.

- GET /api/sessions/unsettled: all past non-cancelled native sessions without a live
  settlement, ordered oldest first. Each row includes confirmed RSVP count.
- GET /api/assistant/status: enabled flag only, no credentials.
- POST /api/assistant/transcribe: multipart audio file; bounded size and supported
  MIME types; returns text. No persistence. Provider failures use safe messages.
- POST /api/assistant/messages: {messages: [{role: user|assistant, content}],
  session_id?: UUID}; returns {message, expense?: ExpensePreview}.
- POST /api/admin/sessions/:id/expense/preview: ExpenseInput, read-only.
- POST /api/admin/sessions/:id/expense: ExpenseInput with expected_preview;
  returns the recorded settlement. The provider cannot invoke this route as a tool.

ExpenseInput accepts optional `extra_participant_ids` for a three-hour session.
Omission means all selected players stayed. An explicit list must be nonempty,
unique, and contained in `participant_ids` (or the default confirmed RSVPs).
Omit this field for two hours. The canonical preview includes the resolved group.
Changing attendance after preview returns `preview_changed` on confirmation.
The `prepare_expense` assistant tool accepts the same optional group.

Missing configuration returns 503, provider quota 429, malformed request 400,
non-admin writes 403, stale or duplicate settlement 409, invalid business input
422. Existing settlement and history endpoints retain their contracts.
