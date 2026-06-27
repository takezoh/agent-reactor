# Getting Started

## Requirements

- Go 1.26+
- Docker (for devcontainer-mode sandboxes; optional otherwise)

## Install

```bash
make install
```

Installs `server` to `~/.local/bin/server` (with support files under `~/.local/lib/agent-reactor/`).

To build the other binaries:

```bash
make build-server             # → ./server (daemon + HTTP/WS gateway)
make build-web                # → ./web    (browser UI host)
make build-orchestrator       # → ./orchestrator
make build-claude-app-server  # → ./claude-app-server
make build-all                # all main binaries
```

## Which binary do I want?

| If you want to… | Use | Guide |
|---|---|---|
| Launch and supervise agents interactively from a browser | `server` + `web` | [web stack](web-server.md) |
| Run the two-process stack as a system service | `server` + `web` | [systemd service](systemd.md) |
| Run an unattended pipeline against a tracker | `orchestrator` | [orchestrator](orchestrator.md) |
| Drive a Claude agent from the orchestrator (no Codex CLI) | `claude-app-server` | [orchestrator → agent selection](orchestrator.md#agent-selection) |

The binaries correspond to the architecture layers — see the [architecture overview](../../ARCHITECTURE.md).

## First run (server backend + browser UI)

For local development the fastest path is `make run-dev`, which boots an
isolated `server` backend (daemon + gateway) plus the `web` host together:

```bash
make run-dev
# → backend : http://127.0.0.1:8443  (sock: $ROOT/.run-dev/server/server.sock)
#   web     : http://127.0.0.1:8080
#   Open  →  http://127.0.0.1:8080/
```

The `server` binary runs **one process** that owns both the pty session
daemon (Unix-socket IPC) and the HTTP/WS gateway (browser-facing REST/WS).
Sessions are managed exclusively through that IPC; the gateway is just a
protocol translator. See [web stack](web-server.md) for the standalone path
and the per-flag reference.

## Agent setup

Register each agent integration once. Setup is **idempotent** — re-running adds only missing entries and never overwrites existing config. The setup scripts are plain `bash` + `jq` and live under `scripts/`:

```bash
# Pass the absolute path of the installed server binary as the first argument.
bash scripts/setup-claude.sh ~/.local/bin/server                      # registers hooks in ~/.claude/settings.json
bash scripts/setup-codex.sh  ~/.local/bin/server                      # no-op (reserved for future Codex hook surface)
bash scripts/setup-gemini.sh ~/.local/bin/server                      # registers hooks in ~/.gemini/settings.json
```

Each script accepts optional flags:

```bash
bash scripts/setup-claude.sh /path/to/server [--data-dir /custom/dir] [--settings /custom/settings.json]
bash scripts/setup-gemini.sh /path/to/server [--data-dir /custom/dir] [--settings /custom/settings.json]
bash scripts/setup-codex.sh  /path/to/server [--data-dir /custom/dir] [--settings /custom/settings.json]
```

- **Claude / Gemini**: hooks are required for real-time state updates.
- **Codex**: hooks are not used — the backend has a built-in Codex integration for state updates. `setup-codex.sh` is currently a no-op kept for symmetry; flags are accepted and ignored so callers can iterate uniformly over all three agents.

## Next steps

- Run the backend + browser UI together: [web stack](web-server.md)
- Deploy the two-process stack as a system service: [systemd service](systemd.md)
- Isolate each agent in a container: [sandbox setup](sandbox.md)
- Automate issue work end to end: [orchestrator](orchestrator.md)
