# Research

## Voice and review
Decision: extend the existing assistant with prepare_game and a typed game preview. Reuse VoiceRecorder and the existing transcription route.
Rationale: current provider interfaces and failure handling already support mobile speech; the tool cannot write.
Alternatives: a second provider integration adds redundant code; browser-only speech narrows support.

## Data and corrections
Decision: four fixed UUID fields, two integer scores, current version, void flag, unique creator/request key, original request fingerprint, and complete immutable revisions.
Rationale: doubles has exactly four players. Derive comparisons instead of storing counters. A row lock and version check prevent lost edits; request identity handles create retries.
Alternatives: permanent team entities and mutable counters add synchronization work without serving the request.

## Session and member lifecycle
Decision: only native scheduled sessions; preserve all referenced users and prevent session deletion when results exist.
Rationale: members are already removed by status. Imported Splitwise history uses synthetic session IDs and cannot own scores. Lock session creation/deletion consistently.

## Comparison queries
Decision: player mode matches opposite sides; exact-team mode matches sorted pairs in either orientation. Aggregate totals in SQL and page rows separately in a repeatable-read transaction.
Rationale: totals include all history and stay consistent with the current page. Same-team games do not establish a head-to-head result.

## Scope clarification
All three user questions are resolved in spec.md. Club scores are accepted as unequal integers 0–99; no tournament rules or external rule dependency is introduced.
