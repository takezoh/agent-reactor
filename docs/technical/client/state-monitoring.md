# State monitoring

For the interactive operation processing flow (TUI → IPC → Reduce → Effect), see [ipc.md](ipc.md). The following describes the background status update pipeline and state monitoring by Drivers.

## Background pipeline

Four parallel event sources feed Driver.Step:

- **Periodic tick (1s)**: `reduceTick` steps the active frame of each running session via `Driver.Step(frame.Driver, DEvTick{...})`. Pane reconciliation and the pane 0.0 health check are performed on the same tick. For the detailed sequence, see [ipc.md](ipc.md#tick-processing-sequence).
- **PaneTap OSC events**: When the `tapManager` reader goroutine receives bytes from `tmux pipe-pane`, it feeds them into a per-frame `driver/vt.Terminal` (a thin wrapper over `charmbracelet/x/vt`). The emulator fires synchronous callbacks for OSC 0/2 (window titles), OSC 9/99/777 (notifications), and OSC 133 (semantic prompt phases). The reader translates these into `EvPaneOsc` and `EvPanePrompt` events.
- **Driver hooks (`EvDriverEvent`)**: hook subprocesses send events through the IPC bridge; `reduceDriverHook` dispatches them to the owning frame's driver as `DEvHook`.
- **Subsystem events (`EvSubsystem`)**: structured backends send typed execution events through the IPC bridge; `reduceSubsystem` dispatches them to the owning frame's driver as `DEvSubsystem`.

Driver.Step returns `[]Effect` — `EffStartJob` for slow I/O (transcript parse, haiku summary, git branch detect), `EffEventLogAppend` for operator-visible event log writes, and so on. Worker results are fed back via `EvJobResult` → `Driver.Step(DEvJobResult)` and reflected in DriverState.

## State monitoring

The Driver plugin's `Step` method is responsible for status updates. For the Driver interface definition, see [interfaces.md](interfaces.md#interfaces).

### Lifecycle:

| Method | Caller | Purpose |
|---------|-----------|------|
| `NewState(now)` | `reduceCreateSession`, `reducePushDriver` | Generates a fresh DriverState value for a new frame. Initial values are Idle / now |
| `Restore(bag, now)` | `runtime.Bootstrap` | Reconstructs each frame's DriverState from the previously saved opaque map on warm/cold restart |
| `PrepareLaunch(s, mode, project, cmd, options)` | `reduceCreateSession`, `reducePushDriver`, cold-start bootstrap | Pure function that resolves the frame's launch plan (command / start_dir / normalized `LaunchOptions`). Called synchronously inside `state.Reduce` and on cold-start restoration; the resolved plan is baked into `EffSpawnTmuxWindow` so the runtime never calls drivers |
| `PrepareCreate(s, sessID, project, cmd, options)` | `reduceCreateSession` (planner-gated drivers only) | Optional extension returning a `CreatePlan` with a `SetupJob` for async pre-launch work (e.g., creating a managed worktree) |
| `CompleteCreate(s, cmd, options, result, err)` | `handlePendingCreate` (planner-gated drivers only) | Runs after the SetupJob completes; returns the final `CreateLaunch` and the normalized `LaunchOptions` to persist on the frame |
| `Step(prev, DEvTick)` | `reduceTick` | Periodic tick on the active frame of each running session. Claude gates on `DEvTick.Active`, emitting transcript parse jobs only when active. Generic transitions Running → Waiting after `IdleThreshold` elapses without OSC activity |
| `Step(prev, DEvPaneOsc)` | `reducePaneOsc` | Routes OSC 0/2 (window title) sequences to the driver. Claude/Codex/Gemini interpret the title to update status (e.g. Braille spinner = Running, "✳" = Waiting) |
| `Step(prev, DEvPanePrompt)` | `reducePanePrompt` | Routes OSC 133 semantic-prompt events. Shell driver sets `SawPromptEvent` on first observation and updates `LastExitCode` on `PromptPhaseComplete` |
| `Step(prev, DEvHook)` | `reduceDriverHook` | Receives hook events targeted at a specific frame and updates that frame's DriverState. Used by hook-driven agents such as Claude and Gemini |
| `Step(prev, DEvSubsystem)` | `reduceSubsystem` | Receives structured subsystem events targeted at a specific frame and updates that frame's DriverState. Used by Codex App Server |
| `Step(prev, DEvJobResult)` | `reduceJobResult` | Reflects results from the worker pool into the owning frame's DriverState. Transcript parse results such as title / lastPrompt |
| `Step(prev, DEvFileChanged)` | `reduceFileChanged` | File change notification from fsnotify. Emits transcript parse job |
| `View(driverState)` | runtime's `broadcastSessionsChanged` / `activeStatusLine` | Pure getter that returns display payloads for the TUI (Card / LogTabs / InfoExtras / StatusLine) |
| `Persist(driverState)` | runtime's `snapshotSessions` | Serializes DriverState to an opaque map. Written to sessions.json alongside the frame's command and normalized `LaunchOptions` |

### Active/Inactive and DEvTick.Active (push model)

"Session is active" means the session's active frame pane is swapped into pane 0.0 (main). The single source of truth is `state.State.ActiveSession` (SessionID), and `reduceTick` evaluates `sessID == state.ActiveSession` when constructing `DEvTick` to set the `DEvTick.Active` flag. Step is called on the active frame of every running session on every tick, passing `DEvTick{Active: false}` to inactive sessions. Activation is detected on the next tick (within 1 second).

### Claude driver (event-driven + active-gated transcript sync)

`claudeDriver`'s status is **fully event-driven**: the status in DriverState is updated only at the moment `Step(prev, DEvHook{Event: "state-change"})` receives a state-change event. If no new event arrives, the status does not change (= the previously restored status continues to be displayed).

Transcript metadata (title / lastPrompt, etc.) is incrementally parsed by `transcript.Tracker` inside the worker pool's `TranscriptParse` runner:

- `Step(prev, DEvTick{Active: true})`: Emits transcript parse job only when active. Returns immediately when inactive
- `Step(prev, DEvHook)`: Always updates DriverState regardless of active/inactive. Also emits transcript parse job
- `Step(prev, DEvJobResult{TranscriptParseResult})`: Reflects parse results (title / lastPrompt / statusLine) into DriverState
- `Step(prev, DEvFileChanged)`: File change notification from fsnotify. Emits transcript parse job

`lastPrompt` is obtained by `transcript.Tracker` walking the parentUuid chain backwards from the tail and returning the text of the first non-synthetic `KindUser` entry.

Hook event → driver.Status mapping:

| Hook event | Status |
|--------------|--------|
| UserPromptSubmit, PreToolUse, PostToolUse, SubagentStart | Running |
| Stop, Notification(idle_prompt) | Waiting |
| StopFailure, SessionEnd | Stopped |
| Notification(permission_prompt) | Pending |
| SessionStart | Idle |
| SessionEnd | Stopped |

The `roost event <eventType>` subcommand repackages the Claude hook payload into `proto.CmdEvent` and sends it via IPC. The runtime's IPC reader converts it into an `EvDriverEvent` and feeds it into the event loop. `reduceDriverHook` locates the owning frame across all sessions using the frame id it received as `SenderID`, and calls `Driver.Step(frame.Driver, DEvHook{...})`. Neither the state layer nor the runtime layer holds any Claude-specific state logic.

### Codex driver (App Server stream + display-only transcript)

`CodexDriver` is driven by structured subsystem events from `codex app-server`, not by hooks.

- `Step(prev, DEvSubsystem{Kind: session_ready | turn_started | turn_completed})`: updates running/waiting lifecycle and stores the logical thread identity
- `Step(prev, DEvSubsystem{Kind: tool_started | tool_completed | approval_requested | approval_resolved})`: updates current tool and pending approval state
- `Step(prev, DEvSubsystem{Kind: plan_updated | diff_updated | message_updated})`: updates plan summary, diff summary, assistant message, and recent turns
- `Step(prev, DEvFileChanged)` / `Step(prev, DEvJobResult)`: transcript parsing still runs, but only to populate display tabs and supplemental fields

For Codex, transcript files are display-only. The source of truth for status, approval, tool execution, plan, and diff is the App Server event stream.

### Hook event routing and race-free identification

A mechanism for the hook subprocess to identify its owning roost frame in a race-free manner.

**Problem**: There is a window after `tmux new-window` where any pane-scoped tmux option written by the daemon is not yet visible to processes inside the pane. If a hook fires during this window, the option-based owner marker is unset and the event is discarded.

**Solution**: Inject a frame-scoped env var into the pane environment at `tmux new-window -e` time. The env var is set at the kernel exec level simultaneously with the window creation, so no race occurs. The hook bridge reads the frame id directly from its own process environment, requiring no round-trip to tmux. The reducer then scans the frame stacks to locate the owning frame and routes the hook to that frame's driver. Hooks whose target frame has already been truncated off the stack are silently dropped — this is the intended behavior when a frame's pane has just died and the reducer is still processing the eviction.

### OSC pipeline (pipe-pane → VT emulator → driver)

Pane status detection is OSC-driven. tmux `pipe-pane` streams the raw byte sequence from each frame into a per-frame `driver/vt.Terminal`; the emulator parses the byte stream and fires synchronous callbacks for the OSC sequences agents and shells emit:

| OSC | Source | Routing |
|-----|--------|---------|
| 0 / 2 | Window title (Claude/Codex/Gemini emit "✳ Working", "✋ Action Required", etc.) | `EvPaneOsc` → `EffEventLogAppend` (EVENTS log) + `DEvPaneOsc` to driver for status interpretation |
| 9 / 99 / 777 | Desktop notification protocols (Growl / Kitty / urxvt) | `EvPaneOsc` → `EffRecordNotification` (writes EVENTS log + dispatches optional desktop toast) |
| 133 | FinalTerm semantic prompt phases (`A`=start, `B`=input, `C`=command, `D`=complete with exit code) | `EvPanePrompt` → `EffEventLogAppend` + `DEvPanePrompt` to driver |

### Generic driver

`genericDriver` runs in a Waiting state by default. OSC events received via `DEvPaneOsc` may transition it to Running (e.g. when the pane reports a working spinner). Without OSC activity, the driver falls back to `IdleThreshold`-based decay: Running → Waiting after the configured duration elapses.

### Shell driver

`shellDriver` consumes OSC 133 prompt events:

- First observation of any phase sets `SawPromptEvent = true`, indicating the shell uses semantic prompt markers.
- `PromptPhaseComplete` updates `LastExitCode` from `\x1b]133;D;<exit-code>\x1b\\`.

### State persistence and restoration

`Driver.Persist(driverState)` returns an opaque `map[string]string` interpreted by the driver, and `EffPersistSnapshot` writes it to `sessions.json`. Frame-to-pane mapping is stored in tmux session-level env vars (not in sessions.json), so pane ids do not leak into the snapshot file.

`sessions.json` is organized as a list of sessions, where each session contains a **frame stack** `frames[]`. Each frame in the stack carries its own `command`, normalized `launch_options`, and the driver-interpreted `driver_state` bag. The active frame is not persisted — it is always the tail of the stack at load time. `LaunchOptions` is stored in its canonical (normalized) form that drivers returned from `PrepareLaunch`; on cold start the bootstrap re-feeds those persisted options back into `PrepareLaunch` so each frame respawns with the same launch flavor (worktree vs in-place, etc.).

#### Writing (runtime)

When a driver's Step updates its frame's DriverState on each tick / hook event, the reducer emits `EffPersistSnapshot`, and the runtime's Effect interpreter writes it to `sessions.json`:

```mermaid
sequenceDiagram
    participant Red as state.Reduce
    participant Drv as Driver.Step (pure)
    participant Interp as Effect interpreter
    participant JSON as sessions.json<br/>(single SoT)

    Note over Red: EvTick or EvDriverEvent
    Red->>Drv: Driver.Step(frame.Driver, driverEvent)
    Drv-->>Red: (driverState', effects, view)
    Note over Red: frame.Driver = driverState'
    Red-->>Interp: [EffPersistSnapshot, ...]
    Interp->>Interp: snapshotSessions():<br/>for each session, serialize frames[] with<br/>command / launch_options / driver_state
    Interp->>JSON: Write to sessions.json
    Note over JSON: sessions.json is the sole persistence target<br/>Pane ids are kept in tmux session-level env vars
```

#### Restoration (warm restart / cold boot)

There are two restoration paths. **Warm restart** (tmux server alive) rebuilds the frame-to-pane map from tmux session-level env vars, restores each frame's DriverState via `Driver.Restore`, and truncates any session at the first frame whose pane has gone missing (dropping the whole session if its root frame is missing). Active session is not restored; startup always returns pane `0.0` to the main TUI. **Cold boot** (tmux server also dead) walks each restored session's frame stack in root-to-tail order and respawns a window per frame via `Driver.PrepareLaunch(LaunchModeColdStart, …)`, re-feeding the persisted `LaunchOptions` so the launch flavor is preserved across restarts:

```mermaid
sequenceDiagram
    participant Boot as runtime.Bootstrap
    participant Tmux as tmux backend
    participant JSON as sessions.json
    participant Drv as Driver

    alt warm restart (tmux server alive)
        Boot->>JSON: Load()
        JSON-->>Boot: sessions with frame stacks<br/>(each frame: command, launch_options, driver_state)
        Boot->>Drv: Driver.Restore(bag, now) (per frame)
        Drv-->>Boot: DriverState (restored)
        Boot->>Tmux: read frame-to-pane env vars
        Boot->>Boot: truncate each session at first<br/>frame whose pane has vanished
    else cold boot (tmux server also dead)
        Boot->>JSON: Load()
        JSON-->>Boot: sessions with frame stacks
        Boot->>Drv: Driver.Restore(bag, now) (per frame)
        Drv-->>Boot: DriverState
        loop each frame, root-to-tail
            Boot->>Drv: Driver.PrepareLaunch(driverState, ColdStart,<br/>project, command, launch_options)
            Drv-->>Boot: LaunchPlan (command, start_dir, normalized options)
            Boot->>Tmux: new-window spawning the frame
        end
    end
```

#### PersistedState schema per Driver

`claudeDriver.PersistedState()`:
```
{
  "roost_session_id":     "abc-123",
  "claude_session_id":    "def-456",
  "working_dir":          "/path/to/workdir",
  "transcript_path":      "/path/to/transcript.jsonl",
  "status":               "running",
  "status_changed_at":    "2026-04-09T12:34:56Z",
  "branch_tag":           "feature/foo",
  "branch_bg":            "#334455",
  "branch_fg":            "#ffffff",
  "branch_target":        "/path/to/repo",
  "branch_at":            "2026-04-09T12:00:00Z",
  "branch_is_worktree":   "1",
  "branch_parent_branch": "main",
  "summary":              "haiku summary text",
  "title":                "conversation title",
  "last_prompt":          "most recent user prompt"
}
```

`codexDriver.PersistedState()`:
```
{
  "thread_id":            "abc-123",
  "requested_thread_id":  "abc-123",
  "observed_thread_id":   "abc-123",
  "resume_phase":         "attached",
  "managed_working_dir":  "/path/to/worktree",
  "status":               "running",
  "status_changed_at":    "2026-04-09T12:34:56Z",
  "summary":              "haiku summary text",
  "title":                "conversation title",
  "last_prompt":          "most recent user prompt"
}
```

`genericDriver.PersistedState()`:
```
{
  "status":             "running",
  "status_changed_at":  "2026-04-09T12:34:56Z"
}
```

| Scenario | Behavior |
|---------|------|
| **New session creation** | `reduceCreateSession` generates the initial DriverState via `Driver.NewState`, calls `Driver.PrepareLaunch` synchronously to resolve the frame's command / start_dir / normalized `LaunchOptions`, stores a root frame carrying the normalized options on the session, and emits `EffSpawnTmuxWindow` pre-baked with the resolved plan. The runtime spawns the tmux pane and reports back via `EvTmuxPaneSpawned` |
| **Push driver on top of a session** | `reducePushDriver` appends a new frame on top of the active frame, running the same `PrepareLaunch` / spawn pipeline as new session creation. The appended frame becomes the new active frame |
| **Warm restart (daemon only restarts)** | `runtime.Bootstrap` loads the frame stacks from sessions.json, restores each frame's DriverState via `Driver.Restore`, rebinds frames to their live tmux panes via frame-scoped env vars, and truncates any session at the first frame whose pane is missing (root-frame truncation drops the whole session). Pane `0.0` is always main TUI at startup |
| **Cold boot (tmux server also dead)** | `runtime.Bootstrap` loads the frame stacks, restores each frame's DriverState, then walks each session's frames in root-to-tail order and calls `Driver.PrepareLaunch(LaunchModeColdStart, …)` with the persisted `LaunchOptions` to reconstruct the launch plan. A tmux window is spawned per frame directly from the resolved plan |
| **Session stop** | `reduceStopSession` emits terminate / unwatch / unregister effects for every frame in the session. The session is removed from State only once the pane/window actually exits and a `EvTmuxWindowVanished` arrives |
| **Dead pane reap** | Pane reconciliation and `EvPaneDied` / `EvTmuxWindowVanished` locate the owning frame and truncate the session from that frame onward. If the root frame is the one that died, the entire session is deleted; otherwise the remaining lower frames stay and the new tail becomes the active frame |

### Cost extraction

Tool names, subagent counts, error counts, and other metrics from Claude sessions are extracted from the transcript JSONL by `transcript.Tracker` (`lib/claude/transcript`). `Tracker` is held within the worker pool's `TranscriptParse` runner, and results are returned to Driver.Step as `TranscriptParseResult`.
