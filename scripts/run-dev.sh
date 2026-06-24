#!/usr/bin/env bash
# Launch a fully-isolated dev stack: a fresh arc daemon (cmd/arc) under a
# scratch data dir, the backend gateway (cmd/server), and the web-client host
# (cmd/web). The scratch data dir means this never collides with the user's
# production arc daemon (~/.agent-reactor/) — they can run side by side.
#
# The daemon owns all pty sessions and exposes them over its Unix socket; the
# gateway translates browser REST/WS traffic into daemon proto calls; the web
# host serves the UI and reverse-proxies /api and /ws to the gateway. Ctrl-C
# stops every process this script started and removes the scratch data dir.
#
# Env overrides:
#   BACKEND_ADDR              gateway listen addr        (default 127.0.0.1:8443)
#   WEB_ADDR                  web host listen addr       (default 127.0.0.1:8080)
#   ARC_DATA_DIR              scratch dir for daemon     (default $ROOT/.run-dev/arc)
#   KEEP_DATA_DIR             1 = preserve ARC_DATA_DIR on exit (default: remove)
#   ROOST_DEVCONTAINER_PREFIX docker container/label prefix this daemon owns
#                             (default: reactor-dev — distinct from the TUI
#                             daemon's "reactor" so the two never compete for
#                             the same container name; if they did, the
#                             mount-hash drift path would `docker rm -f` the
#                             peer's container and kill its sessions.)
#
# Auth is disabled here (gateway runs with -no-auth). The gateway refuses
# -no-auth on non-loopback binds, so BACKEND_ADDR must stay on 127.0.0.1/::1/
# localhost — leaving BACKEND_ADDR at its default is the safe path.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BACKEND_ADDR="${BACKEND_ADDR:-127.0.0.1:8443}"
WEB_ADDR="${WEB_ADDR:-127.0.0.1:8080}"
ARC_DATA_DIR="${ARC_DATA_DIR:-$ROOT/.run-dev/arc}"
KEEP_DATA_DIR="${KEEP_DATA_DIR:-0}"
ROOST_DEVCONTAINER_PREFIX="${ROOST_DEVCONTAINER_PREFIX:-reactor-dev}"
ARC_SOCKET="$ARC_DATA_DIR/arc.sock"
ARC_LOG="$ARC_DATA_DIR/arc.log"

# Always rebuild — guarding on file existence (`[ -x ./server ]`) lets a stale
# binary from a previous checkout/branch run against today's source, which is
# how `flag provided but not defined: -no-auth` surfaced. `go build` is a
# no-op when nothing changed, so the cost of always running it is negligible.
make build
make build-server
make build-web

mkdir -p "$ARC_DATA_DIR"

pids=()
cleanup() {
  kill "${pids[@]}" 2>/dev/null || true
  if [ "$KEEP_DATA_DIR" != "1" ]; then
    # Remove docker containers this daemon spawned BEFORE deleting the data
    # dir. Otherwise the container outlives the daemon with a bind mount
    # pointing at the now-deleted $ARC_DATA_DIR/run/<projectHash>/, and any
    # Claude session re-attached to it sees an empty /opt/agent-reactor/run/
    # (no reactor-bridge, no arc.sock) — every hook → bridge call then dies
    # with `/bin/sh: ... reactor-bridge: not found`.
    #
    # Label filter is prefix-scoped (see platform/sandbox/devcontainer/
    # docker.go), so peer daemons under a different ROOST_DEVCONTAINER_PREFIX
    # — including the user's TUI daemon at prefix `reactor` — are invisible.
    local containers
    containers=$(docker ps -aq --filter "label=${ROOST_DEVCONTAINER_PREFIX}-managed=1" 2>/dev/null || true)
    if [ -n "$containers" ]; then
      docker rm -f $containers >/dev/null 2>&1 || true
    fi
    rm -rf "$ARC_DATA_DIR"
  fi
}
trap cleanup EXIT INT TERM

# Always launch a fresh daemon under ARC_DATA_DIR. Because the data dir is
# unique to this script invocation, there is no flock contention with the
# user's production arc daemon at ~/.agent-reactor/.
#
# ROOST_DEVCONTAINER_PREFIX additionally isolates this daemon's docker
# container namespace from any peer arc daemon on the same host: container
# names and reactor-* label keys all carry the prefix, so the production
# daemon's "reactor-<hash>" and this daemon's "reactor-dev-<hash>" cannot
# collide. Without this, `docker ps --filter` would surface the peer's
# container, mount-hash drift would fire, and `docker rm -f` would kill it.
ROOST_DATA_DIR="$ARC_DATA_DIR" ROOST_DEVCONTAINER_PREFIX="$ROOST_DEVCONTAINER_PREFIX" ./arc >"$ARC_LOG" 2>&1 &
pids+=("$!")

# Wait up to ~5s for the daemon to bind its socket before the gateway dials,
# otherwise the gateway floods stderr with backoff WARNs.
for _ in $(seq 1 50); do
  [ -S "$ARC_SOCKET" ] && break
  sleep 0.1
done
if [ ! -S "$ARC_SOCKET" ]; then
  echo "arc daemon did not create $ARC_SOCKET within 5s. Tail of $ARC_LOG:" >&2
  tail -n 30 "$ARC_LOG" >&2 || true
  exit 1
fi

./server -insecure -no-auth -addr "$BACKEND_ADDR" -arc-sock "$ARC_SOCKET" &
pids+=("$!")
./web -insecure -addr "$WEB_ADDR" -server "http://$BACKEND_ADDR" &
pids+=("$!")

cat <<EOF

agent-reactor dev up (isolated from ~/.agent-reactor):
  data         : $ARC_DATA_DIR
  daemon       : $ARC_SOCKET
  backend      : http://$BACKEND_ADDR  (auth disabled — loopback only)
  web          : http://$WEB_ADDR
  dc prefix    : $ROOST_DEVCONTAINER_PREFIX  (docker container & label prefix)

  Open →   http://$WEB_ADDR/

  Daemon log: tail -f $ARC_LOG
  Set KEEP_DATA_DIR=1 to preserve $ARC_DATA_DIR across runs.

Ctrl-C to stop everything.
EOF

# Exit as soon as ANY process dies (e.g. the gateway fails to bind a port) so
# the EXIT trap tears the others down, instead of hanging behind the banner.
wait -n
