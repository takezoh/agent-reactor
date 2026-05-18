package state

import (
	"encoding/json"
	"fmt"
)

// reduce_session_nav.go holds reducers for session navigation (preview,
// switch, focus, list) and tmux spawn lifecycle events. Kept separate
// from the core session creation / push / fork logic in reduce_session.go.

func reduceTmuxPaneSpawned(s State, e EvTmuxPaneSpawned) (State, []Effect) {
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
	s, pre := ensureMainAtVisibleSlot(s)
	s.ActiveOccupant = OccupantFrame
	s.ActiveSession = e.SessionID

	effs := []Effect{}
	effs = append(effs, bootstrapEffs...)
	effs = append(effs, EffRegisterPane{
		FrameID:    e.FrameID,
		PaneTarget: e.PaneTarget,
		Tap:        frameIdx == 0 || sess.Frames[frameIdx].SubsystemID != "",
	})
	effs = append(effs, pre...)
	effs = append(effs,
		EffActivateSession{SessionID: e.SessionID, Reason: EventCreateSession},
		EffSelectPane{Target: "{sessionName}:0.1"},
		EffSyncStatusLine{Line: ""},
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

func reduceTmuxSpawnFailed(s State, e EvTmuxSpawnFailed) (State, []Effect) {
	var effs []Effect
	if sess, ok := s.Sessions[e.SessionID]; ok {
		if idx := findFrameIndex(sess, e.FrameID); idx >= 0 {
			frame := sess.Frames[idx]
			if frame.SubsystemID != "" {
				next, effs, handled := failSubsystemFrame(s, e.FrameID, "tmux spawn failed: "+e.Err, false)
				if handled {
					if e.ReplyConn == 0 {
						return next, effs
					}
					return next, append(effs,
						errResp(e.ReplyConn, e.ReplyReqID, ErrCodeInternal,
							fmt.Sprintf("tmux spawn failed: %s", e.Err)),
					)
				}
			}
			sess, _ = truncateFrames(sess, idx)
			s.Sessions = cloneSessions(s.Sessions)
			if len(sess.Frames) == 0 {
				delete(s.Sessions, e.SessionID)
			} else {
				s.Sessions[e.SessionID] = sess
			}
		}
	}
	if e.ReplyConn == 0 {
		return s, effs
	}
	return s, append(effs,
		errResp(e.ReplyConn, e.ReplyReqID, ErrCodeInternal,
			fmt.Sprintf("tmux spawn failed: %s", e.Err)),
	)
}

func reducePreviewSession(s State, connID ConnID, reqID string, p PreviewSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	s, pre := ensureMainAtVisibleSlot(s)
	s.ActiveOccupant = OccupantFrame
	s.ActiveSession = sid

	pre = append(pre,
		EffActivateSession{SessionID: sid, Reason: EventPreviewSession},
		EffSyncStatusLine{Line: ""},
		EffBroadcastSessionsChanged{IsPreview: true},
		okResp(connID, reqID, ActiveSessionReply{ActiveSessionID: string(sid)}),
	)
	return s, pre
}

func reduceSwitchSession(s State, connID ConnID, reqID string, p SwitchSessionParams) (State, []Effect) {
	sid := SessionID(p.SessionID)
	if _, ok := s.Sessions[sid]; !ok {
		return s, []Effect{errResp(connID, reqID, ErrCodeNotFound, "session not found")}
	}
	s, pre := ensureMainAtVisibleSlot(s)
	s.ActiveOccupant = OccupantFrame
	s.ActiveSession = sid

	pre = append(pre,
		EffActivateSession{SessionID: sid, Reason: EventSwitchSession},
		EffSelectPane{Target: "{sessionName}:0.1"},
		EffSyncStatusLine{Line: ""},
		EffBroadcastSessionsChanged{},
		okResp(connID, reqID, ActiveSessionReply{ActiveSessionID: string(sid)}),
	)
	return s, pre
}

type ActiveSessionReply struct {
	ActiveSessionID string
}

func reducePreviewProject(s State, connID ConnID, reqID string, p PreviewProjectParams) (State, []Effect) {
	var effs []Effect
	if s.ActiveOccupant == OccupantFrame {
		s.ActiveOccupant = OccupantMain
		effs = append(effs, EffDeactivateSession{})
	}
	s.ActiveSession = ""
	effs = append(effs, okResp(connID, reqID, nil))
	effs = append(effs, EffBroadcastEvent{
		Name:    "project-selected",
		Payload: ProjectSelectedPayload(p),
	})
	return s, effs
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

func reduceFocusPane(s State, connID ConnID, reqID string, p FocusPaneParams) (State, []Effect) {
	if p.Pane == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "pane arg required")}
	}
	return s, []Effect{
		EffSelectPane{Target: p.Pane},
		EffBroadcastEvent{
			Name:    "pane-focused",
			Payload: PaneFocusedPayload(p),
		},
		okResp(connID, reqID, nil),
	}
}

type PaneFocusedPayload struct {
	Pane string
}

func reduceLaunchTool(s State, connID ConnID, reqID string, raw json.RawMessage) (State, []Effect) {
	var m map[string]string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	tool := m["tool"]
	if tool == "" {
		return s, []Effect{errResp(connID, reqID, ErrCodeInvalidArgument, "tool arg required")}
	}
	delete(m, "tool")
	return s, []Effect{
		EffDisplayPopup{
			Width:  "60%",
			Height: "50%",
			Tool:   tool,
			Args:   m,
		},
		okResp(connID, reqID, nil),
	}
}
