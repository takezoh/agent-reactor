package runtime

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
)

const coldStartContainerConcurrency = 4

// PrewarmContainers starts devcontainers for all sandboxed projects in parallel
// before the serial spawn loop. Errors are logged but do not abort cold start;
// the serial path will surface the same error per-frame if it persists.
func (r *Runtime) PrewarmContainers(ctx context.Context) {
	if r.state.SandboxedProject == nil {
		return
	}
	seen := make(map[string]bool)
	for _, sess := range r.state.Sessions {
		if sess.Sandbox == state.SandboxOverrideHost {
			continue
		}
		for _, frame := range sess.Frames {
			if frame.Project != "" && r.state.SandboxedProject(frame.Project) {
				seen[frame.Project] = true
			}
		}
	}
	if len(seen) == 0 {
		return
	}

	l := launcher(r.cfg)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(coldStartContainerConcurrency)
	for p := range seen {
		eg.Go(func() error {
			if err := l.EnsureProject(egCtx, p); err != nil {
				slog.Warn("cold start: container prewarm failed", "project", p, "err", err)
			}
			return nil
		})
	}
	_ = eg.Wait()
}

// RecreateAll spawns fresh pane windows for every session in r.state.
// Used during cold-start (the pane backend session was just created and
// contains no client windows yet). Populates r.sessionPanes.
// Spawn failures are logged but do not remove the session: a transient
// error is not evidence that the user intended to delete the session.
func (r *Runtime) RecreateAll() error {
	for id, sess := range r.state.Sessions {
		if err := r.recreateSessionFrames(id, sess); err != nil {
			slog.Warn("bootstrap: session cold-start incomplete, leaving in state",
				"session", id, "err", err)
		}
	}
	return nil
}

func (r *Runtime) recreateSessionFrames(id state.SessionID, sess state.Session) error {
	var firstErr error
	for _, frame := range sess.Frames {
		if skipColdStartSpawn(frame) {
			slog.Info("bootstrap: skipping spawn for stopped frame",
				"session", id, "frame", frame.ID, "command", frame.Command)
			continue
		}
		// One frame's spawn failure (e.g. a codex resume against a vanished
		// thread) must not strand its healthy siblings — keep spawning the rest
		// and report the first error so the caller logs the session as incomplete.
		if err := r.spawnFrameWindow(id, sess.Sandbox, frame); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// skipColdStartSpawn returns true when a stopped frame must not be respawned
// on cold start. The command exited in a prior session and the frame is kept
// for inspection; resurrecting it as a fresh process would destroy the very
// state the user wants to look at. The exception is a driver whose durable
// state survives the dead pane (codex resumes its thread) — such a frame is
// relaunched, so it is not skipped.
func skipColdStartSpawn(frame state.SessionFrame) bool {
	drv := state.GetDriver(frame.Command)
	if drv == nil || frame.Driver == nil {
		return false
	}
	if drv.Status(frame.Driver) != state.StatusStopped {
		return false
	}
	return !coldStartRecoverable(drv, frame.Driver)
}

// coldStartRecoverable reports whether a driver declares its stopped state
// restorable across a cold start via the optional ColdStartRecoverer capability.
func coldStartRecoverable(drv state.Driver, s state.DriverState) bool {
	rec, ok := drv.(state.ColdStartRecoverer)
	return ok && s != nil && rec.RecoverableOnColdStart(s)
}

// spawnFrameWindow prepares and spawns a single frame's pane window during cold start.
// It runs PrepareLaunch → WrapLaunch → SpawnWindow, registers the cleanup callback,
// and records the pane ID in session env.
func (r *Runtime) spawnFrameWindow(id state.SessionID, sandbox state.SandboxOverride, frame state.SessionFrame) error {
	drv := state.GetDriver(frame.Command)
	if drv == nil {
		return nil
	}
	sandboxed := r.state.SandboxedProject != nil &&
		r.state.SandboxedProject(frame.Project) &&
		sandbox != state.SandboxOverrideHost
	launch, err := drv.PrepareLaunch(frame.Driver, state.LaunchModeColdStart, frame.Project, frame.Command, frame.LaunchOptions, sandboxed)
	if err != nil {
		slog.Error("bootstrap: prepare launch failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}
	launch.Sandbox = sandbox
	launch.Project = frame.Project

	ctx := r.baseContext()
	sub, subsystemID, err := ensureSubsystemOnce(ctx, r.subsystemFactories, id, launch.Subsystem, frame.Project, launch)
	if err != nil {
		slog.Error("bootstrap: ensure subsystem failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}
	bindResult, err := sub.BindFrame(ctx, rsubsystem.BindRequest{
		FrameID: frame.ID,
		Plan:    launch,
		Project: frame.Project,
	})
	if err != nil {
		slog.Error("bootstrap: bind frame failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}
	// Cold start runs serially before the event loop (rt.Run), so writing the
	// loop-owned maps directly here is safe.
	r.subsystems[subsystemID] = sub
	r.frameSubsystems[frame.ID] = sub
	launch = bindResult.Plan

	baseEnv := map[string]string{
		"ROOST_SESSION_ID": string(id),
		"ROOST_FRAME_ID":   string(frame.ID),
	}
	wrapResult, err := wrapLaunchForSpawn(launcher(r.cfg), frame.ID, frame.Project, launch, baseEnv)
	if err != nil {
		slog.Error("bootstrap: wrap launch failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}
	wrapped := wrapResult.wrapped

	paneID, err := r.spawnWrapped(frame.ID, frame.Project, wrapped)
	if err != nil {
		slog.Error("bootstrap: spawn failed", "id", id, "frame", frame.ID, "err", err)
		if wrapped.Cleanup != nil {
			if cerr := wrapped.Cleanup(); cerr != nil {
				slog.Warn("bootstrap: cleanup after spawn failure", "frame", frame.ID, "err", cerr)
			}
		}
		return err
	}

	r.sessionPanes[frame.ID] = paneID
	if wrapped.Cleanup != nil {
		r.storeFrameCleanup(frame.ID, wrapped.Cleanup)
	}
	if wrapResult.token != "" {
		r.registerContainerFrame(frame.ID, frame.Project, wrapped.ContainerSockDir, wrapResult.token, wrapped.Mounts)
	}
	envKey := sessionPaneEnvKey(frame.ID)
	if err := r.cfg.Backend.SetEnv(envKey, paneID); err != nil {
		slog.Warn("bootstrap: set pane env failed", "key", envKey, "err", err)
	}
	return nil
}

// spawnWrapped calls SpawnWindow for a WrappedLaunch. Window sizing is owned by
// the connected client (the browser terminal resizes via the surface RPC after
// attach), so cold-start does not pre-size the pane.
func (r *Runtime) spawnWrapped(frameID state.FrameID, project string, wrapped WrappedLaunch) (string, error) {
	name := windowName(project, string(frameID))
	spawnCmd := buildSpawnCommand(wrapped.Command, nil)
	slog.Info("runtime: spawning window", "frame", frameID, "cmd", spawnCmd, "mode", "coldstart")
	_, paneID, err := r.cfg.Backend.SpawnWindow(name, spawnCmd, wrapped.StartDir, wrapped.Env)
	if err != nil {
		return "", err
	}
	return paneID, nil
}
