#!/usr/bin/env bash
# Build and run the Symphony orchestrator against a WORKFLOW.md.
#
# Rebuilds both the orchestrator and the claude-app-server shim before launch:
# in devcontainer sandbox mode the shim binary is bind-mounted into the agent
# container, and Docker Desktop/WSL snapshots a file mount at container-create
# time, so a stale shim would otherwise be used. Skip with --no-build.
#
# Usage:
#   scripts/run-orchestrator.sh [WORKFLOW_PATH] [--port PORT] [--no-build]
#
#   WORKFLOW_PATH   workflow file (default: <repo>/WORKFLOW.md)
#   --port PORT     observability HTTP port (default: 8080)
#   --no-build      skip rebuilding the binaries
#
# Requires:
#   - LINEAR_API_KEY exported in the environment (the WORKFLOW.md tracker.api_key
#     is the literal $LINEAR_API_KEY; never commit the key).
#   - docker in PATH and the devcontainer image present locally (the orchestrator
#     does not build images) when the workspace project uses devcontainer mode.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

WORKFLOW="$REPO_ROOT/WORKFLOW.md"
PORT=8080
BUILD=1

while [ $# -gt 0 ]; do
	case "$1" in
		--port)
			PORT="${2:?--port needs a value}"
			shift 2
			;;
		--no-build)
			BUILD=0
			shift
			;;
		-h|--help)
			# print the leading comment block (lines after the shebang, stripping "# ")
			awk 'NR>1 && /^#/ {sub(/^# ?/, ""); print; next} NR>1 {exit}' "${BASH_SOURCE[0]}"
			exit 0
			;;
		-*)
			echo "error: unknown flag: $1" >&2
			exit 2
			;;
		*)
			WORKFLOW="$1"
			shift
			;;
	esac
done

# --- preflight ---
if [ -z "${LINEAR_API_KEY:-}" ]; then
	echo "error: LINEAR_API_KEY is not set. export it first (do not commit the key)." >&2
	exit 1
fi
if [ ! -f "$WORKFLOW" ]; then
	echo "error: workflow file not found: $WORKFLOW" >&2
	exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
	echo "warn: docker not found in PATH; devcontainer sandbox mode will fail." >&2
fi

# --- build (orchestrator + the shim that gets mounted into the agent container) ---
if [ "$BUILD" = 1 ]; then
	echo "building orchestrator and claude-app-server..."
	make build-orchestrator
	make build-claude-app-server
fi

echo "starting orchestrator: workflow=$WORKFLOW port=$PORT"
exec ./orchestrator --workflow "$WORKFLOW" --port "$PORT"
