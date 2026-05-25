# User Guide

Documentation for running Agent Roost. Start here if you want to launch agents and watch their status, or run an unattended pipeline.

Agent Roost ships three binaries that map onto the [three-layer architecture](../../ARCHITECTURE.md):

| Binary | Layer | What it is for |
|---|---|---|
| `roost` | client | Interactive tmux TUI for launching and supervising agent sessions |
| `orchestrator` | orchestrator | Unattended scheduler that reads a `WORKFLOW.md` and drives agents against a tracker |
| `claude-app-server` | platform / orchestrator | Codex app-server shim that lets the orchestrator drive a Claude agent |

## Pages

- [Getting started](getting-started.md) — requirements, `make install`, first run, choosing a binary, agent setup
- [roost TUI](roost-tui.md) — the `client` layer for end users: key bindings, session states, command palette, configuration
- [orchestrator](orchestrator.md) — the `orchestrator` layer for end users: running a `WORKFLOW.md` pipeline, agent selection, observability HTTP
- [sandbox setup](sandbox.md) — the `platform` layer for end users: per-project devcontainer isolation and credential proxy

For internals, see the [technical docs](../technical/README.md).
