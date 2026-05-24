# Architecture

This is the canonical overview of the system: its vision, design principles, the three-layer structure, and the cross-cutting conventions (feature flags, side-effect naming, dependencies). **Per-layer deep dives** — terminology, package responsibilities, design decisions, and dependency graphs — live under [`docs/technical/`](docs/technical/README.md).

## Vision

When running AI agents across multiple projects, you lose track of which agents are working, which are waiting for input, and which need tool approval. Switching between them in raw tmux is slow and error-prone. roost solves this: launch sessions in seconds, see their status at a glance, and switch instantly.

roost is a session lifecycle manager — not an agent orchestrator. It does not control what agents do; it gives you visibility and fast access to all of them from a single tmux-based TUI. The separate `orchestrator` binary *does* drive agents autonomously, against an issue tracker — a different concern in a different layer.

## Design Principles

- **Functional Core / Imperative Shell**: All state transitions are expressed as a pure function `state.Reduce(state, event) → (state', effects)`. I/O is emitted as `Effect` values and interpreted by a single event loop (runtime). No goroutines, mutexes, or actors exist in the state layer
- **Driver as Value Type**: Drivers are stateless plugins. Per-frame state is embedded as a `DriverState` value on each `SessionFrame` and round-trips through `Driver.Step`. No goroutines. Drivers run synchronously inside `state.Reduce`; the only permitted synchronous I/O is **bounded read-only filesystem stat** (e.g. checking whether a resume file exists before building a launch command). Subprocess execution, network I/O, and writes must be returned as `[]Effect` and dispatched via the worker pool
- **Single event loop**: State mutation is exclusively owned by one goroutine. Long-lived I/O readers may only emit events — they never read or write state. The worker pool (discrete jobs) and stream readers (continuous sources) are both concrete instances of this general principle. No mutexes are needed outside these sources
- **Driver/Connector/Subsystem isolation**: Concepts specific to `driver/`, `connector/`, and tool-specific helpers in `lib/<tool>/` must not leak into `state/`, `runtime/`, `tui/`, `proto/`, or `sandbox/`. TUI never branches on driver or connector name. `sandbox/` backends are tool-agnostic and never import `driver/` or `lib/<tool>/`. Tool-specific host paths (e.g. `~/.claude*`) must not be hardcoded in any Go source — they live in user config (`~/.roost/settings.toml`). `main.go` only wires generic values from config into runtime/sandbox; it does not embed tool-specific defaults. Tool-specific environment variable names (`AWS_*`, `ANTHROPIC_*`, `GOOGLE_*`, `OPENAI_*`, etc.) must not appear as string literals in `state/`, `runtime/`, `sandbox/`, `tui/`, or `proto/` — they live exclusively in the external `credproxy` library's `providers/<name>/` packages, the local `hostexec/` package, or `lib/<tool>/`. Subsystem-specific backends (e.g. the `stream` subsystem fronting codex app-server) live in `runtime/subsystem/<kind>/` and are the only files permitted to import `driver/<tool>`. The subsystem kind names (`cli`, `stream`) and `SubsystemID` bookkeeping exist in `runtime/` and `state/`; subsystem implementations themselves do not. **Enforcement**: import-boundary violations are caught by `depguard` (see `src/.golangci.yml`); name-literal leaks are caught by `runtime/isolation_test.go`
- **No fallbacks**: Do not synthesize "if source A is unavailable, use B". Until `Driver.Step` updates the state, the status does not change

## Documentation

All documentation is organized by **audience × architecture layer** under [`docs/`](docs/README.md):

- **Users** — [user guide](docs/user/README.md): [getting started](docs/user/getting-started.md), [roost TUI](docs/user/roost-tui.md), [orchestrator](docs/user/orchestrator.md), [sandbox](docs/user/sandbox.md)
- **Agents / contributors** — [agent guide](docs/agent/README.md): [contributing](docs/agent/contributing.md), [WORKFLOW.md authoring](docs/agent/workflow-authoring.md), [testing](docs/agent/testing.md)
- **Technical (per layer)** — [platform/](docs/technical/platform/README.md) · [client/](docs/technical/client/README.md) · [orchestrator/](docs/technical/orchestrator/README.md)
- **Cross-cutting** — [agent guardrails](docs/technical/guardrails.md) (controlling autonomous agents: admission, concurrency, capability, autonomy, liveness) · [code & architecture enforcement](docs/technical/code-enforcement.md) (the enforcement side of this document: depguard rules, length limits, feature-flag mechanics)

## Layer Structure

Three top-level trees under `src/`:

```
platform/      Shared infrastructure — roost and orchestrator both depend on this
client/        roost-specific code — TUI, state machine, runtime, drivers, connectors
orchestrator/  Symphony SPEC implementation — poll/dispatch/reconcile + observability HTTP
cmd/           Binary entry points — cmd/roost/, cmd/roost-bridge/, cmd/orchestrator/, cmd/claude-app-server/
```

**Import direction**: `cmd/*` → `client/*` + `orchestrator/*` + `platform/*` → (no reverse). The three-layer boundary is enforced by `depguard` (see `src/.golangci.yml`, rules `platform-no-client-or-orchestrator` and `client-no-orchestrator`):

- `platform/*` imports neither `client/*` nor `orchestrator/*`
- `client/*` does not import `orchestrator/*`
- `orchestrator/*` does not import `client/*`

The full set of `depguard` rules (including the intra-`client/` isolation rules) and every other code-level enforcement mechanism are catalogued in [code & architecture enforcement](docs/technical/code-enforcement.md).

### The layers at a glance

- **[`platform/`](docs/technical/platform/README.md)** — shared base: the agent-launch primitive (`agentlaunch`: argv-based `Spawn` + `SplitArgs`, host/container `Dispatcher`, on `procgroup`), sandbox backends, host-exec and MCP-proxy brokers, path translation, logger, feature flags, tool wrappers (`lib/<tool>`), trackers, metrics, credential providers. Tool-specific knowledge is allowed here so it stays out of the generic layers above. Agent-agnostic launch lives here; per-agent command construction stays in `lib/<tool>`, while transport, `codexclient.Conn`, and `Handler` remain per-layer.
- **[`client/`](docs/technical/client/README.md)** — all of roost: the pure `state/` domain core, `runtime/` imperative shell, value-type `driver/` and `connector/` plugins, `runtime/subsystem/` (`cli` and `stream`), the `proto/` IPC wire layer, and the Bubbletea `tui/`. Terminology, the design-decision log, and the full dependency graph are documented there.
- **[`orchestrator/`](docs/technical/orchestrator/README.md)** — a TUI-less, single-authority service implementing the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md): `workflowfile/`, `wfconfig/`, `scheduler/` (poll/dispatch/reconcile), `workspace/`, `agent/`, `prompt/`, `httpserver/`, `lineargql/`. It shares `platform/` with roost but does not import `client/`. SPEC ↔ package correspondence and deviation posture: [`docs/technical/orchestrator/symphony-conformance.md`](docs/technical/orchestrator/symphony-conformance.md) and [`plans/05-conformance.md`](plans/05-conformance.md).

Files matching `client/state/reduce_*.go` host state-machine dispatch tables. They are exempt from the 80-line function limit (see [AGENTS.md](AGENTS.md)) because forced extraction of dispatch arms fragments the state machine without adding clarity. File-length (500 lines) and naming rules still apply.

The daemon and TUI are separate processes communicating via typed IPC (`proto`) over a Unix socket, with two physical endpoints (host + container). Details, the per-package breakdown, terminology, and the design-decision log are in the [client deep dive](docs/technical/client/README.md).

## Feature Flags

Experimental features are gated by one of **two independent mechanisms**. They share no key space — pick one based on whether the experimental code should physically exist in the binary. ([code & architecture enforcement → feature flags](docs/technical/code-enforcement.md#4-feature-flags) has the step-by-step add procedure.)

| Mechanism | Where defined | Toggle | Code in binary? | Use when |
|---|---|---|---|---|
| **Runtime flag** | `features.Flag` constant + `features.Set` injected into `state.State` | `~/.roost/settings.toml` `[features.enabled]` | Yes (both branches always compiled) | The user should be able to opt-in without rebuilding |
| **Compile-time flag** | `features` package `const` guarded by `//go:build <tag>` | `go build -tags <tag>` (e.g. `make build-experimental`) | No (off-side is removed by dead code elimination) | The experimental code is unfinished, unsafe, or should not enter release binaries |

The C analogue: runtime flag is `if () {}`, compile-time flag is `#if / #endif`.

### Runtime flag — how to add

1. Add a `Flag` constant in `features/features.go` and append it to `features.All()`.
2. Reference it where needed: `if st.Features.On(features.MyFeature) { ... }`. Allowed in `state/`, `runtime/`, `tui/`. **Not** in `driver/` or `connector/` (driver-specific gating uses `config.Drivers[name]` instead).
3. Users opt in via:
   ```toml
   [features.enabled]
   my-feature = true
   ```
4. When the feature stabilises, delete the constant and inline the enabled branch. Unknown keys in user config are silently ignored, so no migration is needed.

### Compile-time flag — how to add

1. Create paired files in `features/` guarded by build tag:
   ```go
   //go:build my_feat
   package features
   const MyFeat = true
   ```
   ```go
   //go:build !my_feat
   package features
   const MyFeat = false
   ```
2. Gate code with `if features.MyFeat { ... }`. Because `MyFeat` is a `const`, the off-side branch is eliminated entirely from the binary.
3. For larger experimental code, put the implementation in a `//go:build my_feat` file and provide a no-op stub in `//go:build !my_feat`. Callers do not need to be guarded.
4. Add a Makefile target for first-class build variants (e.g. `make build-experimental`). CI should build both variants.

### What goes where

- The `features/` package imports nothing outside the standard library — `state/` can depend on it without breaking the self-contained core.
- `state.State.Features` is set once at startup and never mutated, preserving Reduce's purity.
- `tui/` receives the active flag list over `proto` (daemon → tui via `EvtSessionsChanged.Features`) and rebuilds its own `features.Set`. `proto` carries it as `[]string`, matching the existing pattern of crossing the wire as primitives.

## Side-Effect Naming Convention

Distinguish path computation from side effects by function name.

| Pattern | Side Effect | Example |
|---------|-------------|---------|
| `XxxPath()` | None (pure) | `LogDirPath`, `ConfigDirPath`, `LogPath` |
| `EnsureXxx()` | Directory creation | `EnsureLogDir`, `EnsureConfigDir` |
| `LoadFrom(path)` | File read only | `config.LoadFrom` |
| `Load()` | Directory creation + file read | `config.Load` (convenience wrapper) |

## Testing Strategy

Testability is a primary design constraint. The Functional Core / Imperative Shell split makes the core testable without mocks. Per-layer test patterns, the Coverage Tier scheme, and CI enforcement are documented in [docs/agent/testing.md](docs/agent/testing.md).

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `charm.land/bubbletea/v2` | v2.0.2 | TUI framework |
| `charm.land/lipgloss/v2` | v2.0.2 | Styling |
| `charm.land/bubbles/v2` | v2.1.0 | Key bindings |
| `github.com/BurntSushi/toml` | v1.6.0 | Configuration file |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File watching |
| `golang.org/x/term` | v0.41.0 | Terminal size detection |
