package driver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	codextranscript "github.com/takezoh/agent-roost/lib/codex/transcript"
	"github.com/takezoh/agent-roost/state"
)

func codexHook(fields map[string]string, ts time.Time) state.DEvHook {
	raw, _ := json.Marshal(fields)
	return state.DEvHook{Payload: raw, Timestamp: ts}
}

func newCodex(t *testing.T) (CodexDriver, CodexState, time.Time) {
	t.Helper()
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	d := NewCodexDriver("/tmp/events")
	cs := d.NewState(now).(CodexState)
	return d, cs, now
}

func findCodexEffect[T state.Effect](effs []state.Effect) (T, bool) {
	var zero T
	for _, e := range effs {
		if v, ok := e.(T); ok {
			return v, true
		}
	}
	return zero, false
}

func TestCodexSessionStartSetsIdle(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ev := codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "SessionStart",
		"transcript_path": "/tmp/t.jsonl",
		"source":          "resume",
	}, now)
	ev.RoostSessionID = "r1"
	next, effs := d.handleHook(cs, state.FrameContext{IsRoot: true}, ev)

	if next.Status != state.StatusIdle {
		t.Fatalf("Status = %v, want idle", next.Status)
	}
	if next.CodexSessionID != "sess-1" {
		t.Fatalf("CodexSessionID = %q", next.CodexSessionID)
	}
	if next.RoostSessionID != "r1" {
		t.Fatalf("RoostSessionID = %q", next.RoostSessionID)
	}
	if next.StartDir != "/repo" || next.TranscriptPath != "/tmp/t.jsonl" {
		t.Fatalf("working data not absorbed: %+v", next)
	}
	if len(effs) != 4 {
		t.Fatalf("effects = %d, want 4", len(effs))
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
		if _, ok := job.Input.(CodexTranscriptParseInput); ok {
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
		t.Fatal("expected CodexTranscriptParseInput job")
	}
}

func TestCodexSessionStartNonRootSkipsBranchDetect(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	ev := codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "SessionStart",
		"transcript_path": "/tmp/t.jsonl",
	}, now)
	ev.RoostSessionID = "r1"
	next, effs := d.handleHook(cs, state.FrameContext{IsRoot: false}, ev)
	// Non-root: BranchDetect must NOT be emitted.
	if next.BranchInFlight {
		t.Error("BranchInFlight should be false for non-root frame")
	}
	if len(effs) != 0 {
		t.Fatalf("non-root SessionStart effects = %d, want 0", len(effs))
	}
	if next.CodexSessionID != "sess-1" {
		t.Errorf("CodexSessionID = %q", next.CodexSessionID)
	}
	if next.StartDir != "/repo" {
		t.Errorf("StartDir = %q", next.StartDir)
	}
	if next.TranscriptPath != "" {
		t.Errorf("TranscriptPath = %q, want empty", next.TranscriptPath)
	}
}

func TestCodexBootstrapSessionStartSetsIdleWithoutIdentity(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusRunning
	cs.StartDir = "/repo"
	nextState, effs := d.BootstrapSessionStart(cs, state.FrameContext{
		ID:      "frame-1",
		Project: "/repo",
		Command: "codex",
		IsRoot:  true,
	}, now)
	next := nextState.(CodexState)

	if next.Status != state.StatusIdle {
		t.Fatalf("Status = %v, want idle", next.Status)
	}
	if next.CodexSessionID != "" {
		t.Fatalf("CodexSessionID = %q, want empty", next.CodexSessionID)
	}
	if next.TranscriptPath != "" {
		t.Fatalf("TranscriptPath = %q, want empty", next.TranscriptPath)
	}
	if !next.BranchInFlight {
		t.Fatal("BranchInFlight should be true")
	}
	if next.BranchTarget != "/repo" {
		t.Fatalf("BranchTarget = %q", next.BranchTarget)
	}
	if next.WatchedFile != "" {
		t.Fatalf("WatchedFile = %q, want empty", next.WatchedFile)
	}

	foundBranch := false
	foundLog := false
	for _, eff := range effs {
		if logEff, ok := eff.(state.EffEventLogAppend); ok && logEff.Line == "[event:SessionStart] startup" {
			foundLog = true
		}
		job, ok := eff.(state.EffStartJob)
		if ok {
			_, foundBranch = job.Input.(BranchDetectInput)
		}
	}
	if !foundBranch {
		t.Fatal("expected BranchDetectInput job")
	}
	if !foundLog {
		t.Fatal("expected SessionStart startup event log")
	}
}

func TestCodexBootstrapThenHookAbsorbsIdentity(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	nextState, _ := d.BootstrapSessionStart(cs, state.FrameContext{
		ID:      "frame-1",
		Project: "/repo",
		Command: "codex",
		IsRoot:  true,
	}, now)

	next, effs := d.handleHook(nextState.(CodexState), state.FrameContext{IsRoot: true}, codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "UserPromptSubmit",
		"transcript_path": "/tmp/t.jsonl",
		"prompt":          "implement this",
	}, now.Add(time.Second)))

	if next.CodexSessionID != "sess-1" {
		t.Fatalf("CodexSessionID = %q", next.CodexSessionID)
	}
	if next.TranscriptPath != "/tmp/t.jsonl" {
		t.Fatalf("TranscriptPath = %q", next.TranscriptPath)
	}
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q", next.WatchedFile)
	}
	if _, ok := findCodexEffect[state.EffWatchFile](effs); !ok {
		t.Fatal("expected EffWatchFile")
	}
}

func TestCodexUserPromptTransitionsRunning(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.RecentTurns = []SummaryTurn{
		{Role: "user", Text: "inspect repo"},
		{Role: "assistant", Text: "checking files"},
		{Role: "user", Text: "find failing tests"},
		{Role: "assistant", Text: "found driver failures"},
	}
	next, effs := d.handleHook(cs, state.FrameContext{IsRoot: true}, codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "UserPromptSubmit",
		"prompt":          "implement this",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	if next.LastPrompt != "implement this" {
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
		in, ok := job.Input.(SummaryCommandInput)
		if ok {
			summaryJob = in
			foundSummary = true
		}
	}
	if !foundSummary {
		t.Fatal("expected SummaryCommandInput job")
	}
	if strings.Contains(summaryJob.Prompt, "inspect repo") {
		t.Fatalf("prompt should keep only last 2 user turns: %q", summaryJob.Prompt)
	}
	if !strings.Contains(summaryJob.Prompt, "find failing tests") || !strings.Contains(summaryJob.Prompt, "implement this") {
		t.Fatalf("summary prompt missing recent user turns: %q", summaryJob.Prompt)
	}
}

func TestCodexStopTransitionsWaiting(t *testing.T) {
	d, cs, now := newCodex(t)
	next, _ := d.handleHook(cs, state.FrameContext{IsRoot: true}, codexHook(map[string]string{
		"session_id":             "sess-1",
		"hook_event_name":        "Stop",
		"last_assistant_message": "done",
		"stop_reason":            "finished",
	}, now))
	if next.Status != state.StatusWaiting {
		t.Fatalf("Status = %v, want waiting", next.Status)
	}
	if next.LastAssistantMessage != "done" {
		t.Fatalf("LastAssistantMessage = %q", next.LastAssistantMessage)
	}
}

func TestCodexPendingTransitionsToRunningOnPreToolUse(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusPending
	cs.StatusChangedAt = now.Add(-time.Minute)
	next, _ := d.handleHook(cs, state.FrameContext{IsRoot: true}, codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
	}, now))
	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
	if next.CurrentTool != "Bash" {
		t.Fatalf("CurrentTool = %q, want Bash", next.CurrentTool)
	}
}

func TestCodexWindowTitleActionRequiredTransitionsPending(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusRunning

	next := d.handleWindowTitle(cs, "[ ! ] Action Required | agent-roost", now)

	if next.Status != state.StatusPending {
		t.Fatalf("Status = %v, want pending", next.Status)
	}
	if next.StatusChangedAt != now {
		t.Fatalf("StatusChangedAt = %v, want %v", next.StatusChangedAt, now)
	}
	if next.LastWindowTitle != "[ ! ] Action Required | agent-roost" {
		t.Fatalf("LastWindowTitle = %q", next.LastWindowTitle)
	}
}

func TestCodexWindowTitleSpinnerDoesNotTransitionPending(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusRunning

	next := d.handleWindowTitle(cs, "⠹ agent-roost", now)

	if next.Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.Status)
	}
}

func TestCodexWindowTitleViaStepIgnoresNonRoot(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusRunning

	next, _, _ := d.Step(cs, state.FrameContext{IsRoot: false}, state.DEvPaneOsc{
		Cmd:   0,
		Title: "[ ! ] Action Required | agent-roost",
		Now:   now,
	})

	if next.(CodexState).Status != state.StatusRunning {
		t.Fatalf("Status = %v, want running", next.(CodexState).Status)
	}
}

func TestCodexDropsStaleHook(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.LastHookAt = now
	next, effs := d.handleHook(cs, state.FrameContext{IsRoot: true}, codexHook(map[string]string{
		"session_id":      "sess-1",
		"hook_event_name": "Stop",
	}, now))
	if next.Status != cs.Status {
		t.Fatal("stale hook should not update status")
	}
	if len(effs) != 0 {
		t.Fatalf("effects = %d, want 0", len(effs))
	}
}

func TestCodexPrepareLaunchResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.CodexSessionID = "abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	want := "codex --model gpt-5-codex resume abc-123"
	if got != want {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, want)
	}
}

func TestCodexPrepareLaunchNoDoubleResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.CodexSessionID = "abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex resume abc", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	if got != "codex resume abc" {
		t.Fatalf("PrepareLaunch.Command = %q", got)
	}
}

func TestCodexPrepareLaunchStripsWorktreeOnResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.CodexSessionID = "abc-123"
	cs.ManagedWorkingDir = "/repo/.roost/worktrees/codex-1234"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex --worktree feature --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	want := "codex --model gpt-5-codex resume abc-123"
	if got != want {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, want)
	}
}

func TestCodexPrepareLaunchStripsWorktreeWithoutResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ManagedWorkingDir = "/repo/.roost/worktrees/codex-1234"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --worktree feature --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	want := "codex --model gpt-5-codex"
	if got != want {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, want)
	}
}

func TestCodexPrepareLaunchAddsYoloWhenSandboxed(t *testing.T) {
	d, cs, _ := newCodex(t)
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --model gpt-5-codex", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	want := "codex --model gpt-5-codex --yolo"
	if got != want {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, want)
	}
}

func TestCodexPrepareLaunchDoesNotDuplicateYolo(t *testing.T) {
	d, cs, _ := newCodex(t)
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --model gpt-5-codex --yolo", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	want := "codex --model gpt-5-codex --yolo"
	if got != want {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", got, want)
	}
}

func TestCodexPrepareLaunchSkipsNonCodexBaseCommand(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.CodexSessionID = "abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "env FOO=bar", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	got := plan.Command
	if got != "env FOO=bar" {
		t.Fatalf("PrepareLaunch.Command = %q", got)
	}
}

func TestCodexPersistRestoreRoundTrip(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.CommonState = CommonState{
		RoostSessionID:       "r1",
		StartDir:             "/repo",
		TranscriptPath:       "/repo/t.jsonl",
		WorktreeName:         "codex-abcd",
		Status:               state.StatusRunning,
		StatusChangedAt:      now,
		BranchTag:            "main",
		BranchBG:             "#111111",
		BranchFG:             "#ffffff",
		BranchTarget:         "/repo",
		BranchAt:             now,
		BranchIsWorktree:     true,
		BranchParentBranch:   "origin/main",
		LastPrompt:           "p",
		LastAssistantMessage: "a",
		LastHookEvent:        "Stop",
		LastHookAt:           now,
	}
	cs.CodexSessionID = "c1"
	cs.ManagedWorkingDir = "/repo/.roost/worktrees/codex-abcd"

	bag := d.Persist(cs)
	got := d.Restore(bag, now.Add(time.Hour)).(CodexState)
	if got.CodexSessionID != "c1" || got.StartDir != "/repo" {
		t.Fatalf("restore mismatch: %+v", got)
	}
	if got.ManagedWorkingDir != "/repo/.roost/worktrees/codex-abcd" || got.WorktreeName != "codex-abcd" {
		t.Fatalf("worktree fields mismatch: %+v", got)
	}
	if got.Status != state.StatusRunning {
		t.Fatalf("Status = %v", got.Status)
	}
	if got.BranchTag != "main" || got.BranchParentBranch != "origin/main" {
		t.Fatalf("branch fields mismatch: %+v", got)
	}
	if got.LastPrompt != "p" || got.LastAssistantMessage != "a" {
		t.Fatalf("message fields mismatch: %+v", got)
	}
}

func TestCodexWarmStartRecoverReinstallsTranscriptWatch(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.TranscriptPath = "/tmp/t.jsonl"
	nextState, effs := d.WarmStartRecover(cs, now)
	next := nextState.(CodexState)
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if !next.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should be true")
	}
	if len(effs) != 2 {
		t.Fatalf("effects = %d, want 2", len(effs))
	}
	if _, ok := effs[0].(state.EffWatchFile); !ok {
		t.Fatalf("first effect = %T, want EffWatchFile", effs[0])
	}
	job, ok := effs[1].(state.EffStartJob)
	if !ok {
		t.Fatalf("second effect = %T, want EffStartJob", effs[1])
	}
	if _, ok := job.Input.(CodexTranscriptParseInput); !ok {
		t.Fatalf("job input = %T, want CodexTranscriptParseInput", job.Input)
	}
	if next.Status != cs.Status {
		t.Fatalf("Status = %v, want %v", next.Status, cs.Status)
	}
}

func TestCodexWarmStartRecoverDedupesTranscriptParse(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.TranscriptPath = "/tmp/t.jsonl"
	cs.TranscriptInFlight = true
	nextState, effs := d.WarmStartRecover(cs, now)
	next := nextState.(CodexState)
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if len(effs) != 1 {
		t.Fatalf("effects = %d, want 1", len(effs))
	}
	if _, ok := effs[0].(state.EffWatchFile); !ok {
		t.Fatalf("effect = %T, want EffWatchFile", effs[0])
	}
}

func TestCodexTranscriptChangedStartsParse(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.TranscriptPath = "/tmp/t.jsonl"
	next, effs := d.handleTranscriptChanged(cs, state.DEvFileChanged{Path: "/tmp/t.jsonl"})
	if next.WatchedFile != "/tmp/t.jsonl" {
		t.Fatalf("WatchedFile = %q, want /tmp/t.jsonl", next.WatchedFile)
	}
	if !next.TranscriptInFlight {
		t.Fatal("expected TranscriptInFlight")
	}
	if len(effs) != 2 {
		t.Fatalf("effects = %d, want 2", len(effs))
	}
	if _, ok := effs[0].(state.EffWatchFile); !ok {
		t.Fatalf("first effect = %T, want EffWatchFile", effs[0])
	}
	job, ok := effs[1].(state.EffStartJob)
	if !ok {
		t.Fatalf("second effect = %T, want EffStartJob", effs[1])
	}
	if _, ok := job.Input.(CodexTranscriptParseInput); !ok {
		t.Fatalf("job input = %T, want CodexTranscriptParseInput", job.Input)
	}
}

func TestCodexTranscriptParseResultMergesFields(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.TranscriptInFlight = true
	cs.Status = state.StatusRunning
	cs.CurrentTool = "Bash"
	next := d.handleJobResult(cs, state.DEvJobResult{
		Now: now,
		Result: CodexTranscriptParseResult{
			Title:                "saved-session",
			LastPrompt:           "Run tests",
			LastAssistantMessage: "done",
			StatusLine:           "gpt-5-codex | 7,205 tok",
			RecentTurns: []SummaryTurn{
				{Role: "user", Text: "Run tests"},
				{Role: "assistant", Text: "done"},
			},
		},
	})
	if next.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should clear")
	}
	if next.Title != "saved-session" || next.LastPrompt != "Run tests" {
		t.Fatalf("unexpected transcript fields: %+v", next)
	}
	if next.LastAssistantMessage != "done" || next.StatusLine != "gpt-5-codex | 7,205 tok" {
		t.Fatalf("unexpected transcript fields: %+v", next)
	}
	if next.CurrentTool != "Bash" {
		t.Fatalf("CurrentTool = %q, want preserved while running", next.CurrentTool)
	}
	if len(next.RecentTurns) != 2 {
		t.Fatalf("RecentTurns len = %d, want 2", len(next.RecentTurns))
	}
}

func TestCodexSummaryResultMergesFields(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.SummaryInFlight = true
	next := d.handleJobResult(cs, state.DEvJobResult{
		Now:    now,
		Result: SummaryCommandResult{Summary: "test failures investigation"},
	})
	if next.SummaryInFlight {
		t.Fatal("SummaryInFlight should clear")
	}
	if next.Summary != "test failures investigation" {
		t.Fatalf("Summary = %q", next.Summary)
	}
}

func TestCodexViewAddsTranscriptTab(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.TranscriptPath = "/tmp/t.jsonl"
	cs.Title = "saved-session"
	cs.Summary = "session summary"
	cs.CurrentTool = "Bash"
	v := d.view(cs)
	if len(v.LogTabs) == 0 {
		t.Fatal("expected tabs")
	}
	if v.LogTabs[0].Label != "TRANSCRIPT" {
		t.Fatalf("first tab = %q", v.LogTabs[0].Label)
	}
	if v.LogTabs[0].Kind != codextranscript.KindTranscript {
		t.Fatalf("tab kind = %q", v.LogTabs[0].Kind)
	}
	if v.Card.Title != "saved-session" {
		t.Fatalf("title = %q", v.Card.Title)
	}
	if v.Card.Subtitle != "session summary" {
		t.Fatalf("subtitle = %q", v.Card.Subtitle)
	}
	if len(v.Card.Indicators) != 1 || v.Card.Indicators[0] != "▸ Bash" {
		t.Fatalf("indicators = %#v", v.Card.Indicators)
	}
}

func TestParseCodexWorktree(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantReq worktreeRequest
		wantCmd string
	}{
		{"none", "codex --model gpt-5", worktreeRequest{}, "codex --model gpt-5"},
		{"bare", "codex --worktree", worktreeRequest{Enabled: true}, "codex"},
		{"spaced", "codex --worktree feature --model gpt-5", worktreeRequest{Enabled: true, Name: "feature"}, "codex --model gpt-5"},
		{"equals", "codex --model gpt-5 --worktree=feature", worktreeRequest{Enabled: true, Name: "feature"}, "codex --model gpt-5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReq, gotCmd := parseWorktreeFlags(tt.command, "--worktree")
			if gotReq != tt.wantReq || gotCmd != tt.wantCmd {
				t.Fatalf("parseWorktreeFlags(%q) = (%+v, %q), want (%+v, %q)", tt.command, gotReq, gotCmd, tt.wantReq, tt.wantCmd)
			}
		})
	}
}

func TestGeneratedWorktreeNamesLookLikePetnames(t *testing.T) {
	names := generatedWorktreeNames()
	if len(names) != worktreeNameAttempts {
		t.Fatalf("len(names) = %d, want %d", len(names), worktreeNameAttempts)
	}
	for _, name := range names {
		if parts := strings.Split(name, "-"); len(parts) != 4 {
			t.Fatalf("name = %q, want 4 hyphen-separated words", name)
		}
	}
}

func TestCodexPrepareCreateWithoutWorktree(t *testing.T) {
	d, cs, _ := newCodex(t)
	next, plan, err := d.PrepareCreate(cs, "sess-1", "/repo", "codex --model gpt-5", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	got := next.(CodexState)
	if got.WorktreeName != "" {
		t.Fatalf("unexpected worktree state: %+v", next)
	}
	if got.StartDir != "/repo" {
		t.Fatalf("StartDir = %q, want /repo", got.StartDir)
	}
	if plan.SetupJob != nil {
		t.Fatal("expected no setup job")
	}
	if plan.Launch.Command != "codex --model gpt-5" || plan.Launch.StartDir != "/repo" {
		t.Fatalf("launch = %+v", plan.Launch)
	}
}

func TestCodexPrepareCreateWithWorktree(t *testing.T) {
	d, cs, _ := newCodex(t)
	next, plan, err := d.PrepareCreate(cs, "sess-1", "/repo", "codex --worktree feature --model gpt-5", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	got := next.(CodexState)
	if got.WorktreeName != "feature" {
		t.Fatalf("WorktreeName = %q", got.WorktreeName)
	}
	if plan.Launch.Command != "codex --model gpt-5" {
		t.Fatalf("launch command = %q", plan.Launch.Command)
	}
	in, ok := plan.SetupJob.(WorktreeSetupInput)
	if !ok {
		t.Fatalf("SetupJob = %T, want WorktreeSetupInput", plan.SetupJob)
	}
	if in.RepoDir != "/repo" || len(in.CandidateNames) != 1 || in.CandidateNames[0] != "feature" {
		t.Fatalf("setup input = %+v", in)
	}
}

func TestCodexCompleteCreateWithWorktree(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.WorktreeName = "feature"
	next, launch, err := d.CompleteCreate(cs, "codex --worktree feature --model gpt-5", state.LaunchOptions{}, WorktreeSetupResult{
		StartDir: "/repo/.roost/worktrees/feature",
		Name:     "feature",
	}, nil)
	if err != nil {
		t.Fatalf("CompleteCreate error: %v", err)
	}
	got := next.(CodexState)
	if got.ManagedWorkingDir != "/repo/.roost/worktrees/feature" || got.StartDir != "/repo/.roost/worktrees/feature" {
		t.Fatalf("working dir fields = %+v", got)
	}
	if launch.Command != "codex --worktree feature --model gpt-5" || launch.StartDir != "/repo/.roost/worktrees/feature" {
		t.Fatalf("launch = %+v", launch)
	}
}

func TestCodexManagedWorktreePath(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ManagedWorkingDir = "/repo/.roost/worktrees/feature"
	if got := d.ManagedWorktreePath(cs); got != "/repo/.roost/worktrees/feature" {
		t.Fatalf("ManagedWorktreePath = %q", got)
	}
	cs.ManagedWorkingDir = "/repo/feature"
	if got := d.ManagedWorktreePath(cs); got != "" {
		t.Fatalf("ManagedWorktreePath = %q, want empty", got)
	}
}

func TestCodexViewIncludesEventsTab(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.RoostSessionID = "r1"
	cs.BranchTag = "feat-x"
	cs.BranchBG = "#123456"
	cs.BranchFG = "#ffffff"
	v := d.view(cs)
	if len(v.LogTabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(v.LogTabs))
	}
	if v.LogTabs[0].Label != "EVENTS" {
		t.Fatalf("tab label = %q", v.LogTabs[0].Label)
	}
	if len(v.Card.Tags) != 1 {
		t.Fatalf("tags = %d, want 1", len(v.Card.Tags))
	}
	if v.Card.Tags[0].Text != "feat-x" {
		t.Fatalf("tag text = %q", v.Card.Tags[0].Text)
	}
	if v.Card.Tags[0].Background != "#123456" {
		t.Fatalf("tag bg = %q", v.Card.Tags[0].Background)
	}
	if v.Card.Tags[0].Foreground != "#ffffff" {
		t.Fatalf("tag fg = %q", v.Card.Tags[0].Foreground)
	}
	if v.Card.BorderTitle.Text != CodexDriverName {
		t.Fatalf("border title text = %q", v.Card.BorderTitle.Text)
	}
	if v.Card.BorderTitle.Background != codexTagBg {
		t.Fatalf("border title bg = %q, want %q", v.Card.BorderTitle.Background, codexTagBg)
	}
	if v.Card.BorderTitle.Foreground != codexTagFg {
		t.Fatalf("border title fg = %q, want %q", v.Card.BorderTitle.Foreground, codexTagFg)
	}
}

func TestCodexBranchDetectJobResultUpdatesTag(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.BranchInFlight = true
	next, _, _ := d.Step(cs, state.FrameContext{IsRoot: true}, state.DEvJobResult{
		Now: now,
		Result: BranchDetectResult{
			Branch:       "main",
			Background:   "#222222",
			Foreground:   "#ffffff",
			IsWorktree:   true,
			ParentBranch: "origin/main",
		},
	})
	got := next.(CodexState)
	if got.BranchInFlight {
		t.Fatal("BranchInFlight should be false")
	}
	if got.BranchTag != "main" || got.BranchParentBranch != "origin/main" {
		t.Fatalf("branch state mismatch: %+v", got)
	}
}
