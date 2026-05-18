package driver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/state"
)

func geminiHook(fields map[string]string, ts time.Time) state.DEvHook {
	raw, _ := json.Marshal(fields)
	return state.DEvHook{Payload: raw, Timestamp: ts}
}

func newGemini(t *testing.T) (GeminiDriver, GeminiState, time.Time) {
	t.Helper()
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	d := NewGeminiDriver("/tmp/events")
	gs := d.NewState(now).(GeminiState)
	return d, gs, now
}

func TestGeminiPrepareCreateWithoutWorktree(t *testing.T) {
	d, gs, _ := newGemini(t)
	_, plan, err := d.PrepareCreate(gs, "sess-1", "/repo", "gemini --model flash", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if plan.Command != "gemini --model flash" || plan.StartDir != "/repo" {
		t.Fatalf("launch = %+v", plan)
	}
	if plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree disabled")
	}
}

func TestGeminiPrepareCreateWithWorktree(t *testing.T) {
	d, gs, _ := newGemini(t)
	_, plan, err := d.PrepareCreate(gs, "sess-1", "/repo", "gemini --worktree feature", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if plan.Command != "gemini" {
		t.Fatalf("launch command = %q, want worktree flag stripped", plan.Command)
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree enabled")
	}
}

func TestGeminiPrepareCreateWithWorkspaceAlias(t *testing.T) {
	d, gs, _ := newGemini(t)
	_, plan, err := d.PrepareCreate(gs, "sess-1", "/repo", "gemini --workspace feature", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree enabled via --workspace alias")
	}
}

func TestGeminiPrepareLaunchManagedWorktreeSkipsFlag(t *testing.T) {
	d, gs, _ := newGemini(t)
	gs.StartDir = "/repo/.roost/worktrees/feature"
	plan, err := d.PrepareLaunch(gs, state.LaunchModeCreate, "/repo", "gemini", state.LaunchOptions{
		Worktree: state.WorktreeOption{Enabled: true},
	}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "gemini" {
		t.Errorf("PrepareLaunch.Command = %q, want %q", plan.Command, "gemini")
	}
	if plan.StartDir != "/repo/.roost/worktrees/feature" {
		t.Errorf("StartDir = %q", plan.StartDir)
	}
}

func TestGeminiPrepareLaunchWorktreeFromCommand(t *testing.T) {
	d, gs, _ := newGemini(t)
	plan, err := d.PrepareLaunch(gs, state.LaunchModeCreate, "/repo", "gemini --worktree", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "gemini" {
		t.Errorf("PrepareLaunch.Command = %q, want worktree flag stripped", plan.Command)
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected Options.Worktree.Enabled = true")
	}
}

func TestGeminiPrepareLaunchWorktreeEnabledFromOptions(t *testing.T) {
	d, gs, _ := newGemini(t)
	plan, err := d.PrepareLaunch(gs, state.LaunchModeCreate, "/repo", "gemini", state.LaunchOptions{
		Worktree: state.WorktreeOption{Enabled: true},
	}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "gemini" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", plan.Command, "gemini")
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected Options.Worktree.Enabled = true")
	}
}

func TestGeminiPrepareLaunchSandboxedAddsYolo(t *testing.T) {
	d, gs, _ := newGemini(t)
	plan, err := d.PrepareLaunch(gs, state.LaunchModeCreate, "/repo", "gemini", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if got := plan.Command; got != "gemini --yolo" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, "gemini --yolo")
	}
}

func TestGeminiPrepareLaunchSandboxedResumeAddsYolo(t *testing.T) {
	d, gs, _ := newGemini(t)
	gs.GeminiSessionID = "sess-123"
	plan, err := d.PrepareLaunch(gs, state.LaunchModeColdStart, "/repo", "gemini", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	// Expected order: gemini --yolo --resume sess-123
	// Note: stripGeminiWorktreeFlag strips --worktree/--workspace, but keeps --yolo
	if got := plan.Command; got != "gemini --yolo --resume sess-123" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, "gemini --yolo --resume sess-123")
	}
}

func TestGeminiPrepareLaunchSandboxedNoDuplicateYolo(t *testing.T) {
	d, gs, _ := newGemini(t)
	plan, err := d.PrepareLaunch(gs, state.LaunchModeCreate, "/repo", "gemini --yolo", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if got := plan.Command; got != "gemini --yolo" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, "gemini --yolo")
	}
}

func TestGeminiSessionStartSetsIdle(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.StartDir = "/repo"
	ev := geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "SessionStart",
		"transcript_path": "/tmp/t.jsonl",
		"source":          "startup",
	}, now)
	ev.RoostSessionID = "r1"
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, ev)

	if next.Status != state.StatusIdle {
		t.Fatalf("Status = %v, want idle", next.Status)
	}
	if next.GeminiSessionID != "sess-1" {
		t.Fatalf("GeminiSessionID = %q", next.GeminiSessionID)
	}
	if next.RoostSessionID != "r1" {
		t.Fatalf("RoostSessionID = %q", next.RoostSessionID)
	}
	if next.StartDir != "/repo" || next.TranscriptPath != "/tmp/t.jsonl" {
		t.Fatalf("working data not absorbed: %+v", next)
	}
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if !next.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should be true")
	}
	foundBranch := false
	foundWatch := false
	foundTranscriptParse := false
	for _, eff := range effs {
		if watch, ok := eff.(state.EffWatchFile); ok {
			foundWatch = watch.Path == "/tmp/t.jsonl"
		}
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			foundBranch = true
		}
		if _, ok := job.Input.(GeminiTranscriptParseInput); ok {
			foundTranscriptParse = true
		}
	}
	if !foundBranch {
		t.Fatal("expected BranchDetectInput job")
	}
	if !foundWatch {
		t.Fatal("expected EffWatchFile")
	}
	if !foundTranscriptParse {
		t.Fatal("expected GeminiTranscriptParseInput job")
	}
}

func TestGeminiSessionStartNonRootSkipsBranchDetect(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.StartDir = "/repo"
	ev := geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "SessionStart",
	}, now)
	ev.RoostSessionID = "r1"
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: false}, ev)
	// Non-root: BranchDetect must NOT be emitted.
	if next.BranchInFlight {
		t.Error("BranchInFlight should be false for non-root frame")
	}
	if len(effs) != 0 {
		t.Fatalf("non-root SessionStart effects = %d, want 0", len(effs))
	}
	if next.GeminiSessionID != "sess-1" {
		t.Errorf("GeminiSessionID = %q", next.GeminiSessionID)
	}
	if next.StartDir != "/repo" {
		t.Errorf("StartDir = %q", next.StartDir)
	}
	if next.Status != state.StatusIdle {
		t.Errorf("Status = %v, want Idle", next.Status)
	}
}

func TestGeminiBeforeAgentTransitionsRunning(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "BeforeAgent",
		"prompt":          "do something",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	if next.LastPrompt != "do something" {
		t.Fatalf("LastPrompt = %q", next.LastPrompt)
	}
	if !next.SummaryInFlight {
		t.Fatal("SummaryInFlight should be true")
	}
	var summaryJob SummaryCommandInput
	foundSummary := false
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if in, ok := job.Input.(SummaryCommandInput); ok {
			foundSummary = true
			summaryJob = in
		}
	}
	if !foundSummary {
		t.Fatal("expected SummaryCommandInput job")
	}
	if !strings.Contains(summaryJob.Prompt, "do something") {
		t.Fatalf("summary prompt missing prompt: %q", summaryJob.Prompt)
	}
}

func TestGeminiBeforeToolTransitionsRunning(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	next, _ := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "BeforeTool",
		"tool_name":       "read_file",
		"tool_use_id":     "tool-1",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	if next.CurrentTool != "read_file" {
		t.Fatalf("CurrentTool = %q, want read_file", next.CurrentTool)
	}
}

func TestGeminiAfterToolTransitionsRunning(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.StartDir = "/repo"
	gs, _ = d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "BeforeTool",
		"tool_name":       "read_file",
		"tool_use_id":     "tool-1",
	}, now))
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "AfterTool",
		"tool_name":       "read_file",
		"tool_use_id":     "tool-1",
	}, now.Add(2*time.Second)))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	appendEff, ok := findEffect[state.EffToolLogAppend](effs)
	if !ok {
		t.Fatal("expected EffToolLogAppend")
	}
	var entry toolLogEntry
	if err := json.Unmarshal([]byte(appendEff.Line), &entry); err != nil {
		t.Fatalf("unmarshal tool log: %v", err)
	}
	if entry.Kind != "auto" {
		t.Fatalf("Kind = %q, want auto", entry.Kind)
	}
}

func TestGeminiAfterAgentTransitionsWaiting(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.Status = state.StatusRunning
	gs.TranscriptPath = "/tmp/t.jsonl"
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "AfterAgent",
		"prompt_response": "here is my answer",
	}, now))
	if next.Status != state.StatusWaiting {
		t.Fatalf("Status = %v, want waiting", next.Status)
	}
	if next.LastAssistantMessage != "here is my answer" {
		t.Fatalf("LastAssistantMessage = %q", next.LastAssistantMessage)
	}
	foundTranscriptParse := false
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(GeminiTranscriptParseInput); ok {
			foundTranscriptParse = true
		}
	}
	if !foundTranscriptParse {
		t.Fatal("expected GeminiTranscriptParseInput job")
	}
}

func TestGeminiNotificationToolPermissionTransitionsPending(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.PendingTools = map[string]geminiPendingTool{
		"tool-1": {Name: "Bash", StartedAt: now.Add(-time.Second)},
	}
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":        "sess-1",
		"hook_event_name":   "Notification",
		"notification_type": "ToolPermission",
	}, now))
	if next.Status != state.StatusPending {
		t.Fatalf("Status = %v, want pending", next.Status)
	}
	if _, ok := findEffect[state.EffEventLogAppend](effs); !ok {
		t.Fatal("expected EffEventLogAppend")
	}
	if !next.PendingTools["tool-1"].SawPrompt {
		t.Fatal("pending tool should be marked approved prompt")
	}
}

func TestGeminiNotificationUnknownTypeDoesNotChangeStatus(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.Status = state.StatusRunning
	gs.StatusChangedAt = now.Add(-time.Minute)
	next, _ := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":        "sess-1",
		"hook_event_name":   "Notification",
		"notification_type": "something_else",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
}

func TestGeminiSessionEndTransitionsStopped(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.Status = state.StatusWaiting
	gs.CurrentTool = "Bash"
	gs.PendingTools = map[string]geminiPendingTool{"tool-1": {Name: "Bash"}}
	next, _ := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "SessionEnd",
	}, now))
	if next.Status != state.StatusStopped {
		t.Fatalf("Status = %v, want stopped", next.Status)
	}
	if next.CurrentTool != "" || len(next.PendingTools) != 0 {
		t.Fatalf("session end should clear tool state: %+v", next)
	}
}

func TestGeminiDropsStaleHook(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.LastHookAt = now
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "AfterAgent",
	}, now))
	if next.Status != gs.Status {
		t.Fatal("stale hook should not update status")
	}
	if len(effs) != 0 {
		t.Fatalf("effects = %d, want 0", len(effs))
	}
}

func TestGeminiEmptySessionIDDropped(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusRunning
	next, effs := d.handleHook(gs, state.FrameContext{IsRoot: true}, geminiHook(map[string]string{
		"hook_event_name": "AfterAgent",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatal("empty session_id should not change status")
	}
	if len(effs) != 0 {
		t.Fatalf("effects = %d, want 0", len(effs))
	}
}

func TestGeminiPersistRestoreWorktreeName(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.GeminiSessionID = "sess-1"
	gs.WorktreeName = "gemini-1234"

	bag := d.Persist(gs)
	restored := d.Restore(bag, now).(GeminiState)

	if restored.WorktreeName != gs.WorktreeName {
		t.Fatalf("WorktreeName = %q, want %q", restored.WorktreeName, gs.WorktreeName)
	}
}

func TestGeminiWindowTitleActionRequiredTransitionsPending(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusRunning

	next := d.handleWindowTitle(gs, "[ ! ] Action Required | agent-roost", now)

	if next.Status != state.StatusPending {
		t.Fatalf("Status = %v, want pending", next.Status)
	}
	if next.LastWindowTitle != "[ ! ] Action Required | agent-roost" {
		t.Fatalf("LastWindowTitle = %q", next.LastWindowTitle)
	}
}

func TestGeminiWindowTitleHandTransitionsPending(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusRunning

	next := d.handleWindowTitle(gs, "✋ Action Required | Gemini CLI", now)

	if next.Status != state.StatusPending {
		t.Fatalf("Status = %v, want pending", next.Status)
	}
}

func TestGeminiWindowTitleWorkingTransitionsRunning(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusWaiting

	next := d.handleWindowTitle(gs, "✦ Thinking about files", now)

	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
}

func TestGeminiWindowTitleReadyTransitionsWaiting(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusRunning

	next := d.handleWindowTitle(gs, "◇ Ready", now)

	if next.Status != state.StatusWaiting {
		t.Fatalf("Status = %v, want waiting", next.Status)
	}
}

func TestGeminiWindowTitleIdleUnchanged(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusIdle
	gs.StatusChangedAt = now.Add(-time.Minute)

	next := d.handleWindowTitle(gs, "✦ Thinking about files", now)

	if next.Status != state.StatusIdle {
		t.Fatalf("Status = %v, want idle", next.Status)
	}
	if !next.StatusChangedAt.Equal(gs.StatusChangedAt) {
		t.Fatalf("StatusChangedAt = %v, want %v", next.StatusChangedAt, gs.StatusChangedAt)
	}
}

func TestGeminiWindowTitleStoppedUnchanged(t *testing.T) {
	d, gs, now := newGemini(t)
	gs.Status = state.StatusStopped
	gs.StatusChangedAt = now.Add(-time.Minute)

	next := d.handleWindowTitle(gs, "◇ Ready", now)

	if next.Status != state.StatusStopped {
		t.Fatalf("Status = %v, want stopped", next.Status)
	}
	if !next.StatusChangedAt.Equal(gs.StatusChangedAt) {
		t.Fatalf("StatusChangedAt = %v, want %v", next.StatusChangedAt, gs.StatusChangedAt)
	}
}
