package state

import "time"

func failSubsystemFrame(s State, frameID FrameID, message string, unregisterPane bool) (State, []Effect, bool) {
	sessID, sess, frameIdx, ok := findFrame(s, frameID)
	if !ok {
		return s, nil, false
	}
	// Root frames (idx 0) must be evicted, not failed as a subsystem.
	if frameIdx == 0 {
		return s, nil, false
	}
	frame := sess.Frames[frameIdx]
	if frame.SubsystemID == "" {
		// BindFrame has not completed yet for this frame; no subsystem
		// owns it, so a subsystem-level failure cannot route here.
		return s, nil, false
	}
	next, rawEffs, ok := stepDriver(s, frameID, DEvSubsystem{
		Source:    SubsystemStream,
		Kind:      SubsystemFailed,
		Timestamp: time.Now(),
		Payload:   SubsystemPayload{TargetID: string(frame.TargetID), Error: message},
	})
	if !ok {
		return s, nil, false
	}
	s = next
	sess, ok = s.Sessions[sessID]
	if !ok {
		return s, nil, false
	}
	sess.ActiveFrameID = frameID
	s.Sessions = cloneSessions(s.Sessions)
	s.Sessions[sessID] = sess
	s, effs, _ := resolvePushDriverEffects(s, rawEffs)
	if unregisterPane {
		effs = append(effs, EffUnregisterPane{FrameID: frameID}, EffUnwatchFile{FrameID: frameID})
	}
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	return s, effs, true
}
