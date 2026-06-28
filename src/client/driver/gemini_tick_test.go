package driver

import (
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

func TestGeminiHandleTickCompletesStartDir(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)

	// Before: StartDir is empty
	if gs.StartDir != "" {
		t.Errorf("gs.StartDir = %q, want empty", gs.StartDir)
	}

	// After one tick: StartDir should be filled from e.Project,
	// but BranchDetect should NOT start because status is Idle (Claude-aligned).
	e := state.DEvTick{
		Now:     now.Add(time.Second),
		Watched: true,
		Project: "/repo/project",
	}
	next, effs, _ := d.Step(gs, state.FrameContext{IsRoot: true}, e)
	gs = next.(GeminiState)

	if gs.StartDir != "/repo/project" {
		t.Errorf("gs.StartDir = %q, want /repo/project", gs.StartDir)
	}

	for _, eff := range effs {
		if ej, ok := eff.(state.EffStartJob); ok {
			if _, ok := ej.Input.(BranchDetectInput); ok {
				t.Fatal("BranchDetectInput job started on Idle, want skip")
			}
		}
	}

	// Transition to Running
	gs.Status = state.StatusRunning

	// Next tick should now fire BranchDetect
	next, effs, _ = d.Step(gs, state.FrameContext{IsRoot: true}, e)
	gs = next.(GeminiState)

	found := false
	for _, eff := range effs {
		if ej, ok := eff.(state.EffStartJob); ok {
			if bdi, ok := ej.Input.(BranchDetectInput); ok {
				if bdi.WorkingDir == "/repo/project" {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("expected BranchDetectInput job in effects after transition to Running")
	}
}

// IsRoot=false ガード: 非 root frame は DEvTick を無視する。

func TestGeminiStepNonRootSkipsTick(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)
	gs.Status = state.StatusRunning
	gs.StartDir = "/repo"
	next, effs, _ := d.Step(gs, state.FrameContext{IsRoot: false}, state.DEvTick{
		Now: now.Add(time.Second), Watched: true, Project: "/repo",
	})
	if len(effs) != 0 {
		t.Errorf("non-root DEvTick effects = %d, want 0", len(effs))
	}
	if next.(GeminiState).StartDir != "/repo" {
		t.Errorf("non-root DEvTick mutated StartDir: got %q", next.(GeminiState).StartDir)
	}
}

func TestGeminiViewIncludesBranchTag(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)

	gs.BranchTag = "main"
	gs.BranchBG = "#123456"
	gs.BranchFG = "#ffffff"

	v := d.View(gs)
	if len(v.Card.Tags) == 0 {
		t.Fatal("expected branch tag in view, but got none")
	}

	found := false
	for _, tag := range v.Card.Tags {
		if tag.Text == "main" && tag.Background == "#123456" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("branch tag not found or mismatched: %+v", v.Card.Tags)
	}
}

func TestGeminiWarmStartRecover(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)
	gs.TranscriptPath = "/tmp/t.jsonl"

	nextState, effs := d.WarmStartRecover(gs, now)
	next := nextState.(GeminiState)

	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if !next.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should be true")
	}
	if _, ok := effs[0].(state.EffWatchFile); !ok {
		t.Fatalf("first effect = %T, want EffWatchFile", effs[0])
	}
}

func TestGeminiHandleTranscriptChangedStartsParse(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)
	gs.TranscriptPath = "/tmp/t.jsonl"

	next, effs := d.handleTranscriptChanged(gs, state.DEvFileChanged{Path: "/tmp/t.jsonl"})
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if !next.TranscriptInFlight {
		t.Fatal("expected TranscriptInFlight")
	}
	if _, ok := effs[0].(state.EffWatchFile); !ok {
		t.Fatalf("first effect = %T, want EffWatchFile", effs[0])
	}
}

func TestGeminiCommandExitCodeZero(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)
	gs.Status = state.StatusRunning

	ev := state.DEvCommandExited{
		Timestamp: now.Add(time.Second),
		ExitCode:  0, // 正常終了
	}
	next, _, _ := d.Step(gs, state.FrameContext{IsRoot: true}, ev)
	gsNext := next.(GeminiState)

	if gsNext.Status == state.StatusStopped {
		t.Error("expected StatusStopped NOT to be set on exit code 0")
	}
}

func TestGeminiCommandExitCodeNonZero(t *testing.T) {
	d := NewGeminiDriver("/tmp/events")
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	gs := d.NewState(now).(GeminiState)
	gs.Status = state.StatusRunning

	ev := state.DEvCommandExited{
		Timestamp: now.Add(time.Second),
		ExitCode:  1, // 異常終了
	}
	next, _, _ := d.Step(gs, state.FrameContext{IsRoot: true}, ev)
	gsNext := next.(GeminiState)

	if gsNext.Status != state.StatusStopped {
		t.Error("expected StatusStopped to be set on exit code 1")
	}
}
