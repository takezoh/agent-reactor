package driver

import (
	"strings"
	"testing"
	"time"

	codextranscript "github.com/takezoh/agent-roost/client/lib/codex/transcript"
	"github.com/takezoh/agent-roost/client/state"
)

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

func TestCodexSubsystemSessionReadySetsIdle(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	cs.Status = state.StatusRunning
	next, effs := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemSessionReady,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID:         "thread-1",
			TargetID:          "thread-1",
			RequestedTargetID: "thread-1",
			ObservedTargetID:  "thread-1",
			ResumePhase:       "attached",
		},
	})
	if next.Status != state.StatusIdle {
		t.Fatalf("Status = %v, want idle", next.Status)
	}
	if next.ThreadID != "thread-1" {
		t.Fatalf("ThreadID = %q", next.ThreadID)
	}
	if next.RequestedThreadID != "thread-1" {
		t.Fatalf("RequestedThreadID = %q", next.RequestedThreadID)
	}
	if next.ObservedThreadID != "thread-1" {
		t.Fatalf("ObservedThreadID = %q", next.ObservedThreadID)
	}
	if next.ResumePhase != "attached" {
		t.Fatalf("ResumePhase = %q", next.ResumePhase)
	}
	foundBranch := false
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if !ok {
			continue
		}
		if _, ok := job.Input.(BranchDetectInput); ok {
			foundBranch = true
		}
	}
	if !foundBranch {
		t.Fatal("expected BranchDetectInput job")
	}
}

func TestCodexSubsystemSessionReadyNonRootSkipsBranchDetect(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	next, effs := d.handleSubsystem(cs, state.FrameContext{IsRoot: false}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemSessionReady,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			TargetID:  "thread-1",
		},
	})
	if next.BranchInFlight {
		t.Error("BranchInFlight should be false for non-root frame")
	}
	if len(effs) != 0 {
		t.Fatalf("non-root SessionReady effects = %d, want 0", len(effs))
	}
}

func TestCodexBootstrapSessionStartSetsIdle(t *testing.T) {
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
	if next.ThreadID != "" {
		t.Fatalf("ThreadID = %q, want empty", next.ThreadID)
	}
	if !next.BranchInFlight {
		t.Fatal("BranchInFlight should be true")
	}
	if next.BranchTarget != "/repo" {
		t.Fatalf("BranchTarget = %q", next.BranchTarget)
	}

	foundBranch := false
	for _, eff := range effs {
		job, ok := eff.(state.EffStartJob)
		if ok {
			_, foundBranch = job.Input.(BranchDetectInput)
		}
	}
	if !foundBranch {
		t.Fatal("expected BranchDetectInput job")
	}
}

func TestCodexSubsystemPromptSubmittedTransitionsRunning(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.RecentTurns = []SummaryTurn{
		{Role: "user", Text: "inspect repo"},
		{Role: "assistant", Text: "checking files"},
		{Role: "user", Text: "find failing tests"},
		{Role: "assistant", Text: "found driver failures"},
	}
	next, effs := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemPromptSubmitted,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			TargetID:  "thread-1",
			Prompt:    "implement this",
		},
	})
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

func TestCodexSubsystemTurnCompletedTransitionsWaiting(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.Status = state.StatusRunning
	next, _ := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemTurnCompleted,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID:            "thread-1",
			TargetID:             "thread-1",
			LastAssistantMessage: "done",
		},
	})
	if next.Status != state.StatusWaiting {
		t.Fatalf("Status = %v, want waiting", next.Status)
	}
	if next.LastAssistantMessage != "done" {
		t.Fatalf("LastAssistantMessage = %q", next.LastAssistantMessage)
	}
}

func TestCodexSubsystemApprovalRequestedTransitionsPending(t *testing.T) {
	d, cs, now := newCodex(t)
	next, _ := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemApprovalRequested,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			Approval:  &state.SubsystemApproval{ID: "ap1", Kind: "command"},
		},
	})
	if next.Status != state.StatusPending {
		t.Fatalf("Status = %v, want pending", next.Status)
	}
	if !next.PendingApproval {
		t.Fatal("PendingApproval should be true")
	}
}

func TestCodexSubsystemToolLifecycleUpdatesState(t *testing.T) {
	d, cs, now := newCodex(t)
	cs.StartDir = "/repo"
	start, _ := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolStarted,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			Tool: &state.SubsystemTool{
				ID:      "tool-1",
				Name:    "Bash",
				Command: "go test ./...",
			},
		},
	})
	if start.CurrentTool != "Bash" {
		t.Fatalf("CurrentTool = %q", start.CurrentTool)
	}
	if _, ok := start.PendingTools["tool-1"]; !ok {
		t.Fatal("expected pending tool")
	}

	done, effs := d.handleSubsystem(start, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemToolCompleted,
		Timestamp: now.Add(time.Second),
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			Tool: &state.SubsystemTool{
				ID:      "tool-1",
				Name:    "Bash",
				Command: "go test ./...",
			},
		},
	})
	if done.CurrentTool != "" {
		t.Fatalf("CurrentTool = %q, want empty", done.CurrentTool)
	}
	if _, ok := done.PendingTools["tool-1"]; ok {
		t.Fatal("pending tool should be cleared")
	}
	if _, ok := findCodexEffect[state.EffToolLogAppend](effs); !ok {
		t.Fatal("expected EffToolLogAppend")
	}
}

func TestCodexSubsystemMessageAndPlanUpdates(t *testing.T) {
	d, cs, now := newCodex(t)
	next, _ := d.handleSubsystem(cs, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemMessageUpdated,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID:            "thread-1",
			LastAssistantMessage: "done",
			Message: &state.SubsystemMessage{
				RecentTurns: []state.SubsystemTurn{
					{Role: "user", Text: "do x"},
					{Role: "assistant", Text: "done"},
				},
			},
		},
	})
	if next.LastAssistantMessage != "done" {
		t.Fatalf("LastAssistantMessage = %q", next.LastAssistantMessage)
	}
	if len(next.RecentTurns) != 2 {
		t.Fatalf("RecentTurns = %d", len(next.RecentTurns))
	}

	next, _ = d.handleSubsystem(next, state.FrameContext{IsRoot: true}, state.DEvSubsystem{
		Source:    state.SubsystemStream,
		Kind:      state.SubsystemPlanUpdated,
		Timestamp: now,
		Payload: state.SubsystemPayload{
			SessionID: "thread-1",
			Plan:      &state.SubsystemPlan{Summary: "implement subsystem support"},
		},
	})
	if next.PlanSummary != "implement subsystem support" {
		t.Fatalf("PlanSummary = %q", next.PlanSummary)
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

func TestCodexPrepareLaunchResumesWithThreadID(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ThreadID = "thread-abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "codex --model gpt-5-codex" {
		t.Fatalf("PrepareLaunch.Command = %q", plan.Command)
	}
	if plan.Subsystem != state.LaunchSubsystemStream {
		t.Fatalf("PrepareLaunch.Subsystem = %q, want %q", plan.Subsystem, state.LaunchSubsystemStream)
	}
	if plan.Stream.ResumeThreadID != "thread-abc-123" {
		t.Fatalf("Stream.ResumeThreadID = %q", plan.Stream.ResumeThreadID)
	}
}

func TestCodexPrepareLaunchNoDoubleResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ThreadID = "thread-abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex resume abc", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "codex resume abc" {
		t.Fatalf("PrepareLaunch.Command = %q", plan.Command)
	}
	if plan.Stream.ResumeThreadID != "" {
		t.Fatalf("Stream.ResumeThreadID = %q, want empty", plan.Stream.ResumeThreadID)
	}
}

func TestCodexPrepareLaunchStripsWorktreeOnResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ThreadID = "thread-abc-123"
	cs.StartDir = "/repo/.roost/worktrees/codex-1234"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "codex --worktree feature --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "codex --model gpt-5-codex" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", plan.Command, "codex --model gpt-5-codex")
	}
	if plan.Stream.ResumeThreadID != "thread-abc-123" {
		t.Fatalf("Stream.ResumeThreadID = %q", plan.Stream.ResumeThreadID)
	}
}

func TestCodexPrepareLaunchStripsWorktreeWithoutResume(t *testing.T) {
	d, cs, _ := newCodex(t)
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --worktree feature --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "codex --model gpt-5-codex" {
		t.Fatalf("PrepareLaunch.Command = %q, want %q", plan.Command, "codex --model gpt-5-codex")
	}
	if plan.Stream.ResumeThreadID != "" {
		t.Fatalf("Stream.ResumeThreadID = %q, want empty", plan.Stream.ResumeThreadID)
	}
}

func TestCodexPrepareLaunchSetsExternalSandboxWhenSandboxed(t *testing.T) {
	d, cs, _ := newCodex(t)
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --model gpt-5-codex", state.LaunchOptions{}, true)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "codex --model gpt-5-codex" {
		t.Fatalf("PrepareLaunch.Command = %q", plan.Command)
	}
	if plan.Subsystem != state.LaunchSubsystemStream {
		t.Fatalf("PrepareLaunch.Subsystem = %q, want %q", plan.Subsystem, state.LaunchSubsystemStream)
	}
	if plan.Stream.SandboxPolicy != state.StreamSandboxPolicyExternal {
		t.Fatalf("Stream.SandboxPolicy = %q, want %q", plan.Stream.SandboxPolicy, state.StreamSandboxPolicyExternal)
	}
	if plan.Stream.ApprovalPolicy != state.StreamApprovalPolicyAutoApprove {
		t.Fatalf("Stream.ApprovalPolicy = %q, want %q", plan.Stream.ApprovalPolicy, state.StreamApprovalPolicyAutoApprove)
	}
}

func TestCodexPrepareLaunchLeavesStreamPoliciesDefaultWithoutSandbox(t *testing.T) {
	d, cs, _ := newCodex(t)
	plan, err := d.PrepareLaunch(cs, state.LaunchModeCreate, "/repo", "codex --model gpt-5-codex", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Stream.SandboxPolicy != state.StreamSandboxPolicyDefault {
		t.Fatalf("Stream.SandboxPolicy = %q, want default", plan.Stream.SandboxPolicy)
	}
	if plan.Stream.ApprovalPolicy != state.StreamApprovalPolicyDefault {
		t.Fatalf("Stream.ApprovalPolicy = %q, want default", plan.Stream.ApprovalPolicy)
	}
}

func TestCodexPrepareLaunchSkipsNonCodexBaseCommand(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.ThreadID = "thread-abc-123"
	plan, err := d.PrepareLaunch(cs, state.LaunchModeColdStart, "/repo", "env FOO=bar", state.LaunchOptions{}, false)
	if err != nil {
		t.Fatalf("PrepareLaunch error: %v", err)
	}
	if plan.Command != "env FOO=bar" {
		t.Fatalf("PrepareLaunch.Command = %q", plan.Command)
	}
}

func TestCodexPrepareLaunchColdStartUsesPersistedStartDir(t *testing.T) {
	d, cs, _ := newCodex(t)
	cs.StartDir = "/proj/.roost/worktrees/foo"
	for _, mode := range []state.LaunchMode{state.LaunchModeCreate, state.LaunchModeColdStart} {
		plan, err := d.PrepareLaunch(cs, mode, "/proj", "codex", state.LaunchOptions{}, false)
		if err != nil {
			t.Fatalf("mode=%v PrepareLaunch error: %v", mode, err)
		}
		if plan.StartDir != "/proj/.roost/worktrees/foo" {
			t.Errorf("mode=%v StartDir = %q, want worktree path", mode, plan.StartDir)
		}
	}
}

func TestCodexPrepareLaunchFallsBackToProjectWhenStartDirEmpty(t *testing.T) {
	d, cs, _ := newCodex(t)
	for _, mode := range []state.LaunchMode{state.LaunchModeCreate, state.LaunchModeColdStart} {
		plan, err := d.PrepareLaunch(cs, mode, "/proj", "codex", state.LaunchOptions{}, false)
		if err != nil {
			t.Fatalf("mode=%v PrepareLaunch error: %v", mode, err)
		}
		if plan.StartDir != "/proj" {
			t.Errorf("mode=%v StartDir = %q, want /proj", mode, plan.StartDir)
		}
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
	cs.ThreadID = "thread-t1"
	cs.WorktreeName = "codex-abcd"

	bag := d.Persist(cs)
	got := d.Restore(bag, now.Add(time.Hour)).(CodexState)
	if got.ThreadID != "thread-t1" || got.StartDir != "/repo" {
		t.Fatalf("restore mismatch: %+v", got)
	}
	if got.WorktreeName != "codex-abcd" {
		t.Fatalf("WorktreeName = %q, want codex-abcd", got.WorktreeName)
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

func TestCodexPrepareCreateWithoutWorktree(t *testing.T) {
	d, cs, _ := newCodex(t)
	_, plan, err := d.PrepareCreate(cs, "sess-1", "/repo", "codex --model gpt-5", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if plan.Command != "codex --model gpt-5" || plan.StartDir != "/repo" {
		t.Fatalf("launch = %+v", plan)
	}
	if plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree disabled")
	}
}

func TestCodexPrepareCreateWithWorktree(t *testing.T) {
	d, cs, _ := newCodex(t)
	_, plan, err := d.PrepareCreate(cs, "sess-1", "/repo", "codex --worktree feature --model gpt-5", state.LaunchOptions{})
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	if plan.Command != "codex --model gpt-5" {
		t.Fatalf("launch command = %q, want worktree flag stripped", plan.Command)
	}
	if !plan.Options.Worktree.Enabled {
		t.Fatal("expected worktree enabled")
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
