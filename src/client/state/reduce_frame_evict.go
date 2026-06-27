package state

// reduce_frame_evict.go holds helpers for removing frames from the state.
// Kept separate from reduce_helpers.go for file-size compliance.

// evictFrame removes a frame and its cleanup effects.
//
// When frameID is the root frame (index 0), all sibling frames are also
// removed and the session is deleted — root death ends the session.
// When frameID is a child frame (index > 0), only that frame is removed;
// siblings are unaffected.
//
// killWindow controls whether EffKillSessionWindow is emitted per removed
// frame. Pass true when the backend window still exists (e.g. EvPaneDied);
// pass false when the window has already vanished (e.g. EvPaneWindowVanished).
//
// Effect ordering: deactivate → reactivate → cleanup → persist → broadcast.
// Reactivate precedes cleanup so that EffActivateSession swaps the parent
// pane into 0.1 before EffKillSessionWindow destroys the old window —
// preventing kill-window from targeting window 0.
func evictFrame(s State, frameID FrameID, killWindow bool) (State, []Effect, bool) {
	sessID, sess, idx, ok := findFrame(s, frameID)
	if !ok {
		return s, nil, false
	}
	if idx == 0 {
		return evictRootFrame(s, sessID, sess, killWindow)
	}
	return evictChildFrame(s, sessID, sess, idx, frameID, killWindow)
}

func evictRootFrame(s State, sessID SessionID, sess Session, killWindow bool) (State, []Effect, bool) {
	allRemoved := truncateFrames(sess, 0)
	s.Sessions = cloneSessions(s.Sessions)
	delete(s.Sessions, sessID)
	var effs []Effect
	if s.ActiveSession == sessID {
		s.ActiveSession = ""
		if s.ActiveOccupant == OccupantFrame {
			s.ActiveOccupant = OccupantMain
			effs = append(effs, EffDeactivateSession{})
		}
	}
	for _, frame := range allRemoved {
		if killWindow {
			effs = append(effs, EffKillSessionWindow{FrameID: frame.ID})
		}
		effs = append(effs, EffUnregisterPane{FrameID: frame.ID}, EffUnwatchFile{FrameID: frame.ID})
	}
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

func evictChildFrame(s State, sessID SessionID, sess Session, idx int, frameID FrameID, killWindow bool) (State, []Effect, bool) {
	wasActive := sess.ActiveFrameID == frameID
	sess, removed := removeFrameByIndex(sess, idx)
	if wasActive {
		fallback, next := popMRU(sess)
		sess = next
		if fallback == "" {
			fallback = sess.Frames[0].ID
		}
		sess.ActiveFrameID = fallback
	}
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sessID] = sess

	var effs []Effect
	if s.ActiveSession == sessID && wasActive {
		var pre []Effect
		s, pre = ensureMainAtVisibleSlot(s)
		s.ActiveOccupant = OccupantFrame
		effs = append(effs, pre...)
		effs = append(effs,
			EffActivateSession{SessionID: sessID, Reason: EventSwitchSession},
			EffSyncStatusLine{Line: ""},
		)
	}
	if killWindow {
		effs = append(effs, EffKillSessionWindow{FrameID: removed.ID})
	}
	effs = append(effs, EffUnregisterPane{FrameID: removed.ID}, EffUnwatchFile{FrameID: removed.ID})
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

// === ErrCode constants used by reducers ===
//
// These mirror proto.ErrCode but live in state pkg so reducers can
// emit them without importing proto. The runtime translates them to
// proto.ErrCode values when serializing the response.
const (
	ErrCodeNotFound          = "not_found"
	ErrCodeInvalidArgument   = "invalid_argument"
	ErrCodeInternal          = "internal"
	ErrCodeAlreadyExists     = "already_exists"
	ErrCodeUnsupported       = "unsupported"
	ErrCodeResourceExhausted = "resource_exhausted"
	ErrCodeFrameNotReady     = "frame_not_ready"
)
