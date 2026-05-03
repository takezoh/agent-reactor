package state

// reducePaneOsc handles an OSC notification event fired by the PaneTap reader.
// OSC 0/2 (window title) is routed to the driver via DEvPaneOsc so each driver
// can interpret the title string in its own way (e.g. Claude reads Braille
// spinner vs ✳ to infer working/waiting status). OSC 9/99/777 are broadcast
// directly as EffRecordNotification.
func reducePaneOsc(s State, e EvPaneOsc) (State, []Effect) {
	if e.Cmd == 0 || e.Cmd == 2 {
		if e.Title == "" {
			return s, nil
		}
		next, effs, _ := stepDriver(s, e.FrameID, DEvPaneOsc{Cmd: e.Cmd, Title: e.Title, Now: e.Now})
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

// reducePanePrompt routes an OSC 133 semantic-prompt event to the owning
// frame's driver as DEvPanePrompt. Drivers may use this to update
// SawPromptEvent / LastExitCode state.
func reducePanePrompt(s State, e EvPanePrompt) (State, []Effect) {
	s.Now = e.Now
	next, effs, ok := stepDriver(s, e.FrameID, DEvPanePrompt{
		Phase:    e.Phase,
		ExitCode: e.ExitCode,
		Now:      e.Now,
	})
	if !ok {
		return s, nil
	}
	if len(effs) > 0 {
		effs = append(effs, EffPersistSnapshot{}, EffBroadcastSessionsChanged{})
	}
	return next, effs
}
