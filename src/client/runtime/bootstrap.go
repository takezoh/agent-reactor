package runtime

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

// Bootstrap helpers used at startup before the event loop starts.
// These mutate r.state directly — safe because no goroutine is reading
// state yet.

// LoadSnapshot reads sessions.json and registers each session in
// r.state. Driver state is restored via the registered Driver's
// Restore method.
//
// On cold start the frame backend is brand new — any dead frames from
// the previous daemon run have evaporated along with the old server.
// Frames whose driver was at status=stopped have nothing left to
// inspect (the tail output lived in the dead frame, which is gone)
// and cannot be respawned without overwriting the very state the
// user wanted to keep. Those frames are dropped here, and snapshots
// whose every frame was stopped are removed from disk so they do
// not show up again on the next cold start. The exception is a driver
// whose durable state outlives the frame (a ColdStartRecoverer such as
// codex, which resumes its thread): its stopped frames are kept and
// relaunched.
//
// Warm start keeps everything: stopped frames in a live frame backend
// still have their dead pty sessions attached for inspection.
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
		if coldStart && fsnap.DriverState["status"] == "running" {
			fsnap.DriverState["status"] = "waiting"
		}
		driverState := drv.Restore(fsnap.DriverState, now)
		// Cold start has no live frame to inherit the dead command's
		// tail output, so a stopped frame turns into a zombie (no
		// surface to display, no session to close). Drop it here instead
		// — unless the driver's durable state survives the frame (codex
		// resumes its thread against a fresh app-server), in which case
		// it is kept and relaunched by the cold-start spawn path.
		if coldStart && drv.Status(driverState) == state.StatusStopped && !coldStartRecoverable(drv, driverState) {
			continue
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
			Driver:        driverState,
		})
	}
	if len(sess.Frames) == 0 {
		return state.Session{}, false
	}
	sess.Command = sess.Frames[0].Command
	sess.LaunchOptions = sess.Frames[0].LaunchOptions
	sess.Driver = sess.Frames[0].Driver
	if snap.HeadFrameID != "" {
		sess.HeadFrameID = state.FrameID(snap.HeadFrameID)
	} else {
		sess.HeadFrameID = sess.Frames[len(sess.Frames)-1].ID
	}
	mru := make([]state.FrameID, 0, len(snap.MRUFrameIDs))
	for _, id := range snap.MRUFrameIDs {
		mru = append(mru, state.FrameID(id))
	}
	sess.MRUFrameIDs = mru
	return sess, true
}

// Warm-start layout recovery was removed alongside the TUI: PtyBackend pairs
// one pty session per frame so the main-frame-owner concept no longer exists,
// and cross-daemon-restart frame recovery is out of scope per ADR 0004
// decision 2 (daemon and frames share a lifetime).

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

// Shutdown layout repair was removed alongside the TUI: daemon shutdown now
// closes the IPC socket and tears down frames via PtyBackend without any
// layout fix-up.

// RecoverSandboxFrames tries to re-attach each persisted frame to its sandbox
// (e.g. Docker container) before the cold-start spawn path runs. PtyBackend
// terminal sessions die with the daemon, but devcontainer containers and
// codex remote threads outlive a daemon restart — adopting them here lets the
// subsequent fresh frame re-attach to the same running agent instead of losing
// the container's working state.
//
// Returns the set of frame IDs that were successfully adopted so the cold-start
// path can decide which frames need a fresh container vs an attach. Frames for
// which AdoptFrame fails (no surviving container, dispatcher refuses) are left
// alone — RecreateAll will spawn a fresh container for them via the normal
// EnsureProject path.
//
// Per-frame ordering matters (issues/029, F6): each frame's warm token + mounts
// are registered atomically via RegisterWithMounts and only THEN is the
// container endpoint started. Previously recoverWarmTokens registered every
// token first (token-only, no mounts) and per-frame StoreMounts followed in
// the same loop body — this opened a window where the same-project endpoint
// could already be live (from a sibling frame's iteration) while a later
// frame's token was looked-up-able but its mounts were not yet stored, causing
// container-relative paths to leak unconverted to the wire.
func (r *Runtime) RecoverSandboxFrames(ctx context.Context) map[state.FrameID]struct{} {
	l := launcher(r.cfg)
	warmTokens := r.recoverWarmTokens()

	adopted := make(map[state.FrameID]struct{})
	for _, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			cleanup, mounts, err := l.AdoptFrame(ctx, frame.ID, frame.Project)
			if err != nil {
				slog.Info("bootstrap: sandbox adopt skipped (no survivor)",
					"frame", frame.ID, "project", frame.Project, "err", err)
				continue
			}
			adopted[frame.ID] = struct{}{}
			if cleanup != nil {
				r.storeFrameCleanup(frame.ID, cleanup)
			}
			// Register token + mounts in one atomic step so a same-project hook
			// arriving on the already-live endpoint never observes a half-set
			// (token without mounts). Tokens with no mounts and mounts with no
			// token both stay valid — RegisterWithMounts handles both shapes.
			if tok := warmTokens[frame.ID]; tok != "" {
				r.frameReg.RegisterWithMounts(frame.ID, tok, mounts)
			} else if len(mounts) > 0 {
				r.frameReg.StoreMounts(frame.ID, mounts)
			}
			if r.state.SandboxedProject != nil && r.state.SandboxedProject(frame.Project) && r.cfg.DataDir != "" {
				runDirKey := frame.Project
				if dl := devcontainerLauncherFor(l); dl != nil {
					runDirKey = dl.RunDirKey(frame.Project)
				}
				runDir := ProjectRunDir(filepath.Join(r.cfg.DataDir, "run"), runDirKey)
				r.startContainerEndpointIfNeeded(frame.Project, ContainerSockPath(runDir))
			}
			slog.Info("bootstrap: sandbox frame adopted",
				"frame", frame.ID, "project", frame.Project, "mounts", len(mounts))
		}
	}
	return adopted
}

// recoverWarmTokens reads warm/ and returns a per-frame map of saved container
// tokens for frames still present in r.state. Stale warm entries (frames no
// longer in the snapshot) are deleted proactively. Returns an empty map when
// no warm store is configured, so callers can iterate unconditionally.
//
// Does NOT touch frameReg — the caller stages tokens into the per-frame
// AdoptFrame loop and uses RegisterWithMounts so token + mounts land behind
// the same lock (issues/029, F6).
func (r *Runtime) recoverWarmTokens() map[state.FrameID]string {
	out := map[state.FrameID]string{}
	if r.warmFrames == nil {
		return out
	}
	liveFrames := make(map[string]struct{})
	for _, sess := range r.state.Sessions {
		for _, frame := range sess.Frames {
			liveFrames[string(frame.ID)] = struct{}{}
		}
	}
	states, err := r.warmFrames.LoadAll()
	if err != nil {
		slog.Warn("bootstrap: warm frame load failed", "err", err)
	}
	for _, st := range states {
		if _, ok := liveFrames[st.FrameID]; !ok {
			_ = r.warmFrames.Delete(state.FrameID(st.FrameID))
			continue
		}
		if st.ContainerToken != "" {
			out[state.FrameID(st.FrameID)] = st.ContainerToken
		}
	}
	return out
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

// SetSyncCallbacks installs the optional frame backend sync callbacks.
// Kept as a stable hook for future use.
func (r *Runtime) SetSyncCallbacks(active, status func(string)) {
	// Reserved.
}
