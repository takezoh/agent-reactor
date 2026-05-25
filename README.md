# Agent Roost

**Run many AI agents in parallel without losing track of any of them.**

Agent Roost is a tmux-based control surface for running Claude, Codex, Gemini, and other CLI agents across all your projects at once. It replaces the manual work of opening tabs, remembering which agent is doing what, and checking back for completion — and it can also run agents unattended against an issue tracker.

### What it does

- **Launch an agent without typing commands.** Select a project from the list and Roost handles the directory, environment, and command for you.
- **See every agent's status at a glance.** Each session shows whether the agent is running, waiting for your input, awaiting tool approval, or idle.
- **Jump into any session instantly.** Live-preview a session, then press Enter to take over. Supervise dozens of concurrent tasks without losing focus.
- **Keep agents running after you disconnect.** Built on tmux, so closing the UI or dropping the connection doesn't stop the work.
- **Run each agent in its own sandbox.** Optional per-project devcontainer with brokered AWS / gcloud / SSH credentials and a policy-gated host-exec channel. Long-lived secrets stay on the host.
- **Automate end to end.** The orchestrator reads a `WORKFLOW.md`, polls a tracker, and drives agents through issues with no human in the loop.

## Three binaries, three layers

This module builds three binaries from a single Go module, mapping onto a three-layer architecture (`platform/` · `client/` · `orchestrator/`):

- **`roost`** — the tmux control surface (interactive TUI for managing agent sessions) — the `client` layer
- **`orchestrator`** — the scheduling brain that reads `WORKFLOW.md`, dispatches work to agents, and reconciles state — the `orchestrator` layer
- **`claude-app-server`** — a stdio JSON-RPC shim that wraps a Claude agent as a drop-in Codex app-server

The layers and their enforced import boundaries are defined in [ARCHITECTURE.md](ARCHITECTURE.md).

## Requirements

- Go 1.26+
- tmux 3.2+

## Install

```bash
make install
```

Installs to `~/.local/bin/roost`. Then:

```bash
roost                 # create/attach a tmux session with a 3-pane layout
roost claude setup    # register agent integration (also: codex / gemini)
```

See [Getting started](docs/user/getting-started.md) for the full walkthrough.

## Documentation

Documentation is organized by **audience × architecture layer** under [`docs/`](docs/README.md).

| | Start here |
|---|---|
| **Using the tools** | [User guide](docs/user/README.md) — [getting started](docs/user/getting-started.md), [roost TUI](docs/user/roost-tui.md), [orchestrator](docs/user/orchestrator.md), [sandbox](docs/user/sandbox.md) |
| **Working in the repo** | [Agent guide](docs/agent/README.md) — [contributing](docs/agent/contributing.md), [WORKFLOW.md authoring](docs/agent/workflow-authoring.md), [testing](docs/agent/testing.md) |
| **Internals** | [Technical docs](docs/technical/README.md) — [platform/](docs/technical/platform/README.md), [client/](docs/technical/client/README.md), [orchestrator/](docs/technical/orchestrator/README.md) · [ARCHITECTURE.md](ARCHITECTURE.md) |
