package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/shellalias"
)

// spawnDeps is the narrow set of capabilities given to the spawn goroutine.
// It holds no *Runtime reference, so the goroutine cannot touch loop-owned
// state (conns, sessionPanes, subsystems, …) directly. Results flow back to
// the event loop via sendInternal (internalSpawnComplete) / sendEvent
// (EvSpawnFailed), preserving the single-writer discipline.
type spawnDeps struct {
	backend      PaneBackend
	launcher     AgentLauncher
	factories    map[state.LaunchSubsystem]rsubsystem.Factory
	sessionName  string
	mainPaneSize func() paneSize
	sendInternal func(internalEvent)
	sendEvent    func(state.Event)
}

// buildSpawnDeps snapshots the dependencies needed by the spawn goroutine.
// The goroutine holds no *Runtime reference so it cannot access loop-owned
// state (conns, sessionPanes, subsystems, …) directly.
func (r *Runtime) buildSpawnDeps() spawnDeps {
	return spawnDeps{
		backend:      r.cfg.Backend,
		launcher:     launcher(r.cfg),
		factories:    r.subsystemFactories,
		sessionName:  r.cfg.SessionName,
		mainPaneSize: r.mainPaneSize,
		sendInternal: r.sendSpawnComplete,
		sendEvent:    r.Enqueue,
	}
}

// spawnPaneWindow runs in a goroutine, performs all slow I/O (subsystem
// ensure, bind, launch wrap, pane spawn), and posts results back via
// internalSpawnComplete / EvSpawnFailed. It holds no *Runtime reference,
// so every state mutation is deferred to the event loop in handleSpawnComplete.
//
// The deferred panic recovery (recoverSpawnPanic) is load-bearing: a panic
// inside the spawn pipeline (devcontainer manager, subsystem factory,
// launcher wrapper, …) would otherwise propagate out of this goroutine and
// crash the entire daemon — killing every session inside it, including the
// agent session that issued the POST /api/sessions that triggered the
// spawn. Converting a panic into EvSpawnFailed keeps the failure scoped
// to the one session being created; the rest of the daemon continues serving.
func spawnPaneWindow(deps spawnDeps, e state.EffSpawnPaneWindow) {
	sendFailed := func(msg string) {
		deps.sendEvent(state.EvSpawnFailed{
			SessionID: e.SessionID, FrameID: e.FrameID,
			Err: msg, ReplyConn: e.ReplyConn, ReplyReqID: e.ReplyReqID,
		})
	}
	defer recoverSpawnPanic(e, sendFailed)

	ctx := context.Background()
	plan := state.LaunchPlan{
		Command:   e.Command,
		StartDir:  e.StartDir,
		Project:   e.Project,
		Sandbox:   e.Sandbox,
		Options:   e.Options,
		Subsystem: e.Subsystem,
		Stream:    e.Stream,
		Stdin:     e.Stdin,
	}

	sub, subsystemID, err := ensureSubsystemOnce(ctx, deps.factories, e.SessionID, e.Subsystem, e.Project, plan)
	if err != nil {
		slog.Error("runtime: ensure subsystem failed", "frame", e.FrameID, "err", err)
		sendFailed(err.Error())
		return
	}
	bindResult, err := sub.BindFrame(ctx, rsubsystem.BindRequest{
		FrameID: e.FrameID,
		Plan:    plan,
		Stdin:   e.Stdin,
		Project: e.Project,
	})
	if err != nil {
		slog.Error("runtime: bind frame failed", "frame", e.FrameID, "err", err)
		sendFailed(err.Error())
		return
	}
	plan = bindResult.Plan

	wrapResult, err := wrapLaunchForSpawn(deps.launcher, e.FrameID, e.Project, plan, e.Env)
	if err != nil {
		slog.Error("runtime: wrap launch failed", "frame", e.FrameID, "err", err)
		sendFailed(err.Error())
		return
	}
	wrapped := wrapResult.wrapped

	name := windowName(e.Project, string(e.FrameID))
	spawnCmd := buildSpawnCommand(wrapped.Command, e.Stdin)
	slog.Info("runtime: spawning window", "frame", e.FrameID, "cmd", spawnCmd)
	size := deps.mainPaneSize()
	target, paneID, err := deps.backend.SpawnWindow(name, spawnCmd, wrapped.StartDir, wrapped.Env)
	if err != nil {
		// wrapLaunchForSpawn already acquired the sandbox/container; the pane never
		// launched and no EvPaneSpawned/kill path will reach this frame, so
		// release it here to avoid leaking the container ref + cleanup closure.
		if wrapped.Cleanup != nil {
			if cerr := wrapped.Cleanup(); cerr != nil {
				slog.Warn("runtime: cleanup after spawn failure", "frame", e.FrameID, "err", cerr)
			}
		}
		sendFailed(err.Error())
		return
	}
	if size.width > 0 && size.height > 0 {
		if rerr := deps.backend.ResizeWindow(deps.sessionName+":"+target, size.width, size.height); rerr != nil {
			slog.Debug("runtime: resize-window failed", "target", target, "err", rerr)
		}
	}

	deps.sendInternal(internalSpawnComplete{
		effect:           e,
		subsystemID:      subsystemID,
		sub:              sub,
		cleanup:          wrapped.Cleanup,
		token:            wrapResult.token,
		mounts:           wrapped.Mounts,
		containerSockDir: wrapped.ContainerSockDir,
		paneID:           paneID,
		bindResult:       bindResult,
	})
}

// recoverSpawnPanic is the deferred panic handler for spawnPaneWindow. Logs
// the panic at Error with a full stack so an operator can trace the root
// cause, then surfaces the failure on the spawn reply channel via sendFailed
// so the HTTP POST that triggered the spawn gets a clean 502 instead of
// dropping its reply when the daemon crashes.
//
// Kept out of spawnPaneWindow's body so spawnPaneWindow stays under the
// project-wide 80-line function cap (funlen lint).
func recoverSpawnPanic(e state.EffSpawnPaneWindow, sendFailed func(string)) {
	rec := recover()
	if rec == nil {
		return
	}
	slog.Error("runtime: spawn goroutine panicked — daemon survives, session fails",
		"frame", e.FrameID,
		"session", e.SessionID,
		"panic", fmt.Sprintf("%v", rec),
		"stack", string(debug.Stack()),
	)
	sendFailed(fmt.Sprintf("spawn panicked: %v", rec))
}

// handleSpawnComplete runs on the event loop. It stores the per-frame I/O
// handles produced by spawnPaneWindow into loop-owned maps (and the container
// registry), then dispatches the pure EvPaneSpawned event.
func (r *Runtime) handleSpawnComplete(e internalSpawnComplete) {
	r.subsystems[e.subsystemID] = e.sub
	r.frameSubsystems[e.effect.FrameID] = e.sub
	r.frameSubsystemIDs[e.effect.FrameID] = e.subsystemID
	r.storeFrameCleanup(e.effect.FrameID, e.cleanup)

	if e.token != "" {
		r.registerContainerFrame(e.effect.FrameID, e.effect.Project, e.containerSockDir, e.token, e.mounts)
	}

	r.dispatch(state.EvPaneSpawned{
		SessionID:        e.effect.SessionID,
		FrameID:          e.effect.FrameID,
		SubsystemID:      e.subsystemID,
		PaneTarget:       e.paneID,
		WorktreeStartDir: e.bindResult.WorktreeStartDir,
		WorktreeName:     e.bindResult.WorktreeName,
		ReplyConn:        e.effect.ReplyConn,
		ReplyReqID:       e.effect.ReplyReqID,
	})
}

// ensureSubsystemOnce dispatches to the factory registered for the given kind
// and returns the Subsystem and its SubsystemID without storing into any
// runtime map. Called from the spawn goroutine; the event loop stores the
// result in handleSpawnComplete. An empty kind is treated as CLI (the default
// for drivers that do not set LaunchPlan.Subsystem explicitly).
func ensureSubsystemOnce(ctx context.Context, factories map[state.LaunchSubsystem]rsubsystem.Factory, sessionID state.SessionID, kind state.LaunchSubsystem, project string, plan state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	if kind == "" {
		kind = state.LaunchSubsystemCLI
	}
	factory, ok := factories[kind]
	if !ok {
		return nil, "", fmt.Errorf("runtime: unknown subsystem kind %q", kind)
	}
	return factory.Ensure(ctx, sessionID, project, plan)
}

// reapSubsystemIfLast removes and stops the backend for frameID if it was
// the last frame using that backend. Call after ReleaseFrame. Runs on the
// event loop, so frameSubsystemIDs is accessed as a plain loop-owned map.
func (r *Runtime) reapSubsystemIfLast(sub rsubsystem.Subsystem, frameID state.FrameID) {
	subsystemID, ok := r.frameSubsystemIDs[frameID]
	if !ok {
		return
	}
	delete(r.frameSubsystemIDs, frameID)
	// Check whether any other live frame still uses the same backend.
	hasOther := false
	for _, id := range r.frameSubsystemIDs {
		if id == subsystemID {
			hasOther = true
			break
		}
	}
	if hasOther {
		return
	}
	factory, ok := r.subsystemFactories[sub.Kind()]
	if !ok {
		return
	}
	if reaper, ok := factory.(rsubsystem.Reaper); ok {
		// Remove blocks until the backend process exits (up to stopGrace ≈ 6 s).
		// Run in a goroutine to avoid stalling the event loop.
		go reaper.Remove(context.Background(), subsystemID)
	}
}

// buildSpawnCommand builds the spawn command string for a resolved wrapped.Command.
// The bare shell command explicitly execs the user's passwd login shell rather
// than relying on a default-shell option.
func buildSpawnCommand(command string, stdin []byte) string {
	if isShellCommand(command) {
		return "exec " + shellalias.LoginShellCommand + " -l"
	}
	if len(stdin) > 0 {
		return wrapCommandWithStdin(command, stdin)
	}
	return "exec " + command
}

// windowName builds a stable display name for a new pane window from
// project + session id (matches the legacy SessionService format).
func windowName(project, sessionID string) string {
	if i := strings.LastIndex(project, "/"); i >= 0 {
		project = project[i+1:]
	}
	if project == "" {
		project = "session"
	}
	return project + ":" + sessionID
}

func substitutePlaceholdersString(s, sessionName, roostExe string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "{sessionName}", sessionName)
	s = strings.ReplaceAll(s, "{roostExe}", roostExe)
	return s
}

// isShellCommand returns true if the command should be spawned as the user's
// login shell.
func isShellCommand(command string) bool {
	return command == "shell"
}

// wrapCommandWithStdin writes input to a temp file and returns a shell
// command that feeds the file to command on stdin, then deletes it.
func wrapCommandWithStdin(command string, input []byte) string {
	f, err := os.CreateTemp("", "reactor-push-*.in")
	if err != nil {
		slog.Warn("buildStdinCommand: could not create temp file, stdin ignored",
			"err", err)
		return "exec " + command
	}
	if _, err := f.Write(input); err != nil {
		slog.Warn("buildStdinCommand: could not write temp file, stdin ignored",
			"err", err, "path", f.Name())
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "exec " + command
	}
	_ = f.Close()
	tmp := f.Name() // CreateTemp paths never contain special shell chars
	return "bash -c " + shellQuote(command+" < "+tmp+"; _ec=$?; rm -f "+tmp+"; exit $_ec")
}

// shellQuote wraps s in single quotes and escapes inner single quotes
// with the standard POSIX '\" sequence.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
