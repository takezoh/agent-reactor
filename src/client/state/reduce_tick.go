package state

// Tick reducer. Fans the tick out to every session's driver and emits the
// periodic frame liveness reconcile.

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

	// Frame liveness reconcile: every 5 ticks the runtime walks r.sessionPanes
	// and emits EvFrameCommandExited / EvPaneWindowVanished per dead frame.
	// reconcileWindows is the sole frame-death detection path now — the legacy
	// per-tick EffCheckPaneAlive emissions targeted tmux control panes
	// (0.0/0.1/0.2/__hidden__.0) that no longer exist under PtyBackend.
	if e.N%5 == 0 {
		effs = append(effs, EffReconcileWindows{})
	}

	if changed {
		effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	}
	return s, effs
}

// reducePaneWindowVanished evicts a session whose backend window has
// disappeared (agent process exited) and broadcasts the new list.
func reducePaneWindowVanished(s State, e EvPaneWindowVanished) (State, []Effect) {
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
// standard termination signal) trigger full eviction — the dead backend
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
// the legacy web server detected dead panes only via reconcileWindows.
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
