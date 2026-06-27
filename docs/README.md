# Documentation

Agent Reactor is one Go module that ships **three binaries** built on a **three-layer architecture**. This documentation is organized along two axes:

- **Audience** — who is reading: an end **user** running the tools, an **agent** (AI agent or contributor) doing work in the repo, or a developer who needs the **technical** internals.
- **Architecture layer** — which part of the system: `platform/` (shared infrastructure), `client/` (the session daemon + web frontend), or `orchestrator/` (the autonomous Symphony pipeline).

See [ARCHITECTURE.md](../ARCHITECTURE.md) for the canonical definition of the three layers and the import boundaries enforced by `depguard`.

## Audience × Layer map

| Audience \ Layer | platform | client (server) | orchestrator | Cross-cutting |
|---|---|---|---|---|
| **User** | [sandbox setup](user/sandbox.md) | [web stack](user/web-server.md) · [systemd service](user/systemd.md) | [orchestrator](user/orchestrator.md) | [getting started](user/getting-started.md) |
| **Agent** | — | — | [WORKFLOW.md authoring](agent/workflow-authoring.md) | [contributing](agent/contributing.md), [testing](agent/testing.md) |
| **Technical** | [platform/](technical/platform/README.md) | [client/](technical/client/README.md) | [orchestrator/](technical/orchestrator/README.md) | [ARCHITECTURE.md](../ARCHITECTURE.md) |

## By audience

### [User](user/README.md) — running the tools

You want to launch agents, watch their status, and (optionally) run an unattended pipeline.

- [Getting started](user/getting-started.md) — requirements, install, first run, choosing a binary, agent setup
- [web stack (ad-hoc launch)](user/web-server.md) — running `server` (daemon + HTTP/WS gateway) + `web` for browser-driven session management
- [run as a systemd service](user/systemd.md) — production deployment of the three-process stack with token persistence
- [orchestrator](user/orchestrator.md) — running an unattended pipeline from a `WORKFLOW.md`, agent selection, observability HTTP
- [sandbox setup](user/sandbox.md) — per-project devcontainer isolation and credential proxy configuration

### [Agent](agent/README.md) — doing work in the repo

You are an AI agent or a contributor changing the code, or authoring the workflow that drives the autonomous agent.

- [Contributing](agent/contributing.md) — build/test/vet/lint, coding rules, library selection
- [WORKFLOW.md authoring](agent/workflow-authoring.md) — front matter, the prompt template, the issue state flow, the `linear_graphql` tool
- [Testing](agent/testing.md) — testability as a design constraint, Tier-based coverage targets

### [Technical](technical/README.md) — internals by layer

You need to understand how a layer is built.

- [platform/](technical/platform/README.md) — shared infrastructure: sandbox, brokers, credential proxy, logger, trackers, tool wrappers
- [client/](technical/client/README.md) — agent-reactor's session daemon: Functional Core / Imperative Shell, the state machine, drivers, subsystems, IPC, web frontend
- [orchestrator/](technical/orchestrator/README.md) — the poll / dispatch / reconcile pipeline and Symphony SPEC conformance
