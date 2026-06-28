package runtime

import (
	"errors"

	"github.com/takezoh/agent-reactor/client/state"
)

// resident.go holds the small grab-bag of frame helpers that survived the
// TUI removal: frame-missing error classification and the per-session "head"
// / "root" frame projections used by snapshot helpers.

func isMissingFrameErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrFrameMissing)
}

func sessionHeadFrame(sess state.Session) (state.SessionFrame, bool) {
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
	if sess.HeadFrameID != "" {
		for _, f := range sess.Frames {
			if f.ID == sess.HeadFrameID {
				return f, true
			}
		}
	}
	return sess.Frames[len(sess.Frames)-1], true
}

func sessionRootFrame(sess state.Session) (state.SessionFrame, bool) {
	if len(sess.Frames) == 0 {
		return sessionHeadFrame(sess)
	}
	return sess.Frames[0], true
}
