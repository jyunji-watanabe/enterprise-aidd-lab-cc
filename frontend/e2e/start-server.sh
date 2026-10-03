#!/usr/bin/env bash
# Builds the frontend and backend, then starts the server on a fresh seeded DB.
set -euo pipefail
PORT="${1:-18080}"
HERE="$(cd "$(dirname "$0")" && pwd)"
FRONTEND="$(dirname "$HERE")"
BACKEND="$(dirname "$FRONTEND")/backend"
WORK="$FRONTEND/test-results/e2e-server"
mkdir -p "$WORK"
rm -f "$WORK"/expense.db*

if [ "${E2E_SKIP_BUILD:-}" != "1" ]; then
  (cd "$FRONTEND" && npx vite build --logLevel error)
fi
(cd "$BACKEND" && go build -o "$WORK/expense" ./cmd/expense)
exec "$WORK/expense" serve -addr "127.0.0.1:$PORT" -db "$WORK/expense.db" -static "$FRONTEND/dist" -seed
