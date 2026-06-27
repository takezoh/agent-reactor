#!/usr/bin/env bash
# Register arc hooks in Gemini CLI's settings.json.
#
# Mirror of scripts/setup-claude.sh — replaces the in-binary `arc gemini setup`
# subcommand (deleted in phase F-C) with a jq-driven editor. Gemini's hook
# entry format is the same `{hooks: [{type, command}]}` shape Claude uses, so
# the script structure stays identical; the only differences are the event
# list (Gemini exposes a smaller, differently-named set) and the hook command
# trailer (`event gemini` instead of `event claude`).
#
# Usage:
#   scripts/setup-gemini.sh [SERVER_BIN] [--data-dir DIR] [--settings PATH]
#
# Arguments:
#   SERVER_BIN       Absolute path to the arc/server binary the hook should
#                    invoke. If omitted, the script tries (in order):
#                      1. $SERVER_BIN environment variable
#                      2. ./arc relative to the repo root (script dir / ..)
#                      3. `which arc`
#                      4. exits non-zero with an error
#
# Options:
#   --data-dir DIR   Pass `-data-dir DIR` to the hook command. Use this when
#                    the daemon was started with a non-default ROOST_DATA_DIR
#                    so the hook reaches the correct socket.
#   --settings PATH  Override the settings.json path. Default:
#                    $GEMINI_SETTINGS_PATH if set, else ~/.gemini/settings.json
#   -h, --help       Show this help.
#
# Behavior:
#   - Creates a `<settings>.bak` of the existing file before each write.
#   - Adds the hook entry to every Gemini event listed in EVENTS (below).
#   - Skips events whose hook list already contains the exact command (idempotent).
#
# Requires: bash, jq (>= 1.6). On Debian/Ubuntu: `sudo apt install jq`.

set -euo pipefail

EVENTS=(
  SessionStart
  SessionEnd
  BeforeTool
  AfterTool
  BeforeAgent
  AfterAgent
  Notification
  PreCompress
)

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
}

# ---------- argument parsing -------------------------------------------------

SERVER_BIN="${SERVER_BIN:-}"
DATA_DIR=""
SETTINGS_PATH="${GEMINI_SETTINGS_PATH:-$HOME/.gemini/settings.json}"

while [ $# -gt 0 ]; do
  case "$1" in
    --data-dir)
      [ $# -ge 2 ] || { echo "setup-gemini: --data-dir requires a value" >&2; exit 2; }
      DATA_DIR="$2"
      shift 2
      ;;
    --settings)
      [ $# -ge 2 ] || { echo "setup-gemini: --settings requires a value" >&2; exit 2; }
      SETTINGS_PATH="$2"
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
      echo "setup-gemini: unknown flag: $1" >&2
      exit 2
      ;;
    *)
      if [ -z "$SERVER_BIN" ]; then
        SERVER_BIN="$1"
      else
        echo "setup-gemini: unexpected positional argument: $1" >&2
        exit 2
      fi
      shift
      ;;
  esac
done

# ---------- prerequisites ----------------------------------------------------

if ! command -v jq >/dev/null 2>&1; then
  echo "setup-gemini: jq is required but not installed. On Debian/Ubuntu: sudo apt install jq" >&2
  exit 1
fi

# ---------- resolve server binary --------------------------------------------

resolve_server_bin() {
  if [ -n "$SERVER_BIN" ]; then
    return
  fi
  local script_dir repo_root candidate
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  repo_root="$(cd "$script_dir/.." && pwd)"
  for candidate in "$repo_root/arc" "$repo_root/server"; do
    if [ -x "$candidate" ]; then
      SERVER_BIN="$candidate"
      return
    fi
  done
  if command -v arc >/dev/null 2>&1; then
    SERVER_BIN="$(command -v arc)"
    return
  fi
  echo "setup-gemini: cannot locate the arc/server binary; pass it as the first argument or set SERVER_BIN" >&2
  exit 1
}

resolve_server_bin

if command -v readlink >/dev/null 2>&1; then
  if resolved=$(readlink -f "$SERVER_BIN" 2>/dev/null); then
    SERVER_BIN="$resolved"
  fi
fi

# ---------- build hook command -----------------------------------------------

# Flag goes AFTER `event gemini` so the in-binary parser (which manually
# scans for -data-dir anywhere in args) and the human reader both see the
# subcommand first.
#
# printf %q produces a shell-safe escaped form so the persisted HOOK_CMD
# round-trips when SERVER_BIN or DATA_DIR contains spaces or shell
# metacharacters — the agent will hand this string back to a shell.
if [ -n "$DATA_DIR" ]; then
  HOOK_CMD="$(printf '%q' "$SERVER_BIN") event gemini -data-dir $(printf '%q' "$DATA_DIR")"
else
  HOOK_CMD="$(printf '%q' "$SERVER_BIN") event gemini"
fi

# ---------- write settings ---------------------------------------------------

mkdir -p "$(dirname "$SETTINGS_PATH")"

if [ ! -f "$SETTINGS_PATH" ]; then
  echo "{}" >"$SETTINGS_PATH"
fi

cp "$SETTINGS_PATH" "$SETTINGS_PATH.bak"

events_json=$(printf '%s\n' "${EVENTS[@]}" | jq -R . | jq -s .)

tmp_out=$(mktemp)
trap 'rm -f "$tmp_out"' EXIT

jq \
  --arg cmd "$HOOK_CMD" \
  --argjson events "$events_json" \
  '
    .hooks //= {}
    | reduce $events[] as $ev (
        {settings: ., registered: []};
        .settings.hooks[$ev] as $entries
        | ($entries // []) as $list
        | ($list | map(select(.hooks // [] | any(.command == $cmd))) | length > 0) as $already
        | if $already then
            .
          else
            .settings.hooks[$ev] = ($list + [{hooks: [{type: "command", command: $cmd}]}])
            | .registered += [$ev]
          end
      )
  ' "$SETTINGS_PATH" >"$tmp_out"

registered_count=$(jq -r '.registered | length' "$tmp_out")

if [ "$registered_count" -eq 0 ]; then
  echo "setup-gemini: hooks already registered (settings: $SETTINGS_PATH)"
  rm -f "$SETTINGS_PATH.bak"
  exit 0
fi

jq '.settings' "$tmp_out" >"$SETTINGS_PATH"

echo "setup-gemini: registered events: $(jq -r '.registered | join(", ")' "$tmp_out")"
echo "setup-gemini:   command:  $HOOK_CMD"
echo "setup-gemini:   settings: $SETTINGS_PATH (backup: $SETTINGS_PATH.bak)"
