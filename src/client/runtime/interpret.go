package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/worker"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/client/uiproc"
)

// execute is the side-effect interpreter. Each Effect type has a
// dedicated case that performs the I/O on the appropriate backend.
// Effects that produce events back into the loop (pane spawn, pane
// alive, etc.) call r.Enqueue, which is non-blocking and goroutine-
// safe so the case can fire from inside the event loop without
// risking deadlock on the channel.
func (r *Runtime) execute(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffSpawnPaneWindow, state.EffKillSessionWindow, state.EffActivateSession,
		state.EffDeactivateSession, state.EffRegisterPane, state.EffUnregisterPane,
		state.EffSelectPane, state.EffSyncStatusLine, state.EffSetPaneEnv,
		state.EffUnsetPaneEnv, state.EffCheckPaneAlive, state.EffRespawnPane,
		state.EffSwapHidden, state.EffDetachClient, state.EffDisplayPopup,
		state.EffKillSession, state.EffReconcileWindows:
		r.executePaneEffect(e)

	case state.EffSendResponse, state.EffSendResponseSync, state.EffSendError,
		state.EffBroadcastSessionsChanged, state.EffBroadcastEvent, state.EffCloseConn:
		r.executeIPCEffect(e)

	case state.EffWatchFile, state.EffUnwatchFile:
		r.executeFSEffect(e)

	case state.EffSendPaneKeys:
		r.executeSendPaneKeys(e)

	case state.EffInjectPrompt:
		r.executeInjectPrompt(e)

	case state.EffSurfaceSubscribeStart, state.EffSurfaceSubscribeStop,
		state.EffSurfaceResize, state.EffSurfaceWriteRaw,
		state.EffBroadcastSurfaceOutput, state.EffBroadcastPromptEvent:
		r.executeSurfaceEffect(e)

	default:
		r.executeMiscEffect(eff)
	}
}

// executeMiscEffect handles effects that don't fit the pane/IPC/FS categories.
func (r *Runtime) executeMiscEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffPersistSnapshot:
		if err := r.cfg.Persist.Save(r.snapshotSessions()); err != nil {
			slog.Error("runtime: persist failed", "err", err)
		}

	case state.EffEventLogAppend:
		if err := r.cfg.EventLog.Append(e.FrameID, e.Line); err != nil {
			slog.Debug("runtime: event log append failed", "frame", e.FrameID, "err", err)
		}

	case state.EffToolLogAppend:
		if err := r.cfg.ToolLog.Append(e.Namespace, e.Project, e.Line); err != nil {
			slog.Debug("runtime: tool log append failed",
				"namespace", e.Namespace, "project", e.Project, "err", err)
		}

	case state.EffStartJob:
		r.submitJob(e)

	case state.EffNotify:
		r.cfg.Notifier.Dispatch(e)

	case state.EffRecordNotification:
		r.executeRecordNotification(e)

	case state.EffReleaseFrameSandboxes:
		ctx := context.Background()
		for _, sub := range r.subsystems {
			sub.Stop(ctx)
		}
		// Drain sandbox (container/VM) cleanup closures in parallel.
		r.drainFrameCleanups()

	default:
		slog.Warn("runtime: unhandled effect type", "type", fmt.Sprintf("%T", eff))
	}
}

func (r *Runtime) executeRecordNotification(e state.EffRecordNotification) {
	r.broadcastAgentNotification(e)
	source := fmt.Sprintf("osc%d", e.Cmd)
	if err := r.cfg.EventLog.Append(e.FrameID, oscEventLogLine(source, e.Title, e.Body)); err != nil {
		slog.Debug("runtime: osc event log failed", "frame", e.FrameID, "err", err)
	}
	r.cfg.Notifier.DispatchOSC(e.Title, e.Body, source)
}

func (r *Runtime) executeSendPaneKeys(e state.EffSendPaneKeys) {
	pane := r.sessionPaneForSession(e.SessionID)
	if pane == "" {
		r.executeIPCEffect(state.EffSendError{
			ConnID:  e.ConnID,
			ReqID:   e.ReqID,
			Code:    "not_found",
			Message: "no pane registered for session: " + string(e.SessionID),
		})
		return
	}
	var err error
	if e.WithEnter {
		err = r.cfg.Backend.SendKeys(pane, e.Text)
	} else {
		err = r.cfg.Backend.SendKey(pane, e.Key)
	}
	if err != nil {
		slog.Warn("runtime: send-keys failed", "session", e.SessionID, "err", err)
		r.executeIPCEffect(state.EffSendError{
			ConnID:  e.ConnID,
			ReqID:   e.ReqID,
			Code:    "internal",
			Message: err.Error(),
		})
		return
	}
	r.executeIPCEffect(state.EffSendResponse{ConnID: e.ConnID, ReqID: e.ReqID, Body: nil})
}

func (r *Runtime) executeInjectPrompt(e state.EffInjectPrompt) {
	inj := NewRuntimePaneInjector(r.sessionPanes, r.cfg.Backend)
	if err := InjectPrompt(inj, e.FrameID, e.Text); err != nil {
		slog.Warn("runtime: inject prompt failed", "frame", e.FrameID, "err", err)
	}
}

// executeSurfaceEffect dispatches the six surface-streaming effects to
// TerminalRelay or proto_bridge helpers.
func (r *Runtime) executeSurfaceEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffSurfaceSubscribeStart:
		if r.terminalRelay == nil {
			return
		}
		paneID := r.sessionPaneForSession(e.SessionID)
		if paneID == "" {
			slog.Warn("runtime: surface subscribe: no pane for session",
				"session", e.SessionID, "conn", e.ConnID)
			return
		}
		if err := r.terminalRelay.Subscribe(e.ConnID, e.SessionID, paneID); err != nil {
			slog.Warn("runtime: surface subscribe failed",
				"session", e.SessionID, "conn", e.ConnID, "err", err)
		}

	case state.EffSurfaceSubscribeStop:
		if r.terminalRelay == nil {
			return
		}
		r.terminalRelay.Unsubscribe(e.ConnID, e.SessionID)

	case state.EffSurfaceResize:
		if r.terminalRelay == nil {
			return
		}
		paneID := r.sessionPaneForSession(e.SessionID)
		if paneID == "" {
			return
		}
		if err := r.terminalRelay.Resize(paneID, int(e.Cols), int(e.Rows)); err != nil {
			slog.Warn("runtime: surface resize failed",
				"session", e.SessionID, "err", err)
		}

	case state.EffSurfaceWriteRaw:
		if r.terminalRelay == nil {
			return
		}
		paneID := r.sessionPaneForSession(e.SessionID)
		if paneID == "" {
			return
		}
		if err := r.terminalRelay.Write(paneID, e.Data); err != nil {
			slog.Warn("runtime: surface write failed",
				"session", e.SessionID, "err", err)
		}

	case state.EffBroadcastSurfaceOutput:
		r.broadcastSurfaceOutput(e)

	case state.EffBroadcastPromptEvent:
		r.broadcastPromptEvent(e)
	}
}

func (r *Runtime) executePaneEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffSpawnPaneWindow:
		go spawnPaneWindow(r.buildSpawnDeps(), e)
	case state.EffKillSessionWindow:
		r.executeKillSessionWindow(e)
	case state.EffActivateSession:
		r.activateSession(e.SessionID)
	case state.EffDeactivateSession:
		r.deactivateSession()
	case state.EffRegisterPane:
		r.executeRegisterPane(e)
	case state.EffUnregisterPane:
		r.executeUnregisterPane(e)
	case state.EffSelectPane:
		target := substitutePlaceholdersString(e.Target, r.cfg.SessionName, r.cfg.RoostExe)
		_ = r.cfg.Backend.SelectPane(target)
	case state.EffSyncStatusLine:
		r.executeSyncStatusLine(e)
	case state.EffSetPaneEnv:
		_ = r.cfg.Backend.SetEnv(e.Key, e.Value)
	case state.EffUnsetPaneEnv:
		_ = r.cfg.Backend.UnsetEnv(e.Key)
	case state.EffCheckPaneAlive:
		r.executeCheckPaneAlive(e)
	case state.EffRespawnPane:
		target := substitutePlaceholdersString(e.Pane, r.cfg.SessionName, r.cfg.RoostExe)
		_ = r.cfg.Backend.RespawnPane(target, e.Proc.Command(r.cfg.RoostExe))
	case state.EffSwapHidden:
		r.swapHidden()
	case state.EffDetachClient:
		_ = r.cfg.Backend.DetachClient()
	case state.EffDisplayPopup:
		_ = r.cfg.Backend.DisplayPopup(e.Width, e.Height, uiproc.Palette(e.Tool, e.Args, "").Command(r.cfg.RoostExe))
	case state.EffKillSession:
		_ = r.cfg.Backend.KillSession()
	case state.EffReconcileWindows:
		r.reconcileWindows()
	}
}

func (r *Runtime) executeKillSessionWindow(e state.EffKillSessionWindow) {
	if target := r.sessionPanes[e.FrameID]; target != "" {
		if tail, err := r.cfg.Backend.CapturePane(target, 20); err == nil && tail != "" {
			slog.Info("runtime: pane tail on kill", "frame", e.FrameID, "target", target, "tail", tail)
		}
		if err := r.cfg.Backend.KillPaneWindow(target); err != nil {
			slog.Error("runtime: kill window failed", "target", target, "err", err)
		}
		delete(r.sessionPanes, e.FrameID)
	}
	if r.warmFrames != nil {
		if err := r.warmFrames.Delete(e.FrameID); err != nil {
			slog.Warn("runtime: warm frame delete failed", "frame", e.FrameID, "err", err)
		}
	}
	// Release subsystem resources (worktree removal, thread cleanup).
	// The container token+mounts are removed by invokeFrameCleanup → frameReg.Delete.
	if sub, ok := r.frameSubsystems[e.FrameID]; ok {
		delete(r.frameSubsystems, e.FrameID)
		sub.ReleaseFrame(e.FrameID)
		r.reapSubsystemIfLast(sub, e.FrameID)
	}
	r.invokeFrameCleanup(e.FrameID)
}

func (r *Runtime) executeRegisterPane(e state.EffRegisterPane) {
	r.sessionPanes[e.FrameID] = e.PaneTarget
	_ = r.cfg.Backend.SetEnv(sessionPaneEnvKey(e.FrameID), e.PaneTarget)
	if e.Tap && r.taps != nil {
		r.taps.start(e.FrameID, e.PaneTarget, r.Enqueue)
	}
}

func (r *Runtime) executeUnregisterPane(e state.EffUnregisterPane) {
	target, ok := r.sessionPanes[e.FrameID]
	if !ok {
		return
	}
	if r.taps != nil {
		r.taps.stop(e.FrameID)
	}
	delete(r.sessionPanes, e.FrameID)
	_ = r.cfg.Backend.UnsetEnv(sessionPaneEnvKey(e.FrameID))
	r.cfg.EventLog.Close(e.FrameID)
	if r.cfg.TerminalEvict != nil {
		r.cfg.TerminalEvict(target)
	}
}

func (r *Runtime) executeSyncStatusLine(e state.EffSyncStatusLine) {
	line := e.Line
	if line == "" {
		line = r.activeStatusLine()
	}
	_ = r.cfg.Backend.SetStatusLine(line)
}

func (r *Runtime) executeCheckPaneAlive(e state.EffCheckPaneAlive) {
	// active frame の 0.1 は positional ではなく pane_id で probe する。
	// remain-on-exit off で frame pane が破棄されると backend が layout を詰めて
	// 別 pane が 0.1 を占めるため、positional クエリは alive を誤検知する。
	// pane_id ならプロセスと共に消えるので、err も dead 扱いにする。
	if e.Pane == "{sessionName}:0.1" && r.activeFrameID != "" {
		if paneID := r.sessionPanes[r.activeFrameID]; paneID != "" {
			alive, err := r.cfg.Backend.PaneAlive(paneID)
			if err != nil && !isMissingPaneErr(err) {
				// A transient probe failure (e.g. "context deadline exceeded"
				// when the backend is slow under load) is NOT death — re-probe on the
				// next tick instead of evicting a live session.
				slog.Warn("runtime: active frame pane probe transient error (ignored)",
					"pane", e.Pane, "target", paneID, "owner", r.activeFrameID, "err", err)
				return
			}
			if err == nil && alive {
				return
			}
			// Genuine death: either err==nil && !alive (dead pane kept by
			// remain-on-exit) or isMissingPaneErr(err) (pane_id vanished).
			ev := state.EvPaneDied{Pane: e.Pane, OwnerFrameID: r.activeFrameID}
			slog.Info("runtime: active frame pane died",
				"pane", e.Pane, "target", paneID, "owner", ev.OwnerFrameID, "err", err)
			r.Enqueue(ev)
			return
		}
	}
	target := substitutePlaceholdersString(e.Pane, r.cfg.SessionName, r.cfg.RoostExe)
	if alive, err := r.cfg.Backend.PaneAlive(target); err == nil && !alive {
		ev := state.EvPaneDied{Pane: e.Pane}
		if e.Pane == "{sessionName}:0.1" {
			ev.OwnerFrameID = r.findPaneOwner(target)
		}
		slog.Info("runtime: pane alive check failed",
			"pane", e.Pane, "target", target, "owner", ev.OwnerFrameID)
		r.Enqueue(ev)
	}
}

func (r *Runtime) executeIPCEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffSendResponse:
		r.sendResponse(e)
	case state.EffSendResponseSync:
		r.sendResponseSync(e)
	case state.EffSendError:
		r.sendError(e)
	case state.EffBroadcastSessionsChanged:
		r.broadcastSessionsChanged(e.IsPreview)
	case state.EffBroadcastEvent:
		r.broadcastGenericEvent(e)
	case state.EffCloseConn:
		r.closeConn(e.ConnID)
	}
}

func (r *Runtime) executeFSEffect(eff state.Effect) {
	switch e := eff.(type) {
	case state.EffWatchFile:
		_ = r.cfg.Watcher.Watch(e.FrameID, e.Path)
		if r.relay != nil {
			r.relay.WatchFile(e.FrameID, e.Path, e.Kind)
		}

	case state.EffUnwatchFile:
		_ = r.cfg.Watcher.Unwatch(e.FrameID)
		if r.relay != nil {
			r.relay.UnwatchFile(e.FrameID)
		}
	}
}

// submitJob dispatches an EffStartJob to the worker pool via the
// global runner registry.
func (r *Runtime) submitJob(e state.EffStartJob) {
	worker.Dispatch(r.workers, e.JobID, e.Input)
}

// oscEventLogLine formats a single EVENTS log line for an OSC notification.
// Format: "[osc9] title" / "[osc99] title | body" / "[osc777] title | body"
func oscEventLogLine(source, title, body string) string {
	if body == "" {
		return fmt.Sprintf("[%s] %s", source, title)
	}
	if title == "" {
		return fmt.Sprintf("[%s] %s", source, body)
	}
	return fmt.Sprintf("[%s] %s | %s", source, title, body)
}

// snapshotSessions converts the current state.Sessions map into the
// on-disk snapshot format. Driver bag is filled by calling each
// driver's Persist method.
func (r *Runtime) snapshotSessions() []SessionSnapshot {
	out := make([]SessionSnapshot, 0, len(r.state.Sessions))
	for _, sess := range r.state.Sessions {
		frames := make([]SessionFrameSnapshot, 0, len(sess.Frames))
		for _, frame := range sess.Frames {
			drv := state.GetDriver(frame.Command)
			var bag map[string]string
			var driverName string
			if drv != nil {
				bag = drv.Persist(frame.Driver)
				driverName = drv.Name()
			}
			// Strip InitialInput before persisting: it is a one-shot spawn
			// parameter and must not be written to sessions.json (would
			// re-pipe stale stdin on cold-start recovery and leak content).
			persistOpts := frame.LaunchOptions
			persistOpts.InitialInput = nil
			frames = append(frames, SessionFrameSnapshot{
				ID:            string(frame.ID),
				SubsystemID:   string(frame.SubsystemID),
				TargetID:      string(frame.TargetID),
				Project:       frame.Project,
				Command:       frame.Command,
				LaunchOptions: persistOpts,
				CreatedAt:     frame.CreatedAt.UTC().Format(time.RFC3339),
				Driver:        driverName,
				DriverState:   bag,
			})
		}
		mruIDs := make([]string, len(sess.MRUFrameIDs))
		for i, id := range sess.MRUFrameIDs {
			mruIDs[i] = string(id)
		}
		out = append(out, SessionSnapshot{
			ID:            string(sess.ID),
			Project:       sess.Project,
			CreatedAt:     sess.CreatedAt.UTC().Format(time.RFC3339),
			Frames:        frames,
			ActiveFrameID: string(sess.ActiveFrameID),
			MRUFrameIDs:   mruIDs,
			Sandbox:       sess.Sandbox,
		})
	}
	return out
}

// activeStatusLine returns the View().StatusLine of whichever session
// is currently shown in pane 0.0. Empty when no session is active
// or no driver matches.
func (r *Runtime) activeStatusLine() string {
	if r.mainPaneSession == "" {
		return ""
	}
	sess, ok := r.state.Sessions[r.mainPaneSession]
	if !ok {
		return ""
	}
	frame, ok := sessionActiveFrame(sess)
	if !ok {
		return ""
	}
	drv := state.GetDriver(frame.Command)
	if drv == nil {
		return ""
	}
	return drv.View(frame.Driver).StatusLine
}

// reconcileWindows checks whether each tracked session pane still
// exists. Two distinct conditions are surfaced:
//
//   - The pane is dead but still around (remain-on-exit=on holds it):
//     the command process exited. Read #{pane_dead_status} and emit
//     EvFrameCommandExited so the reducer can decide between
//     eviction (exit 0) and keeping the frame as stopped (exit != 0).
//
//   - The query for the pane itself failed with a missing-pane style
//     error: the pane window was destroyed externally (user kill-window).
//     Emit EvPaneWindowVanished to evict unconditionally — there is
//     nothing left to inspect.
//
// The active frame is skipped because it is swap-paned into arc:0.1
// and detected reactively via swapSessionIntoMain returning
// isMissingPaneErr.
func (r *Runtime) reconcileWindows() {
	for frameID, target := range r.sessionPanes {
		if frameID == r.activeFrameID {
			slog.Debug("runtime: reconcile pane skipped (active)", "frame", frameID, "target", target)
			continue
		}
		dead, code, err := r.cfg.Backend.PaneExitStatus(target)
		if err != nil {
			if isMissingPaneErr(err) {
				slog.Debug("runtime: reconcile pane vanished", "frame", frameID, "pane", target, "err", err)
				r.Enqueue(state.EvPaneWindowVanished{FrameID: frameID})
			} else {
				// Transient query failure (timeout/busy): keep the frame and
				// re-probe next reconcile rather than treating it as vanished.
				slog.Warn("runtime: reconcile pane transient error (ignored)", "frame", frameID, "pane", target, "err", err)
			}
			continue
		}
		if !dead {
			continue
		}
		if tail, terr := r.cfg.Backend.CapturePane(target, 20); terr == nil && tail != "" {
			slog.Info("runtime: pane tail on exit", "frame", frameID, "target", target, "exit_code", code, "tail", tail)
		} else {
			slog.Info("runtime: pane exited", "frame", frameID, "target", target, "exit_code", code)
		}
		r.Enqueue(state.EvFrameCommandExited{FrameID: frameID, ExitCode: code})
	}
}

// findPaneOwner returns the FrameID currently active in pane 0.0.
func (r *Runtime) findPaneOwner(_ string) state.FrameID {
	slog.Info("runtime: findPaneOwner",
		"activeFrameID", r.activeFrameID, "mainPaneSession", r.mainPaneSession)
	return r.activeFrameID
}
