# Architecture

This is the canonical overview of the system: its vision, design principles, the three-layer structure, and the cross-cutting conventions (feature flags, side-effect naming, dependencies). **Per-layer deep dives** — terminology, package responsibilities, design decisions, and dependency graphs — live under [`docs/technical/`](docs/technical/README.md).

## Vision

When running AI agents across multiple projects, you lose track of which agents are working, which are waiting for input, and which need tool approval. Switching between them in raw tmux is slow and error-prone. roost solves this: launch sessions in seconds, see their status at a glance, and switch instantly.

roost is a session lifecycle manager — not an agent orchestrator. It does not control what agents do; it gives you visibility and fast access to all of them from a single tmux-based TUI. The separate `orchestrator` binary *does* drive agents autonomously, against an issue tracker — a different concern in a different layer.

## Design Principles

The **core principles** below are normative for every layer that owns a decision loop; what differs is each layer's role, not whether the principles apply. The unifying goal is **testability** — and specifically the kind of testability that lets the code be written, tested, and corrected without a live environment: decision logic must be reachable by feeding inputs and asserting outputs, with no real I/O, concurrency, or wall-clock reads inside the code under test.

### Core principles (all layers)

- **Testability is the primary design constraint**: decision logic is a pure function of its inputs, so it can be exercised by feeding inputs and asserting outputs/state. "We can't test it" is a design defect, not a justification. This is the *why* behind the next two principles. Per-layer test patterns and the Coverage Tier scheme: [docs/agent/testing.md](docs/agent/testing.md)
- **Single-writer event loop**: state mutation is owned by one loop. Long-lived I/O sources (worker pool, stream readers, retry timers, file watchers) may only *emit events* to that loop — they never mutate state themselves. The roost `runtime` loop and the orchestrator's `scheduler.Run` (`src/orchestrator/scheduler/scheduler.go`, one `for { select {} }`) are both instances of this.
- **Decisions separated from I/O**: the code that decides *what should happen* is a pure function; I/O, concurrency, and live handles live in a thin imperative shell. The shell performs the I/O and feeds the result back to the core as the next event — it never lets I/O leak into the decision.
- **No fabricated fallbacks**: do not synthesize "if source A is unavailable, use B" in a way that invents truth. In roost the status does not change until `Driver.Step` updates it; in the orchestrator issue truth comes from the tracker via reconcile and is never faked (a failed workflow reload keeps last-known-good config but *gates* dispatch rather than fabricating issue state).

### Per-layer realizations

A layer's *role* decides how it realizes the core. The canonical detail lives in the per-layer deep dive linked under [Layer Structure](#layer-structure).

- **Decision-loop layers — `client/` and `orchestrator/` — realize the core as strict Functional Core / Imperative Shell.** Each is a pure `Reduce(state, event) → (state', []Effect)` over an **immutable `State` (no mutex)**, interpreted by a single event-loop shell that owns all I/O and live handles (workers, timers) in id→handle maps. Both enforce no-mutex on the functional core via `forbidigo` (`client/state`, `orchestrator/scheduler`). Time enters `Reduce` as a value, never read from the wall clock inside it. Observability reads an immutable published snapshot **lock-free** (`atomic.Pointer[State]`), so there is no lock to contend or time out.
  - **`client/`** adds: value-type Driver/Connector plugins (per-frame `DriverState` round-trips through `Driver.Step`); Driver/Connector/Subsystem **isolation** keeping tool-specific concepts out of `state/`, `runtime/`, `tui/`, `proto/`, `sandbox/`. The only synchronous I/O permitted inside `Reduce` is bounded read-only filesystem stat. Full detail: [client deep dive](docs/technical/client/README.md).
  - **`orchestrator/`** adds: **single-authority** (`ErrDuplicateDispatch` enforces SPEC §7.4); **agent-agnostic** dispatch (codex and `claude-app-server` emit one uniform event sequence); **reconcile = truth reconciliation** (agents transition issue state autonomously; reconcile re-reads the tracker and detects it). `scheduler.Reduce` returns `[]Effect`; the shell in `scheduler.go` interprets them and feeds I/O results back as events. Full detail: [orchestrator deep dive](docs/technical/orchestrator/README.md).
- **`platform/` is a library layer, not a decision loop, so FC/IS does not apply** — its testability comes from **dependency-injection seams** instead: external dependencies (`exec`, docker, network) sit behind injectable interfaces or env-var overrides (e.g. `lib/github.Runner`) so callers substitute fakes in tests. It is the base layer (imports neither `client/` nor `orchestrator/`); tool-specific knowledge (paths, env-var names, CLI invocations) is concentrated here so it stays out of the generic layers above; the agent-launch primitive (`agentlaunch`) is agent-agnostic; wire-format and persistence types are stdlib-only. Enforcement (import boundaries, name-literal leaks, no-mutex) is catalogued in [code & architecture enforcement](docs/technical/code-enforcement.md).

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

Testability is the first [core principle](#core-principles-all-layers), realized differently per layer: roost's Functional Core / Imperative Shell split makes the core testable without mocks, while the orchestrator injects fakes through `Deps`. Per-layer test patterns, the Coverage Tier scheme, and CI enforcement are documented in [docs/agent/testing.md](docs/agent/testing.md).

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `charm.land/bubbletea/v2` | v2.0.2 | TUI framework |
| `charm.land/lipgloss/v2` | v2.0.2 | Styling |
| `charm.land/bubbles/v2` | v2.1.0 | Key bindings |
| `github.com/BurntSushi/toml` | v1.6.0 | Configuration file |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File watching |
| `golang.org/x/term` | v0.41.0 | Terminal size detection |
