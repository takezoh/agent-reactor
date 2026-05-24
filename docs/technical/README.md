# Technical Documentation

Internals organized by the three architecture layers. The canonical overview — vision, design principles, the layer trees, import boundaries, feature flags, side-effect naming, and dependencies — is in [ARCHITECTURE.md](../../ARCHITECTURE.md). This directory holds the per-layer deep dives.

## The three layers

```
platform/      Shared infrastructure — roost and orchestrator both depend on this
client/        roost-specific code — TUI, state machine, runtime, drivers, connectors
orchestrator/  Symphony SPEC implementation — poll/dispatch/reconcile + observability HTTP
cmd/           Binary entry points
```

Import direction (enforced by `depguard`, `src/.golangci.yml`): `cmd/* → client/* + orchestrator/* + platform/*` with no reverse. `platform/*` imports neither `client/*` nor `orchestrator/*`; `client/*` does not import `orchestrator/*`; `orchestrator/*` does not import `client/*`.

## Per-layer deep dives

- **[platform/](platform/README.md)** — shared infrastructure
  - [Spawn & launch](platform/spawn-and-launch.md) — `agentlaunch`/`procgroup`/`pathmap`: the command-string → process launch layer
  - [Brokers](platform/brokers.md) — `hostexec`/`mcpproxy`/`credproxy`: host mediation and policy enforcement
  - [Agent protocol](platform/agent-protocol.md) — `codexclient`/`codexschema`/`lib`: the Codex app-server stdio protocol
  - [Sandbox backends](platform/sandbox.md) — per-project devcontainer isolation, image resolution, credential proxy
- **[client/](client/README.md)** — the roost session lifecycle manager
  - [Process model](client/process-model.md) — daemon/TUI processes, pane layout, rendering boundary
  - [IPC and tool system](client/ipc.md) — message format, command surface, concurrency model
  - [State monitoring](client/state-monitoring.md) — driver plugins, the polling pipeline, hook routing
  - [Interfaces](client/interfaces.md) — Go type definitions, data files, source tree
- **[orchestrator/](orchestrator/README.md)** — the autonomous Symphony pipeline
  - [Symphony conformance](orchestrator/symphony-conformance.md) — SPEC §17 ↔ test table and documented posture

## Cross-cutting

- **[Guardrails](guardrails.md)** — a cross-cutting catalogue of the enforcement mechanisms that keep the architecture honest: import boundaries (10 depguard rules), no mutexes in `state/`, function/file length, the orchestrator's runtime gates (preflight / eligibility / slot / claim), security brokers, feature flags, and the wire-format convention.
