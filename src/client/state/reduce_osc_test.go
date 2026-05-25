package state

import (
	"testing"
)

func TestReducePaneOsc_EmitsRecordNotification(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	ev := EvPaneOsc{FrameID: frameID, Cmd: 9, Title: "hello", Body: ""}
	_, effs := Reduce(s, ev)

	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	rec, ok := effs[0].(EffRecordNotification)
	if !ok {
		t.Fatalf("expected EffRecordNotification, got %T", effs[0])
	}
	if rec.FrameID != frameID {
		t.Errorf("frameID: got %q, want %q", rec.FrameID, frameID)
	}
	if rec.SessionID != sessID {
		t.Errorf("sessionID: got %q, want %q", rec.SessionID, sessID)
	}
	if rec.Cmd != 9 || rec.Title != "hello" {
		t.Errorf("unexpected content: cmd=%d title=%q", rec.Cmd, rec.Title)
	}
}

func TestReducePaneOsc_EmptyTitleBody_NoEffect(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 9, Title: "", Body: ""})
	if len(effs) != 0 {
		t.Errorf("expected no effects for empty notification, got %d", len(effs))
	}
}

func TestReducePaneOsc_UnknownFrame_NoEffect(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvPaneOsc{FrameID: "ghost", Cmd: 9, Title: "hi"})
	if len(effs) != 0 {
		t.Errorf("expected no effects for unknown frame, got %d", len(effs))
	}
}

func TestReducePaneOsc_OSC0_RoutesToDriver_NotRecordNotification(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 0, Title: "✳ Claude Code"})

	for _, e := range effs {
		if _, ok := e.(EffRecordNotification); ok {
			t.Error("OSC 0 should not produce EffRecordNotification")
		}
	}
}

func TestReducePaneOsc_OSC0_AppendsEventLog(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 0, Title: "✳ Claude Code"})

	logEff, ok := findEff[EffEventLogAppend](effs)
	if !ok {
		t.Fatal("OSC 0 should produce EffEventLogAppend")
	}
	if logEff.FrameID != frameID {
		t.Errorf("FrameID = %q, want %q", logEff.FrameID, frameID)
	}
	if logEff.Line != "[osc0] ✳ Claude Code" {
		t.Errorf("Line = %q, want %q", logEff.Line, "[osc0] ✳ Claude Code")
	}
}

func TestReducePaneOsc_OSC2_AppendsEventLog(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 2, Title: "✋ Action Required"})

	logEff, ok := findEff[EffEventLogAppend](effs)
	if !ok {
		t.Fatal("OSC 2 should produce EffEventLogAppend")
	}
	if logEff.Line != "[osc2] ✋ Action Required" {
		t.Errorf("Line = %q, want %q", logEff.Line, "[osc2] ✋ Action Required")
	}
}

func TestReducePaneOsc_OSC0_EmptyTitle_NoEffect(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 0, Title: ""})
	if len(effs) != 0 {
		t.Errorf("expected no effects for empty OSC 0 title, got %d", len(effs))
	}
}

func TestReducePaneOsc_OSC2_RoutesToDriver_NotRecordNotification(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 2, Title: "✋ Action Required"})

	for _, e := range effs {
		if _, ok := e.(EffRecordNotification); ok {
			t.Error("OSC 2 should not produce EffRecordNotification")
		}
	}
}

func TestReducePaneOsc_OSC2_EmptyTitle_NoEffect(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 2, Title: ""})
	if len(effs) != 0 {
		t.Errorf("expected no effects for empty OSC 2 title, got %d", len(effs))
	}
}

func TestReducePanePrompt_UnknownFrame_NoEffect(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvPanePrompt{FrameID: "ghost", Phase: PromptPhaseInput})
	if len(effs) != 0 {
		t.Errorf("expected no effects for unknown frame, got %d", len(effs))
	}
}

func TestReducePanePrompt_RoutesToDriver(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	// stubSession uses stubDriver which returns nil effects for all events,
	// so we just verify no EffRecordNotification (prompt events are not notifications).
	_, effs := Reduce(s, EvPanePrompt{FrameID: frameID, Phase: PromptPhaseInput})
	for _, e := range effs {
		if _, ok := e.(EffRecordNotification); ok {
			t.Error("EvPanePrompt should not produce EffRecordNotification")
		}
	}
}

func TestReducePanePrompt_AppendsEventLog_Input(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPanePrompt{FrameID: frameID, Phase: PromptPhaseInput})
	logEff, ok := findEff[EffEventLogAppend](effs)
	if !ok {
		t.Fatal("EvPanePrompt should produce EffEventLogAppend")
	}
	if logEff.Line != "[osc133] phase=input" {
		t.Errorf("Line = %q, want %q", logEff.Line, "[osc133] phase=input")
	}
}

func TestReducePanePrompt_AppendsEventLog_CompleteWithExitCode(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	code := 42
	_, effs := Reduce(s, EvPanePrompt{FrameID: frameID, Phase: PromptPhaseComplete, ExitCode: &code})
	logEff, ok := findEff[EffEventLogAppend](effs)
	if !ok {
		t.Fatal("EvPanePrompt should produce EffEventLogAppend")
	}
	if logEff.Line != "[osc133] phase=complete exit=42" {
		t.Errorf("Line = %q, want %q", logEff.Line, "[osc133] phase=complete exit=42")
	}
}

func TestReducePanePrompt_AppendsPersistAndBroadcastWithoutDriverEffects(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPanePrompt{FrameID: frameID, Phase: PromptPhaseInput})
	if _, ok := findEff[EffPersistSnapshot](effs); !ok {
		t.Fatal("expected EffPersistSnapshot")
	}
	if _, ok := findEff[EffBroadcastSessionsChanged](effs); !ok {
		t.Fatal("expected EffBroadcastSessionsChanged")
	}
}

func TestReducePaneOsc_OSC9_StillEmitsRecordNotification(t *testing.T) {
	s := New()
	sessID := SessionID("sess1")
	s.Sessions = map[SessionID]Session{sessID: stubSession(sessID)}
	frameID := FrameID(sessID)

	_, effs := Reduce(s, EvPaneOsc{FrameID: frameID, Cmd: 9, Title: "ping"})
	if _, ok := findEff[EffRecordNotification](effs); !ok {
		t.Error("OSC 9 should still produce EffRecordNotification")
	}
}
