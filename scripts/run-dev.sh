#!/usr/bin/env bash
# Launch a fully-isolated dev stack: the merged backend (cmd/server — daemon +
# HTTP/WS gateway in one process) under a scratch data dir, plus the web-client
# host (cmd/web). The scratch data dir means this never collides with another
# user-scope agent-reactor backend ($HOME/.agent-reactor/) — they can run side
# by side.
#
# The backend owns all pty sessions and exposes them over its Unix socket; the
# co-resident gateway translates browser REST/WS traffic into in-process
# DaemonClient calls; the web host serves the UI and reverse-proxies /api and
# /ws to the gateway. Ctrl-C stops every process this script started and tears
# down the docker containers the daemon spawned; the scratch data dir is
# preserved so the next run restores the same sessions (set CLEAN_DATA_DIR=1
# to wipe it).
#
# Env overrides:
#   BACKEND_ADDR              gateway listen addr        (default 127.0.0.1:8443)
#   WEB_ADDR                  web host listen addr       (default 127.0.0.1:8080)
#   SERVER_DATA_DIR           scratch dir for backend    (default $ROOT/.run-dev/server)
#   CLEAN_DATA_DIR            1 = wipe SERVER_DATA_DIR on exit (default: preserve,
#                             so sessions/ persists across restarts and the
#                             next run restores the same session list)
#   ROOST_DEVCONTAINER_PREFIX docker container/label prefix this daemon owns
#                             (default: reactor-dev — distinct from any peer
#                             agent-reactor backend's "reactor" so the two
#                             never compete for the same container name; if
#                             they did, the mount-hash drift path would
#                             `docker rm -f` the peer's container and kill its
#                             sessions.)
#
# Auth is disabled here (server runs with -no-auth). The server refuses
# -no-auth on non-loopback binds, so BACKEND_ADDR must stay on 127.0.0.1/::1/
# localhost — leaving BACKEND_ADDR at its default is the safe path.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BACKEND_ADDR="${BACKEND_ADDR:-127.0.0.1:8443}"
WEB_ADDR="${WEB_ADDR:-127.0.0.1:8080}"
SERVER_DATA_DIR="${SERVER_DATA_DIR:-$ROOT/.run-dev/server}"
CLEAN_DATA_DIR="${CLEAN_DATA_DIR:-0}"
ROOST_DEVCONTAINER_PREFIX="${ROOST_DEVCONTAINER_PREFIX:-reactor-dev}"
SERVER_SOCKET="$SERVER_DATA_DIR/server.sock"
SERVER_LOG="$SERVER_DATA_DIR/server.log"

# Always rebuild — guarding on file existence (`[ -x ./server ]`) lets a stale
# binary from a previous checkout/branch run against today's source. `go build`
# is a no-op when nothing changed, so the cost of always running it is
# negligible. `make build-web` depends on $(WEB_DIST), which itself depends on
# web sources, so the React bundle is rebuilt only when src/client/web/
# actually changed.
make build-server
make build-web

mkdir -p "$SERVER_DATA_DIR"

pids=()
cleanup() {
  kill "${pids[@]}" 2>/dev/null || true
  # Always remove docker containers this backend spawned. Otherwise the
  # container outlives the backend with a bind mount pointing at the
  # previous $SERVER_DATA_DIR/run/<projectHash>/, and any Claude session
  # re-attached to it on the next run sees a stale reactor-bridge — every
  # hook → bridge call would dial the dead socket.
  #
  # Label filter is prefix-scoped (see platform/sandbox/devcontainer/
  # docker.go), so peer backends under a different ROOST_DEVCONTAINER_PREFIX
  # are invisible. Killing the containers does NOT touch
  # $SERVER_DATA_DIR/sessions/, which is what we want to keep so the next
  # run-dev.sh restores the same sessions.
  local containers
  containers=$(docker ps -aq --filter "label=${ROOST_DEVCONTAINER_PREFIX}-managed=1" 2>/dev/null || true)
  if [ -n "$containers" ]; then
    docker rm -f $containers >/dev/null 2>&1 || true
  fi
  if [ "$CLEAN_DATA_DIR" = "1" ]; then
    rm -rf "$SERVER_DATA_DIR"
  fi
}
trap cleanup EXIT INT TERM

# Launch the merged backend (daemon + gateway) under SERVER_DATA_DIR. Because
# the data dir is unique to this script invocation, there is no flock
# contention with any peer agent-reactor backend at ~/.agent-reactor/.
#
# ROOST_DEVCONTAINER_PREFIX additionally isolates this backend's docker
# container namespace from peer agent-reactor backends on the same host:
# container names and reactor-* label keys all carry the prefix, so different
# backend instances' containers cannot collide. Without this, `docker ps
# --filter` would surface the peer's container, mount-hash drift would fire,
# and `docker rm -f` would kill it.
ROOST_DATA_DIR="$SERVER_DATA_DIR" ROOST_DEVCONTAINER_PREFIX="$ROOST_DEVCONTAINER_PREFIX" \
  ./server -insecure -no-auth -addr "$BACKEND_ADDR" -data-dir "$SERVER_DATA_DIR" \
  >"$SERVER_LOG" 2>&1 &
pids+=("$!")

# Wait up to ~5s for the backend to bind its socket before the web proxy
# starts routing requests; otherwise the first /api hit hits a 502.
for _ in $(seq 1 50); do
  [ -S "$SERVER_SOCKET" ] && break
  sleep 0.1
done
if [ ! -S "$SERVER_SOCKET" ]; then
  echo "server did not create $SERVER_SOCKET within 5s. Tail of $SERVER_LOG:" >&2
  tail -n 30 "$SERVER_LOG" >&2 || true
  exit 1
fi

./web -insecure -addr "$WEB_ADDR" -server "http://$BACKEND_ADDR" &
pids+=("$!")

cat <<EOF

agent-reactor dev up (isolated from ~/.agent-reactor):
  data         : $SERVER_DATA_DIR
  server sock  : $SERVER_SOCKET
  backend      : http://$BACKEND_ADDR  (auth disabled — loopback only)
  web          : http://$WEB_ADDR
  dc prefix    : $ROOST_DEVCONTAINER_PREFIX  (docker container & label prefix)

  Open →   http://$WEB_ADDR/

  Server log: tail -f $SERVER_LOG
  Sessions persist across restarts under $SERVER_DATA_DIR/sessions/.
  Set CLEAN_DATA_DIR=1 to wipe $SERVER_DATA_DIR on exit (fresh start).

Ctrl-C to stop everything.
EOF

# Exit as soon as ANY process dies (e.g. the gateway fails to bind a port) so
# the EXIT trap tears the others down, instead of hanging behind the banner.
wait -n
