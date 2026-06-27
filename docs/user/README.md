# User Guide

Documentation for running Agent Reactor. Start here if you want to launch agents and watch their status, or run an unattended pipeline.

Agent Reactor ships three binaries that map onto the [three-layer architecture](../../ARCHITECTURE.md):

| Binary | Layer | What it is for |
|---|---|---|
| `server` | client | Single-process backend — session daemon + HTTP/WS gateway in one binary (xterm.js front-end runs through the embedded gateway) |
| `web` | client | Browser UI host — serves the React/xterm.js bundle and reverse-proxies REST/WS to `server` |
| `orchestrator` | orchestrator | Unattended scheduler that reads a `WORKFLOW.md` and drives agents against a tracker |
| `claude-app-server` | platform / orchestrator | Codex app-server shim that lets the orchestrator drive a Claude agent |

## Pages

- [Getting started](getting-started.md) — requirements, `make install`, first run, choosing a binary, agent setup
- [web stack (ad-hoc launch)](web-server.md) — the browser-facing `server` + `web` processes for local/dev use
- [run as a systemd service](systemd.md) — production stack (`server` + `web`) as per-user systemd units, with token persistence and boot-time autostart
- [orchestrator](orchestrator.md) — the `orchestrator` layer for end users: running a `WORKFLOW.md` pipeline, agent selection, observability HTTP
- [sandbox setup](sandbox.md) — the `platform` layer for end users: per-project devcontainer isolation and credential proxy

For internals, see the [technical docs](../technical/README.md).
