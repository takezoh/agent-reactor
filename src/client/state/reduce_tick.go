package state

import (
	"log/slog"

	"github.com/takezoh/agent-reactor/client/uiproc"
)

// Tick reducer. Fans the tick out to every session's driver and
// emits periodic reconciliation + health-check effects.

func reduceTick(s State, e EvTick) (State, []Effect) {
	s.Now = e.Now

	var seq uint64
	s, effs, changed := stepActiveSessions(s, func(sessID SessionID, sess Session, active bool) DriverEvent {
		frame, _ := rootFrame(sess)
		ev := DEvTick{
			Now:        e.Now,
			Active:     active,
			Project:    frame.Project,
			PaneTarget: e.PaneTargets[frame.ID],
			N:          e.N,
			Seq:        seq,
		}
		seq++
		return ev
	})

	// Initialize connectors (once).
	if !s.ConnectorsReady && len(AllConnectors()) > 0 {
		s.ConnectorsReady = true
		s.Connectors = cloneConnectors(s.Connectors)
		for _, c := range AllConnectors() {
			s.Connectors[c.Name()] = c.NewState()
		}
	}

	// Step all connectors.
	s, connEffs := stepConnectors(s)
	effs = append(effs, connEffs...)
	if len(connEffs) > 0 {
		changed = true
	}

	// Active-pane death check: every tick (covers the no-active-frame case
	// where the fast ticker skips; fast ticker handles the active-frame case).
	effs = append(effs, EffCheckPaneAlive{Pane: "{sessionName}:0.1"})

	// Control-pane health and window reconcile: every 5 ticks to reduce
	// subprocess pressure. These are non-latency-sensitive: a respawn
	// triggered 5 s late is indistinguishable from 1 s late for the user.
	if e.N%5 == 0 {
		effs = append(effs,
			EffCheckPaneAlive{Pane: "{sessionName}:0.0"},
			EffCheckPaneAlive{Pane: "{sessionName}:0.2"},
			EffCheckPaneAlive{Pane: "{sessionName}:__hidden__.0"},
			EffReconcileWindows{},
		)
	}

	if changed {
		effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	}
	return s, effs
}

func stepConnectors(s State) (State, []Effect) {
	var effs []Effect
	for _, c := range AllConnectors() {
		next, cEffs, ok := stepConnector(s, c.Name(), CEvTick{Now: s.Now})
		if !ok {
			continue
		}
		s = next
		effs = append(effs, cEffs...)
	}
	return s, effs
}

// reducePaneDied handles a dead pane detected by EffCheckPaneAlive.
//   - Pane __hidden__.0 (log TUI): respawn log TUI in the hidden window
//   - Panes 0.0/0.2 (header/sessions): always respawn via RespawnTarget
//   - Pane 0.1 with no active session: main or log TUI crashed — respawn it
//   - Pane 0.1 with active session: evict the owning session
func reducePaneDied(s State, e EvPaneDied) (State, []Effect) {
	slog.Info("state: reducePaneDied entry",
		"pane", e.Pane, "owner", e.OwnerFrameID,
		"occupant", s.ActiveOccupant, "activeSession", s.ActiveSession)

	// Hidden pane (log TUI): log TUI process crashed — respawn it in place.
	if e.Pane == "{sessionName}:__hidden__.0" {
		slog.Info("state: reducePaneDied branch=hidden-log")
		return s, []Effect{EffRespawnPane{Pane: e.Pane, Proc: uiproc.Log()}}
	}

	// Control pane respawn
	if proc, ok := uiproc.RespawnTarget(e.Pane); ok {
		slog.Info("state: reducePaneDied branch=control-respawn", "proc", proc.Name)
		return s, []Effect{
			EffRespawnPane{Pane: e.Pane, Proc: proc},
		}
	}

	// Pane 0.1 dead with no active frame: main or log TUI crashed.
	if e.Pane == "{sessionName}:0.1" && s.ActiveOccupant != OccupantFrame {
		proc := uiproc.Main()
		if s.ActiveOccupant == OccupantLog {
			proc = uiproc.Log()
		}
		slog.Info("state: reducePaneDied branch=no-active-frame-respawn",
			"proc", proc.Name, "occupant", s.ActiveOccupant)
		return s, []Effect{
			EffRespawnPane{Pane: e.Pane, Proc: proc},
		}
	}

	// Pane 0.1 dead with active session: evict the owning session.
	// OwnerFrameID is set by the runtime; fall back to ActiveSession
	// if the runtime couldn't identify the owner via pane_id.
	ownerID := e.OwnerFrameID
	if ownerID == "" {
		if sess, ok := s.Sessions[s.ActiveSession]; ok {
			if frame, ok := activeFrame(sess); ok {
				ownerID = frame.ID
				slog.Info("state: reducePaneDied owner fallback via ActiveSession",
					"owner", ownerID)
			}
		}
	}
	if ownerID == "" {
		slog.Info("state: reducePaneDied bail=no-owner")
		return s, nil
	}
	s, effs, ok := evictFrame(s, ownerID, true)
	if !ok {
		slog.Info("state: reducePaneDied evictFrame returned !ok", "owner", ownerID)
		return s, nil
	}
	slog.Info("state: reducePaneDied evictFrame ok", "owner", ownerID, "neffs", len(effs))
	return s, effs
}

// reduceTmuxWindowVanished evicts a session whose tmux window has
// disappeared (agent process exited) and broadcasts the new list.
// If the vanished session was active, deactivation restores the main TUI.
func reduceTmuxWindowVanished(s State, e EvTmuxWindowVanished) (State, []Effect) {
	s, effs, ok := evictFrame(s, e.FrameID, false)
	if !ok {
		return s, nil
	}
	return s, effs
}

// isIntentionalExit returns true when an exit code looks like a
// user-driven termination (clean exit or kill via a standard signal)
// rather than a genuine crash. Intentional exits evict the frame
// outright so the session list does not accumulate user-terminated
// entries that the next cold start would otherwise restore.
//
// The codes recognised here:
//   - 0:   clean exit (script finished, agent typed /quit cleanly)
//   - 129: SIGHUP  — controlling terminal closed
//   - 130: SIGINT  — Ctrl-C
//   - 137: SIGKILL — explicit `kill -9` / OOM kill
//   - 143: SIGTERM — graceful termination signal
//
// Any other code is treated as an abnormal exit (crash, panic,
// non-zero return from a failing tool) and keeps the frame as
// Stopped so the tail output remains available for inspection.
func isIntentionalExit(code int) bool {
	switch code {
	case 0, 129, 130, 137, 143:
		return true
	}
	return false
}

// reduceFrameCommandExited routes a command-exit signal based on its
// exit code. Codes recognised by isIntentionalExit (clean exit or
// standard termination signal) trigger full eviction — the dead tmux
// pane is also closed via EffKillSessionWindow. Other codes are
// treated as crashes: the frame is kept in state with driver
// status=Stopped so the user can still find it in the session list,
// and the dead pane is left attached so the tail output (stack trace,
// error message) remains visible.
//
// Intentional eviction runs first because the driver may have already
// transitioned to StatusStopped via its own hook stream (e.g. claude's
// SessionEnd hook sets status=stopped before the pty actually closes).
// If the idempotency guard below ran first, a hook-driven Stopped state
// would suppress eviction every subsequent tick and the session would
// stick around forever as "Stopped" — the bug reproduced when the
// tmux-free web server detected dead panes only via reconcileWindows.
//
// The remaining idempotency check protects the crash path:
// reconcileWindows may re-detect the same dead pane on subsequent
// ticks, and once stepDriver has already advanced the driver to
// StatusStopped we must not re-emit further effects.
func reduceFrameCommandExited(s State, e EvFrameCommandExited) (State, []Effect) {
	_, sess, idx, ok := findFrame(s, e.FrameID)
	if !ok {
		return s, nil
	}
	if isIntentionalExit(e.ExitCode) {
		next, effs, _ := evictFrame(s, e.FrameID, true)
		return next, effs
	}

	frame := sess.Frames[idx]
	drv := GetDriver(frame.Command)
	if drv != nil && frame.Driver != nil && drv.Status(frame.Driver) == StatusStopped {
		return s, nil
	}

	next, rawEffs, _ := stepDriver(s, e.FrameID, DEvCommandExited{
		ExitCode:  e.ExitCode,
		Timestamp: s.Now,
	})
	return next, rawEffs
}
