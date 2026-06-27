package state

// SurfaceReadTextReply is the marker passed to EffSendResponseSync.Body for
// surface.read_text. The runtime resolves the pane target from its internal
// sessionPanes map and calls CapturePane to fill in the text.
type SurfaceReadTextReply struct {
	SessionID SessionID
	Lines     int
}

// DriverListReply is the marker passed to EffSendResponseSync.Body for
// driver.list. The runtime builds the response from the driver registry.
type DriverListReply struct{}

func reduceSurfaceReadText(s State, e EvCmdSurfaceReadText) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{EffSendError{
			ConnID:  e.ConnID,
			ReqID:   e.ReqID,
			Code:    "not_found",
			Message: "session not found: " + string(sid),
		}}
	}
	lines := e.Lines
	if lines <= 0 {
		lines = 30
	}
	return s, []Effect{EffSendResponseSync{
		ConnID: e.ConnID,
		ReqID:  e.ReqID,
		Body:   SurfaceReadTextReply{SessionID: sid, Lines: lines},
	}}
}

func reduceSurfaceSendText(s State, e EvCmdSurfaceSendText) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{EffSendError{
			ConnID:  e.ConnID,
			ReqID:   e.ReqID,
			Code:    "not_found",
			Message: "session not found: " + string(sid),
		}}
	}
	return s, []Effect{EffSendPaneKeys{
		ConnID:    e.ConnID,
		ReqID:     e.ReqID,
		SessionID: sid,
		Text:      e.Text,
		WithEnter: true,
	}}
}

func reduceSurfaceSendKey(s State, e EvCmdSurfaceSendKey) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{EffSendError{
			ConnID:  e.ConnID,
			ReqID:   e.ReqID,
			Code:    "not_found",
			Message: "session not found: " + string(sid),
		}}
	}
	return s, []Effect{EffSendPaneKeys{
		ConnID:    e.ConnID,
		ReqID:     e.ReqID,
		SessionID: sid,
		Key:       e.Key,
		WithEnter: false,
	}}
}

func reduceDriverList(s State, e EvCmdDriverList) (State, []Effect) {
	return s, []Effect{EffSendResponseSync{
		ConnID: e.ConnID,
		ReqID:  e.ReqID,
		Body:   DriverListReply{},
	}}
}

func reduceSurfaceSubscribe(s State, e EvCmdSurfaceSubscribe) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	sess, ok := s.Sessions[sid]
	if !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "session not found: "+string(sid))}
	}
	if _, ok := activeFrame(sess); !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeFrameNotReady, "frame-not-ready: "+string(sid))}
	}
	existing := s.SurfaceSubs[e.ConnID]
	_, already := existing[sid]
	if !already && len(existing) >= 8 {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeResourceExhausted, "per-conn surface subscribe cap (8) exceeded")}
	}
	s.SurfaceSubs = cloneSurfaceSubs(s.SurfaceSubs)
	inner := s.SurfaceSubs[e.ConnID]
	if inner == nil {
		inner = map[SessionID]struct{}{}
		s.SurfaceSubs[e.ConnID] = inner
	}
	if _, already := inner[sid]; already {
		return s, []Effect{okResp(e.ConnID, e.ReqID, nil)}
	}
	inner[sid] = struct{}{}
	return s, []Effect{
		EffSurfaceSubscribeStart{ConnID: e.ConnID, SessionID: sid},
		okResp(e.ConnID, e.ReqID, nil),
	}
}

func reduceSurfaceUnsubscribe(s State, e EvCmdSurfaceUnsubscribe) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.SurfaceSubs[e.ConnID][sid]; !ok {
		return s, []Effect{okResp(e.ConnID, e.ReqID, nil)}
	}
	s.SurfaceSubs = cloneSurfaceSubs(s.SurfaceSubs)
	delete(s.SurfaceSubs[e.ConnID], sid)
	if len(s.SurfaceSubs[e.ConnID]) == 0 {
		delete(s.SurfaceSubs, e.ConnID)
	}
	return s, []Effect{
		EffSurfaceSubscribeStop{ConnID: e.ConnID, SessionID: sid},
		okResp(e.ConnID, e.ReqID, nil),
	}
}

func reduceSurfaceResize(s State, e EvCmdSurfaceResize) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "session not found: "+string(sid))}
	}
	return s, []Effect{
		EffSurfaceResize{SessionID: sid, Cols: e.Cols, Rows: e.Rows},
		okResp(e.ConnID, e.ReqID, nil),
	}
}

func reduceSurfaceWriteRaw(s State, e EvCmdSurfaceWriteRaw) (State, []Effect) {
	sid := e.SessionID
	if sid == "" {
		sid = s.ActiveSession
	}
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(e.ConnID, e.ReqID, ErrCodeNotFound, "session not found: "+string(sid))}
	}
	return s, []Effect{
		EffSurfaceWriteRaw{SessionID: sid, Data: e.Data},
		okResp(e.ConnID, e.ReqID, nil),
	}
}
