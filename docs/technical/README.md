# Technical Documentation

Internals organized by the three architecture layers. The canonical overview — scope, design principles, the layer trees, and import boundaries — is in [ARCHITECTURE.md](../../ARCHITECTURE.md). This directory holds the per-layer deep dives.

## The three layers

```
platform/      Shared infrastructure — the client and orchestrator both depend on this
client/        client-specific code — state machine, runtime, drivers, IPC, web frontend
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
- **[client/](client/README.md)** — the client session lifecycle manager
  - [Process model](client/process-model.md) — daemon process, pty multiplexer, rendering boundary
  - [IPC and tool system](client/ipc.md) — message format, command surface, concurrency model
  - [State monitoring](client/state-monitoring.md) — driver plugins, the polling pipeline, hook routing
  - [Interfaces](client/interfaces.md) — Go type definitions, data files, source tree
- **[orchestrator/](orchestrator/README.md)** — the autonomous Symphony pipeline
  - [Symphony conformance](orchestrator/symphony-conformance.md) — SPEC §17 ↔ test table and documented posture

## Cross-cutting

- **[Guardrails](guardrails.md)** — controlling the autonomous agents the orchestrator dispatches: admission (eligibility / blockers / claim), concurrency caps, capability sandboxing (devcontainer / hostexec / mcpproxy / credproxy), autonomy policy (approval & sandbox, requestUserInput hard-fail), and liveness bounds (timeouts, retry/backoff).
- **[Code & architecture enforcement](code-enforcement.md)** — keeping the codebase true to its architecture: import boundaries (10 depguard rules), no mutexes in `state/`, function/file length, feature-flag mechanics, and the wire-format convention.
- **[Harness engineering assessment](harness-engineering-assessment.md)** — a dated evaluation of how well agent-reactor (the outer harness) drives Claude/Codex (the inner harness), graded across design, implementation, test, documentation, and CI, with prioritized recommendations.
