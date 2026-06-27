package state

import (
	"fmt"
)

// reduce_session_nav.go holds reducers for session navigation (preview,
// switch, focus, list) and pane spawn lifecycle events. Kept separate
// from the core session creation / push / fork logic in reduce_session.go.

func reducePaneSpawned(s State, e EvPaneSpawned) (State, []Effect) {
	sess, ok := s.Sessions[e.SessionID]
	if !ok {
		return s, nil
	}
	frameIdx := findFrameIndex(sess, e.FrameID)
	if frameIdx < 0 {
		return s, nil
	}

	// The subsystem factory chose this frame's backend identity during ensureSubsystem;
	// record it on the frame so cleanup and routing have a stable handle.
	if e.SubsystemID != "" && sess.Frames[frameIdx].SubsystemID != e.SubsystemID {
		s.Sessions = cloneSessions(s.Sessions)
		sess = s.Sessions[e.SessionID]
		frames := make([]SessionFrame, len(sess.Frames))
		copy(frames, sess.Frames)
		frames[frameIdx].SubsystemID = e.SubsystemID
		sess.Frames = frames
		s.Sessions[e.SessionID] = sess
	}

	// When the subsystem created a managed worktree during BindFrame, propagate
	// the resolved path to the driver so it is persisted for cold-start reconstruction.
	if e.WorktreeStartDir != "" {
		if next, _, ok := stepDriver(s, e.FrameID, DEvWorktreeResolved{
			StartDir: e.WorktreeStartDir,
			Name:     e.WorktreeName,
		}); ok {
			s = next
		}
	}

	var bootstrapEffs []Effect
	if frameIdx == 0 {
		s, bootstrapEffs, _ = bootstrapDriverSessionStart(s, e.FrameID)
	}
	s.ActiveSession = e.SessionID

	effs := []Effect{}
	effs = append(effs, bootstrapEffs...)
	effs = append(effs, EffRegisterPane{
		FrameID:    e.FrameID,
		PaneTarget: e.PaneTarget,
		Tap:        frameIdx == 0 || sess.Frames[frameIdx].SubsystemID != "",
	})
	effs = append(effs,
		EffPersistSnapshot{},
		EffBroadcastSessionsChanged{},
	)
	if e.ReplyConn != 0 {
		effs = append(effs, okResp(e.ReplyConn, e.ReplyReqID, CreateSessionReply{
			SessionID: string(e.SessionID),
		}))
	}
	return s, effs
}

type CreateSessionReply struct {
	SessionID string
}

func reduceSpawnFailed(s State, e EvSpawnFailed) (State, []Effect) {
	var effs []Effect
	if next, evictEffs, ok := evictFrame(s, e.FrameID, false); ok {
		s = next
		effs = evictEffs
	}
	if e.ReplyConn == 0 {
		return s, effs
	}
	return s, append(effs,
		errResp(e.ReplyConn, e.ReplyReqID, ErrCodeInternal,
			fmt.Sprintf("pane spawn failed: %s", e.Err)),
	)
}

func reducePreviewSession(s State, connID ConnID, reqID string, p PreviewSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	s.ActiveSession = sid
	return s, []Effect{
		EffBroadcastSessionsChanged{IsPreview: true},
		okResp(connID, reqID, ActiveSessionReply{ActiveSessionID: string(sid)}),
	}
}

func reduceSwitchSession(s State, connID ConnID, reqID string, p SwitchSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	s.ActiveSession = sid
	return s, []Effect{
		EffBroadcastSessionsChanged{},
		okResp(connID, reqID, ActiveSessionReply{ActiveSessionID: string(sid)}),
	}
}

type ActiveSessionReply struct {
	ActiveSessionID string
}

func reducePreviewProject(s State, connID ConnID, reqID string, p PreviewProjectParams) (State, []Effect) {
	s.ActiveSession = ""
	return s, []Effect{
		okResp(connID, reqID, nil),
		EffBroadcastEvent{
			Name:    "project-selected",
			Payload: ProjectSelectedPayload(p),
		},
	}
}

type ProjectSelectedPayload struct {
	Project string
}

func reduceListSessions(s State, connID ConnID, reqID string, _ struct{}) (State, []Effect) {
	return s, []Effect{
		okResp(connID, reqID, SessionsReply{}),
	}
}

type SessionsReply struct{}
