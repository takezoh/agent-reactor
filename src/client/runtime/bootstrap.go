package runtime

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/client/uiproc"
)

// Bootstrap helpers used at startup before the event loop starts.
// These mutate r.state and r.sessionPanes directly — safe because no
// goroutine is reading state yet.

// LoadSnapshot reads sessions.json and registers each session in
// r.state. Driver state is restored via the registered Driver's
// Restore method.
//
// On cold start the tmux session is brand new — any dead panes from
// the previous daemon run have evaporated along with the old server.
// Frames whose driver was at status=stopped have nothing left to
// inspect (the tail output lived in the dead pane, which is gone)
// and cannot be respawned without overwriting the very state the
// user wanted to keep. Those frames are dropped here, and snapshots
// whose every frame was stopped are removed from disk so they do
// not show up again on the next cold start.
//
// Warm start keeps everything: stopped frames in a live tmux session
// still have their dead panes attached for inspection.
func (r *Runtime) LoadSnapshot(coldStart bool) error {
	snaps, err := r.cfg.Persist.Load()
	if err != nil {
		return err
	}
	now := time.Now()
	dropped := 0
	for _, snap := range snaps {
		sess, ok := restoreSession(snap, coldStart, now)
		if !ok {
			if coldStart {
				dropped++
				if err := r.cfg.Persist.Delete(snap.ID); err != nil {
					slog.Warn("bootstrap: drop unrecoverable snapshot failed", "id", snap.ID, "err", err)
				}
			}
			continue
		}
		r.state.Sessions[sess.ID] = sess
	}
	slog.Info("bootstrap: snapshot loaded", "count", len(snaps)-dropped, "dropped_stopped", dropped)
	return nil
}

func restoreSession(snap SessionSnapshot, coldStart bool, now time.Time) (state.Session, bool) {
	createdAt, _ := time.Parse(time.RFC3339, snap.CreatedAt)
	if createdAt.IsZero() {
		createdAt = now
	}
	sess := state.Session{ID: state.SessionID(snap.ID), Project: snap.Project, CreatedAt: createdAt, Sandbox: snap.Sandbox}
	for _, fsnap := range snap.Frames {
		drv := state.GetDriver(fsnap.Command)
		if drv == nil {
			slog.Warn("bootstrap: no driver for command, skipping frame", "command", fsnap.Command)
			break
		}
		// Cold start has no live tmux pane to inherit the dead command's
		// tail output, so a stopped frame turns into a zombie (no pane
		// to display, no window to close). Drop it here instead. The
		// frame's purpose — running a command — is gone, and the pane
		// inspection state died with the old tmux server.
		if coldStart && fsnap.DriverState["status"] == "stopped" {
			continue
		}
		if coldStart && fsnap.DriverState["status"] == "running" {
			fsnap.DriverState["status"] = "waiting"
		}
		frameCreatedAt, _ := time.Parse(time.RFC3339, fsnap.CreatedAt)
		if frameCreatedAt.IsZero() {
			frameCreatedAt = createdAt
		}
		subsystemID := fsnap.SubsystemID
		if subsystemID == "" {
			subsystemID = fsnap.ID
		}
		targetID := fsnap.TargetID
		if targetID == "" {
			targetID = fsnap.ID
		}
		sess.Frames = append(sess.Frames, state.SessionFrame{
			ID:            state.FrameID(fsnap.ID),
			SubsystemID:   state.SubsystemID(subsystemID),
			TargetID:      state.TargetID(targetID),
			Project:       fsnap.Project,
			Command:       fsnap.Command,
			LaunchOptions: fsnap.LaunchOptions,
			CreatedAt:     frameCreatedAt,
			Driver:        drv.Restore(fsnap.DriverState, now),
		})
	}
	if len(sess.Frames) == 0 {
		return state.Session{}, false
	}
	sess.Command = sess.Frames[0].Command
	sess.LaunchOptions = sess.Frames[0].LaunchOptions
	sess.Driver = sess.Frames[0].Driver
	if snap.ActiveFrameID != "" {
		sess.ActiveFrameID = state.FrameID(snap.ActiveFrameID)
	} else {
		sess.ActiveFrameID = sess.Frames[len(sess.Frames)-1].ID
	}
	mru := make([]state.FrameID, 0, len(snap.MRUFrameIDs))
	for _, id := range snap.MRUFrameIDs {
		mru = append(mru, state.FrameID(id))
	}
	sess.MRUFrameIDs = mru
	return sess, true
}

// LoadSessionPanes reads the ROOST_FRAME_* tmux session environment
// variables and populates r.sessionPanes. Called on warm start after
// LoadSnapshot.
func (r *Runtime) LoadSessionPanes() error {
	type envLister interface {
		ShowEnvironment() (string, error)
	}
	el, ok := r.cfg.Tmux.(envLister)
	if !ok {
		return nil
	}
	out, err := el.ShowEnvironment()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == "ROOST_HIDDEN_PANE" {
			r.sessionPanes["_log"] = parts[1]
			continue
		}
		if !strings.HasPrefix(parts[0], "ROOST_FRAME_") {
			continue
		}
		frameID := state.FrameID(strings.TrimPrefix(parts[0], "ROOST_FRAME_"))
		r.sessionPanes[frameID] = parts[1]
	}
	return nil
}

// ReconcileOrphans compares the loaded sessionPanes against the snapshot
// sessions, drops orphan sessions (in JSON but not in sessionPanes) and
// cleans up stale env entries (in windowMap but not in JSON).
func (r *Runtime) ReconcileOrphans() {
	for id, sess := range r.state.Sessions {
		cut := len(sess.Frames)
		for i, frame := range sess.Frames {
			if _, ok := r.sessionPanes[frame.ID]; !ok {
				cut = i
				break
			}
		}
		if cut == len(sess.Frames) {
			continue
		}
		if cut == 0 {
			slog.Warn("bootstrap: dropping orphan session (missing root frame)", "id", id)
			delete(r.state.Sessions, id)
			continue
		}
		sess.Frames = append([]state.SessionFrame(nil), sess.Frames[:cut]...)
		r.state.Sessions[id] = sess
	}

	// Find sessionPanes entries without a matching frame (stale env).
	for frameID := range r.sessionPanes {
		if frameID == "_main" {
			continue
		}
		found := false
		for _, sess := range r.state.Sessions {
			for _, frame := range sess.Frames {
				if frame.ID == frameID {
					found = true
					break
				}
			}
		}
		if !found {
			delete(r.sessionPanes, frameID)
			slog.Warn("bootstrap: removing stale pane env", "frame", frameID)
			_ = r.cfg.Tmux.UnsetEnv(sessionPaneEnvKey(frameID))
		}
	}

	if len(r.state.Sessions) > 0 {
		if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
			slog.Error("bootstrap: persist after reconcile failed", "err", err)
		}
	}
}

// RecoverActivePaneAtMain restores a consistent main-pane owner on warm start.
func (r *Runtime) RecoverActivePaneAtMain() {
	paneAtZero, err := r.cfg.Tmux.PaneID(r.mainPaneTarget())
	if err != nil {
		slog.Debug("bootstrap: could not get pane id at 0.0", "err", err)
		return
	}
	if paneAtZero == "" {
		return
	}
	// Detect if the log TUI ended up at 0.1 (roost crashed while log was visible).
	if r.sessionPanes["_log"] != "" && r.sessionPanes["_log"] == paneAtZero {
		r.state.ActiveOccupant = state.OccupantLog
		slog.Info("bootstrap: log TUI detected at 0.1; setting ActiveOccupant=log", "pane", paneAtZero)
		return
	}
	// Otherwise 0.1 holds main TUI or a frame — normalize occupant from
	// physical pane state rather than relying on stale snapshot value.
	r.state.ActiveOccupant = state.OccupantMain

	var owner state.FrameID
	for id, paneID := range r.sessionPanes {
		if id == "_main" || id == "_log" || paneID != paneAtZero {
			continue
		}
		owner = id
		break
	}
	if owner == "" {
		if r.sessionPanes["_main"] != paneAtZero {
			r.sessionPanes["_main"] = paneAtZero
			_ = r.cfg.Tmux.SetEnv("ROOST_FRAME__main", paneAtZero)
		}
		r.mainPaneSession = ""
		slog.Info("bootstrap: main TUI active at 0.1", "pane", paneAtZero)
		return
	}
	if r.sessionPanes["_main"] == "" {
		r.activeFrameID = owner
		for sid, sess := range r.state.Sessions {
			for _, frame := range sess.Frames {
				if frame.ID == owner {
					r.mainPaneSession = sid
				}
			}
		}
		slog.Warn("bootstrap: main pane id missing; leaving active frame in place", "frame", owner, "pane", paneAtZero)
		return
	}
	r.activeFrameID = owner
	for sid, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			if frame.ID == owner {
				r.mainPaneSession = sid
			}
		}
	}
	slog.Info("bootstrap: session left active at 0.0; restoring main TUI", "frame", owner, "pane", paneAtZero, "main_pane", r.sessionPanes["_main"])
	if !r.swapMainIntoMain() {
		slog.Warn("bootstrap: failed to restore main TUI at 0.0", "session", owner)
		return
	}
	r.mainPaneSession = ""
}

func (r *Runtime) RecoverWarmStartSessions() {
	now := time.Now()
	changed := false
	for sessID, sess := range r.state.Sessions {
		for i, frame := range sess.Frames {
			drv := state.GetDriver(frame.Command)
			if drv == nil {
				continue
			}
			recoverer, ok := drv.(state.WarmStartRecoverer)
			if !ok {
				continue
			}
			next, effs := recoverer.WarmStartRecover(frame.Driver, now)
			sess.Frames[i].Driver = next
			r.state.Sessions[sessID] = sess
			for _, eff := range effs {
				r.execute(r.bootstrapSessionEffect(sessID, frame.ID, now, eff))
			}
			changed = true
		}
	}
	if changed {
		if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
			slog.Error("bootstrap: persist after warm start recovery failed", "err", err)
		}
	}
}

func (r *Runtime) bootstrapSessionEffect(sessID state.SessionID, frameID state.FrameID, now time.Time, eff state.Effect) state.Effect {
	switch e := eff.(type) {
	case state.EffStartJob:
		r.state.NextJobID++
		jobID := r.state.NextJobID
		r.state.Jobs[jobID] = state.JobMeta{
			SessionID: sessID,
			FrameID:   frameID,
			StartedAt: now,
		}
		e.JobID = jobID
		return e
	case state.EffEventLogAppend:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	case state.EffWatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	case state.EffUnwatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e
	default:
		return eff
	}
}

// deactivateBeforeExit ensures pane 0.1 shows the main TUI when the
// coordinator re-attaches. Handles both an active session frame and the
// log TUI occupant. Called from the event loop's defer stack in Run.
func (r *Runtime) deactivateBeforeExit() {
	if r.mainPaneSession != "" {
		r.deactivateSession()
		slog.Info("bootstrap: deactivated session before exit")
		return
	}
	if r.state.ActiveOccupant == state.OccupantLog {
		r.swapHidden()
		r.state.ActiveOccupant = state.OccupantMain
		slog.Info("bootstrap: swapped log TUI back to hidden before exit")
	}
}

// RecoverSandboxFrames re-registers all surviving frames with the sandbox
// backend during warm start. It calls AdoptFrame for each frame that has a
// registered pane so the backend (e.g. Docker) can reclaim its container and
// register a Cleanup callback for when that frame is later killed.
// Must be called after ReconcileOrphans so only live frames are processed.
func (r *Runtime) RecoverSandboxFrames() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	l := launcher(r.cfg)

	liveFrames := r.buildLiveFrameSet()
	r.recoverWarmTokens(liveFrames)

	for _, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			if _, ok := r.sessionPanes[frame.ID]; !ok {
				continue
			}
			cleanup, mounts, err := l.AdoptFrame(ctx, frame.ID, frame.Project)
			if err != nil {
				slog.Warn("bootstrap: sandbox adopt failed", "frame", frame.ID, "err", err)
				continue
			}
			if cleanup != nil {
				r.storeFrameCleanup(frame.ID, cleanup)
			}
			if len(mounts) > 0 {
				r.containerMounts.Store(frame.ID, mounts)
			}
			// Start the container endpoint for sandboxed frames so hook events
			// can be received immediately after daemon warm restart.
			if r.state.SandboxedProject != nil && r.state.SandboxedProject(frame.Project) && r.cfg.DataDir != "" {
				runDirKey := frame.Project
				if dl := devcontainerLauncherFor(l); dl != nil {
					runDirKey = dl.RunDirKey(frame.Project)
				}
				runDir := ProjectRunDir(filepath.Join(r.cfg.DataDir, "run"), runDirKey)
				r.startContainerEndpointIfNeeded(frame.Project, ContainerSockPath(runDir))
			}
		}
	}
}

func (r *Runtime) buildLiveFrameSet() map[string]struct{} {
	live := make(map[string]struct{})
	for _, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			if _, ok := r.sessionPanes[frame.ID]; ok {
				live[string(frame.ID)] = struct{}{}
			}
		}
	}
	return live
}

// recoverWarmTokens restores container tokens from warm/ so container agents
// can still send hook events after a daemon warm restart.
func (r *Runtime) recoverWarmTokens(liveFrames map[string]struct{}) {
	if r.warmFrames == nil {
		return
	}
	states, err := r.warmFrames.LoadAll()
	if err != nil {
		slog.Warn("bootstrap: warm frame load failed", "err", err)
	}
	for _, st := range states {
		if _, ok := liveFrames[st.FrameID]; !ok {
			// orphan: no live frame — delete proactively so warm restarts don't accumulate stale files
			_ = r.warmFrames.Delete(state.FrameID(st.FrameID))
			continue
		}
		if st.ContainerToken != "" {
			r.containerTokens.Register(state.FrameID(st.FrameID), st.ContainerToken)
		}
	}
}

// SetAliases sets the command alias map on state. Called once at
// startup from main.go with the config's [session] aliases.
func (r *Runtime) SetAliases(aliases map[string]string) {
	r.state.Aliases = aliases
}

// SetSandboxedProjectResolver installs a function that returns true when a
// project path runs inside a sandbox. Called once at startup. Mirrors the
// SetAliases pattern — generic signal, no sandbox-specific types in state.
func (r *Runtime) SetSandboxedProjectResolver(fn func(string) bool) {
	r.state.SandboxedProject = fn
}

// SetDefaultCommand sets the fallback command for sessions created
// without an explicit command. Called once at startup from main.go.
func (r *Runtime) SetDefaultCommand(cmd string) {
	r.state.DefaultCommand = cmd
}

// SetSyncCallbacks installs the optional tmux sync callbacks.
// Kept as a stable hook for future use.
func (r *Runtime) SetSyncCallbacks(active, status func(string)) {
	// Reserved.
}

// RespawnMainPane runs respawn-pane for the main TUI.
func (r *Runtime) RespawnMainPane() {
	target := r.sessionPanes["_main"]
	if target == "" {
		target = r.mainPaneTarget()
	}

	// Double check to protect active sessions if mapping failed
	if target == r.mainPaneTarget() && r.mainPaneSession != "" {
		slog.Warn("bootstrap: skipping main TUI respawn to protect active session at 0.0")
		return
	}

	slog.Info("bootstrap: respawning main TUI", "target", target)
	_ = r.cfg.Tmux.RespawnPane(target, uiproc.Main().Command(r.cfg.RoostExe))
}
