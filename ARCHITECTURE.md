# Architecture

## Vision

When running AI agents across multiple projects, you lose track of which agents are working, which are waiting for input, and which need tool approval. Switching between them in raw tmux is slow and error-prone. roost solves this: launch sessions in seconds, see their status at a glance, and switch instantly.

roost is a session lifecycle manager — not an agent orchestrator. It does not control what agents do; it gives you visibility and fast access to all of them from a single tmux-based TUI.

## Design Principles

- **Functional Core / Imperative Shell**: All state transitions are expressed as a pure function `state.Reduce(state, event) → (state', effects)`. I/O is emitted as `Effect` values and interpreted by a single event loop (runtime). No goroutines, mutexes, or actors exist in the state layer
- **Driver as Value Type**: Drivers are stateless plugins. Per-frame state is embedded as a `DriverState` value on each `SessionFrame` and round-trips through `Driver.Step`. No goroutines. Drivers run synchronously inside `state.Reduce`; the only permitted synchronous I/O is **bounded read-only filesystem stat** (e.g. checking whether a resume file exists before building a launch command). Subprocess execution, network I/O, and writes must be returned as `[]Effect` and dispatched via the worker pool
- **Single event loop**: State mutation is exclusively owned by one goroutine. Long-lived I/O readers may only emit events — they never read or write state. The worker pool (discrete jobs) and stream readers (continuous sources) are both concrete instances of this general principle. No mutexes are needed outside these sources
- **Driver/Connector/Subsystem isolation**: Concepts specific to `driver/`, `connector/`, and tool-specific helpers in `lib/<tool>/` must not leak into `state/`, `runtime/`, `tui/`, `proto/`, or `sandbox/`. TUI never branches on driver or connector name. `sandbox/` backends are tool-agnostic and never import `driver/` or `lib/<tool>/`. Tool-specific host paths (e.g. `~/.claude*`) must not be hardcoded in any Go source — they live in user config (`~/.roost/settings.toml`). `main.go` only wires generic values from config into runtime/sandbox; it does not embed tool-specific defaults. Tool-specific environment variable names (`AWS_*`, `ANTHROPIC_*`, `GOOGLE_*`, `OPENAI_*`, etc.) must not appear as string literals in `state/`, `runtime/`, `sandbox/`, `tui/`, or `proto/` — they live exclusively in the external `credproxy` library's `providers/<name>/` packages, the local `hostexec/` package, or `lib/<tool>/`; generic layers compose these via provider-supplied helpers. Subsystem-specific backend implementations (e.g. the `stream` subsystem that fronts codex app-server) live in `runtime/subsystem/<kind>/` and are the only files permitted to import `driver/<tool>` — all other files in `runtime/` must not import `driver/`. The subsystem kind names (`cli`, `stream`) and `SubsystemID` bookkeeping exist in `runtime/` and `state/`; subsystem implementations themselves do not. **Enforcement**: import-boundary violations are caught by `depguard` (see `.golangci.yml`); driver command name strings, env variable name literals, and connector name literals in generic layers are caught by `runtime/isolation_test.go`
- **No fallbacks**: Do not synthesize "if source A is unavailable, use B". Until `Driver.Step` updates the state, the status does not change

## Documentation

- [Process Model, tmux Layout, Rendering Responsibilities](docs/process-model.md) — Daemon/TUI process structure, pane layout, rendering boundary between Driver and TUI
- [Inter-Process Communication and Tool System](docs/ipc.md) — IPC message format, command list, concurrency model (event loop + worker pool), Tool abstraction, proto type extension guidelines
- [State Monitoring](docs/state-monitoring.md) — State detection via Driver plugins, Claude/Generic driver, persistence/restoration
- [Interface and File Reference](docs/interfaces.md) — Go type definitions, data files, source tree
- [Sandbox Backends](docs/sandbox.md) — per-project devcontainer isolation, image resolution from devcontainer.json, devcontainer.json support, credential proxy

## Terminology

| Term | Meaning | tmux Entity |
|------|---------|-------------|
| **Session** | A unit of work for an AI agent. `state.Session` owns a stack of execution **frames** (`[]SessionFrame`). The active frame is always the stack tail; the root frame is the stack base and defines the session's existence — if the root frame dies, the session is deleted | None directly (frames hold the tmux panes) |
| **Frame** | One execution context within a session. Each frame carries its own `Command`, `LaunchOptions`, `DriverState`, `SubsystemID`, and `TargetID`. For CLI agents the target is one process/session; for Codex the target is one App Server thread. Frame death truncates the stack from that frame onward; push-driver appends a new frame on top of the active frame | Logical execution target. The visible pane is owned by the subsystem, not by the frame |
| **Subsystem** | Runtime-owned execution backend. Subsystems implement `Start/BindFrame/ReleaseFrame/Stop`: `cli` manages single-process pane launch and worktree lifecycle for CLI agents; `stream` fronts long-lived structured backends such as Codex App Server. `BindFrame` is called synchronously before tmux spawn and resolves worktree paths into `Plan.StartDir`. `ReleaseFrame` destroys any managed worktree when a frame is released. Subsystems own goroutines and I/O; Drivers are pure per-frame plugins with no I/O. **Subsystems are the only layer that knows whether a frame runs on the host or inside a container** — `SubsystemID` is opaque to Runtime/Driver/Frame, generated by the subsystem `Factory` (registered once at Runtime construction) and written onto the frame via `EvTmuxPaneSpawned`. Each `SessionFrame` carries both a `DriverState` (pure) and a `SubsystemID` (opaque key into `Runtime.subsystems`) | Usually one tmux **pane** per subsystem |
| **Control Session** | The tmux session that houses all of roost | tmux **session** (`roost`) |
| **Pane** | Control panes within Window 0 | tmux **pane** (`0.0`, `0.1`, `0.2`) |
| **Connector** | A per-daemon external service integration plugin. Fetches data from external services like GitHub/Linear/Jira and displays it in the TUI. While Drivers are per-frame, Connectors have one instance per daemon | None (holds no tmux resources) |
| **Warm start** | Runtime startup while a tmux session is alive. Restores the frame stack from `sessions.json`, rebinds each frame to its live tmux pane via tmux session-level env vars, and truncates any session at the first frame whose pane has vanished (dropping the whole session if the root frame is missing). Sandbox containers are adopted as-is | Reuses existing tmux session/pane and reuses surviving containers |
| **Cold start** | Runtime startup when the tmux session is gone (PC reboot / tmux server death / explicit shutdown). Restores the frame stack from `sessions.json` and respawns each frame's pane in root-to-tail order via `Driver.PrepareLaunch(LaunchModeColdStart, …)`, using the persisted `LaunchOptions`. **Any container that survived from a previous run is discarded; sandboxes are provisioned fresh** so `postCreate`-launched daemons (sockbridge, codex app-server) are guaranteed to be present | Creates new tmux session/window and fresh containers |

Hereafter, "session" refers to a roost session. tmux sessions are explicitly noted as "tmux session."

Runtime startup is always either a Warm start or a Cold start; there is no separate first-launch branch (if sessions.json does not exist, it simply Cold starts with an empty session list).

## Layer Structure

```
state/         Pure domain layer — State, Event, Effect, Reduce (no I/O, no goroutine)
state/view/    Wire-safe view types — Status, View, Card, Tag, ConnectorSection, etc. (stdlib-only; no state import)
driver/        Driver implementations — value-type Driver plugins + per-frame DriverState. No I/O
connector/     Connector implementations — value-type Connector plugins + per-daemon ConnectorState. No I/O
runtime/       Imperative shell — single event loop, Effect interpreter, backend abstraction
runtime/worker/ Worker pool — slow I/O job execution (haiku, transcript parse, git, github fetch)
runtime/subsystem/ `Subsystem` and `Factory` interfaces, shared worktree utilities (`CreateWorktree`, `RemoveWorktree`, `CleanupUntracked`, `GenerateWorktreeNames`, `IsManagedWorktreePath`). Subsystem implementations — `cli` (per-frame process and worktree lifecycle) and `stream` (WebSocket-over-UDS backend for structured app-servers). Each implementation provides a `Factory` keyed by its own opaque `SubsystemID` scheme (`cli:<project>` for CLI; `stream:host:<projectPath>` for host-mode stream; `stream:container:<containerKey>` for container-mode stream, where `containerKey` is `__shared__` in shared isolation or the project path in project isolation — this collapses every frame inside one container onto one backend); the runtime dispatches uniformly through the `subsystemFactories` map registered in `runtime.New`. Owns goroutines and I/O. The only location in `runtime/` permitted to import `driver/<tool>` (for tool-specific constants and socket paths)
proto/         Typed IPC wire layer — Command / Response / ServerEvent sum types + codec. No state import (imports state/view only)
proto/sessions/ Session management helpers — sessions.Client wraps proto.Client with session-management methods. Imports state
tools/         Palette tools — Tool abstraction for TUI + DefaultRegistry
tui/           Presentation layer — Bubbletea UI state management, rendering, key input
tmux/          Infrastructure layer — tmux command execution wrapper
features/      Feature flags — Flag/Set types (runtime), build-tag const (compile-time). No external deps
lib/           Utilities — external tool integration (lib/git/, lib/claude/, lib/github/)
sandbox/       Project-level sandbox backends (generic Manager[I]). devcontainer/ implements per-project container lifecycle via docker — see docs/sandbox.md
hostexec/      Host-exec broker (`container.Provider` for running allowlisted host binaries on behalf of container processes via SCM_RIGHTS stdio forwarding)
mcpproxy/      MCP proxy broker (`container.Provider` for running MCP servers on the host with JSON-RPC stdio relayed into the container; tool-level policy enforcement; generates a `.mcp.json` overlay so Claude Code routes the configured aliases through the broker automatically)
               Credential providers (AWS SSO, gcloud CLI, ssh-agent) live in the external `credproxy` library
config/        Configuration — TOML loading, DataDir injection, SandboxResolver (user + per-project mode resolution)
logger/        Logging — slog initialization, log file management
```

Files matching `state/reduce_*.go` host state-machine dispatch tables. They are exempt from the 80-line function limit (see AGENTS.md) because forced extraction of dispatch arms produces single-use helpers that fragment the state machine without adding clarity. File-length (500 lines) and naming rules still apply.

The daemon process and TUI process are separate processes that communicate via typed IPC (`proto` package) over a Unix socket. The daemon exposes two physical endpoints: the **host endpoint** (`<dataDir>/roost.sock`, SO_PEERCRED auth) serves TUI, CLI, and palette clients; the **container endpoint** (`<dataDir>/run/<project-hash>/roost.sock`, bearer-token auth) serves sandboxed agent processes and currently accepts `hook-event` and `subsystem-event`. See [IPC](docs/ipc.md) and [Sandbox Backends](docs/sandbox.md).

Code dependency direction:
- `main` → `runtime`, `driver`, `connector`, `proto`, `tools`, `tmux`, `config`, `logger`
- `runtime` → `state` (calls Reduce), `proto` (wire encode/decode), `runtime/worker` (Pool + Dispatch), `runtime/subsystem` (interface only — no concrete subsystem imports)
- `runtime/subsystem/<kind>` → `state`, `driver/<tool>` (constants/socket paths only), `lib/*`, `sandbox/`
- `runtime/worker` → `state` (JobID, JobInput, EvJobResult). Does not import driver/connector/lib
- `state` is self-contained — imports no third-party packages; only stdlib and stdlib-only internal packages (`features`, `uiproc`) are permitted (pure functional core)
- `state/view` imports only stdlib — wire-safe types that can be used without pulling in the full state layer
- `state` re-exports `state/view` types as type aliases (transparent to all existing callers)
- `driver` → `state` (DriverStateBase embed, Effect/View types), `runtime/worker` (RegisterRunner), `lib/*` (implementation)
- `connector` → `state` (ConnectorStateBase embed, Effect types), `runtime/worker` (RegisterRunner), `lib/*` (implementation)
- `proto` → `state/view` only (carries Status enum, View/ConnectorSection types on wire). Does **not** import `state`
- `proto/sessions` → `proto` + `state` (session-management helpers; not used by roost-bridge). The `make verify-bridge-deps` CI target enforces that roost-bridge's dependency graph contains no `state`, `uiproc`, or `features` packages
- `tools` → `proto/sessions` (sessions.Client calls)
- `tui` → `proto/sessions` (sessions.Client + SessionInfo + ConnectorInfo), `proto` (wire types), `state` (Status/View/ConnectorSection/TabRenderer types), `tools` (ToolRegistry). Does not import driver/connector/lib
- `lib/claude/command.go` (hook bridge) → `event` (sends CmdEvent via event.Send), `config`
- `lib/claude/transcript` → `state` (registers TabRenderer factory via RegisterTabRenderer)
- `cli/` provides a subcommand registry. Tool-specific subcommands are registered in `cli/<tool>.go` via `init()`, keeping driver names out of `main`; `main` dispatches via `cli.Dispatch`
- `event/send.go` (event subcommand) → `proto` (sends CmdEvent), `cli` (registers "event" subcommand)
- `state.Session` owns a stack of `SessionFrame` values, each carrying its own DriverState plus `SubsystemID` / `TargetID`. Reduce routes session-level events by sessionID and frame-level events (hooks, subsystem events, frame lifecycle) by frameID, and passes them to the owning frame's `Driver.Step`
- `state.State.Connectors` holds per-daemon ConnectorState. Reduce routes by connector name and passes to Connector.Step
- `runtime.AgentLauncher` wraps every `LaunchPlan` before tmux spawn. `SandboxDispatcher` selects `DirectLauncher` or `DevcontainerLauncher` per project via `config.SandboxResolver`; the devcontainer launcher adapts `sandbox/devcontainer.Manager` and returns a `WrappedLaunch{Command, Env, Mounts, ContainerSockDir, Cleanup}`
- `lib/pathmap` translates container↔host paths at the IPC boundary using the bind-mount table captured in `WrappedLaunch.Mounts`, so hook events from sandboxed agents land at the same paths as host frames. `state/`, `tui/`, and `proto/` see only host paths

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Palette implementation approach | tmux popup (separate process) | Crash isolation. As a Bubbletea submodel, panics would be shared within the TUI |
| Ctrl+C disabling | Consume KeyPressMsg | Prevents accidental termination of the resident process. Pane becomes inoperable until respawn |
| No optimistic updates | Do not modify UI state on IPC error | Auto-recovers on next poll. Avoids risk of state inconsistency |
| shutdown (`C-b q`) behavior | `EffReleaseFrameSandboxes` (drain all container cleanups) then `EffKillSession`; sessions.json is preserved | Containers must be destroyed before the tmux session is killed so they receive a clean stop signal. sessions.json is preserved to restore sessions on next cold start. `detach` emits only `EffDetachClient` — no sandbox release — so containers survive for warm-restart adoption. See [Detach vs Shutdown](docs/process-model.md#detach-vs-shutdown) and [Sandbox Backends](docs/sandbox.md). |
| Claude startup on Cold start | Assemble `claude --resume <id>` inside `Driver.PrepareLaunch(LaunchModeColdStart, …)` using the persisted `LaunchOptions` | Claude-specific `--resume` knowledge is confined to the driver. The resolved launch plan is baked into `EffSpawnTmuxWindow` so the runtime never calls drivers |
| Launch plan resolution layer | Reducer (pure), with one bootstrap exception | `Driver.PrepareLaunch` runs synchronously inside `state.Reduce`, and the resolved command / start_dir / normalized options are written to `EffSpawnTmuxWindow`. The runtime interprets the effect verbatim without touching drivers, keeping driver-specific logic entirely in the pure functional core. **Exception**: during cold-start `RecreateAll` (`runtime/bootstrap_coldstart.go`), `PrepareLaunch` is called directly from the bootstrap goroutine before the event loop starts — no other goroutine reads state at that point so there is no consistency risk. All subsequent plan resolutions (new session, push-driver) go through the reducer as normal. |
| Resident tracking | `SubsystemID -> Subsystem` (`subsystems` map), `FrameID -> Subsystem` (`frameSubsystems` map), `FrameID -> TargetID` | A single sync.Map (`subsystems`) holds every live Subsystem keyed by its opaque SubsystemID, dispatched via per-kind Factories registered in `runtime.New`. The `frameSubsystems` map routes `ReleaseFrame` calls from `executeKillSessionWindow` to the owning subsystem without needing to know the subsystem kind. Shutdown (`EffReleaseFrameSandboxes`) ranges `subsystems` and calls `Stop` on each — kind-independent. CLI uses one Subsystem per project; the stream subsystem maps many frames (threads) onto one Subsystem per (sandbox-mode × project) |
| IPC timeout | Not set on the protocol itself | Runtime-side I/O (tmux/git/gh subprocesses via `exec.CommandContext`, `worker.Pool.Stop()` bounded to 500 ms) is fully ctx-scoped, so detach and exit never hang. A pure event-loop deadlock still requires external restart |
| Frame ownership of DriverState | Each `SessionFrame` holds its own `DriverState` value, updated in-place by `Driver.Step` within Reduce | Session lifetime outlives any single frame; letting frames own their DriverState lets `push-driver` layer a fresh driver context on top and lets frame death truncate only that slice of the stack. Updates happen inside Reduce so inconsistency between frame metadata and DriverState is structurally impossible |
| Identifying the target of hook events | Inject a frame-scoped env var into the pane environment at `tmux new-window -e` time | Env vars are set at kernel exec level and are race-free, so hook bridge processes spawned inside a frame's pane can identify their owning frame without racing against tmux option writes. Details in [state-monitoring.md](docs/state-monitoring.md#hook-event-routing-and-race-free-identification) |
| Hook payload abstraction | Carry `CmdEvent.Payload` as an opaque `json.RawMessage` | Adding driver-specific fields requires no changes to state / runtime / proto |
| Agent hook integration | `roost event <eventType>` → `proto.CmdEvent` or `proto.CmdHookEvent` → `EvDriverEvent` → `reduceDriverHook` → `Driver.Step(DEvHook)` | Used by hook-driven agents such as Claude and Gemini. Host-side events carry `SenderID`; sandboxed events resolve the frame via bearer token. Hooks for truncated frames are dropped |
| Structured stream integration | `codex app-server` → bridge/gateway → `proto.CmdSubsystemEvent` → `EvSubsystem` → `reduceSubsystem` → `Driver.Step(DEvSubsystem)` | Used by Codex. **Exactly one `codex app-server --listen unix://<runDir>/codex.sock` runs per container** — in shared isolation all projects collapse onto a single app-server (SubsystemID `stream:container:__shared__`); in project isolation each container has its own (SubsystemID `stream:container:<projectPath>`). Host-mode launches are keyed per project (`stream:host:<projectPath>`) because each runs its own app-server in its own cwd. Frames join the app-server via `BindFrame` → `bindThread`, and `sockbridge` bridges container TCP loopback port 8282 to the codex UDS so each frame can connect with `codex --remote ws://127.0.0.1:8282`. `LaunchSubsystem` (`cli` / `stream`) distinguishes CLI-spawned agents from stream-backed ones; the `stream` subsystem implementation handles the codex-specific binding. The stream layer emits structured tool / approval / plan / diff / message / thread lifecycle events, and `TargetID` carries the logical thread identity |
| Connector scope | Per-daemon (one instance each), no state persistence (TTL-based), initialization on first EvTick | External service information is tied to the entire user account. Embedding in Driver would cause duplicate fetching. Initializing within the reducer enables pure function test coverage |
| Container egress restriction | Delegate to host (`docker network` + iptables) and pass through via `extra_create_args` | Hostname allowlists cannot be expressed by `docker create` flags alone; a typed network config in `config/` would duplicate the existing passthrough with no expressive gain. iptables rule lifecycle belongs to host operations, not the daemon |
| Sandbox launcher abstraction | `runtime.AgentLauncher` wraps each `LaunchPlan` before tmux spawn; `SandboxDispatcher` resolves `direct` vs `devcontainer` per project | Sandbox-specific command rewriting, bind-mount setup, and bearer-token generation stay out of the reducer and out of `tmux_real.go`. Per-project mode resolution lets one daemon mix sandboxed and direct projects without restart |
| Container↔host path translation | `lib/pathmap` rewrites paths in IPC payloads using `WrappedLaunch.Mounts` | A sandboxed agent's hook events report container-absolute paths; daemon and TUI operate on host-absolute paths. Translating at the IPC boundary keeps `state/`, `runtime/` (above the launcher), and `tui/` unaware of container filesystem layout |

## Feature Flags

Experimental features are gated by one of **two independent mechanisms**. They share no key space — pick one based on whether the experimental code should physically exist in the binary.

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

Test files are placed in the same directory as the target file as `*_test.go`.

- **state.Reduce tests**: No mocks needed. Pure function tests that directly verify the return value `(state', effects)` of `Reduce(state, event)`. No goroutine / channel / timing dependencies
- **Driver.Step tests**: No mocks needed. Directly verify the return value `(next, effects, view)` of `Step(prev, driverEvent)`
- **runtime tests**: Inject fakes for backend interfaces. Set `noopTmux` / `noopPersist` etc. in `runtime.Config` for testing. Inject `t.TempDir()` into `Config.DataDir` to isolate file I/O
- **TUI tests**: Pass messages directly to Bubbletea's `Model.Update` and verify the returned Model state. No actual terminal required

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `charm.land/bubbletea/v2` | v2.0.2 | TUI framework |
| `charm.land/lipgloss/v2` | v2.0.2 | Styling |
| `charm.land/bubbles/v2` | v2.1.0 | Key bindings |
| `github.com/BurntSushi/toml` | v1.6.0 | Configuration file |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File watching |
| `golang.org/x/term` | v0.41.0 | Terminal size detection |
