package state

import "log/slog"

func reduceEvent(s State, e EvEvent) (State, []Effect) {
	fn := eventHandlers[e.Event]
	return fn(s, e)
}

func reduceDriverHook(s State, e EvDriverEvent) (State, []Effect) {
	if e.SenderID == "" {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "driver event requires sender_id: "+e.Event)}
	}
	if _, _, _, ok := findFrame(s, e.SenderID); !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "unknown session")}
	}

	next, rawEffs, ok := stepDriver(s, e.SenderID, DEvHook{
		Event:          e.Event,
		Timestamp:      e.Timestamp,
		RoostSessionID: string(e.SenderID),
		Payload:        e.Payload,
	})
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInternal, "no driver for session")}
	}
	s = next

	// Resolve EffPushDriver effects emitted by the driver.
	s, effs := resolvePushDriverEffects(s, rawEffs)

	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	if e.ConnID != 0 {
		effs = append(effs, okResp(e.ConnID, e.ReqID, nil))
	}
	return s, effs
}

func reduceSubsystem(s State, e EvSubsystem) (State, []Effect) {
	if e.FrameID == "" {
		slog.Debug("state: subsystem event rejected", "reason", "missing_frame_id", "source", e.Source, "kind", e.Kind)
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInvalidArgument, "subsystem event requires frame_id")}
	}
	sessID, sess, frameIdx, ok := findFrame(s, e.FrameID)
	if !ok {
		slog.Debug("state: subsystem event rejected", "reason", "unknown_frame", "frame", e.FrameID, "source", e.Source, "kind", e.Kind)
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "unknown session")}
	}
	frame := sess.Frames[frameIdx]
	if targetID := TargetID(e.Payload.TargetID); targetID != "" && targetID != frame.TargetID {
		slog.Debug("state: subsystem target updated",
			"frame", e.FrameID,
			"from", frame.TargetID,
			"to", targetID,
			"source", e.Source,
			"kind", e.Kind)
		sess.Frames = append([]SessionFrame(nil), sess.Frames...)
		sess.Frames[frameIdx].TargetID = targetID
		s.Sessions = cloneSessions(s.Sessions)
		s.Sessions[sessID] = sess
	}
	slog.Debug("state: subsystem event routed",
		"frame", e.FrameID,
		"target", sess.Frames[frameIdx].TargetID,
		"source", e.Source,
		"kind", e.Kind)

	next, rawEffs, ok := stepDriver(s, e.FrameID, DEvSubsystem{
		Source:    e.Source,
		Kind:      e.Kind,
		Timestamp: e.Timestamp,
		Payload:   e.Payload,
	})
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeInternal, "no driver for session")}
	}
	s = next

	s, effs := resolvePushDriverEffects(s, rawEffs)
	effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	if e.ConnID != 0 {
		effs = append(effs, okResp(e.ConnID, e.ReqID, nil))
	}
	return s, effs
}
