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
// killWindow controls whether EffKillFrame is emitted per removed
// frame. Pass true when the backend frame still exists (e.g. EvFrameCommandExited);
// pass false when the frame has already vanished (e.g. EvFrameVanished).
//
// Effect ordering: cleanup → persist → broadcast.
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
	effs := make([]Effect, 0, len(allRemoved)*4+2)
	for _, frame := range allRemoved {
		effs = append(effs, frameTeardownEffects(frame.ID, killWindow)...)
	}
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

func evictChildFrame(s State, sessID SessionID, sess Session, idx int, frameID FrameID, killWindow bool) (State, []Effect, bool) {
	wasActive := sess.HeadFrameID == frameID
	sess, removed := removeFrameByIndex(sess, idx)
	if wasActive {
		fallback, next := popMRU(sess)
		sess = next
		if fallback == "" {
			fallback = sess.Frames[0].ID
		}
		sess.HeadFrameID = fallback
	}
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sessID] = sess

	effs := frameTeardownEffects(removed.ID, killWindow)
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}

// frameTeardownEffects builds the canonical per-frame teardown effect set
// emitted whenever a frame leaves the session graph: backend frame kill
// (only when the frame is still alive — frame-vanished routes set
// killWindow=false), sandbox release (always — frame-vanished must still
// drop the container refcount), frame unregister, file unwatch.
//
// Centralising the set is load-bearing: any new reducer path that
// removes a frame must call this helper, otherwise the silent regression
// is "container survives forever even though no frame uses it" — exactly
// the bug reduceStopSession had until this helper existed.
func frameTeardownEffects(frameID FrameID, killWindow bool) []Effect {
	effs := make([]Effect, 0, 4)
	if killWindow {
		effs = append(effs, EffKillFrame{FrameID: frameID})
	}
	effs = append(effs,
		EffReleaseFrameSandbox{FrameID: frameID},
		EffUnregisterFrame{FrameID: frameID},
		EffUnwatchFile{FrameID: frameID},
	)
	return effs
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
