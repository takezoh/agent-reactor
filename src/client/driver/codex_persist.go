package driver

import (
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

func (CodexDriver) Persist(s state.DriverState) map[string]string {
	cs, ok := s.(CodexState)
	if !ok {
		return nil
	}
	out := make(map[string]string, 13)
	cs.PersistCommon(out)
	if cs.ThreadID != "" {
		out[codexKeyThreadID] = cs.ThreadID
	}
	if cs.SessionID != "" {
		out[codexKeySessionID] = cs.SessionID
	}
	if cs.RolloutPath != "" {
		out[codexKeyRolloutPath] = cs.RolloutPath
	}
	if cs.RequestedThreadID != "" {
		out[codexKeyRequestedThreadID] = cs.RequestedThreadID
	}
	if cs.ObservedThreadID != "" {
		out[codexKeyObservedThreadID] = cs.ObservedThreadID
	}
	if cs.ResumePhase != "" {
		out[codexKeyResumePhase] = cs.ResumePhase
	}
	return out
}

func (d CodexDriver) Restore(bag map[string]string, now time.Time) state.DriverState {
	cs := CodexState{
		CommonState: CommonState{
			Status:          state.StatusIdle,
			StatusChangedAt: now,
		},
	}
	if len(bag) == 0 {
		return cs
	}
	cs.RestoreCommon(bag)
	cs.ThreadID = bag[codexKeyThreadID]
	cs.SessionID = bag[codexKeySessionID]
	cs.RolloutPath = bag[codexKeyRolloutPath]
	if cs.RolloutPath == "" && cs.TranscriptPath != "" {
		cs.RolloutPath = cs.TranscriptPath
	}
	cs.RequestedThreadID = bag[codexKeyRequestedThreadID]
	cs.ObservedThreadID = bag[codexKeyObservedThreadID]
	cs.ResumePhase = bag[codexKeyResumePhase]
	if cs.TranscriptPath == "" && cs.RolloutPath != "" {
		cs.TranscriptPath = cs.RolloutPath
	}
	return cs
}
