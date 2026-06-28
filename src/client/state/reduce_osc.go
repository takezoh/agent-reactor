package state

import "fmt"

// reduceFrameOsc handles an OSC notification event fired by the FrameTap reader.
// Every observed OSC sequence is appended to the EVENTS log so operators can
// trace what each frame is emitting. OSC 0/2 (window title) is also routed to
// the driver via DEvFrameOsc; OSC 9/99/777 are emitted as EffRecordNotification
// (which itself writes the EVENTS log line and may dispatch a desktop toast).
func reduceFrameOsc(s State, e EvFrameOsc) (State, []Effect) {
	if e.Cmd == 0 || e.Cmd == 2 {
		if e.Title == "" {
			return s, nil
		}
		effs := []Effect{EffEventLogAppend{
			FrameID: e.FrameID,
			Line:    fmt.Sprintf("[osc%d] %s", e.Cmd, e.Title),
		}}
		next, dEffs, _ := stepDriver(s, e.FrameID, DEvFrameOsc{Cmd: e.Cmd, Title: e.Title, Now: e.Now})
		effs = append(effs, dEffs...)
		return next, effs
	}

	if e.Title == "" && e.Body == "" {
		return s, nil
	}
	sessID, _, _, ok := findFrame(s, e.FrameID)
	if !ok {
		return s, nil
	}
	return s, []Effect{EffRecordNotification{
		SessionID: sessID,
		FrameID:   e.FrameID,
		Cmd:       e.Cmd,
		Title:     e.Title,
		Body:      e.Body,
	}}
}

// reduceFramePrompt routes an OSC 133 semantic-prompt event to the owning
// frame's driver as DEvFramePrompt and writes a line to the EVENTS log so
// operators can see prompt-phase transitions in real time.
func reduceFramePrompt(s State, e EvFramePrompt) (State, []Effect) {
	s.Now = e.Now
	if _, _, _, ok := findFrame(s, e.FrameID); !ok {
		return s, nil
	}
	effs := []Effect{EffEventLogAppend{
		FrameID: e.FrameID,
		Line:    promptEventLogLine(e.Phase, e.ExitCode),
	}}
	next, dEffs, _ := stepDriver(s, e.FrameID, DEvFramePrompt{
		Phase:    e.Phase,
		ExitCode: e.ExitCode,
		Now:      e.Now,
	})
	effs = append(effs, dEffs...)
	effs = appendMissingSessionRefreshEffects(effs)
	return next, effs
}

func appendMissingSessionRefreshEffects(effs []Effect) []Effect {
	var hasPersist, hasBroadcast bool
	for _, eff := range effs {
		switch eff.(type) {
		case EffPersistSnapshot:
			hasPersist = true
		case EffBroadcastSessionsChanged:
			hasBroadcast = true
		}
	}
	if !hasPersist {
		effs = append(effs, EffPersistSnapshot{})
	}
	if !hasBroadcast {
		effs = append(effs, EffBroadcastSessionsChanged{})
	}
	return effs
}

func promptEventLogLine(phase PromptPhase, exitCode *int) string {
	name := promptPhaseName(phase)
	if phase == PromptPhaseComplete && exitCode != nil {
		return fmt.Sprintf("[osc133] phase=%s exit=%d", name, *exitCode)
	}
	return fmt.Sprintf("[osc133] phase=%s", name)
}

func promptPhaseName(p PromptPhase) string {
	switch p {
	case PromptPhaseStart:
		return "start"
	case PromptPhaseInput:
		return "input"
	case PromptPhaseCommand:
		return "command"
	case PromptPhaseComplete:
		return "complete"
	default:
		return "none"
	}
}
