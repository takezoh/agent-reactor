package runtime

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	rsubsystem "github.com/takezoh/agent-roost/runtime/subsystem"
	"github.com/takezoh/agent-roost/state"
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

// RecreateAll spawns fresh tmux windows for every session in r.state.
// Used during cold-start (the tmux session was just created and
// contains no roost windows yet). Populates r.sessionPanes.
// Spawn failures are logged but do not remove the session: a transient
// error is not evidence that the user intended to delete the session.
func (r *Runtime) RecreateAll() error {
	size := r.mainPaneSize()
	for id, sess := range r.state.Sessions {
		if err := r.recreateSessionFrames(id, sess, size); err != nil {
			slog.Warn("bootstrap: session cold-start incomplete, leaving in state",
				"session", id, "err", err)
		}
	}
	return nil
}

func (r *Runtime) recreateSessionFrames(id state.SessionID, sess state.Session, size paneSize) error {
	for _, frame := range sess.Frames {
		if isStoppedFrame(frame) {
			slog.Info("bootstrap: skipping spawn for stopped frame",
				"session", id, "frame", frame.ID, "command", frame.Command)
			continue
		}
		if err := r.spawnFrameWindow(id, sess.Sandbox, frame, size); err != nil {
			return err
		}
	}
	return nil
}

// isStoppedFrame returns true when the frame's driver reports
// StatusStopped — the command exited abnormally in a prior session
// and the frame is kept for inspection. Cold-start must not respawn
// such frames: the command is gone, and resurrecting it as a fresh
// process would destroy the very state the user wants to look at.
func isStoppedFrame(frame state.SessionFrame) bool {
	drv := state.GetDriver(frame.Command)
	if drv == nil || frame.Driver == nil {
		return false
	}
	return drv.Status(frame.Driver) == state.StatusStopped
}

// spawnFrameWindow prepares and spawns a single frame's tmux window during cold start.
// It runs PrepareLaunch → WrapLaunch → SpawnWindow, registers the cleanup callback,
// and records the pane ID in session env.
func (r *Runtime) spawnFrameWindow(id state.SessionID, sandbox state.SandboxOverride, frame state.SessionFrame, size paneSize) error {
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

	ctx := context.Background()
	sub, _, err := r.ensureSubsystem(ctx, launch.Subsystem, frame.Project, launch)
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
	r.frameSubsystems.Store(frame.ID, sub)
	launch = bindResult.Plan

	baseEnv := map[string]string{
		"ROOST_SESSION_ID": string(id),
		"ROOST_FRAME_ID":   string(frame.ID),
	}
	wrapped, err := r.wrapWithContainerToken(frame.ID, frame.Project, launch, baseEnv)
	if err != nil {
		slog.Error("bootstrap: wrap launch failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}

	paneID, err := r.spawnWrapped(frame.ID, frame.Project, wrapped, size)
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
	envKey := sessionPaneEnvKey(frame.ID)
	if err := r.cfg.Tmux.SetEnv(envKey, paneID); err != nil {
		slog.Warn("bootstrap: set pane env failed", "key", envKey, "err", err)
	}
	return nil
}

// spawnWrapped calls SpawnWindow for a WrappedLaunch and resizes the resulting window.
func (r *Runtime) spawnWrapped(frameID state.FrameID, project string, wrapped WrappedLaunch, size paneSize) (string, error) {
	name := windowName(project, string(frameID))
	tmuxCmd := buildSpawnCommand(wrapped.Command, nil)
	slog.Info("runtime: spawning window", "frame", frameID, "cmd", tmuxCmd, "mode", "coldstart")
	target, paneID, err := r.cfg.Tmux.SpawnWindow(name, tmuxCmd, wrapped.StartDir, wrapped.Env)
	if err != nil {
		return "", err
	}
	r.resizeWindowToMain(r.cfg.SessionName+":"+target, size)
	return paneID, nil
}
