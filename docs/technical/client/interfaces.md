# Interfaces, Data Files, and File Structure

## Interfaces

All state, runtime, and driver layers are defined as interfaces for testability. The state layer consists of pure value types and pure functions, while the runtime layer can be swapped with fakes during testing via backend interfaces.

```go
// state/state.go — All domain state (plain data, no methods)
type State struct {
    Sessions       map[SessionID]Session
    PendingCreates map[JobID]PendingCreate
    ActiveSession  SessionID
    Subscribers    map[ConnID]Subscriber
    Jobs           map[JobID]JobMeta
    NextJobID      JobID
    NextConnID     ConnID
    Now            time.Time
    Aliases        map[string]string
    DefaultCommand string
    Connectors     map[string]ConnectorState
}

// Session owns a stack of SessionFrames. The active frame is always
// Frames[len-1]; the root frame is Frames[0]. Frame death truncates
// the stack from that index onward.
type Session struct {
    ID        SessionID
    Project   string
    CreatedAt time.Time
    Frames    []SessionFrame
}

// SessionFrame is one execution context within a session. Each frame
// owns one tmux pane and carries its own DriverState, so push-driver
// can layer a fresh driver context on top and frame death can truncate
// just that slice of the stack.
type SessionFrame struct {
    ID            FrameID
    Project       string
    Command       string
    LaunchOptions LaunchOptions
    CreatedAt     time.Time
    Driver        DriverState   // sum type: concrete state per driver impl
}

// LaunchOptions is the driver-agnostic, normalized set of options that
// shape a frame's launch. Drivers receive the user's request via
// PrepareLaunch, normalize it, and return the canonical form, which
// round-trips through sessions.json and is re-applied on cold start.
type LaunchOptions struct {
    Worktree WorktreeOption
}

type WorktreeOption struct {
    Enabled bool
}
```

```go
// state/driver_iface.go — Driver interface (value-type plugin)
type Driver interface {
    Name() string
    DisplayName() string
    NewState(now time.Time) DriverState
    Step(prev DriverState, ev DriverEvent) (DriverState, []Effect, View)
    Status(s DriverState) Status
    View(s DriverState) View
    Persist(s DriverState) map[string]string
    Restore(bag map[string]string, now time.Time) DriverState

    // PrepareLaunch resolves the launch plan (command, start dir,
    // normalized options) for one frame. Invoked synchronously inside
    // state.Reduce for new frames and during cold-start restoration
    // for existing frames. Must be a pure function.
    PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string,
                  options LaunchOptions) (LaunchPlan, error)
}

// CreateSessionPlanner is an optional extension for drivers that need
// async setup work (e.g. creating a managed worktree) between the
// create-session request and the tmux spawn. PrepareCreate returns a
// CreatePlan with an optional SetupJob; once the job completes the
// reducer calls CompleteCreate to get the final CreateLaunch.
type CreateSessionPlanner interface {
    PrepareCreate(s DriverState, sessionID SessionID, project, command string,
                  options LaunchOptions) (DriverState, CreatePlan, error)
    CompleteCreate(s DriverState, command string, options LaunchOptions,
                   result any, err error) (DriverState, CreateLaunch, error)
}

// DriverState — closed sum type marker for per-frame state
type DriverState interface {
    driverStateMarker()
}

// DriverEvent — input to Driver.Step (closed sum type)
// DEvTick, DEvHook, DEvJobResult, DEvFileChanged, DEvPaneOsc, DEvPanePrompt, DEvSubsystem
```

Hook-driven agents (Claude, Gemini) receive `DEvHook`. Stream-backed agents (Codex) receive `DEvSubsystem` carrying structured thread / turn / tool / approval events from `codex app-server`.

Driver is a **value-type plugin**: no goroutines, no I/O, no mutexes. Per-frame state is embedded on each `SessionFrame.Driver` as a `DriverState` value, and round-trips as arguments and return values of `Driver.Step`. Side effects are returned as `[]Effect` and executed by the runtime's Effect interpreter.

**Launch plan is resolved in the reducer, not the runtime.** `reduceCreateSession` (or `handlePendingCreate` for planner-gated flows) calls `Driver.PrepareLaunch` synchronously, writes the normalized `LaunchOptions` onto the frame, and bakes `launch.Command` / `launch.StartDir` / `launch.Options` into `EffSpawnTmuxWindow`. The runtime interprets the effect verbatim and never calls driver methods, keeping driver-specific logic entirely inside the pure functional core.

```go
// state/view/status.go — canonical Status definition (stdlib-only; no state import)
type Status int
const (
    StatusRunning Status = iota
    StatusWaiting
    StatusIdle
    StatusStopped
    StatusPending
)

// state/status.go — re-exports via type aliases (transparent to existing callers)
type Status = v.Status
type StatusInfo = v.StatusInfo
// constants re-exported as StatusRunning = v.StatusRunning, etc.
```

```go
// state/reduce.go — Pure state transition function
func Reduce(s State, ev Event) (State, []Effect)
```

`Reduce` is the sole entry point for all state transitions. Event / Effect are closed sum types (`isEvent()` / `isEffect()` markers).

```go
// runtime/runtime.go — Imperative shell
type Runtime struct {
    cfg     Config
    state   state.State        // solely owned by the event loop goroutine
    eventCh chan state.Event    // Event submission from external goroutines
    workers *worker.Pool
    conns   map[state.ConnID]*ipcConn
    // ...
}

func (r *Runtime) Run(ctx context.Context) error  // event loop
func (r *Runtime) Enqueue(ev state.Event)          // goroutine-safe
```

```go
// runtime/backends.go — Backend interfaces swappable for testing
type TmuxBackend interface {
    SpawnWindow(name, cmd, startDir string, env map[string]string) (windowIndex, paneID string, err error)
    KillPaneWindow(paneTarget string) error
    ShowEnvironment() (string, error)
    RunChain(ops ...[]string) error
    SwapPane(srcPane, dstPane string) error
    PaneID(target string) (string, error)
    PaneSize(target string) (width, height int, err error)
    SelectPane(target string) error
    ResizeWindow(target string, width, height int) error
    SetStatusLine(line string) error
    PaneAlive(target string) (bool, error)
    // ...
}

type PersistBackend interface {
    Save(sessions []SessionSnapshot) error
    Load() ([]SessionSnapshot, error)
}
```

```go
// runtime/worker/pool.go — typed worker pool
// runtime/worker/registry.go — JobKind-based runner registry

func NewPool(parent context.Context, size int) *Pool
func RegisterRunner[In state.JobInput, Out any](
    kind string,
    runner func(context.Context, In) (Out, error),
)
func Dispatch(pool *Pool, jobID state.JobID, input state.JobInput)

type Pool struct { /* fixed-size goroutine pool, scoped to pool ctx */ }
func Submit[In state.JobInput, Out any](
    p *Pool, jobID state.JobID, input In,
    runner func(context.Context, In) (Out, error),
)
func (p *Pool) Results() <-chan state.Event   // EvJobResult
func (p *Pool) Stop()                         // bounded 500ms; cancels pool ctx
```

The `context.Context` handed to each runner is the pool's shutdown context. `Stop()` cancels it and waits up to 500 ms; runners must start any subprocess via `exec.CommandContext` (or otherwise honour the ctx) so cancellation propagates as SIGKILL. Jobs still queued when `Stop()` is called are discarded.

```go
// proto/envelope.go — typed IPC wire format
type Envelope struct {
    Type   string          `json:"type"`     // "cmd" | "resp" | "evt"
    ReqID  string          `json:"req_id,omitempty"`
    Cmd    string          `json:"cmd,omitempty"`
    Name   string          `json:"name,omitempty"`
    Status string          `json:"status,omitempty"`
    Data   json.RawMessage `json:"data,omitempty"`
    Error  *ErrorBody      `json:"error,omitempty"`
}

// Command — closed sum type.
// subscribe / unsubscribe / event: session control and domain operations.
// surface.read_text / surface.send_text / surface.send_key: pane surface control.
// driver.list: enumerate registered drivers.
// All session domain operations are dispatched via CmdEvent with Event field
// discriminator + RegisterEvent[T] typed handler lookup.
type Command interface { isCommand(); CommandName() string }

// CmdEvent is the unified envelope for session domain events and driver hooks.
// TUI/tool operations (create-session, etc.) and driver hooks both use this.
type CmdEvent struct {
    Event     string          `json:"event"`
    Timestamp time.Time       `json:"timestamp"`
    SenderID  string          `json:"sender_id"`
    Payload   json.RawMessage `json:"payload,omitempty"`
}
```

Hook-driven agents pass their payloads through typed IPC as `proto.CmdEvent{Event, Timestamp, SenderID, Payload}`. Each hook bridge subcommand (e.g., `roost event <eventType>`) reads the frame id from its pane environment, packs its hook payload into `CmdEvent` with `SenderID = frameID`, and sends it. The runtime's IPC reader converts it into an `EvDriverEvent` and feeds it into the event loop. `reduceDriverHook` locates the owning frame across all sessions and calls `Driver.Step(frame.Driver, DEvHook{...})`. Hooks whose target frame has already been truncated off the stack are silently dropped.

Structured backends such as Codex App Server use `proto.CmdSubsystemEvent{Source, Kind, Payload}` instead. The runtime converts this to `EvSubsystem`, updates the frame's `TargetID` when present, and calls `Driver.Step(frame.Driver, DEvSubsystem{...})`.

On cold start, the bootstrap walks each session's frames in root-to-tail order and calls `Driver.PrepareLaunch(frame.Driver, LaunchModeColdStart, project, command, frame.LaunchOptions)` to reconstruct the launch plan, including any driver-specific resume logic (e.g. the Claude driver assembles `claude --resume <id>` here using the session id it persisted in `DriverState`). The generic driver returns the base command as-is. The resolved launch plan drives `tmux new-window` directly — no separate driver method is involved.

## Data Files

| Path | Format | Contents | Lifecycle |
|------|--------|----------|-----------|
| `~/.roost/config.toml` | TOML | User settings (see below) | Created by user. Falls back to default values if absent |
| `~/.roost/sessions.json` | JSON | Session metadata and the frame stack. Each session holds a list of frames; each frame carries its own command, normalized `launch_options`, `subsystem_id`, `target_id`, and driver-interpreted `driver_state` bag. Active frame is not persisted — it is always the tail of the frame list | Written on `EffPersistSnapshot` (on Tick / hook / subsystem event / session lifecycle changes). Read only at daemon startup via `runtime.Bootstrap`. `driver_state` entries are opaque key/value pairs interpreted by the driver; runtime knows none of the key names |
| `~/.roost/events/{frameID}.log` | Text | Per-frame agent hook event log | Appended via `EffEventLogAppend`. The runtime's EventLogBackend manages file handles with lazy-open |
| `~/.roost/roost.log` | slog | Application log | Created/appended at daemon startup |
| `~/.roost/roost.sock` | Unix socket | Host IPC endpoint (SO_PEERCRED auth) — TUI / CLI / palette clients | Created at daemon startup. Deleted on exit |
| `~/.roost/run/<project-hash>/roost.sock` | Unix socket | Container IPC endpoint (bearer-token auth) — sandboxed agents only; implements `hook-event` and `subsystem-event` | Started on first container spawn for a project. Bind-mounted into the container as `/opt/roost/run/roost.sock` |
| `~/.roost/run/credproxy.sock` | Unix socket | Credential proxy endpoint (single instance per daemon; bearer token per project) | Listens whenever sandbox mode is `devcontainer`. Bind-mounted per project into the container as `/opt/roost/run/credproxy.sock` |
| `~/.roost/warm/<frameID>.json` | JSON | Per-frame container bearer token (atomic, `0o600`) | Written when a sandboxed frame is launched; replayed by `RecoverSandboxFrames` on warm restart. Wiped at cold start |

Base path can be changed via `Config.DataDir` (set to TempDir during tests).

User-facing `settings.toml` fields and defaults are documented in the [roost TUI user guide](../../user/roost-tui.md#configuration).

## File Structure

Source tree is split into three top-level trees under `src/` — see ARCHITECTURE.md "Layer Structure".
`src/platform/` holds shared infrastructure, `src/client/` holds roost-specific code, `src/cmd/` holds binary entry points.

```
src/
├── cmd/roost/           Daemon / TUI binary (main.go + coordinator.go + subcommand.go + tui_entry.go)
├── cmd/roost-bridge/    Bridge binary (thin container-side client)
├── client/cli/
│   └── subcommand.go    Subcommand registry (Register, Dispatch)
├── client/event/
│   └── send.go          Event sender (registers "event" subcommand in init)
├── client/state/        Pure domain layer (no I/O, no goroutine)
│   ├── state.go         State, Session, SessionFrame, Subscriber, JobMeta, LaunchOptions — plain value types
│   ├── event.go         Event closed sum type (EvEvent, EvDriverEvent, EvSubsystem, EvTick, EvJobResult, EvPaneDied, EvTmuxWindowVanished, ...)
│   ├── event_dispatch.go  RegisterEvent[T] registry + dispatch lookup
│   ├── effect.go        Effect closed sum type (EffSpawnTmuxWindow, EffKillSessionWindow, EffRegisterPane, EffUnregisterPane, EffActivateSession, EffDeactivateSession, EffStartJob, ...)
│   ├── reduce.go        Reduce(State, Event) → (State, []Effect) — pure state transition function
│   ├── reduce_event.go  EvEvent → registered handler dispatch, EvDriverEvent / EvSubsystem → Driver.Step routing
│   ├── reduce_session.go  session / frame lifecycle reducers (create-session, push-driver, stop-session, …)
│   ├── reduce_tick.go   EvTick → step active frame of each session → Driver.Step(DEvTick)
│   ├── reduce_osc.go    EvPaneOsc / EvPanePrompt → EffEventLogAppend + driver routing
│   │                    (OSC 0/2 → DEvPaneOsc; OSC 133 → DEvPanePrompt;
│   │                    OSC 9/99/777 → EffRecordNotification)
│   ├── reduce_surface.go  surface.read_text / send_text / send_key / driver.list reducers
│   ├── reduce_job.go    EvJobResult → Driver.Step(DEvJobResult)
│   ├── reduce_conn.go   IPC connection lifecycle
│   ├── reduce_lifecycle.go  shutdown / detach
│   ├── reduce_helpers.go  shared reducer helpers including frame-stack helpers (activeFrame, rootFrame, findFrame, truncateFrames)
│   ├── driver_iface.go  Driver interface (Step, Status, View, Persist, Restore, PrepareLaunch)
│   │                    DriverState / DriverEvent / DriverStateBase marker
│   │                    LaunchMode / LaunchOptions / LaunchPlan / CreateLaunch / CreatePlan
│   ├── status.go        Re-exports state/view.Status, StatusInfo as type aliases
│   ├── view.go          Re-exports state/view.View, Card, Tag, LogTab, InfoLine as type aliases
│   ├── connector.go     Re-exports state/view.ConnectorSection, ConnectorItem as type aliases
│   ├── clone.go         Copy-on-write helpers for State
│   └── view/            Wire-safe view types (stdlib-only; no state import; safe for proto and roost-bridge)
│       ├── status.go    Status enum + StatusInfo — canonical definition (Running/Waiting/Idle/Stopped/Pending)
│       ├── view.go      View / Card / Tag / LogTab / TabKind / InfoLine
│       └── connector.go ConnectorSection / ConnectorItem
├── client/driver/              Driver implementations — value-type plugins (no goroutines, no I/O)
│   ├── claude.go        claudeDriver — event-driven status + transcript job emit
│   ├── claude_event.go  DEvHook dispatch (state-change, session-start, ...)
│   ├── claude_tick.go   DEvTick: active gate + transcript parse job emit
│   ├── claude_view.go   View() — Card, LogTabs, InfoExtras, StatusLine
│   ├── claude_persist.go  Persist / Restore — opaque bag round-trip
│   ├── generic.go       genericDriver — Idle/Waiting transitions driven by tick + OSC events
│   ├── generic_view.go  View()
│   ├── shell.go         shellDriver — OSC 133 prompt-phase consumer (DEvPanePrompt)
│   ├── vt/              VT emulator wrapper — driver/vt.Terminal feeds bytes to charmbracelet/x/vt
│   │                    and exposes OnOscNotification / OnWindowTitle / OnPromptEvent callbacks
│   ├── jobs.go          Job input/output types (TranscriptParseInput, BranchDetectInput, ...)
│   ├── runners.go       built-in runners (TranscriptParse, HaikuSummary, GitBranch)
│   ├── tags.go          CommandTag helper
│   └── register.go      init() registers with state.Register
├── client/connector/           Connector plugin system (external service integration)
│   ├── github.go        GitHub connector — issues, PRs, workflow runs
│   ├── github_state.go  GitHub connector state types
│   ├── jobs.go          Connector job input/output types
│   ├── runners.go       Connector worker pool runners
│   └── register.go      init() registers connectors
├── client/runtime/             Imperative shell — event loop + Effect interpreter
│   ├── runtime.go       Runtime.Run() — single event loop (select)
│   ├── interpret.go     execute(Effect) — interpreter for all side effects
│   ├── ipc.go           Host IPC server (accept + SO_PEERCRED uid check, readLoop, writeLoop)
│   ├── ipc_container.go Container IPC endpoint (per-project Unix socket; `hook-event` / `subsystem-event`)
│   ├── frame_token.go   Per-frame bearer-token registry (`ROOST_SOCKET_TOKEN` → frame id)
│   ├── warm_state.go    Persists / replays container tokens across daemon warm-restart (`<dataDir>/warm/`)
│   ├── rundir.go        Per-project run directory (host-side dir bind-mounted as `/opt/roost/run`)
│   ├── launcher.go      AgentLauncher interface + DirectLauncher + WrappedLaunch + container-token wrap
│   ├── sandbox_dispatcher.go SandboxDispatcher: per-project mode resolution (direct / devcontainer)
│   ├── devcontainer_launcher.go DevcontainerLauncher: adapts sandbox/devcontainer.Manager to AgentLauncher
│   ├── credproxy_runner.go Lifecycle for the in-process credproxy server (Unix socket on `<dataDir>/run/credproxy.sock`; awssso registers an HTTP route, gcloud/sshagent/hostexec contribute env+mounts only)
│   ├── docker_env.go    Auto-detection of rootless docker socket → `DOCKER_HOST`
│   ├── backends.go      TmuxBackend, PersistBackend, EventLogBackend, FSWatcher interface
│   ├── panetap.go       PaneTap interface — raw byte stream abstraction over tmux pipe-pane
│   ├── tmux_pipe_tap.go TmuxPipePaneTap — pipe-pane + FIFO + reader goroutine
│   ├── tap_manager.go   per-frame tap lifecycle; runs a goroutine that feeds pipe-pane bytes
│   │                    into a driver/vt.Terminal and emits EvPaneOsc (OSC 0/2/9/99/777) and
│   │                    EvPanePrompt (OSC 133) into eventCh
│   ├── peercred_linux.go  SO_PEERCRED uid verification (Linux)
│   ├── peercred_other.go  no-op stub (non-Linux)
│   ├── tmux_real.go     TmuxBackend concrete implementation
│   ├── persist.go       PersistBackend concrete implementation (sessions.json)
│   ├── eventlog.go      EventLogBackend concrete implementation
│   ├── fsnotify.go      FSWatcher concrete implementation
│   ├── convert.go       state.View → proto.SessionInfo conversion
│   ├── proto_bridge.go  proto.Command → state.Event conversion
│   ├── bootstrap.go     Initial State construction for warm/cold restart; RecoverSandboxFrames
│   ├── cleanup.go       Per-frame cleanup callbacks (sandbox release, container token revoke)
│   ├── filerelay.go     File relay
│   ├── testing.go       Test helper (fake backend)
│   └── worker/          Worker pool
│       ├── pool.go      Pool + Submit[In,Out] (typed job submission)
│       ├── registry.go  RegisterRunner[In,Out] + Dispatch (JobKind-based runner registry)
│       └── runners.go   built-in runners (TranscriptParse, HaikuSummary, GitBranch)
├── platform/sandbox/             Project-level sandbox backends (generic Manager[I any])
│   ├── manager.go       Instance[I] / Manager[I] / StartOptions interface definitions
│   └── devcontainer/    Devcontainer backend (per-project container lifecycle via docker)
│       ├── manager.go   Manager (impl): EnsureInstance / BuildLaunchCommand / AdoptFrame / DestroyInstance
│       ├── lifecycle.go docker create / start / exec / rm and container reuse logic
│       ├── docker.go    Low-level docker CLI wrapper (singleflight-serialized per project)
│       ├── spec.go      LoadSpec — parses devcontainer.json (image / build.name / mounts / runArgs / containerEnv / containerUser / remoteUser / workspaceFolder / workspaceMount / postCreateCommand / preExecCommand)
│       ├── merge.go     Merges user-scope SandboxConfig with the per-project devcontainer spec
│       └── envscript.go Resolves `${localEnv:VAR}` / `${localWorkspaceFolder*}` / `${containerWorkspaceFolder}` placeholders
├── platform/hostexec/            Host-exec broker (`container.Provider` impl): per-project Unix socket server that runs allowlisted host binaries on behalf of container processes via SCM_RIGHTS stdio forwarding; deny/allow glob policy with env-assignment prefix stripping
│                        Credential providers (awssso / gcloudcli / sshagent) live in the external `credproxy` library under `providers/<name>/`
├── client/proto/               Typed IPC wire layer — imports state/view only (safe for roost-bridge)
│   ├── envelope.go      Envelope wire format ({type, req_id, cmd|name, data})
│   ├── command.go       Command closed sum type (CmdSubscribe, CmdUnsubscribe, CmdEvent, CmdHookEvent (container-only), CmdSurface*, CmdDriverList, CmdPeer*)
│   ├── response.go      Response closed sum type (RespOK, RespSurfaceText, RespDriverList, RespPeerList, RespPeerDrainInbox). Session-related types live in proto/sessions
│   ├── event.go         ServerEvent closed sum type
│   ├── codec.go         NDJSON encode/decode
│   ├── client.go        proto.Client (Dial / DialConn / Send / Events); used by TUI, bridge, palette
│   ├── client_helpers.go  Peer helpers (PeerSend/List/SetSummary/DrainInbox), SendEvent, SendHookEvent
│   ├── reqid.go         Request ID generation
│   └── errors.go        ErrCode enum
├── client/proto/sessions/      Session management layer — imports proto + state (NOT used by roost-bridge)
│   ├── client.go        sessions.Client wraps *proto.Client; session helpers (CreateSession, StopSession, ListSessions, PushDriver, ActivateFrame, ...)
│   ├── helpers.go       sendJSONEvent / sendJSONEventTimeout helpers; timeout constants
│   ├── client_test.go   External tests (TestCreateSessionUsesLongTimeout, TestPushDriverDecodesCreateSessionReply, ...)
│   ├── canonical_test.go  canonicalProjectPath unit tests
│   └── sync_test.go     Integration: proto.CmdNamePeer* == state.EventPeer* (divergence detection)
├── client/tools/
│   └── tools.go         Tool + Param + ToolContext + Registry + DefaultRegistry
├── platform/lib/
│   ├── claude/
│   │   ├── command.go   Claude subcommand handler (registers "claude" in init)
│   │   ├── cli/         Claude CLI launch command assembly (ResumeCommand etc.)
│   │   ├── setup.go     Hook registration/removal in Claude settings.json
│   │   └── transcript/  Claude JSONL transcript parsing + diff tracking
│   ├── git/
│   │   └── git.go       Git branch detection (DetectBranch)
│   ├── github/
│   │   └── github.go    GitHub API client
│   ├── pathmap/         Container↔host path translation for IPC payloads (uses WrappedLaunch.Mounts)
│   ├── vcs/
│   │   └── vcs.go       VCS abstraction
│   └── plastic/
│       └── plastic.go   Plastic SCM integration
├── client/config/
│   ├── config.go        TOML configuration loading (`ROOST_DATA_DIR` env override)
│   ├── merge.go         MergeSandbox — user-scope settings merged into per-project overrides
│   ├── sandbox_resolver.go SandboxResolver — resolves effective sandbox mode per project (mtime-cached)
│   ├── project.go       Project enumeration from `project_roots` / `project_paths`
│   ├── workspace_resolver.go Reads each project's `.roost/settings.toml` for workspace grouping
│   └── notify.go        Notification rule matching
├── tmux/
│   ├── interfaces.go    PaneOperator
│   ├── client.go        tmux command wrapper (concrete implementation)
│   └── pane.go          Pane operations
├── client/tui/
│   ├── model.go         Session list Model (UI state only. Directly imports state.Status)
│   ├── view.go          Session list rendering
│   ├── mouse.go         Mouse input handler
│   ├── keys.go          Keybinding definitions + keyboard input handler
│   ├── main_model.go    Main TUI Model (viewport scrolling)
│   ├── main_view.go     Main TUI rendering
│   ├── palette.go       Command palette
│   ├── log_model.go     Log TUI (dynamic session tabs)
│   ├── log_view.go      Log TUI rendering
│   ├── log_info.go      INFO tab rendering
│   ├── log_io.go        Log file I/O
│   ├── filter.go        Session list filtering
│   ├── layout.go        Layout calculation
│   ├── panes.go         Pane management
│   └── theme.go         Theme (state color mapping)
└── logger/
    └── logger.go        slog initialization
```
