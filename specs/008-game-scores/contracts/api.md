# Game API

All routes require an authenticated approved member. Errors use {code,message}. Invalid input: 400/422; missing entity: 404; permission: 403; stale version or changed idempotency payload: 409.

- GET /api/sessions/:id/games?offset=0: {items,total}; 50 rows, includes voided results.
- POST /api/sessions/:id/games: {team_a:[uuid,uuid],team_b:[uuid,uuid],score_a:int,score_b:int,request_id:uuid}; returns GameView. Retries use the same request ID.
- PUT /api/games/:gameId: {team_a,team_b,score_a,score_b,version:int}; recorder/admin only.
- DELETE /api/games/:gameId: {version:int}; recorder/admin only, soft void.
- GET /api/games/:gameId/revisions: complete saved versions and change metadata.
- GET /api/games/players: display identities of approved members and members referenced in results.
- GET /api/games/head-to-head?team_a=id[,id]&team_b=id[,id]&offset=0: one ID per side for player mode or two for exact-team mode; equal lengths and distinct IDs required. Returns {items,total,wins_a,wins_b,points_a,points_b}.
- POST /api/assistant/messages: existing input; optional game in reply is a GamePreview with session, team arrays and scores. prepare_game validates without writing. Confirmation uses the explicit games POST endpoint.

GameView includes id, session_id, session_title, session_date, team_a/team_b identities ({id,name}), score_a/score_b, version, created_by, updated_by, recorder_name, editor_name, created_at, updated_at, voided_at.
