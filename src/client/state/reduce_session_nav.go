package state

import (
	"fmt"
)

// reduce_session_nav.go holds the session list reducer and the frame
// spawn lifecycle events. Kept separate from the core session creation /
// push / fork logic in reduce_session.go.

func reduceFrameSpawned(s State, e EvFrameSpawned) (State, []Effect) {
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

	effs := []Effect{}
	effs = append(effs, bootstrapEffs...)
	effs = append(effs, EffRegisterFrame{
		FrameID: e.FrameID,
		Tap:     frameIdx == 0 || sess.Frames[frameIdx].SubsystemID != "",
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

// ProjectSelectedPayload is the state-side payload for the generic
// "project-selected" broadcast event (translated to proto.EvtProjectSelected
// by the runtime bridge).
type ProjectSelectedPayload struct {
	Project string
}

func reduceListSessions(s State, connID ConnID, reqID string, _ struct{}) (State, []Effect) {
	return s, []Effect{
		okResp(connID, reqID, SessionsReply{}),
	}
}

type SessionsReply struct{}
