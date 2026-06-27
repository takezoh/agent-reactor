package runtime

import (
	"errors"
	"log/slog"

	"github.com/takezoh/agent-reactor/client/state"
)

func (r *Runtime) activateSession(sessID state.SessionID) {
	sess, ok := r.state.Sessions[sessID]
	if !ok {
		return
	}
	frame, ok := sessionActiveFrame(sess)
	if !ok {
		return
	}
	paneID := r.subsystemPaneForFrame(frame)
	if paneID == "" {
		slog.Warn("runtime: activate session — no pane target", "session", sessID)
		return
	}
	if r.activeFrameID == frame.ID {
		return
	}

	if !r.swapSessionIntoMain(sessID) {
		return
	}
}

func (r *Runtime) deactivateSession() {
	if r.mainPaneSession == "" {
		return
	}
	r.swapMainIntoMain()
}

func (r *Runtime) swapSessionIntoMain(sessID state.SessionID) bool {
	slog.Info("runtime: swapSessionIntoMain entry",
		"session", sessID,
		"prevActiveFrame", r.activeFrameID,
		"prevMainPaneSession", r.mainPaneSession)
	sess, ok := r.state.Sessions[sessID]
	if !ok {
		slog.Info("runtime: swapSessionIntoMain bail=session-missing", "session", sessID)
		return false
	}
	frame, ok := sessionActiveFrame(sess)
	if !ok {
		slog.Info("runtime: swapSessionIntoMain bail=no-active-frame", "session", sessID)
		return false
	}
	paneID := r.subsystemPaneForFrame(frame)
	if paneID == "" {
		slog.Warn("runtime: swap-pane session skipped; pane missing", "session", sessID, "frame", frame.ID)
		return false
	}
	if _, ok := r.ensureMainPaneID(); !ok {
		slog.Warn("runtime: swap-pane session skipped; main pane unknown", "session", sessID)
		return false
	}
	slog.Info("runtime: swapSessionIntoMain SwapPane",
		"session", sessID, "frame", frame.ID,
		"srcPane", paneID, "dstTarget", r.mainPaneTarget())
	if err := r.cfg.Backend.SwapPane(paneID, r.mainPaneTarget()); err != nil {
		if isMissingPaneErr(err) {
			r.Enqueue(state.EvPaneWindowVanished{FrameID: frame.ID})
		}
		slog.Warn("runtime: swap-pane session failed", "session", sessID, "pane", paneID, "err", err)
		return false
	}
	r.mainPaneSession = sessID
	r.activeFrameID = frame.ID
	slog.Info("runtime: swapSessionIntoMain ok",
		"session", sessID, "activeFrame", r.activeFrameID)
	return true
}

func (r *Runtime) swapMainIntoMain() bool {
	if r.mainPaneSession == "" {
		return true
	}
	paneID := r.sessionPanes["_main"]
	if paneID == "" {
		return false
	}

	if err := r.cfg.Backend.SwapPane(paneID, r.mainPaneTarget()); err != nil {
		slog.Warn("runtime: swap-pane main failed", "pane", paneID, "err", err)
		return false
	}
	r.mainPaneSession = ""
	r.activeFrameID = ""
	return true
}

func (r *Runtime) ensureMainPaneID() (string, bool) {
	if id := r.sessionPanes["_main"]; id != "" {
		return id, true
	}
	paneID, err := r.cfg.Backend.PaneID(r.mainPaneTarget())
	if err != nil || paneID == "" {
		slog.Warn("runtime: pane-id lookup failed", "target", r.mainPaneTarget(), "err", err)
		return "", false
	}
	r.sessionPanes["_main"] = paneID
	_ = r.cfg.Backend.SetEnv("ROOST_FRAME__main", paneID)
	return paneID, true
}

func (r *Runtime) mainPaneTarget() string {
	return r.cfg.SessionName + ":0.1"
}

func (r *Runtime) hiddenPaneTarget() string {
	return r.cfg.SessionName + ":__hidden__.0"
}

// swapHidden exchanges 0.1 with __hidden__.0 using positional targets so the
// swap is correct in both directions — pane_ids travel with processes across
// swap-pane, so using sessionPanes["_log"] as source would self-swap after the
// first toggle.
func (r *Runtime) swapHidden() {
	if err := r.cfg.Backend.SwapPane(r.hiddenPaneTarget(), r.mainPaneTarget()); err != nil {
		slog.Warn("runtime: swap-hidden failed", "err", err)
	}
}

func isMissingPaneErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrPaneMissing)
}

func sessionPaneEnvKey(frameID state.FrameID) string {
	return "ROOST_FRAME_" + string(frameID)
}

type paneSize struct {
	width  int
	height int
}

func (r *Runtime) mainPaneSize() paneSize {
	width, height, err := r.cfg.Backend.PaneSize(r.mainPaneTarget())
	if err != nil {
		slog.Debug("runtime: pane-size lookup failed", "target", r.mainPaneTarget(), "err", err)
		return paneSize{}
	}
	return paneSize{width: width, height: height}
}

func (r *Runtime) resizeWindowToMain(target string, size paneSize) {
	if size.width == 0 || size.height == 0 {
		return
	}
	if err := r.cfg.Backend.ResizeWindow(target, size.width, size.height); err != nil {
		slog.Debug("runtime: resize-window failed", "target", target, "width", size.width, "height", size.height, "err", err)
	}
}

func sessionActiveFrame(sess state.Session) (state.SessionFrame, bool) {
	if len(sess.Frames) == 0 {
		if sess.Command == "" || sess.Driver == nil {
			return state.SessionFrame{}, false
		}
		return state.SessionFrame{
			ID:            state.FrameID(sess.ID),
			Project:       sess.Project,
			Command:       sess.Command,
			LaunchOptions: sess.LaunchOptions,
			CreatedAt:     sess.CreatedAt,
			Driver:        sess.Driver,
		}, true
	}
	if sess.ActiveFrameID != "" {
		for _, f := range sess.Frames {
			if f.ID == sess.ActiveFrameID {
				return f, true
			}
		}
	}
	return sess.Frames[len(sess.Frames)-1], true
}

func sessionRootFrame(sess state.Session) (state.SessionFrame, bool) {
	if len(sess.Frames) == 0 {
		return sessionActiveFrame(sess)
	}
	return sess.Frames[0], true
}
