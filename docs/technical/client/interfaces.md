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
// owns one pane and carries its own DriverState, so push-driver
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
// create-session request and the pane spawn. PrepareCreate returns a
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

**Launch plan is resolved in the reducer, not the runtime.** `reduceCreateSession` (or `handlePendingCreate` for planner-gated flows) calls `Driver.PrepareLaunch` synchronously, writes the normalized `LaunchOptions` onto the frame, and bakes `launch.Command` / `launch.StartDir` / `launch.Options` into `EffSpawnPaneWindow`. The runtime interprets the effect verbatim and never calls driver methods, keeping driver-specific logic entirely inside the pure functional core. (`LaunchPlan` and `WrappedLaunch` also carry `Argv []string` for `agentlaunch.Spawn`; the pane spawn path uses `Command`.)

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
//
// Pane operations are split into narrow role interfaces so callers can
// depend on the minimum surface they need. The production backend
// (PtyBackend, built on platform/termvt) implements all of them; test
// fakes can stub a subset.

type PaneLifecycle interface {
    SpawnWindow(name, command, startDir string,
                env map[string]string) (windowIndex, paneID string, err error)
    KillPaneWindow(paneTarget string) error
    RespawnPane(target, command string) error
    PaneAlive(target string) (bool, error)
    PaneExitStatus(target string) (dead bool, code int, err error)
}

type PaneIO interface {
    SendKeys(paneTarget, text string) error          // text + Enter
    SendKey(paneTarget, key string) error            // named key, no Enter
    SendEnter(target string) error
    LoadBuffer(name, text string) error              // bracketed-paste path
    PasteBuffer(name, target string) error
    PipePane(paneTarget, command string) error       // no-op on PtyBackend
}

type PaneInspect interface {
    PaneID(target string) (string, error)
    PaneSize(target string) (width, height int, err error)
    CapturePane(paneTarget string, nLines int) (string, error)
}

// PaneBackend bundles the full set of pane / window operations the
// runtime needs (lifecycle + io + inspect + session env + window
// layout + control). Production wiring uses PtyBackend; new code should
// depend on the narrower role interfaces above where possible.
type PaneBackend interface {
    PaneLifecycle
    PaneIO
    PaneInspect
    SessionEnv
    WindowLayout
    BackendControl
}

// PersistBackend abstracts sessions.json persistence so tests don't
// touch the filesystem. Save is upsert-only; removal is explicit via
// Delete(id), so a transiently empty in-memory state cannot wipe the
// on-disk record.
type PersistBackend interface {
    Save(sessions []SessionSnapshot) error
    Delete(id string) error
    Load() ([]SessionSnapshot, error)
}

// EventLogBackend writes per-frame agent event log lines; the
// implementation lazy-opens the file on first append and keeps it open
// until Close(frameID).
type EventLogBackend interface {
    Append(frameID state.FrameID, line string) error
    Close(frameID state.FrameID)
    CloseAll()
}

// FSWatcher is the fsnotify wrapper. It watches per-frame files and
// emits FSEvent values on Events() when they change.
type FSWatcher interface {
    Watch(frameID state.FrameID, path string) error
    Unwatch(frameID state.FrameID) error
    Events() <-chan FSEvent
    Close() error
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
// Operator-facing tools (create-session, etc.) and driver hooks both use this.
type CmdEvent struct {
    Event     string          `json:"event"`
    Timestamp time.Time       `json:"timestamp"`
    SenderID  string          `json:"sender_id"`
    Payload   json.RawMessage `json:"payload,omitempty"`
}
```

Hook-driven agents pass their payloads through typed IPC as `proto.CmdEvent{Event, Timestamp, SenderID, Payload}`. Each hook bridge subcommand (e.g., `server event <eventType>`) reads the frame id from its pane environment, packs its hook payload into `CmdEvent` with `SenderID = frameID`, and sends it. The runtime's IPC reader converts it into an `EvDriverEvent` and feeds it into the event loop. `reduceDriverHook` locates the owning frame across all sessions and calls `Driver.Step(frame.Driver, DEvHook{...})`. Hooks whose target frame has already been truncated off the stack are silently dropped.

Structured backends such as Codex App Server use `proto.CmdSubsystemEvent{Source, Kind, Payload}` instead. The runtime converts this to `EvSubsystem`, updates the frame's `TargetID` when present, and calls `Driver.Step(frame.Driver, DEvSubsystem{...})`.

On cold start, the bootstrap walks each session's frames in root-to-tail order and calls `Driver.PrepareLaunch(frame.Driver, LaunchModeColdStart, project, command, frame.LaunchOptions)` to reconstruct the launch plan, including any driver-specific resume logic (e.g. the Claude driver assembles `claude --resume <id>` here using the session id it persisted in `DriverState`). The generic driver returns the base command as-is. The resolved launch plan drives the backend's `SpawnWindow` directly — no separate driver method is involved.

## Data Files

| Path | Format | Contents | Lifecycle |
|------|--------|----------|-----------|
| `~/.agent-reactor/config.toml` | TOML | User settings (see below) | Created by user. Falls back to default values if absent |
| `~/.agent-reactor/sessions.json` | JSON | Session metadata and the frame stack. Each session holds a list of frames; each frame carries its own command, normalized `launch_options`, `subsystem_id`, `target_id`, and driver-interpreted `driver_state` bag. Active frame is not persisted — it is always the tail of the frame list | Written on `EffPersistSnapshot` (on Tick / hook / subsystem event / session lifecycle changes). Read only at daemon startup via `runtime.Bootstrap`. `driver_state` entries are opaque key/value pairs interpreted by the driver; runtime knows none of the key names |
| `~/.agent-reactor/events/{frameID}.log` | Text | Per-frame agent hook event log | Appended via `EffEventLogAppend`. The runtime's EventLogBackend manages file handles with lazy-open |
| `~/.agent-reactor/server.log` | slog | Application log | Created/appended at daemon startup |
| `~/.agent-reactor/server.sock` | Unix socket | Host IPC endpoint (SO_PEERCRED auth) — co-resident HTTP/WS gateway, `server event/host-exec/mcp-exec` subcommands | Created at daemon startup. Deleted on exit |
| `~/.agent-reactor/run/<project-hash>/server.sock` | Unix socket | Container IPC endpoint (bearer-token auth) — sandboxed agents only; implements `hook-event` and `subsystem-event` | Started on first container spawn for a project. Bind-mounted into the container as `/opt/agent-reactor/run/server.sock` |
| `~/.agent-reactor/run/credproxy.sock` | Unix socket | Credential proxy endpoint (single instance per daemon; bearer token per project) | Listens whenever sandbox mode is `devcontainer`. Bind-mounted per project into the container as `/opt/agent-reactor/run/credproxy.sock` |
| `~/.agent-reactor/warm/<frameID>.json` | JSON | Per-frame container bearer token (atomic, `0o600`) | Written when a sandboxed frame is launched; replayed by `RecoverSandboxFrames` on warm restart. Wiped at cold start |

Base path can be changed via `Config.DataDir` (set to TempDir during tests).

User-facing `settings.toml` fields and defaults live in the source code (`client/config/config.go`).

## File Structure

Source tree is split into three top-level trees under `src/` — see ARCHITECTURE.md "Layer Structure".
`src/platform/` holds shared infrastructure, `src/client/` holds client-specific code, `src/cmd/` holds binary entry points.

```
src/
├── cmd/server/          Merged backend binary — pty session daemon + HTTP/WS
│   │                    gateway in one process. Exposes the daemon over the
│   │                    Unix socket and the gateway over -addr.
│   ├── main.go             Process entry point (flag classification, logger
│   │                       bootstrap, panic recovery, subcommand dispatch)
│   ├── coordinator.go      Boots Runtime + IPC server + co-resident gateway
│   │                       under a shared ctx
│   ├── daemon_flags.go     Daemon flag set (-addr / -data-dir / -token / -tls-*
│   │                       / -insecure / -no-auth)
│   ├── daemon_lock.go      Per-data-dir flock so a second backend cannot bind
│   │                       the same socket
│   ├── gateway.go          DaemonClient supervisor + http.Server wiring for the
│   │                       co-resident HTTP/WS gateway
│   ├── subcommand.go       One-shot subcommand classification (event /
│   │                       host-exec / mcp-exec / help)
│   ├── hostexec.go         `server host-exec` shim (in-container glue)
│   └── mcpexec.go          `server mcp-exec` shim (in-container glue)
├── cmd/web/             Web frontend host binary — serves embedded client/web/dist
│                        and reverse-proxies /api + /ws to the server gateway
├── cmd/reactor-bridge/  Bridge binary (thin container-side client; uses client/proto)
├── cmd/claude-app-server/  Codex app-server stdio shim for Claude
├── client/event/
│   └── event.go         Event sender (writes proto.CmdEvent over the host socket)
├── client/state/        Pure domain layer (no I/O, no goroutine)
│   ├── state.go         State, Session, SessionFrame, Subscriber, JobMeta,
│   │                    LaunchOptions — plain value types
│   ├── event.go         Event closed sum type (EvEvent, EvDriverEvent, EvSubsystem,
│   │                    EvTick, EvJobResult, EvPaneDied, EvPaneSpawned,
│   │                    EvSpawnFailed, EvPaneWindowVanished,
│   │                    EvFrameCommandExited, EvPaneOsc, EvPanePrompt, ...)
│   ├── event_dispatch.go  RegisterEvent[T] registry + dispatch lookup
│   ├── effect.go        Effect closed sum type (EffSpawnPaneWindow,
│   │                    EffKillSessionWindow, EffRegisterPane, EffUnregisterPane,
│   │                    EffSetPaneEnv, EffUnsetPaneEnv, EffCheckPaneAlive,
│   │                    EffPersistSnapshot, EffEventLogAppend, EffToolLogAppend,
│   │                    EffReconcileWindows, EffStartJob, EffRecordNotification,
│   │                    EffSendPaneKeys, EffInjectPrompt,
│   │                    EffSurfaceSubscribeStart/Stop/Resize/WriteRaw,
│   │                    EffBroadcastSurfaceOutput, ...)
│   ├── reduce.go        Reduce(State, Event) → (State, []Effect) — pure transition
│   ├── reduce_event.go  EvEvent → registered handler dispatch;
│   │                    EvDriverEvent / EvSubsystem → Driver.Step routing
│   ├── reduce_session.go  session / frame lifecycle reducers (create-session,
│   │                      push-driver, stop-session, …)
│   ├── reduce_session_nav.go  preview / switch / activate-frame reducers
│   ├── reduce_frame.go        frame-stack lifecycle helpers and reducers
│   ├── reduce_frame_evict.go  spawn-failure and frame-death eviction paths
│   ├── reduce_tick.go         EvTick → step active frame of each session →
│   │                          Driver.Step(DEvTick)
│   ├── reduce_osc.go    EvPaneOsc / EvPanePrompt → EffEventLogAppend + driver routing
│   │                    (OSC 0/2 → DEvPaneOsc; OSC 133 → DEvPanePrompt;
│   │                     OSC 9/99/777 → EffRecordNotification)
│   ├── reduce_surface.go  surface.read_text / send_text / send_key / driver.list /
│   │                     surface streaming reducers
│   ├── reduce_job.go    EvJobResult → Driver.Step(DEvJobResult)
│   ├── reduce_conn.go   IPC connection lifecycle
│   ├── reduce_lifecycle.go  shutdown / detach
│   ├── reduce_helpers.go    shared reducer helpers including frame-stack helpers
│   │                        (activeFrame, rootFrame, findFrame, truncateFrames)
│   ├── reduce_peer.go   peer-mesh reducers (PeerList, PeerSend, …)
│   ├── peer.go          Peer state types (PeerSummary, InboxEntry, …)
│   ├── driver_iface.go  Driver interface (Step, Status, View, Persist, Restore,
│   │                    PrepareLaunch). DriverState / DriverEvent / DriverStateBase
│   │                    marker. LaunchMode / LaunchOptions / LaunchPlan /
│   │                    CreateLaunch / CreatePlan
│   ├── status.go        Re-exports state/view.Status, StatusInfo as type aliases
│   ├── view.go          Re-exports state/view.View, Card, Tag, LogTab, InfoLine
│   │                    as type aliases
│   ├── tab_renderer.go  Pure renderer for LogTabs from frame state
│   ├── notify.go        Notification rule data structures
│   ├── clone.go         Copy-on-write helpers for State
│   └── view/            Wire-safe view types (stdlib-only; no state import; safe
│       │                for proto and reactor-bridge)
│       ├── status.go    Status enum + StatusInfo — canonical definition
│       │                (Running/Waiting/Idle/Stopped/Pending)
│       └── view.go      View / Card / Tag / LogTab / TabKind / InfoLine
├── client/driver/       Driver implementations — value-type plugins (no goroutines, no I/O)
│   ├── claude.go        claudeDriver — event-driven status + transcript job emit
│   ├── claude_event.go  DEvHook dispatch (state-change, session-start, ...)
│   ├── claude_tick.go   DEvTick: active gate + transcript parse job emit
│   ├── claude_view.go   View() — Card, LogTabs, InfoExtras, StatusLine
│   ├── claude_persist.go     Persist / Restore — opaque bag round-trip
│   ├── claude_fork.go        Push-driver / fork bookkeeping for the Claude driver
│   ├── claude_runners.go     Built-in runners (TranscriptParse, branch detection)
│   ├── claude_statusline.go  Status-line rendering helpers
│   ├── claude_tool_log.go    Tool-log append helpers
│   ├── claude_worktree.go    Managed-worktree CreateSessionPlanner glue
│   ├── codex*.go             codexDriver — Codex app-server subsystem consumer
│   │                         (event, persist, runners, tool_log, view, worktree)
│   ├── gemini*.go            geminiDriver — Gemini hook + tick consumer
│   ├── generic.go            genericDriver — Idle/Waiting transitions driven by
│   │                         tick + OSC events
│   ├── generic_view.go       View()
│   ├── shell.go              shellDriver — OSC 133 prompt-phase consumer
│   ├── vt/              VT emulator wrapper — driver/vt.Terminal feeds bytes to
│   │                    charmbracelet/x/vt and exposes OnOscNotification /
│   │                    OnWindowTitle / OnPromptEvent callbacks
│   ├── jobs.go          Job input/output types (TranscriptParseInput,
│   │                    BranchDetectInput, ...)
│   ├── runners.go       Shared built-in runners (HaikuSummary, GitBranch, ...)
│   ├── summary*.go      Cross-driver summary prompt + job
│   ├── tags.go          CommandTag helper
│   └── register.go      init() registers with state.Register
├── client/runtime/      Imperative shell — event loop + Effect interpreter
│   ├── runtime.go       Runtime.Run() — single event loop (select)
│   ├── interpret.go     execute(Effect) — interpreter for all side effects
│   ├── interpret_spawn.go    EffSpawnPaneWindow interpreter (sandbox dispatch,
│   │                         launcher wiring, env propagation)
│   ├── inject_prompt.go      EffInjectPrompt — load-buffer + paste-buffer + Enter
│   ├── pane_injector.go      Helper used by inject_prompt to address backend panes
│   ├── ipc.go           Host IPC server (accept + SO_PEERCRED uid check,
│   │                    readLoop, writeLoop)
│   ├── ipc_container.go Container IPC endpoint (per-project Unix socket;
│   │                    `hook-event` / `subsystem-event`)
│   ├── framereg/        Per-frame bearer token + bind-mount registry
│   │   └── registry.go  Single-writer / multi-reader (event loop writes;
│   │                    container endpoint handlers read concurrently)
│   ├── warm_state.go    Persists / replays container tokens across daemon
│   │                    warm-restart (`<dataDir>/warm/`)
│   ├── rundir.go        Per-project run directory (host-side dir bind-mounted as
│   │                    `/opt/agent-reactor/run`)
│   ├── launcher.go      AgentLauncher interface + DirectLauncher + WrappedLaunch +
│   │                    container-token wrap
│   ├── notifier.go      Notification dispatch wiring (driven by EffRecordNotification)
│   ├── resident.go      Long-lived per-project resources (sandbox managers, etc.)
│   ├── terminal_relay.go     Per-subscriber pane→client byte relay for the
│   │                         HTTP/WS gateway (EffSurfaceSubscribeStart)
│   ├── backends.go      PaneLifecycle / PaneIO / PaneInspect / SessionEnv /
│   │                    WindowLayout / BackendControl / PaneBackend +
│   │                    PersistBackend / EventLogBackend / ToolLogBackend /
│   │                    FSWatcher interfaces and noop fakes
│   ├── pty_backend.go   PtyBackend — production PaneBackend implementation over
│   │                    platform/termvt (synthetic "%N" pane ids, per-session
│   │                    scrollback cap)
│   ├── panetap.go       PaneTap interface — raw byte stream abstraction
│   ├── pty_tap.go       PtyPaneTap — subscribes directly to termvt.Manager and
│   │                    forwards Output events as the raw byte stream
│   ├── tap_manager.go   per-frame tap lifecycle; runs a goroutine that feeds tap
│   │                    bytes into a driver/vt.Terminal and emits EvPaneOsc
│   │                    (OSC 0/2/9/99/777) and EvPanePrompt (OSC 133) into eventCh
│   ├── stream_backend.go     UDS path resolution for the per-session app-server
│   ├── subsystem.go          Subsystem dispatcher entry point
│   ├── subsystem/            Subsystem backend implementations
│   │   ├── subsystem.go      Subsystem interface + factory registry
│   │   ├── worktree.go       Managed-worktree subsystem (frame BindFrame hook)
│   │   ├── cli/              CLI-backed subsystems (hook-driven agents)
│   │   └── stream/           Stream-backed subsystems (Codex app-server)
│   ├── peercred_linux.go     SO_PEERCRED uid verification (Linux)
│   ├── peercred_other.go     no-op stub (non-Linux)
│   ├── persist.go            PersistBackend concrete implementation (sessions.json)
│   ├── eventlog.go           EventLogBackend concrete implementation
│   ├── tool_log.go           ToolLogBackend concrete implementation
│   ├── fsnotify.go           FSWatcher concrete implementation
│   ├── convert.go            state.View → proto.SessionInfo conversion
│   ├── proto_bridge.go       proto.Command → state.Event conversion
│   ├── proto_bridge_surface.go  surface.* command bridging
│   ├── bootstrap.go          Initial State construction for warm/cold restart;
│   │                         RecoverSandboxFrames
│   ├── bootstrap_coldstart.go    Cold-start frame reconstruction
│   ├── cleanup.go            Per-frame cleanup callbacks (sandbox release,
│   │                         container token revoke)
│   ├── filerelay.go          File relay (per-frame file watcher → event log)
│   ├── testing.go            Test helper (fake backend)
│   └── worker/               Worker pool
│       ├── pool.go           Pool + Submit[In,Out] (typed job submission)
│       └── registry.go       RegisterRunner[In,Out] + Dispatch
│                             (JobKind-based runner registry)
├── client/proto/        Typed IPC wire layer — imports state/view only
│                        (safe for reactor-bridge)
│   ├── envelope.go      Envelope wire format ({type, req_id, cmd|name, data})
│   ├── command.go       Command closed sum type (CmdSubscribe, CmdUnsubscribe,
│   │                    CmdEvent, CmdHookEvent (container-only), CmdSurface*,
│   │                    CmdDriverList, CmdPeer*)
│   ├── surface_command.go    Surface command variants (read_text / send_text /
│   │                         send_key / subscribe / unsubscribe / resize /
│   │                         write_raw)
│   ├── surface_event.go      EvtSurfaceOutput / EvtPromptEvent wire types
│   ├── response.go      Response closed sum type (RespOK, RespSurfaceText,
│   │                    RespDriverList, RespPeerList, RespPeerDrainInbox).
│   │                    Session-related types live in proto/sessions
│   ├── event.go         ServerEvent closed sum type
│   ├── codec.go         NDJSON encode/decode
│   ├── client.go        proto.Client (Dial / DialConn / Send / Events); used by
│   │                    the operator CLI, reactor-bridge, HTTP/WS gateway
│   ├── client_helpers.go     Peer helpers (PeerSend/List/SetSummary/DrainInbox),
│   │                         SendEvent, SendHookEvent
│   ├── reqid.go         Request ID generation
│   ├── errors.go        ErrCode enum
│   └── protofake/       In-memory proto.Client fake for tests
├── client/proto/sessions/    Session management layer — imports proto + state
│                             (NOT used by reactor-bridge)
│   ├── client.go        sessions.Client wraps *proto.Client; session helpers
│   │                    (CreateSession, StopSession, ListSessions, PushDriver,
│   │                    ActivateFrame, ...)
│   └── helpers.go       sendJSONEvent / sendJSONEventTimeout helpers;
│                        timeout constants
├── client/web/          Browser frontend (xterm.js)
│   ├── index.html       Single-page shell
│   ├── package.json     Frontend deps (vite, xterm, vitest)
│   ├── vite.config.ts   Build config (output → dist/)
│   ├── src/             TypeScript sources
│   │   ├── main.tsx          Entry point
│   │   ├── App.tsx           Top-level component
│   │   ├── socket/           WebSocket client (gateway protocol)
│   │   ├── wire/             Wire-format encoders / decoders mirroring proto/
│   │   ├── store/            Client-side state (active session, subscriptions)
│   │   ├── components/       Pane / session-list / palette UI
│   │   ├── hooks/            Reusable React hooks
│   │   ├── lib/              Pure helpers
│   │   ├── css/              Stylesheets
│   │   └── auth.ts           Bearer-token handshake
│   ├── dist/            Built bundle (vite build output)
│   ├── embed.go         //go:embed dist/* — exposes the built bundle to cmd/web
│   ├── headers.go       Static-asset header helpers
│   └── host.go          HTTP host that serves the embedded bundle
├── client/procio/
│   └── procio.go        Buffered stdin/stdout helpers for subcommand wiring
├── client/tools/
│   ├── tools.go         Tool + Param + ToolContext + Registry + DefaultRegistry
│   └── builtin.go       Built-in tool registrations
├── client/lib/          Client-only helpers (not used by reactor-bridge)
│   ├── claude/          Claude CLI + transcript glue
│   ├── codex/           Codex CLI glue
│   ├── editor/          $EDITOR launcher
│   └── peers/           Peer-mesh client helpers
├── client/config/
│   ├── config.go        TOML configuration loading (`ROOST_DATA_DIR` env override).
│   │                    Source of truth for user-facing settings.toml fields
│   ├── notify.go        Notification rule matching
│   ├── project.go       Project enumeration from `project_roots` / `project_paths`
│   └── workspace_resolver.go  Reads each project's `.agent-reactor/settings.toml`
│                              for workspace grouping
├── platform/agentlaunch/
│   ├── spawn.go         Spawn — argv-direct process launch (no host shell),
│   │                    SpawnResult{Stdout,Stdin,Wait,PID}
│   ├── splitargs.go     SplitArgs — POSIX shell-word tokenizer
│   │                    (single/double quotes, backslash-escape)
│   ├── types.go         LaunchPlan{Command,Argv,…} /
│   │                    WrappedLaunch{Command,Argv,…} — dual representation
│   ├── devcontainer.go  DevcontainerLauncher — Wrap produces docker exec argv
│   │                    (TTY conditional on consumer)
│   ├── direct.go        DirectDispatcher — pass-through (no-op wrapping)
│   └── mounts.go        WrappedLaunch.HostPath — container→host path translation
│                        via Mounts
├── platform/sandbox/    Project-level sandbox backends (generic Manager[I any])
│   ├── manager.go       Instance[I] / Manager[I] / StartOptions interface
│   │                    definitions
│   └── devcontainer/    Devcontainer backend (per-project container lifecycle
│                        via docker)
├── platform/hostexec/   Host-exec broker (`container.Provider` impl): per-project
│                        Unix socket server that runs allowlisted host binaries on
│                        behalf of container processes via SCM_RIGHTS stdio
│                        forwarding; deny/allow glob policy with env-assignment
│                        prefix stripping. Credential providers (awssso /
│                        gcloudcli / sshagent) live in the external `credproxy`
│                        library under `providers/<name>/`
├── platform/termvt/     Pty + VT manager that PtyBackend builds on. Owns the
│                        Manager → Session map and emits structured Output events.
├── platform/lib/        Provider-specific helpers usable by both client and
│                        orchestrator layers (claude, codex, gemini, git, github,
│                        plastic, vcs, …)
├── platform/pathmap/    Container↔host path translation for IPC payloads
│                        (uses WrappedLaunch.Mounts)
└── platform/logger/
    └── logger.go        slog initialization
```
