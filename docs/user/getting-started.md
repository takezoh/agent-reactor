# Getting Started

## Requirements

- Go 1.26+
- tmux 3.2+

## Install

```bash
make install
```

Installs `roost` to `~/.local/bin/roost` (with support files under `~/.local/lib/roost/`).

To build the other binaries:

```bash
make build-orchestrator       # → ./orchestrator
make build-claude-app-server  # → ./claude-app-server
make build-all                # all three main binaries
```

## Which binary do I want?

| If you want to… | Use | Guide |
|---|---|---|
| Launch and supervise agents interactively | `roost` | [roost TUI](roost-tui.md) |
| Run an unattended pipeline against a tracker | `orchestrator` | [orchestrator](orchestrator.md) |
| Drive a Claude agent from the orchestrator (no Codex CLI) | `claude-app-server` | [orchestrator → agent selection](orchestrator.md#agent-selection) |

The three binaries correspond to the three architecture layers — see the [architecture overview](../../ARCHITECTURE.md).

## First run (roost)

```bash
roost
```

Creates a tmux session (or attaches to an existing one) and launches a 3-pane layout. From the SESSIONS pane you can launch agents into any of your projects without typing commands. See [roost TUI](roost-tui.md) for the full key map.

## Agent setup

Register each agent integration once. Setup is **idempotent** — re-running adds only missing entries and never overwrites existing config.

```bash
roost claude setup    # registers hooks in ~/.claude/settings.json
roost codex setup     # registers the roost-peers MCP in ~/.codex/ (or $CODEX_CONFIG_DIR)
roost gemini setup    # registers hooks in ~/.gemini/settings.json
```

- **Claude / Gemini**: hooks are required for real-time state updates.
- **Codex**: hooks are not used — roost has a built-in Codex integration for state updates. `roost codex setup` only registers the `roost-peers` MCP server; it does not modify hook settings.

## Next steps

- Customize the TUI and configure projects: [roost TUI → Configuration](roost-tui.md#configuration)
- Isolate each agent in a container: [sandbox setup](sandbox.md)
- Automate issue work end to end: [orchestrator](orchestrator.md)
