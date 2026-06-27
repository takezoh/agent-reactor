package runtime

import (
	"errors"

	"github.com/takezoh/agent-reactor/client/state"
)

// resident.go holds the small grab-bag of pane helpers that survived the
// TUI removal: pane-missing error classification, session-level env-var
// key derivation, and the per-frame "active" / "root" frame projections used
// by snapshot helpers.

func isMissingPaneErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrPaneMissing)
}

func sessionPaneEnvKey(frameID state.FrameID) string {
	return "ROOST_FRAME_" + string(frameID)
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
