package vt

import (
	"testing"
)

func TestFeed_NoOsc(t *testing.T) {
	term := New(40, 10)
	var notifs []OscNotification
	term.OnOscNotification = func(n OscNotification) { notifs = append(notifs, n) }
	if err := term.Feed([]byte("hello $ ")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(notifs) != 0 {
		t.Fatalf("expected no OSC notifications, got %d", len(notifs))
	}
}

func TestFeed_OscHyperlinkNotNotification(t *testing.T) {
	term := New(40, 10)
	var notifs []OscNotification
	term.OnOscNotification = func(n OscNotification) { notifs = append(notifs, n) }
	link := "\x1b]8;;https://example.com\x07Link\x1b]8;;\x07"
	if err := term.Feed([]byte(link)); err != nil {
		t.Fatalf("Feed OSC 8: %v", err)
	}
	for _, n := range notifs {
		if n.Cmd == 8 {
			t.Errorf("OSC 8 should not appear in notifications, got %+v", n)
		}
	}
}

func TestFeed_OscNotification9(t *testing.T) {
	term := New(40, 10)
	var notifs []OscNotification
	term.OnOscNotification = func(n OscNotification) { notifs = append(notifs, n) }
	if err := term.Feed([]byte("\x1b]9;Hello from agent\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Cmd != 9 {
		t.Errorf("Cmd = %d, want 9", notifs[0].Cmd)
	}
	if notifs[0].Payload != "Hello from agent" {
		t.Errorf("Payload = %q, want %q", notifs[0].Payload, "Hello from agent")
	}
}

func TestFeed_OscNotification99(t *testing.T) {
	term := New(40, 10)
	var notifs []OscNotification
	term.OnOscNotification = func(n OscNotification) { notifs = append(notifs, n) }
	if err := term.Feed([]byte("\x1b]99;d=MyTitle:p=MyBody\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Cmd != 99 {
		t.Errorf("Cmd = %d, want 99", notifs[0].Cmd)
	}
	if notifs[0].Payload != "d=MyTitle:p=MyBody" {
		t.Errorf("Payload = %q, want %q", notifs[0].Payload, "d=MyTitle:p=MyBody")
	}
}

func TestFeed_OscNotification777(t *testing.T) {
	term := New(40, 10)
	var notifs []OscNotification
	term.OnOscNotification = func(n OscNotification) { notifs = append(notifs, n) }
	if err := term.Feed([]byte("\x1b]777;notify;MyTitle;MyBody\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Cmd != 777 {
		t.Errorf("Cmd = %d, want 777", notifs[0].Cmd)
	}
	if notifs[0].Payload != "notify;MyTitle;MyBody" {
		t.Errorf("Payload = %q, want %q", notifs[0].Payload, "notify;MyTitle;MyBody")
	}
}

func TestFeed_CallbackFiredImmediately(t *testing.T) {
	term := New(40, 10)
	var count int
	term.OnOscNotification = func(n OscNotification) { count++ }
	_ = term.Feed([]byte("\x1b]9;first\x07"))
	if count != 1 {
		t.Errorf("count after first Feed = %d, want 1", count)
	}
	_ = term.Feed([]byte("\x1b]9;second\x07"))
	if count != 2 {
		t.Errorf("count after second Feed = %d, want 2", count)
	}
}

func TestFeed_Osc133Phases(t *testing.T) {
	tests := []struct {
		name      string
		seq       string
		wantPhase PromptPhase
		wantCode  *int
	}{
		{"133;A start", "\x1b]133;A\x07", PromptPhaseStart, nil},
		{"133;B input", "\x1b]133;B\x07", PromptPhaseInput, nil},
		{"133;C command", "\x1b]133;C\x07", PromptPhaseCommand, nil},
		{"133;D complete no code", "\x1b]133;D\x07", PromptPhaseComplete, nil},
		{"133;D;0 exit 0", "\x1b]133;D;0\x07", PromptPhaseComplete, intPtr(0)},
		{"133;D;42 exit 42", "\x1b]133;D;42\x07", PromptPhaseComplete, intPtr(42)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := New(40, 10)
			var events []PromptEvent
			term.OnPromptEvent = func(e PromptEvent) { events = append(events, e) }
			if err := term.Feed([]byte(tt.seq)); err != nil {
				t.Fatalf("Feed: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("PromptEvents len = %d, want 1", len(events))
			}
			ev := events[0]
			if ev.Phase != tt.wantPhase {
				t.Errorf("Phase = %v, want %v", ev.Phase, tt.wantPhase)
			}
			if tt.wantCode == nil {
				if ev.ExitCode != nil {
					t.Errorf("ExitCode = %v, want nil", *ev.ExitCode)
				}
			} else {
				if ev.ExitCode == nil {
					t.Errorf("ExitCode = nil, want %d", *tt.wantCode)
				} else if *ev.ExitCode != *tt.wantCode {
					t.Errorf("ExitCode = %d, want %d", *ev.ExitCode, *tt.wantCode)
				}
			}
		})
	}
}

func TestFeed_Osc133MultipleEventsOrdered(t *testing.T) {
	term := New(40, 10)
	var events []PromptEvent
	term.OnPromptEvent = func(e PromptEvent) { events = append(events, e) }
	if err := term.Feed([]byte("\x1b]133;C\x07" + "\x1b]133;D;0\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("PromptEvents len = %d, want 2", len(events))
	}
	if events[0].Phase != PromptPhaseCommand {
		t.Errorf("events[0].Phase = %v, want Command", events[0].Phase)
	}
	if events[1].Phase != PromptPhaseComplete {
		t.Errorf("events[1].Phase = %v, want Complete", events[1].Phase)
	}
}

func TestFeed_Osc133UnknownPhaseSilentlyDropped(t *testing.T) {
	term := New(40, 10)
	var events []PromptEvent
	term.OnPromptEvent = func(e PromptEvent) { events = append(events, e) }
	if err := term.Feed([]byte("\x1b]133;Z\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("PromptEvents len = %d, want 0 for unknown phase", len(events))
	}
}

func TestFeed_WindowTitle(t *testing.T) {
	term := New(40, 10)
	var titles []string
	term.OnWindowTitle = func(cmd int, title string) { titles = append(titles, title) }
	if err := term.Feed([]byte("\x1b]0;mytitle\x07")); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(titles) != 1 || titles[0] != "mytitle" {
		t.Errorf("titles = %v, want [mytitle]", titles)
	}
}

func TestResize(t *testing.T) {
	term := New(40, 10)
	term.Resize(100, 30)
}

func TestReset_ClearsEmulatorState(t *testing.T) {
	term := New(40, 10)
	var count int
	term.OnOscNotification = func(n OscNotification) { count++ }
	_ = term.Feed([]byte("\x1b]9;before-reset\x07"))
	if count != 1 {
		t.Fatalf("expected 1 notification before reset, got %d", count)
	}
	term.Reset()
	_ = term.Feed([]byte("\x1b]9;after-reset\x07"))
	if count != 2 {
		t.Fatalf("expected 2 total notifications after reset, got %d", count)
	}
}

func intPtr(v int) *int { return &v }
