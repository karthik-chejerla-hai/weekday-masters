# Data model

- Settlement.ActualShuttles: nullable integer, zero or greater. Nil identifies
  historical/advanced estimated band settlements. Existing band snapshots and
  charge lines remain the authoritative record. Reversals restore the recorded
  stock value and units. For actual counts, units are stored once on the base band.
  A three-hour expense allocates the consumed value 2:1 between base and extra
  bands in integer cents, without rounding physical units per hour. ChargeLine's
  existing in_extra flag records attendance; no extra schema change is needed.
- ExpenseInput: total_hours (2 or 3), shuttles_used (required integer 0..200),
  participant_ids (optional; omission uses confirmed RSVPs), extra_participant_ids
  (optional for three hours; omission means all participants stayed; an explicit
  list must be a nonempty, unique subset), expected_preview (required only on
  confirmation). Two-hour inputs must omit extra_participant_ids. Canonical
  previews expand both groups, and the fingerprint binds their attendance flags.
- ExpensePreview: session identity/date, canonical input, computed settlement,
  remaining court credit, standard next-booking cost, top-up warning, fingerprint.
- Conversation: bounded user/assistant text history and optional selected session.
  The server supplies trusted instructions and current tool results. The client
  cannot supply system or tool messages. No conversation table or audio storage.
- Tool registry: allowed name, description, JSON parameter schema, required role
  and execution function. New score tools can use this boundary in issue 39.
