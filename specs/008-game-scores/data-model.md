# Data Model

## GameResult
UUID id and session_id, team_a1/team_a2/team_b1/team_b2 (user foreign keys), score_a/score_b integers, created_by/updated_by (user foreign keys), version, voided_at, created_at/updated_at, request_id and request_hash. Sort each pair for stable identity. Unique (created_by, request_id). Database checks enforce four distinct players and scores 0–99 with no tie. Session and users cannot be deleted through these foreign keys.

## GameRevision
UUID id, game_id foreign key, version, snapshot JSON, changed_by user foreign key and created_at. Unique (game_id, version). Each successful create, correction or void stores a complete public result snapshot inside the same transaction. Revisions are never changed or removed by the feature.

## Transitions
Create -> active v1. Active vN -> corrected active vN+1, or voided vN+1. Voided is terminal. Only approved recorder/admin may change a result. Stale versions return conflict. Separate identical games are permitted with new request IDs.

## Derived read models
GameView returns member IDs and display names, session date/title, scores, version, void state and recorder/editor. HeadToHead returns games, wins_a/wins_b, points_a/points_b and paged result views. No persisted statistics.
