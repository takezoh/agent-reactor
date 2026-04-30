package runtime

import (
	"log/slog"

	"github.com/takezoh/agent-roost/state"
)

// RecreateAll spawns fresh tmux windows for every session in r.state.
// Used during cold-start (the tmux session was just created and
// contains no roost windows yet). Populates r.sessionPanes.
func (r *Runtime) RecreateAll() error {
	size := r.mainPaneSize()
	var dead []state.SessionID
	for id, sess := range r.state.Sessions {
		if err := r.recreateSessionFrames(id, sess, size); err != nil {
			dead = append(dead, id)
		}
	}
	for _, id := range dead {
		delete(r.state.Sessions, id)
	}
	if len(dead) > 0 {
		if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
			slog.Error("bootstrap: persist after recreate failed", "err", err)
		}
	}
	return nil
}

func (r *Runtime) recreateSessionFrames(id state.SessionID, sess state.Session, size paneSize) error {
	for _, frame := range sess.Frames {
		if err := r.spawnFrameWindow(id, frame, size); err != nil {
			return err
		}
	}
	return nil
}

// spawnFrameWindow prepares and spawns a single frame's tmux window during cold start.
// It runs PrepareLaunch → WrapLaunch → SpawnWindow, registers the cleanup callback,
// and records the pane ID in session env.
func (r *Runtime) spawnFrameWindow(id state.SessionID, frame state.SessionFrame, size paneSize) error {
	drv := state.GetDriver(frame.Command)
	if drv == nil {
		return nil
	}
	sandboxed := r.state.SandboxedProject != nil && r.state.SandboxedProject(frame.Project)
	launch, err := drv.PrepareLaunch(frame.Driver, state.LaunchModeColdStart, frame.Project, frame.Command, frame.LaunchOptions, sandboxed)
	if err != nil {
		slog.Error("bootstrap: prepare launch failed", "id", id, "frame", frame.ID, "err", err)
		return err
	}
	launch.Project = frame.Project

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
	tmuxCmd := "exec " + wrapped.Command
	if isShellCommand(wrapped.Command) {
		tmuxCmd = ""
	}
	slog.Info("runtime: spawning window", "frame", frameID, "cmd", tmuxCmd, "mode", "coldstart")
	target, paneID, err := r.cfg.Tmux.SpawnWindow(name, tmuxCmd, wrapped.StartDir, wrapped.Env)
	if err != nil {
		return "", err
	}
	r.resizeWindowToMain(r.cfg.SessionName+":"+target, size)
	return paneID, nil
}
