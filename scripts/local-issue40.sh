#!/usr/bin/env bash
# Local-only issue 40 environment. Never inherits a remote DATABASE_URL.
set -euo pipefail
RALLY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RALLY_CONTAINER=rally-issue40-db
RALLY_DATABASE='postgres://badminton:badminton123@127.0.0.1:5434/rally_issue40?sslmode=disable'
export DATABASE_URL="$RALLY_DATABASE"
export NOTIFICATIONS_DISABLED=true
export INVITATION_TEST_EMAILS_ENABLED=false
export PORT=8080 GIN_MODE=debug FRONTEND_URL=http://localhost:5173
export VITE_API_URL=/api
export GOCACHE="${TMPDIR:-/tmp}/rally-issue40-go-cache"
mkdir -p "$RALLY_ROOT/tmp/issue40"

run_go() {
  if command -v mise >/dev/null 2>&1; then mise exec go@1.25.5 -- go "$@"; else go "$@"; fi
}
run_npm() {
  if command -v mise >/dev/null 2>&1; then mise exec node@24.21.0 -- npm "$@"; else npm "$@"; fi
}
setup() {
  if [[ ! -f "$RALLY_ROOT/backend/.env" || ! -f "$RALLY_ROOT/frontend/.env" ]]; then
    echo 'Create backend/.env and frontend/.env from their examples. Configure Auth0 and ADMIN_EMAIL first.' >&2
    exit 1
  fi
  if docker container inspect "$RALLY_CONTAINER" >/dev/null 2>&1; then
    docker start "$RALLY_CONTAINER" >/dev/null
    if [[ "$(docker port "$RALLY_CONTAINER" 5432/tcp)" != '127.0.0.1:5434' ]]; then
      echo 'The local container has unexpected ports. Check rally-issue40-db before continuing.' >&2
      exit 1
    fi
  else
    docker run -d --name "$RALLY_CONTAINER" -p 127.0.0.1:5434:5432 \
      -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
      -e POSTGRES_DB=rally_issue40 postgres:16-alpine >/dev/null
  fi
  local ready=false
  for _ in {1..30}; do
    if docker exec "$RALLY_CONTAINER" pg_isready -U badminton -d rally_issue40 >/dev/null 2>&1; then ready=true; break; fi
    sleep 1
  done
  if [[ "$ready" != true ]]; then echo 'Local PostgreSQL did not become ready.' >&2; exit 1; fi
  if ! (cd "$RALLY_ROOT/backend"; run_go run ./cmd/migrate) > "$RALLY_ROOT/tmp/issue40/migrate.log" 2>&1; then
    tail -n 30 "$RALLY_ROOT/tmp/issue40/migrate.log" >&2
    exit 1
  fi
  if ! (cd "$RALLY_ROOT/backend"; run_go run ./cmd/seed -env=local) > "$RALLY_ROOT/tmp/issue40/seed.log" 2>&1; then
    tail -n 30 "$RALLY_ROOT/tmp/issue40/seed.log" >&2
    exit 1
  fi
  tail -n 15 "$RALLY_ROOT/tmp/issue40/seed.log"
}
build_api() { (cd "$RALLY_ROOT/backend"; run_go build -o "$RALLY_ROOT/tmp/issue40/api" ./cmd/server); }
frontend() {
  cd "$RALLY_ROOT/frontend"
  if [[ ! -d node_modules ]]; then run_npm ci; fi
  run_npm run dev -- --host 127.0.0.1 --strictPort
}
case "${1:-start}" in
  setup) setup ;;
  api) build_api; cd "$RALLY_ROOT/backend"; exec "$RALLY_ROOT/tmp/issue40/api" ;;
  ui) frontend ;;
  start)
    setup
    build_api
    (cd "$RALLY_ROOT/backend"; exec "$RALLY_ROOT/tmp/issue40/api") > "$RALLY_ROOT/tmp/issue40/api.log" 2>&1 &
    RALLY_API_PID=$!
    trap 'kill "$RALLY_API_PID" 2>/dev/null || true' EXIT INT TERM
    sleep 1
    if ! kill -0 "$RALLY_API_PID" 2>/dev/null; then
      tail -n 20 "$RALLY_ROOT/tmp/issue40/api.log" >&2
      exit 1
    fi
    echo 'Rally local test: http://localhost:5173'
    echo 'API log: tmp/issue40/api.log'
    echo 'Add GROQ_API_KEY to backend/.env, then restart this command.'
    frontend
    ;;
  *) echo 'Usage: scripts/local-issue40.sh [start|setup|api|ui]' >&2; exit 1 ;;
esac
