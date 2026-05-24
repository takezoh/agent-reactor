# roost TUI

`roost` is the interactive tmux control surface — the user-facing side of the [`client` layer](../technical/client/README.md). It launches agent sessions, shows their status at a glance, and lets you jump into any of them instantly. Sessions are built on tmux, so closing the UI or dropping the connection does not stop the work.

```bash
roost
```

Creates a tmux session (or attaches to an existing one) and launches a 3-pane layout: a MAIN pane (the active session), a SESSIONS pane (the list of all sessions), and a LOG pane.

## Key bindings

**Prefix bindings** work regardless of which pane is focused (pane navigation, detach, palette open). **SESSIONS pane bindings** are active only when the SESSIONS pane is focused (session operations).

### Prefix keys

Default prefix: `Ctrl+b` (same as tmux default). Configurable via `[tmux] prefix`.

| Key | Action |
|---|---|
| `prefix Space` | Toggle MAIN ↔ SESSIONS pane |
| `prefix Escape` | Preview project |
| `prefix z` | Zoom MAIN pane |
| `prefix d` | Detach (tmux stays alive; re-run `roost` to resume) |
| `prefix q` | Quit all (tmux session is destroyed) |
| `prefix p` | Command palette |
| `prefix C-p` | Push-driver palette (overlay a new agent onto the current session) |

`detach` (`prefix d`) keeps containers alive for warm restart; `quit all` (`prefix q`) destroys the tmux session but preserves `sessions.json` so the next launch restores your sessions. The internal distinction is documented in [process model — detach vs shutdown](../technical/client/process-model.md#detach-vs-shutdown).

### Command palette (`prefix p`)

Displayed as a popup. Filter tools by typing, press Enter to execute.

```text
> new█
▸ new-session       Create session
  create-project    Create new project dir and start session
```

| Tool | Description |
|---|---|
| `new-session` | Create session |
| `create-project` | Create new project dir and start session |
| `stop-session` | Stop session |
| `detach` | Detach (keep session) |
| `shutdown` | Shutdown (discard sessions) |

### SESSIONS pane bindings

| Key | Action |
|---|---|
| `j`/`k` or `↑`/`↓` | Select session (previews in MAIN pane) |
| `Enter` | Switch to selected session → return to MAIN |
| `n` | Quick launch (default command) |
| `N` | Launch with command selection |
| `d` | Stop session |
| `Tab` | Collapse/expand project |
| `1`-`5` | Toggle status filter (Running / Waiting / Idle / Stopped / Pending) |
| `0` | Reset filter |
| `[` / `]` | Cycle workspace (shown only when 2+ workspaces exist) |
| `` ` `` | Reset workspace filter to All |

## Session states

| Display | State |
|---|---|
| `●` green | Running (producing output) |
| `◆` yellow | Waiting (awaiting input) |
| `◇` yellow | Pending approval (awaiting tool execution permission) |
| `○` gray | Idle (no output for 30+ seconds) |
| `■` red | Stopped |

How these states are detected (driver plugins, the polling pipeline, hook events) is described in [state monitoring](../technical/client/state-monitoring.md).

### Codex notes

- `roost codex setup` only registers MCP config.
- Codex status and approvals work without hook registration.
- Codex transcripts are display-only; session state is managed by roost.

## Configuration

roost works with default values even without a config file. To customize, create `~/.roost/settings.toml`:

```toml
# ~/.roost/settings.toml

# data_dir = ""                 # Override config/data directory (default: ~/.roost)

[log]
level = "info"                  # "debug" | "info" | "warn" | "error"

[tmux]
session_name = "roost"
prefix = "C-b"                  # Prefix key
pane_ratio_horizontal = 75      # Main pane width % (1-99)
pane_ratio_vertical = 75        # Main pane height % (1-99)

[monitor]
poll_interval_ms = 1000         # Background polling interval
fast_poll_interval_ms = 100     # Polling interval while TUI is active
idle_threshold_sec = 30         # Seconds of silence before "Idle" (○)

[session]
auto_name = true                # Auto-generate session names
default_command = "shell"       # Command run by `n` (quick launch)
commands = [                    # Commands available via `N`
  "claude",
  "codex",
  "gemini",
  "shell",
]
push_commands = [               # Commands available via push-driver palette
  "shell",
  "git diff",
  "git diff --staged",
]

[projects]
project_roots = ["~/projects"]  # Subdirs of each root become projects
project_paths = ["~/myproject"] # Explicit project paths

[sandbox]
mode = "direct"                 # "direct" (default) | "devcontainer"  — see sandbox.md

[driver]
# summarize_command = "claude -p --model=haiku --no-session-persistence --setting-sources user"

[drivers.claude]
show_thinking = false           # Show extended thinking blocks in MAIN pane

# Desktop notifications — empty rules = disabled.
# Each rule AND-combines driver / command / project / kind.
# Empty string or "*" on any axis means "match any".
# Driver / command / project are glob patterns (path.Match); project supports "~" expansion.

[[notifications.rules]]
driver  = "claude"
project = "~/projects/prjA"
kind    = "pending_approval"

[[notifications.rules]]
kind = "done"
```

Sandbox-related keys (`[sandbox]`, `[sandbox.devcontainer]`, `[sandbox.proxy]`) are documented in [sandbox setup](sandbox.md).

### Per-project configuration

Each project directory can have its own `.roost/settings.toml`:

```toml
# <project-dir>/.roost/settings.toml

[workspace]
name = "work"    # Group this project under a named workspace
```

The workspace switcher chip bar appears in the SESSIONS pane automatically when two or more distinct workspaces exist, and is hidden for single-workspace setups. Projects without a settings file fall back to the `default` workspace.
