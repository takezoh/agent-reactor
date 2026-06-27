package driver

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

func newGenericState(t *testing.T, threshold time.Duration) (GenericDriver, GenericState, time.Time) {
	t.Helper()
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	d := NewGenericDriver("bash", "bash", threshold)
	s := d.NewState(now).(GenericState)
	return d, s, now
}

func TestGenericNewStateDefaults(t *testing.T) {
	d, s, now := newGenericState(t, 5*time.Second)
	if s.Name != "bash" {
		t.Errorf("Name = %q, want bash", s.Name)
	}
	if s.Status != state.StatusWaiting {
		t.Errorf("Status = %v, want Waiting", s.Status)
	}
	if !s.StatusChangedAt.Equal(now) {
		t.Errorf("StatusChangedAt = %v, want %v", s.StatusChangedAt, now)
	}
	_ = d
}

func TestGenericTickSkipsWhenParkedAndWaiting(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	// Status=Waiting (default), Active=false → self-gate skips tick
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: false, PaneTarget: "5"})
	if len(effs) != 0 {
		t.Errorf("expected 0 effects for parked+waiting, got %d", len(effs))
	}
}

func TestGenericTickWithoutWindowEmitsNothing(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true})
	if len(effs) != 0 {
		t.Errorf("expected 0 effects when PaneTarget empty, got %d", len(effs))
	}
}

func TestGenericPersistRoundTrip(t *testing.T) {
	d, s, now := newGenericState(t, 5*time.Second)
	s.Status = state.StatusWaiting
	s.StatusChangedAt = now
	s.Summary = "summary text"
	s.StartDir = "/repo/.agent-reactor/worktrees/alpha-beta"
	s.WorktreeName = "alpha-beta"
	bag := d.Persist(s)
	if bag[keyStatus] != "waiting" {
		t.Errorf("persisted status = %q, want waiting", bag[keyStatus])
	}
	if bag[keyStatusChangedAt] == "" {
		t.Error("persisted changed_at should not be empty")
	}
	if bag[keySummary] != "summary text" {
		t.Errorf("persisted summary = %q, want summary text", bag[keySummary])
	}
	if bag[keyStartDir] != "/repo/.agent-reactor/worktrees/alpha-beta" {
		t.Errorf("persisted working dir = %q", bag[keyStartDir])
	}
	restored := d.Restore(bag, time.Now()).(GenericState)
	if restored.Status != state.StatusWaiting {
		t.Errorf("restored status = %v, want waiting", restored.Status)
	}
	if !restored.StatusChangedAt.Equal(now) {
		t.Errorf("restored changed_at = %v, want %v", restored.StatusChangedAt, now)
	}
	if restored.Summary != "summary text" {
		t.Errorf("restored summary = %q, want summary text", restored.Summary)
	}
	if restored.StartDir != "/repo/.agent-reactor/worktrees/alpha-beta" || restored.WorktreeName != "alpha-beta" {
		t.Errorf("restored worktree fields = %+v", restored)
	}
}

func TestGenericPrepareCreateWithWorktree(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	_, plan, err := d.PrepareCreate(s, "sess-1", "/repo", "bash --worktree", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if plan.Command != "bash" {
		t.Fatalf("command = %q, want worktree flag stripped", plan.Command)
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree enabled")
	}
}

func TestGenericRestoreEmptyBag(t *testing.T) {
	d := NewGenericDriver("bash", "bash", 5*time.Second)
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	restored := d.Restore(nil, now).(GenericState)
	if restored.Status != state.StatusWaiting {
		t.Errorf("empty restore status = %v, want Waiting", restored.Status)
	}
	if !restored.StatusChangedAt.Equal(now) {
		t.Errorf("empty restore changed_at = %v, want %v", restored.StatusChangedAt, now)
	}
}

func TestGenericHookEventNoOp(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	next, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvHook{Event: "session-start"})
	if len(effs) != 0 {
		t.Errorf("hook effects = %d, want 0", len(effs))
	}
	gs := next.(GenericState)
	if gs.Status != state.StatusWaiting {
		t.Errorf("Status changed by hook event: %v", gs.Status)
	}
}

func TestGenericViewNoCommandTag(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	v := d.view(s)
	if len(v.Card.Tags) != 0 {
		t.Errorf("tags = %d, want 0", len(v.Card.Tags))
	}
}

func TestGenericViewDisplayName(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	v := d.view(s)
	if v.DisplayName != "bash" {
		t.Errorf("DisplayName = %q, want bash", v.DisplayName)
	}
}

func TestGenericViewBorderTitle(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	v := d.view(s)
	if v.Card.BorderTitle.Text != "bash" {
		t.Errorf("BorderTitle.Text = %q, want bash", v.Card.BorderTitle.Text)
	}
}

func TestGenericApplySummaryResult(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.SummaryInFlight = true
	next, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvJobResult{
		Now:    now,
		Result: SummaryCommandResult{Summary: "new summary"},
	})
	if len(effs) != 0 {
		t.Fatalf("effects = %d, want 0", len(effs))
	}
	gs := next.(GenericState)
	if gs.SummaryInFlight {
		t.Fatal("SummaryInFlight should be false")
	}
	if gs.Summary != "new summary" {
		t.Errorf("Summary = %q, want new summary", gs.Summary)
	}
}

func TestGenericApplySummaryResultErrorKeepsPrevious(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.SummaryInFlight = true
	s.Summary = "old summary"
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvJobResult{
		Now:    now,
		Result: SummaryCommandResult{Summary: "new summary"},
		Err:    errors.New("failed"),
	})
	gs := next.(GenericState)
	if gs.SummaryInFlight {
		t.Fatal("SummaryInFlight should be false after error")
	}
	if gs.Summary != "old summary" {
		t.Errorf("Summary = %q, want old summary", gs.Summary)
	}
}

// TestGenericViewSummaryPromotesToTitle locks the title priority chain on
// the generic driver: when no upstream Title is set, the user-prompt summary
// is promoted into the Card.Title slot. Card.Subtitle still mirrors Summary
// for downstream non-rendering consumers; the UI layer hides the duplicate
// row.
func TestGenericViewSummaryPromotesToTitle(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	s.Summary = "running tests"
	v := d.view(s)
	if v.Card.Title != "running tests" {
		t.Errorf("Title = %q, want running tests", v.Card.Title)
	}
	if v.Card.Subtitle != "running tests" {
		t.Errorf("Subtitle = %q, want running tests (UI dedups; data layer keeps it)", v.Card.Subtitle)
	}
}

func TestGenericFallbackHasNoBorderTitle(t *testing.T) {
	d := NewGenericDriver("", "", 0)
	s := d.NewState(time.Now()).(GenericState)
	v := d.view(s)
	if v.Card.BorderTitle.Text != "" {
		t.Errorf("fallback BorderTitle.Text = %q, want empty", v.Card.BorderTitle.Text)
	}
}

func TestGenericFallbackHasNoCommandTag(t *testing.T) {
	d := NewGenericDriver("", "", 0)
	s := d.NewState(time.Now()).(GenericState)
	v := d.view(s)
	if len(v.Card.Tags) != 0 {
		t.Errorf("fallback tags = %d, want 0", len(v.Card.Tags))
	}
	if v.DisplayName != "" {
		t.Errorf("fallback DisplayName = %q, want empty", v.DisplayName)
	}
}

func TestGetDriverFallbackFactory(t *testing.T) {
	state.ClearRegistry()
	state.RegisterDefaultFactory(func(command string) state.Driver {
		name := state.FirstToken(command)
		return NewGenericDriver(name, name, 0)
	})

	// "tig status" のような未知のコマンドに対してフォールバックファクトリが呼ばれることを確認
	d := state.GetDriver("tig status")
	if d.Name() != "tig" {
		t.Errorf("Driver Name = %q, want tig", d.Name())
	}
	if d.DisplayName() != "tig" {
		t.Errorf("Driver DisplayName = %q, want tig", d.DisplayName())
	}

	// 登録済みのドライバはフォールバックファクトリが呼ばれないことを確認
	state.Register(NewGenericDriver("mycmd", "My Command", 0))
	d2 := state.GetDriver("mycmd args")
	if d2.Name() != "mycmd" {
		t.Errorf("Registered Driver Name = %q, want mycmd", d2.Name())
	}
	if d2.DisplayName() != "My Command" {
		t.Errorf("Registered Driver DisplayName = %q, want My Command", d2.DisplayName())
	}
}

func TestGenericViewFallbackChip(t *testing.T) {
	state.ClearRegistry()
	state.RegisterDefaultFactory(func(command string) state.Driver {
		name := state.FirstToken(command)
		return NewGenericDriver(name, name, 0)
	})

	d := state.GetDriver("tig status")
	s := d.NewState(time.Now())
	v := d.View(s) // View() メソッドは Driver インターフェースにある
	if v.Card.BorderTitle.Text != "tig" {
		t.Errorf("Fallback Driver View BorderTitle = %q, want tig", v.Card.BorderTitle.Text)
	}
	if v.DisplayName != "tig" {
		t.Errorf("Fallback Driver View DisplayName = %q, want tig", v.DisplayName)
	}
}

// ---- Branch detection tests ----

func TestGenericTickActiveSchedulesBranchJob(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true, Project: "/repo"})
	var found bool
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if in, ok := job.Input.(BranchDetectInput); ok && in.WorkingDir == "/repo" {
			found = true
		}
	}
	if !found {
		t.Error("expected BranchDetectInput job when active with project")
	}
}

func TestGenericTickActiveSchedulesBranchJobUpdatesState(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true, Project: "/repo"})
	gs := next.(GenericState)
	if !gs.BranchInFlight {
		t.Error("BranchInFlight should be true after branch job scheduled")
	}
	if gs.BranchTarget != "/repo" {
		t.Errorf("BranchTarget = %q, want /repo", gs.BranchTarget)
	}
}

func TestGenericTickInactiveSkipsBranchDetect(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: false, Project: "/repo", PaneTarget: "5"})
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			t.Error("branch detect should not be scheduled when inactive")
		}
	}
}

func TestGenericTickFreshCacheSkipsBranchDetect(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchTarget = "/repo"
	s.BranchAt = now.Add(-10 * time.Second) // within 30s
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true, Project: "/repo"})
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			t.Error("branch detect should be skipped when cache is fresh")
		}
	}
}

func TestGenericTickStaleCacheRefreshesBranchDetect(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchTarget = "/repo"
	s.BranchAt = now.Add(-31 * time.Second) // stale
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true, Project: "/repo"})
	var found bool
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected branch detect when cache is stale")
	}
}

func TestGenericTickBranchInFlightSkips(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchInFlight = true
	_, effs, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvTick{Now: now, Active: true, Project: "/repo"})
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			t.Error("branch detect should be skipped when in-flight")
		}
	}
}

func TestGenericBranchDetectResultUpdatesTag(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchInFlight = true
	ev := state.DEvJobResult{
		Now: now,
		Result: BranchDetectResult{
			Branch:       "feature/x",
			Background:   "#aaa",
			Foreground:   "#fff",
			IsWorktree:   true,
			ParentBranch: "main",
		},
	}
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, ev)
	gs := next.(GenericState)
	if gs.BranchInFlight {
		t.Error("BranchInFlight should be false after result")
	}
	if gs.BranchTag != "feature/x" {
		t.Errorf("BranchTag = %q, want feature/x", gs.BranchTag)
	}
	if gs.BranchBG != "#aaa" {
		t.Errorf("BranchBG = %q, want #aaa", gs.BranchBG)
	}
	if gs.BranchFG != "#fff" {
		t.Errorf("BranchFG = %q, want #fff", gs.BranchFG)
	}
	if !gs.BranchIsWorktree {
		t.Error("BranchIsWorktree should be true")
	}
	if gs.BranchParentBranch != "main" {
		t.Errorf("BranchParentBranch = %q, want main", gs.BranchParentBranch)
	}
	if !gs.BranchAt.Equal(now) {
		t.Errorf("BranchAt = %v, want %v", gs.BranchAt, now)
	}
}

func TestGenericBranchDetectResultEmptyPreservesTag(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchTag = "main"
	s.BranchInFlight = true
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvJobResult{
		Now:    now,
		Result: BranchDetectResult{Branch: ""},
	})
	gs := next.(GenericState)
	if gs.BranchInFlight {
		t.Error("BranchInFlight should be false")
	}
	if gs.BranchTag != "main" {
		t.Errorf("BranchTag = %q, want main (preserved)", gs.BranchTag)
	}
}

func TestGenericBranchDetectResultErrorPreservesTag(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.BranchTag = "main"
	s.BranchInFlight = true
	next, _, _ := d.Step(s, state.FrameContext{IsRoot: true}, state.DEvJobResult{
		Now:    now,
		Err:    errors.New("git error"),
		Result: BranchDetectResult{Branch: "feature/x"},
	})
	gs := next.(GenericState)
	if gs.BranchInFlight {
		t.Error("BranchInFlight should be false")
	}
	if gs.BranchTag != "main" {
		t.Errorf("BranchTag = %q, want main (preserved on error)", gs.BranchTag)
	}
}

func TestGenericViewIncludesBranchTag(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	s.BranchTag = "main"
	s.BranchBG = "#89b4fa"
	s.BranchFG = "#1e1e2e"
	v := d.view(s)
	if len(v.Card.Tags) == 0 {
		t.Fatal("expected branch tag in Card.Tags, got none")
	}
	var found bool
	for _, tag := range v.Card.Tags {
		if tag.Text == "main" || strings.Contains(tag.Text, "main") {
			found = true
		}
	}
	if !found {
		t.Errorf("branch tag with text 'main' not found in Tags: %v", v.Card.Tags)
	}
}

// IsRoot=false ガード: 非 root frame は DEvTick を無視する。

func TestGenericStepNonRootSkipsTick(t *testing.T) {
	d, s, now := newGenericState(t, 0)
	s.Status = state.StatusRunning
	next, effs, _ := d.Step(s, state.FrameContext{IsRoot: false}, state.DEvTick{
		Now: now.Add(2 * time.Second), Active: true, Project: "/repo", PaneTarget: "%5",
	})
	if len(effs) != 0 {
		t.Errorf("non-root DEvTick effects = %d, want 0", len(effs))
	}
	if next.(GenericState).Status != s.Status {
		t.Errorf("non-root DEvTick mutated Status: got %v, want %v", next.(GenericState).Status, s.Status)
	}
}

// TestGenericPrepareLaunchInheritedPlainStartDir verifies that a child frame
// inheriting a plain project StartDir does not trigger worktree creation.
// Previously generic forced Worktree.Enabled=true whenever StartDir was set,
// causing a new worktree for every non-root frame.
func TestGenericPrepareLaunchInheritedPlainStartDir(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	s.StartDir = "/repo"
	plan, err := d.PrepareLaunch(s, state.LaunchModeCreate, "/repo", "bash", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.StartDir != "/repo" {
		t.Errorf("StartDir = %q, want /repo", plan.StartDir)
	}
	if plan.Options.Worktree.Enabled {
		t.Error("Worktree.Enabled should be false; inherited plain StartDir must not trigger worktree creation")
	}
}

// TestGenericPrepareLaunchManagedWorktreePathNoForcedEnable verifies that
// a managed worktree StartDir does not force Worktree.Enabled; adoption
// is handled by BindFrame (IsManagedWorktreePath), not PrepareLaunch.
func TestGenericPrepareLaunchManagedWorktreePathNoForcedEnable(t *testing.T) {
	d, s, _ := newGenericState(t, 0)
	s.StartDir = "/repo/.agent-reactor/worktrees/test-name"
	plan, err := d.PrepareLaunch(s, state.LaunchModeColdStart, "/repo", "bash", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.StartDir != "/repo/.agent-reactor/worktrees/test-name" {
		t.Errorf("StartDir = %q, want managed worktree path", plan.StartDir)
	}
	if plan.Options.Worktree.Enabled {
		t.Error("Worktree.Enabled should be false; BindFrame handles adoption via IsManagedWorktreePath")
	}
}
