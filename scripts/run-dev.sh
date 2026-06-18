#!/usr/bin/env bash
# Launch the agent-reactor backend (cmd/server) and the web-client host (cmd/web)
# together for local development. The backend hosts the pty sessions (REST + WS
# API); the web client serves the browser UI and reverse-proxies /api and /ws to
# the backend. Both bind to localhost over plain HTTP. Ctrl-C stops both.
#
# Env overrides: BACKEND_ADDR, WEB_ADDR, TOKEN.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BACKEND_ADDR="${BACKEND_ADDR:-127.0.0.1:8443}"
WEB_ADDR="${WEB_ADDR:-127.0.0.1:8080}"
TOKEN="${TOKEN:-$(openssl rand -hex 24 2>/dev/null || head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')}"

# Safety net when invoked directly (make run-dev builds first).
[ -x ./server ] || make build-server
[ -x ./web ] || make build-web

pids=()
cleanup() { kill "${pids[@]}" 2>/dev/null || true; }
trap cleanup EXIT INT TERM

./server -insecure -addr "$BACKEND_ADDR" -token "$TOKEN" &
pids+=("$!")
./web -insecure -addr "$WEB_ADDR" -server "http://$BACKEND_ADDR" &
pids+=("$!")

cat <<EOF

agent-reactor dev up:
  backend : http://$BACKEND_ADDR   (clients connect here; sessions outlive disconnects)
  web     : http://$WEB_ADDR

  Open →   http://$WEB_ADDR/#token=$TOKEN

Ctrl-C to stop both.
EOF

wait
