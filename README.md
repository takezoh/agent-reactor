# Agent Roost

**Run many AI agents in parallel without losing track of any of them.**

Agent Roost is a tmux-based control surface for running Claude, Codex, Gemini, and other CLI agents across all your projects at once. It replaces the manual work of opening tabs, remembering which agent is doing what, and checking back for completion.

### What it does

- **Launch an agent without typing commands.** Select a project from the list and Roost handles the directory, environment, and command for you.
- **See every agent's status at a glance.** Each session shows whether the agent is running, waiting for your input, awaiting tool approval, or idle. No tab-switching to find out who needs you.
- **Jump into any session instantly.** Move the cursor over a session for a live preview; press Enter to take over. Supervise dozens of concurrent tasks without losing focus.
- **Keep agents running after you disconnect.** Built on tmux, so closing the UI or dropping the connection doesn't stop the work.
- **Run each agent in its own sandbox.** Optional per-project devcontainer with brokered AWS / gcloud / SSH credentials and a policy-gated host-exec channel. Long-lived secrets stay on the host; the container only sees short-lived tokens or stdio.

## Requirements

- Go 1.26+
- tmux 3.2+

## Installation

```bash
make install
```

Installs to `~/.local/bin/roost`.

## Usage

```bash
roost
```

Creates a tmux session (or attaches to an existing one) and launches with a 3-pane layout.

### Hook Setup

Register roost hooks so agent status (● ◆ ◇ ○ ■) updates in real time. Run once per agent type:

```bash
roost claude setup    # registers hooks in ~/.claude/settings.json
roost codex setup     # registers hooks in ~/.codex/
roost gemini setup    # registers hooks in ~/.gemini/settings.json
```

Hooks are idempotent — re-running adds only missing entries and never overwrites existing config.

Without hooks, roost still launches sessions but status detection degrades to polling (idle detection only).

### Key Bindings

**Prefix bindings** work regardless of which pane is focused (pane navigation, detach, palette open).
**SESSIONS pane bindings** are active only when the SESSIONS pane is focused (session operations).

#### Prefix Keys

Default prefix: `Ctrl+b` (same as tmux default). Configurable via `[tmux] prefix`.

| Key | Action |
|------|-----------|
| `prefix Space` | Toggle MAIN ↔ SESSIONS pane |
| `prefix Escape` | Preview project |
| `prefix z` | Zoom MAIN pane |
| `prefix d` | Detach (tmux stays alive; re-run `roost` to resume) |
| `prefix q` | Quit all (tmux session is destroyed) |
| `prefix p` | Command palette |
| `prefix C-p` | Push driver palette (overlay a new agent onto the current session) |

#### Command Palette (`prefix p`)

Displayed as a popup. Filter tools by typing, press Enter to execute.

```text
> new█
▸ new-session       Create session
  create-project    Create new project dir and start session
```

| Tool | Description |
|--------|------|
| `new-session` | Create session |
| `create-project` | Create new project dir and start session |
| `stop-session` | Stop session |
| `detach` | Detach (keep session) |
| `shutdown` | Shutdown (discard sessions) |

#### SESSIONS Pane Bindings

| Key | Action |
|------|-----------|
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

### Session States

| Display | State |
|------|------|
| `●` green | Running (producing output) |
| `◆` yellow | Waiting (awaiting input) |
| `◇` yellow | Pending approval (awaiting tool execution permission) |
| `○` gray | Idle (no output for 30+ seconds) |
| `■` red | Stopped |

## Configuration

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
project_roots = ["~/projects"]       # Subdirs of each root become projects
project_paths = ["~/myproject"] # Explicit project paths

[sandbox]
mode = "devcontainer"               # "direct" (default) | "devcontainer"

[sandbox.devcontainer]
# extra_create_args = []            # appended to "docker create"
# env_script = ""                   # script that prints KEY=VALUE lines for a project
# host_path_mount_prefix = ""       # prefix for the auto-mounted workspace inside the container
                                    # empty (default): mirror host path as-is (e.g. /home/u/proj → /home/u/proj)
                                    # "/mnt": prepend prefix (e.g. /home/u/proj → /mnt/home/u/proj)
                                    # ignored when devcontainer.json sets workspaceFolder or workspaceMount

# [sandbox.proxy]                   # credential proxy — see docs/sandbox-setup.md
# aws_profiles = ["default"]        # populate per-provider keys to activate

[driver]
# summarize_command = "claude -p --model=haiku --no-session-persistence --setting-sources user"
# summarize_command = "codex exec --ephemeral --model gpt-4o-mini -"
# summarize_command = "gemini -p '' -m gemini-2.5-flash-lite"

[drivers.claude]
show_thinking = false           # Show extended thinking blocks in MAIN pane

# Desktop notifications — empty rules = disabled.
# Each rule AND-combines driver / command / project / kind.
# Empty string or "*" on any axis means "match any".
# Driver / command / project are glob patterns (path.Match);
# project supports "~" expansion.

[[notifications.rules]]
# claude requests tool approval under ~/projects/prjA
driver  = "claude"
project = "~/projects/prjA"
kind    = "pending_approval"

[[notifications.rules]]
# any agent finishes its turn
kind = "done"
```

Works with default values even without a config file.

### Per-Project Configuration

Each project directory can have its own `.roost/settings.toml`:

```toml
# <project-dir>/.roost/settings.toml

[workspace]
name = "work"    # Group this project under a named workspace
```

The workspace switcher chip bar appears in the SESSIONS pane automatically when
two or more distinct workspaces exist, and is hidden for single-workspace setups.
Projects without a settings file fall back to the `default` workspace.

## Sandbox

Run each agent inside a project-scoped devcontainer, isolating filesystem, network, and credentials per project.

- Setup and config: [docs/sandbox-setup.md](docs/sandbox-setup.md)
- Architecture and security model: [docs/sandbox.md](docs/sandbox.md)

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md).
