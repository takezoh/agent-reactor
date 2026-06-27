#!/usr/bin/env bash
# Codex integration setup — no-op as of phase F-C.
#
# Historically this responsibility lived in `arc codex setup`, which wrote a
# `reactor-peers` MCP server entry into ~/.codex/mcp.json so Codex sessions
# could send frame-to-frame messages. The peers feature was removed in phase
# F-A and the in-binary subcommand became a no-op stub; phase F-C extracts
# what remains into this script for symmetry with setup-claude.sh and
# setup-gemini.sh.
#
# Codex hooks (the lifecycle events Claude exposes) are not modeled by the
# Codex CLI today, so there is nothing to register in a settings file. If a
# future Codex release ships a hook configuration mechanism, extend this
# script to match — keep the same flag surface (--data-dir / --settings) as
# setup-claude.sh so callers stay uniform.
#
# Usage:
#   scripts/setup-codex.sh [SERVER_BIN] [--data-dir DIR] [--settings PATH]
#
# All flags are accepted (and ignored) so a caller iterating over the three
# agents can pass identical arguments to each script.

set -euo pipefail

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
}

# Drain any flags / arguments without using them, so the script signature
# matches setup-claude.sh / setup-gemini.sh.
while [ $# -gt 0 ]; do
  case "$1" in
    --data-dir|--settings)
      [ $# -ge 2 ] || { echo "setup-codex: $1 requires a value" >&2; exit 2; }
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "setup-codex: unknown flag: $1" >&2
      exit 2
      ;;
    *)
      shift
      ;;
  esac
done

echo "setup-codex: nothing to register (no-op since phase F-A removed the peers MCP server)."
exit 0
