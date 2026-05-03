package driver

import (
	"testing"
	"time"

	"github.com/takezoh/agent-roost/state"
)

func newShellState(t *testing.T, threshold time.Duration) (ShellDriver, ShellState, time.Time) {
	t.Helper()
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	d := NewShellDriver("shell", "bash", threshold)
	s := d.NewState(now).(ShellState)
	return d, s, now
}

func TestShellPersistRestoreRoundTrip(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	s.Status = state.StatusWaiting
	s.StatusChangedAt = now
	s.Summary = "doing stuff"
	s.SawPromptEvent = true

	bag := d.Persist(s)
	if bag[keyShellSawPromptEvent] != "1" {
		t.Errorf("persisted saw_prompt_event = %q, want 1", bag[keyShellSawPromptEvent])
	}

	restored := d.Restore(bag, time.Now()).(ShellState)
	if restored.Status != state.StatusWaiting {
		t.Errorf("restored status = %v, want Waiting", restored.Status)
	}
	if !restored.SawPromptEvent {
		t.Error("restored SawPromptEvent should be true")
	}
	if restored.Summary != "doing stuff" {
		t.Errorf("restored summary = %q, want %q", restored.Summary, "doing stuff")
	}
}

func TestShellViewNonZeroExitShowsIndicatorTag(t *testing.T) {
	d, s, now := newShellState(t, 0)
	code := 127
	s.LastExitCode = &code
	_ = now
	v := d.View(s)
	found := false
	for _, tag := range v.Card.Tags {
		if tag.Background == "#cc3333" && tag.Text == "✘ 127" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected exit indicator tag for exit 127, got %v", v.Card.Tags)
	}
}

func TestShellViewZeroExitNoIndicatorTag(t *testing.T) {
	d, s, now := newShellState(t, 0)
	code := 0
	s.LastExitCode = &code
	_ = now
	v := d.View(s)
	for _, tag := range v.Card.Tags {
		if tag.Background == "#cc3333" {
			t.Error("should not show exit indicator tag for exit 0")
		}
	}
}

func TestShellViewNilExitNoIndicatorTag(t *testing.T) {
	d, s, now := newShellState(t, 0)
	_ = now
	v := d.View(s)
	for _, tag := range v.Card.Tags {
		if tag.Background == "#cc3333" {
			t.Error("should not show exit indicator tag when LastExitCode is nil")
		}
	}
}

func TestShellPersistRestoreLastExitCode(t *testing.T) {
	d, s, now := newShellState(t, 0)
	code := 2
	s.LastExitCode = &code
	bag := d.Persist(s)
	restored := d.Restore(bag, now).(ShellState)
	if restored.LastExitCode == nil {
		t.Fatal("LastExitCode not restored from bag")
	}
	if *restored.LastExitCode != 2 {
		t.Errorf("restored LastExitCode = %d, want 2", *restored.LastExitCode)
	}
}

// IsRoot=false guard: non-root frames ignore DEvTick.

func TestShellStepNonRootSkipsTick(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	s.Status = state.StatusRunning
	next, effs, _ := d.Step(s, state.FrameContext{IsRoot: false}, state.DEvTick{
		Now: now.Add(time.Second), Active: true, Project: "/repo", PaneTarget: "%5",
	})
	if len(effs) != 0 {
		t.Errorf("non-root DEvTick effects = %d, want 0", len(effs))
	}
	if next.(ShellState).Status != s.Status {
		t.Errorf("non-root DEvTick mutated Status: got %v, want %v", next.(ShellState).Status, s.Status)
	}
}

func TestShellStepDEvPanePromptInputSetsSawPromptEvent(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvPanePrompt{
		Phase: state.PromptPhaseInput,
		Now:   now,
	})
	ns := next.(ShellState)
	if !ns.SawPromptEvent {
		t.Error("SawPromptEvent should be true after DEvPanePrompt{PromptPhaseInput}")
	}
	if ns.Status != state.StatusWaiting {
		t.Errorf("Status = %v, want Waiting", ns.Status)
	}
	if ns.LastExitCode != nil {
		t.Error("LastExitCode should remain nil for PromptPhaseInput")
	}
}

func TestShellStepDEvPanePromptCommandSetsRunning(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvPanePrompt{
		Phase: state.PromptPhaseCommand,
		Now:   now.Add(time.Second),
	})
	ns := next.(ShellState)
	if ns.Status != state.StatusRunning {
		t.Errorf("Status = %v, want Running", ns.Status)
	}
	if !ns.StatusChangedAt.Equal(now.Add(time.Second)) {
		t.Errorf("StatusChangedAt = %v, want %v", ns.StatusChangedAt, now.Add(time.Second))
	}
}

func TestShellStepDEvPanePromptCompleteSetsLastExitCodeAndWaiting(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	s.Status = state.StatusRunning
	s.StatusChangedAt = now
	code := 42
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvPanePrompt{
		Phase:    state.PromptPhaseComplete,
		ExitCode: &code,
		Now:      now.Add(2 * time.Second),
	})
	ns := next.(ShellState)
	if !ns.SawPromptEvent {
		t.Error("SawPromptEvent should be true after DEvPanePrompt{PromptPhaseComplete}")
	}
	if ns.Status != state.StatusWaiting {
		t.Errorf("Status = %v, want Waiting", ns.Status)
	}
	if ns.LastExitCode == nil || *ns.LastExitCode != 42 {
		t.Errorf("LastExitCode = %v, want 42", ns.LastExitCode)
	}
	if !ns.StatusChangedAt.Equal(now.Add(2 * time.Second)) {
		t.Errorf("StatusChangedAt = %v, want %v", ns.StatusChangedAt, now.Add(2*time.Second))
	}
}

func TestShellStepDEvPanePromptInputPreservesStatusChangedAtWhenAlreadyWaiting(t *testing.T) {
	d, s, now := newShellState(t, 5*time.Second)
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvPanePrompt{
		Phase: state.PromptPhaseInput,
		Now:   now.Add(time.Second),
	})
	ns := next.(ShellState)
	if !ns.StatusChangedAt.Equal(now) {
		t.Errorf("StatusChangedAt = %v, want %v", ns.StatusChangedAt, now)
	}
}
