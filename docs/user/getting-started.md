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

Hook registration is owned by the runtime — there is no manual install step.

- **Claude (host)**: the daemon registers `~/.claude/settings.json` against its own binary path at every boot (`cmd/server/coordinator.go:registerHostAgentHooks`). Idempotent: an unchanged settings file is a no-op; a stale entry pointing at an older binary path is rewritten in place.
- **Gemini (host)**: same code path, `~/.gemini/settings.json`. Different event vocabulary (`BeforeTool` / `AfterTool` / …) but the same `client/lib/agenthook.Install` call.
- **Inside a devcontainer**: each devcontainer's `postCreate` runs `reactor-bridge claude-setup-hooks` and `reactor-bridge gemini-setup-hooks`, so the in-container settings files point at `/opt/agent-reactor/run/reactor-bridge`. Same `agenthook` package as the host path.
- **Codex**: no hooks. The backend has a built-in Codex integration via the app-server protocol.

If the registration ever needs to be inspected or rebuilt by hand, the canonical JSON shape lives in `client/lib/agenthook.Install` and the per-agent event lists are exported as `agenthook.Claude` / `agenthook.Gemini`.

## Next steps

- Run the backend + browser UI together: [web stack](web-server.md)
- Deploy the two-process stack as a system service: [systemd service](systemd.md)
- Isolate each agent in a container: [sandbox setup](sandbox.md)
- Automate issue work end to end: [orchestrator](orchestrator.md)
