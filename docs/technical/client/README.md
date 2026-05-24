# client/ — roost (Session Lifecycle Manager)

`client/` is all of roost: the tmux TUI, the state machine, the runtime, drivers, and connectors. It depends on `platform/` but **must not** import `orchestrator/` (enforced by the `depguard` rule `client-no-orchestrator`).

roost is a *session lifecycle manager*, not an agent orchestrator. It gives you visibility and fast access to agents running across many projects; it does not decide what those agents do.

## Functional Core / Imperative Shell

The defining structural principle:

- **Functional Core (`client/state/`)** — all state transitions are a pure function `state.Reduce(state, event) → (state', []Effect)`. No goroutines, mutexes, or actors. Drivers run synchronously inside `Reduce`; the only permitted synchronous I/O is bounded read-only filesystem stat (e.g. checking whether a resume file exists). Everything else is emitted as an `Effect`.
- **Imperative Shell (`client/runtime/`)** — a single event loop owns state mutation and interprets `Effect` values into real I/O. Long-lived I/O readers only *emit* events; they never read or write state. The worker pool (discrete jobs) and stream readers (continuous sources) are both instances of this principle.

This split is why the core is testable without mocks: `Reduce` and `Driver.Step` are verified purely by their return values.

## Packages

| Package | Responsibility |
|---|---|
| `client/state/` | Pure domain layer — `State`, `Event`, `Effect`, `Reduce`. No I/O, no goroutines. Imports only stdlib + stdlib-only internal packages (`features`). |
| `client/state/view/` | Wire-safe view types — `Status`, `View`, `Card`, `Tag`, `ConnectorSection`. Stdlib-only; no `state` import. |
| `client/driver/` | Driver implementations — value-type plugins + per-frame `DriverState`. No I/O. |
| `client/connector/` | Connector implementations — value-type plugins + per-daemon `ConnectorState`. No I/O. |
| `client/runtime/` | Imperative shell — single event loop, Effect interpreter, backend abstraction. |
| `client/runtime/worker/` | Worker pool — slow I/O jobs (summarize, transcript parse, git, github fetch). |
| `client/runtime/subsystem/` | `Subsystem`/`Factory` interfaces + the `cli` and `stream` implementations. The only place in `runtime/` allowed to import `driver/<tool>`. |
| `client/proto/` | Typed IPC wire layer — Command / Response / ServerEvent sum types + codec. Imports `state/view` only. |
| `client/proto/sessions/` | Session-management helpers wrapping `proto.Client`. Imports `state`. |
| `client/tools/` | Palette tools — Tool abstraction + DefaultRegistry. |
| `client/tui/` | Presentation layer — Bubbletea UI, rendering, key input. Never branches on driver/connector name. |
| `client/config/` | TOML loading, DataDir injection, SandboxResolver. |
| `client/cli/` | Subcommand registry — tool-specific subcommands registered via `init()`. |
| `client/lib/peers/` | Peers MCP server (roost-specific IPC). |
| `client/lib/{claude,codex}/transcript/` | Transcript renderers (depend on `state` for TUI integration). |

## Terminology

| Term | Meaning |
|---|---|
| **Session** | A unit of work for an agent. `state.Session` owns a stack of execution **frames** (`[]SessionFrame`). The active frame is the stack tail; the root frame defines the session's existence — if it dies, the session is deleted. |
| **Frame** | One execution context within a session, carrying its own `Command`, `LaunchOptions`, `DriverState`, `SubsystemID`, `TargetID`. Frame death truncates the stack from that frame onward; push-driver appends a new frame on top. |
| **Subsystem** | Runtime-owned execution backend (`Start/BindFrame/ReleaseFrame/Stop`). `cli` manages single-process pane launch and worktree lifecycle; `stream` fronts long-lived structured backends (Codex App Server). The stream subsystem selects the host/container UDS path (`chooseSockPath`) but delegates exec wrapping (direct vs `docker exec`) to the `agentlaunch.Dispatcher` it holds. |
| **Connector** | A per-daemon external service integration plugin (GitHub/Linear/Jira). One instance per daemon, vs Drivers which are per-frame. |
| **Control Session** | The tmux session that houses all of roost. |
| **Warm start** | Runtime startup while the tmux session is alive — restores the frame stack and rebinds to live panes; surviving containers are adopted. |
| **Cold start** | Runtime startup when the tmux session is gone — respawns panes in root-to-tail order; surviving containers are discarded and provisioned fresh so `postCreate` daemons are guaranteed present. |

Hereafter "session" means a roost session; tmux sessions are called out explicitly.

## Code dependency direction

- `main` → `runtime`, `driver`, `connector`, `proto`, `tools`, `tmux`, `config`, `logger`
- `runtime` → `state` (calls `Reduce`), `proto` (wire codec), `runtime/worker`, `runtime/subsystem` (interface only — no concrete subsystem imports)
- `runtime/subsystem/<kind>` → `state`, `driver/<tool>` (constants/socket paths only), `lib/*`, `sandbox/`
- `runtime/worker` → `state` only (JobID, JobInput, EvJobResult); not driver/connector/lib
- `state` is self-contained — stdlib + stdlib-only internal packages (`features`) only
- `state/view` → stdlib only; `state` re-exports its types as aliases
- `driver` / `connector` → `state` (embed base types), `runtime/worker` (RegisterRunner), `lib/*`
- `proto` → `state/view` only (does **not** import `state`)
- `tui` → `proto/sessions`, `proto`, `state` (types), `tools`; not driver/connector/lib

Frames route events: `Reduce` routes session-level events by sessionID and frame-level events (hooks, subsystem events, lifecycle) by frameID to the owning frame's `Driver.Step`. `state.State.Connectors` routes by connector name to `Connector.Step`.

## Daemon ↔ TUI processes

The daemon and TUI are separate processes communicating via typed IPC (`proto`) over a Unix socket. The daemon exposes two physical endpoints: the **host endpoint** (`<dataDir>/roost.sock`, SO_PEERCRED auth) serves TUI/CLI/palette; the **container endpoint** (`<dataDir>/run/<project-hash>/roost.sock`, bearer-token auth) serves sandboxed agents and accepts only `hook-event`/`subsystem-event`. See [process model](process-model.md) and [IPC](ipc.md).

## Design decisions

| Decision | Choice | Rationale |
|---|---|---|
| Palette implementation | tmux popup (separate process) | Crash isolation — a submodel panic would take down the TUI. |
| Ctrl+C disabling | Consume KeyPressMsg | Prevents accidental termination of the resident process. |
| No optimistic updates | Do not modify UI state on IPC error | Auto-recovers on next poll; avoids state inconsistency. |
| Shutdown (`C-b q`) | `EffReleaseFrameSandboxes` then `EffKillSession`; `sessions.json` preserved | Containers get a clean stop before tmux dies; sessions restore on next cold start. `detach` releases no sandboxes so containers survive warm restart. See [detach vs shutdown](process-model.md#detach-vs-shutdown). |
| Claude cold-start launch | Assemble `claude --resume <id>` in `Driver.PrepareLaunch(LaunchModeColdStart, …)` | `--resume` knowledge stays in the driver; the runtime interprets the baked plan verbatim. |
| Launch plan resolution | In the reducer (pure), with one cold-start bootstrap exception | Driver-specific logic stays in the pure core; the bootstrap goroutine is the only safe direct caller. |
| Resident tracking | `SubsystemID -> Subsystem` (`subsystems` map), `FrameID -> Subsystem` (`frameSubsystems` map), `FrameID -> SubsystemID` (`frameSubsystemIDs` sync.Map), `FrameID -> TargetID` | A single `sync.Map` (`subsystems`) holds every live Subsystem keyed by its opaque SubsystemID, dispatched via per-kind Factories registered in `runtime.New`. `frameSubsystems` routes `ReleaseFrame` to the owning subsystem. `frameSubsystemIDs` is used by `reapSubsystemIfLast`: when the last frame of a Session is released, `Factory.Remove` is called to stop the app-server (stream subsystem reap). Shutdown ranges `subsystems` and calls `Stop` on each. CLI uses one Subsystem per project; the stream subsystem uses one Subsystem per roost Session (`stream:session:<id>`). |
| IPC timeout | Not set on the protocol itself | Runtime-side I/O (tmux/git/gh subprocesses via `exec.CommandContext`, `worker.Pool.Stop()` bounded to 500 ms) is fully ctx-scoped, so detach and exit never hang. A pure event-loop deadlock still requires external restart. |
| Frame ownership of DriverState | Each `SessionFrame` holds its own `DriverState`, updated in-place by `Driver.Step` inside `Reduce` | Session outlives any frame; push-driver layers a fresh context; frame death truncates only its slice. |
| Hook event target identification | Inject a frame-scoped env var at `tmux new-window -e` | Env vars are race-free at kernel exec level. See [state monitoring](state-monitoring.md#hook-event-routing-and-race-free-identification). |
| Hook payload abstraction | `CmdEvent.Payload` as opaque `json.RawMessage` | Driver-specific fields need no state/runtime/proto changes. |
| Agent hook integration | `roost event <eventType>` → `proto.CmdEvent`/`CmdHookEvent` → `EvDriverEvent` → `reduceDriverHook` → `Driver.Step(DEvHook)` | Used by hook-driven agents (Claude, Gemini). Host-side events carry `SenderID`; sandboxed events resolve the frame via bearer token. Hooks for truncated frames are dropped. |
| Structured stream integration | `codex app-server` → routing bridge → `proto.CmdSubsystemEvent` → `EvSubsystem` → `reduceSubsystem` → `Driver.Step(DEvSubsystem)` | Used by Codex. **Exactly one `codex app-server` runs per roost Session** (`stream:session:<id>`). All frames within the same Session share one app-server; different Sessions get separate processes. The app-server is launched via `agentlaunch.Dispatcher.Wrap` + `agentlaunch.Spawn` (argv-direct; no bespoke `docker exec` construction in the stream backend). Frames join via `BindFrame` → `bindThread`. A routing bridge (`roost-bridge sockbridge -route-dir`) maps TCP loopback 8282 to per-session UDS files (`codex-<sessionID>.sock`) — one bridge per container/host handles all sessions concurrently. The stream layer emits structured tool/approval/plan/diff/message/thread-lifecycle events; `TargetID` carries the logical thread identity. When a session's last frame is released, the app-server is reaped. |
| Connector scope | Per-daemon (one instance each), no state persistence (TTL-based), initialization on first `EvTick` | External service info is tied to the whole user account; embedding in Driver would duplicate fetching. Initializing inside the reducer keeps it pure-function testable. |
| Container egress restriction | Delegate to host (`docker network` + iptables) via `extra_create_args` | Hostname allowlists cannot be expressed by `docker create` flags alone. |
| Sandbox launcher abstraction | `runtime.AgentLauncher` wraps each `LaunchPlan`; `SandboxDispatcher` resolves direct vs devcontainer per project. The stream daemon holds a separate `runtime.Config.StreamDispatcher` backed by a non-TTY `DevcontainerLauncher` (`docker exec -i`) that shares the same `sandbox.Manager` as the TUI-pane launcher (`-it`) | Keeps sandbox rewriting out of the reducer; one daemon mixes sandboxed and direct projects. TUI vs daemon consumers require different TTY settings but must share the same container lifecycle. |
| Container↔host path translation | `lib/pathmap` rewrites IPC payload paths using `WrappedLaunch.Mounts` | `state/`, `runtime/` (above the launcher), and `tui/` stay unaware of container layout. |

## Deep dives

- [Process model](process-model.md) — daemon/TUI processes, pane layout, rendering responsibilities
- [IPC and tool system](ipc.md) — message format, command surface, concurrency model, Tool abstraction
- [State monitoring](state-monitoring.md) — driver plugins, the polling pipeline, hook routing, persistence
- [Interfaces](interfaces.md) — Go type definitions, data files, source tree
