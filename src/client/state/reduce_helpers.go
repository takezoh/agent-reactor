package state

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
)

// Reducer helpers shared by reduce_*.go files. These are pure
// functions that operate on State values; they may allocate new
// SessionIDs / JobIDs and post-process the side-effect lists driver
// Step methods return.

// allocSessionID generates a fresh, random session id. crypto/rand is
// the same source the legacy session.SessionService used so old and
// new ids are interchangeable in the on-disk snapshot. The 12-byte
// width matches the legacy format.
func allocSessionID() SessionID {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on Linux is effectively impossible (it
		// reads from /dev/urandom). If it ever happens we want to fail
		// loud rather than emit a deterministic id that could collide.
		panic("state: crypto/rand failed: " + err.Error())
	}
	return SessionID(hex.EncodeToString(b[:]))
}

func allocFrameID() FrameID {
	return FrameID(allocSessionID())
}

// isSandboxed reports whether a frame for the given project should run inside a sandbox.
// Returns false when the session has SandboxOverrideHost or no resolver is configured.
func isSandboxed(s State, project string, override SandboxOverride) bool {
	return s.SandboxedProject != nil && s.SandboxedProject(project) && override != SandboxOverrideHost
}

// spawnEffect builds an EffSpawnPaneWindow from a resolved LaunchPlan.
// plan.Project, plan.Sandbox, and plan.Stdin must be set by the caller before invoking.
// The runtime resolves the frame's SubsystemID during ensureSubsystem and
// reports it back via EvPaneSpawned.
func spawnEffect(sessID SessionID, frameID FrameID, plan LaunchPlan, connID ConnID, reqID string) EffSpawnPaneWindow {
	return EffSpawnPaneWindow{
		SessionID:  sessID,
		FrameID:    frameID,
		Mode:       LaunchModeCreate,
		Project:    plan.Project,
		Command:    plan.Command,
		StartDir:   plan.StartDir,
		Sandbox:    plan.Sandbox,
		Options:    plan.Options,
		Subsystem:  plan.Subsystem,
		Stream:     plan.Stream,
		Stdin:      plan.Stdin,
		Env:        map[string]string{"ROOST_SESSION_ID": string(sessID), "ROOST_FRAME_ID": string(frameID)},
		ReplyConn:  connID,
		ReplyReqID: reqID,
	}
}

func rootFrame(sess Session) (SessionFrame, bool) {
	if len(sess.Frames) == 0 {
		if sess.Command == "" || sess.Driver == nil {
			return SessionFrame{}, false
		}
		return SessionFrame{
			ID:            FrameID(sess.ID),
			Project:       sess.Project,
			Command:       sess.Command,
			LaunchOptions: sess.LaunchOptions,
			CreatedAt:     sess.CreatedAt,
			Driver:        sess.Driver,
		}, true
	}
	return sess.Frames[0], true
}

func activeFrame(sess Session) (SessionFrame, bool) {
	if len(sess.Frames) == 0 {
		return rootFrame(sess)
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

// pushMRU prepends frameID to sess.MRUFrameIDs, capped at 16 entries.
func pushMRU(sess Session, frameID FrameID) Session {
	if frameID == "" {
		return sess
	}
	mru := make([]FrameID, 0, len(sess.MRUFrameIDs)+1)
	mru = append(mru, frameID)
	for _, id := range sess.MRUFrameIDs {
		if id != frameID {
			mru = append(mru, id)
		}
	}
	if len(mru) > 16 {
		mru = mru[:16]
	}
	sess.MRUFrameIDs = mru
	return sess
}

// popMRU returns the first MRU frame that still exists in sess, or "" if none.
// It also trims stale entries from the front of MRUFrameIDs.
func popMRU(sess Session) (FrameID, Session) {
	existing := make(map[FrameID]bool, len(sess.Frames))
	for _, f := range sess.Frames {
		existing[f.ID] = true
	}
	for i, id := range sess.MRUFrameIDs {
		if existing[id] {
			sess.MRUFrameIDs = append([]FrameID(nil), sess.MRUFrameIDs[i+1:]...)
			return id, sess
		}
	}
	sess.MRUFrameIDs = nil
	return "", sess
}

// removeFrameByIndex removes the frame at position i, preserving all others.
func removeFrameByIndex(sess Session, i int) (Session, SessionFrame) {
	removed := sess.Frames[i]
	frames := make([]SessionFrame, 0, len(sess.Frames)-1)
	frames = append(frames, sess.Frames[:i]...)
	frames = append(frames, sess.Frames[i+1:]...)
	sess.Frames = frames
	return sess, removed
}

func findFrameIndex(sess Session, frameID FrameID) int {
	for i, frame := range sess.Frames {
		if frame.ID == frameID {
			return i
		}
	}
	return -1
}

func findFrame(s State, frameID FrameID) (SessionID, Session, int, bool) {
	for sessID, sess := range s.Sessions {
		if idx := findFrameIndex(sess, frameID); idx >= 0 {
			return sessID, sess, idx, true
		}
	}
	return "", Session{}, -1, false
}

func truncateFrames(sess Session, from int) []SessionFrame {
	if from < 0 || from >= len(sess.Frames) {
		return nil
	}
	return append([]SessionFrame(nil), sess.Frames[from:]...)
}

// stepDriver runs the per-session driver Step inside the reducer and
// post-processes the returned effects so callers don't have to clone
// the State map themselves. Returns the new State (with the updated
// session and any newly registered jobs), the post-processed effects,
// and a "found" bool that's false when the session id is unknown.
//
// Effect post-processing:
//   - EffStartJob: assigns a fresh JobID, records JobMeta with the
//     owning session id and kind, and rewrites the effect to carry
//     the new id.
//   - EffEventLogAppend / EffWatchFile / EffUnwatchFile:
//     fills in the SessionID field if the driver left it blank.
func stepDriver(s State, frameID FrameID, ev DriverEvent) (State, []Effect, bool) {
	sessID, sess, frameIdx, ok := findFrame(s, frameID)
	if !ok {
		return s, nil, false
	}
	frame := sess.Frames[frameIdx]
	drv := GetDriver(frame.Command)
	if drv == nil {
		return s, nil, false
	}

	ctx := FrameContext{
		ID:            frame.ID,
		Project:       frame.Project,
		Command:       frame.Command,
		LaunchOptions: frame.LaunchOptions,
		CreatedAt:     frame.CreatedAt,
		IsRoot:        frameIdx == 0,
	}

	if frame.Driver == nil {
		frame.Driver = drv.NewState(s.Now)
	}
	nextDS, rawEffs, _ := drv.Step(frame.Driver, ctx, ev)

	s.Sessions = cloneSessions(s.Sessions)
	sess.Frames = append([]SessionFrame(nil), sess.Frames...)
	frame.Driver = nextDS
	sess.Frames[frameIdx] = frame
	s.Sessions[sessID] = sess

	out := make([]Effect, 0, len(rawEffs))
	for _, eff := range rawEffs {
		patched, newState := postProcessEffect(s, sessID, frameID, eff)
		s = newState
		out = append(out, patched)
	}

	return s, out, true
}

func bootstrapDriverSessionStart(s State, frameID FrameID) (State, []Effect, bool) {
	sessID, sess, frameIdx, ok := findFrame(s, frameID)
	if !ok || frameIdx != 0 {
		return s, nil, false
	}
	frame := sess.Frames[frameIdx]
	drv := GetDriver(frame.Command)
	if drv == nil {
		return s, nil, false
	}
	bootstrapper, ok := drv.(SessionBootstrapper)
	if !ok {
		return s, nil, false
	}

	ctx := FrameContext{
		ID:            frame.ID,
		Project:       frame.Project,
		Command:       frame.Command,
		LaunchOptions: frame.LaunchOptions,
		CreatedAt:     frame.CreatedAt,
		IsRoot:        true,
	}

	nextDS, rawEffs := bootstrapper.BootstrapSessionStart(frame.Driver, ctx, s.Now)

	s.Sessions = cloneSessions(s.Sessions)
	sess.Frames = append([]SessionFrame(nil), sess.Frames...)
	frame.Driver = nextDS
	sess.Frames[frameIdx] = frame
	s.Sessions[sessID] = sess

	out := make([]Effect, 0, len(rawEffs))
	for _, eff := range rawEffs {
		patched, newState := postProcessEffect(s, sessID, frameID, eff)
		s = newState
		out = append(out, patched)
	}

	return s, out, true
}

// postProcessEffect fills in session-context fields the driver Step
// left blank and (for EffStartJob) registers JobMeta + assigns a fresh
// JobID. Returns the patched effect and the (possibly mutated) State.
func postProcessEffect(s State, sessID SessionID, frameID FrameID, eff Effect) (Effect, State) {
	switch e := eff.(type) {
	case EffStartJob:
		s.NextJobID++
		jobID := s.NextJobID
		s.Jobs = cloneJobs(s.Jobs)
		s.Jobs[jobID] = JobMeta{
			SessionID: sessID,
			FrameID:   frameID,
			StartedAt: s.Now,
		}
		e.JobID = jobID
		return e, s
	case EffEventLogAppend:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e, s
	case EffWatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e, s
	case EffUnwatchFile:
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e, s
	case EffPushDriver:
		if e.SessionID == "" {
			e.SessionID = sessID
		}
		return e, s
	case EffRecordNotification:
		if e.SessionID == "" {
			e.SessionID = sessID
		}
		if e.FrameID == "" {
			e.FrameID = frameID
		}
		return e, s
	default:
		return eff, s
	}
}

// stepActiveSessions runs Step against every live session's driver.
// Each driver decides internally whether to react to a tick — return
// a no-op Step result to skip. Returns whether any session emitted
// effects, so the caller can decide whether to broadcast/persist.
func stepActiveSessions(s State, makeEv func(sessID SessionID, sess Session, active bool) DriverEvent) (State, []Effect, bool) {
	if len(s.Sessions) == 0 {
		return s, nil, false
	}
	// Sort session IDs for deterministic effect ordering.
	ids := make([]SessionID, 0, len(s.Sessions))
	for id := range s.Sessions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var effs []Effect
	changed := false
	for _, sessID := range ids {
		sess := s.Sessions[sessID]
		frame, ok := rootFrame(sess)
		if !ok {
			continue
		}
		drv := GetDriver(frame.Command)
		if drv == nil {
			continue
		}
		active := sessID == s.ActiveSession
		ev := makeEv(sessID, sess, active)
		next, sessEffs, ok := stepDriver(s, frame.ID, ev)
		if !ok {
			continue
		}
		s = next
		if len(sessEffs) > 0 {
			changed = true
			effs = append(effs, sessEffs...)
		}
	}
	return s, effs, changed
}

// errResp wraps a typed error code + message into an EffSendError
// effect. Reducers use this to keep error reply construction terse.
func errResp(connID ConnID, reqID, code, message string) Effect {
	return EffSendError{
		ConnID:  connID,
		ReqID:   reqID,
		Code:    code,
		Message: message,
	}
}

// okResp wraps a typed success body into an EffSendResponse.
func okResp(connID ConnID, reqID string, body any) Effect {
	return EffSendResponse{
		ConnID: connID,
		ReqID:  reqID,
		Body:   body,
	}
}
