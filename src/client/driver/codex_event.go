package driver

import (
	"log/slog"
	"strings"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

func statusTime(ts, fallback time.Time) time.Time {
	if !ts.IsZero() {
		return ts
	}
	return fallback
}

func applyHookStatus(cs CodexState, status state.Status, ts time.Time) CodexState {
	cs.Status = status
	cs.StatusChangedAt = statusTime(ts, cs.StatusChangedAt)
	return cs
}

func (d CodexDriver) handleWindowTitle(cs CodexState, title string, now time.Time) CodexState {
	if title == cs.LastWindowTitle {
		return cs
	}
	cs.LastWindowTitle = title
	if codexTitleNeedsUserAction(title) && cs.Status != state.StatusPending {
		cs.Status = state.StatusPending
		cs.StatusChangedAt = statusTime(now, cs.StatusChangedAt)
	}
	return cs
}

func (d CodexDriver) BootstrapSessionStart(s state.DriverState, ctx state.FrameContext, now time.Time) (state.DriverState, []state.Effect) {
	cs, ok := s.(CodexState)
	if !ok {
		cs = d.NewState(now).(CodexState)
	}
	effs := watchCodexTranscript(&cs)
	cs, effs = d.applySessionStart(cs, ctx, now, effs)
	return cs, effs
}

func codexTitleNeedsUserAction(title string) bool {
	return strings.Contains(title, "Action Required")
}

func (d CodexDriver) handleSubsystem(cs CodexState, ctx state.FrameContext, e state.DEvSubsystem) (CodexState, []state.Effect) {
	prevStatus := cs.Status
	prevThreadID := cs.ThreadID
	p := e.Payload
	if p.RequestedTargetID != "" {
		cs.RequestedThreadID = p.RequestedTargetID
	}
	if p.ObservedTargetID != "" {
		cs.ObservedThreadID = p.ObservedTargetID
	}
	if p.ResumePhase != "" {
		cs.ResumePhase = p.ResumePhase
	}
	if p.ColdStartSessionID != "" {
		cs.SessionID = p.ColdStartSessionID
	}
	if p.TargetID != "" {
		cs.ThreadID = p.TargetID
	}
	if !ctx.IsRoot {
		slog.Debug("codex: subsystem event ignored for non-root frame",
			"frame", ctx.ID,
			"kind", e.Kind,
			"thread", cs.ThreadID,
			"prev_thread", prevThreadID)
		return cs, nil
	}
	if p.TranscriptPath != "" {
		cs.setRolloutPath(p.TranscriptPath)
	}
	if p.StatusLine != "" {
		cs.StatusLine = p.StatusLine
	}
	effs := watchCodexTranscript(&cs)
	cs, effs = d.applySubsystemKind(cs, ctx, e, effs)
	slog.Debug("codex: subsystem event applied",
		"frame", ctx.ID,
		"kind", e.Kind,
		"source", e.Source,
		"thread", cs.ThreadID,
		"prev_thread", prevThreadID,
		"status", cs.Status,
		"prev_status", prevStatus,
		"tool", cs.CurrentTool,
		"pending_approval", cs.PendingApproval)
	return cs, effs
}

func (d CodexDriver) applySubsystemKind(cs CodexState, ctx state.FrameContext, e state.DEvSubsystem, effs []state.Effect) (CodexState, []state.Effect) {
	p := e.Payload
	switch e.Kind {
	case state.SubsystemSessionReady:
		cs, effs = d.applySessionStart(cs, ctx, e.Timestamp, effs)
	case state.SubsystemFailed:
		cs.PendingTools = nil
		cs.CurrentTool = ""
		cs.PendingApproval = false
		cs.FailureReason = strings.TrimSpace(p.Error)
		cs = applyHookStatus(cs, state.StatusStopped, e.Timestamp)
	case state.SubsystemPromptSubmitted:
		cs.LastPrompt = strings.TrimSpace(p.Prompt)
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		turns := userOnlyTurns(appendHookPromptTurn(cs.RecentTurns, p.Prompt), 2)
		effs, cs.SummaryInFlight = enqueueSummaryJob(effs, cs.SummaryInFlight, formatSummaryPrompt(cs.Summary, turns))
		effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	case state.SubsystemTurnStarted:
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
	case state.SubsystemTurnCompleted:
		cs.CurrentTool = ""
		cs.PendingTools = nil
		cs.PendingApproval = false
		if msg := strings.TrimSpace(p.LastAssistantMessage); msg != "" {
			cs.LastAssistantMessage = msg
		}
		cs = applyHookStatus(cs, state.StatusWaiting, e.Timestamp)
	case state.SubsystemToolStarted:
		cs.PendingApproval = false
		cs.CurrentTool = subsystemToolName(p.Tool)
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		cs, effs = d.handleSubsystemToolStarted(cs, p, e.Timestamp, effs)
	case state.SubsystemToolCompleted:
		cs.CurrentTool = ""
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		cs, effs = d.handleSubsystemToolCompleted(cs, p, e.Timestamp, effs)
	case state.SubsystemApprovalRequested:
		cs.PendingApproval = true
		cs = applyHookStatus(cs, state.StatusPending, e.Timestamp)
	case state.SubsystemApprovalResolved:
		cs.PendingApproval = false
		if p.Approval != nil && p.Approval.Denied {
			cs = applyHookStatus(cs, state.StatusWaiting, e.Timestamp)
		} else {
			cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		}
	case state.SubsystemPlanUpdated:
		if p.Plan != nil {
			cs.PlanSummary = strings.TrimSpace(p.Plan.Summary)
		}
	case state.SubsystemDiffUpdated:
		if p.Diff != nil {
			cs.DiffSummary = strings.TrimSpace(p.Diff.Summary)
			cs.DiffPaths = append([]string(nil), p.Diff.Paths...)
		}
	case state.SubsystemMessageUpdated:
		if msg := strings.TrimSpace(p.LastAssistantMessage); msg != "" {
			cs.LastAssistantMessage = msg
		}
		if p.Message != nil {
			cs.RecentTurns = subsystemTurnsToSummaryTurns(p.Message.RecentTurns)
		}
	}
	return cs, effs
}

func (d CodexDriver) applySessionStart(cs CodexState, ctx state.FrameContext, now time.Time, effs []state.Effect) (CodexState, []state.Effect) {
	cs.PendingTools = nil
	cs.CurrentTool = ""
	cs.PendingApproval = false
	cs.FailureReason = ""
	cs = applyHookStatus(cs, state.StatusIdle, now)
	effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	if ctx.IsRoot {
		target := cs.StartDir
		if target != "" && !cs.BranchInFlight {
			cs.BranchInFlight = true
			cs.BranchTarget = target
			effs = append(effs, state.EffStartJob{Input: BranchDetectInput{WorkingDir: target}})
		}
	}
	return cs, effs
}

func subsystemToolName(tool *state.SubsystemTool) string {
	if tool == nil {
		return ""
	}
	return strings.TrimSpace(tool.Name)
}

func subsystemTurnsToSummaryTurns(turns []state.SubsystemTurn) []SummaryTurn {
	if len(turns) == 0 {
		return nil
	}
	out := make([]SummaryTurn, 0, len(turns))
	for _, turn := range turns {
		text := strings.TrimSpace(turn.Text)
		role := strings.TrimSpace(turn.Role)
		if text == "" || role == "" {
			continue
		}
		out = append(out, SummaryTurn{Role: role, Text: text})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (d CodexDriver) handleSubsystemToolStarted(cs CodexState, payload state.SubsystemPayload, now time.Time, effs []state.Effect) (CodexState, []state.Effect) {
	tool := payload.Tool
	if tool == nil || tool.ID == "" || tool.Name == "" {
		return cs, effs
	}
	if cs.PendingTools == nil {
		cs.PendingTools = make(map[string]codexPendingTool)
	}
	input := subsystemToolInput(tool)
	cs.PendingTools[tool.ID] = codexPendingTool{
		Name:      tool.Name,
		Input:     input,
		StartedAt: now,
	}
	return cs, effs
}

func (d CodexDriver) handleSubsystemToolCompleted(cs CodexState, payload state.SubsystemPayload, now time.Time, effs []state.Effect) (CodexState, []state.Effect) {
	tool := payload.Tool
	if tool == nil || tool.Name == "" {
		return cs, effs
	}
	ev := codexToolEvent{
		Kind:        "PostToolUse",
		ToolName:    tool.Name,
		ToolInput:   subsystemToolInput(tool),
		ToolUseID:   tool.ID,
		Error:       tool.Error,
		IsInterrupt: tool.IsInterrupt,
	}
	kind := "auto"
	if tool.Error != "" {
		ev.Kind = "PostToolUseFailure"
		if tool.IsInterrupt {
			kind = "denied"
		} else {
			kind = "failed"
		}
	}
	return d.emitToolLog(cs, ev, now, kind, effs)
}

func subsystemToolInput(tool *state.SubsystemTool) map[string]any {
	if tool == nil {
		return nil
	}
	input := make(map[string]any, 2)
	if tool.Command != "" {
		input["command"] = tool.Command
	}
	if tool.Path != "" {
		input["file_path"] = tool.Path
	}
	if len(input) == 0 {
		return nil
	}
	return input
}

func (d CodexDriver) handleJobResult(cs CodexState, e state.DEvJobResult) CodexState {
	if summary, inFlight, ok := applySummaryJobResult(cs.Summary, cs.SummaryInFlight, e); ok {
		cs.Summary = summary
		cs.SummaryInFlight = inFlight
		return cs
	}

	switch r := e.Result.(type) {
	case CodexTranscriptParseResult:
		cs.TranscriptInFlight = false
		if e.Err != nil {
			return cs
		}
		if r.Title != "" {
			cs.Title = r.Title
		}
		if r.LastPrompt != "" {
			cs.LastPrompt = r.LastPrompt
		}
		if r.LastAssistantMessage != "" {
			cs.LastAssistantMessage = r.LastAssistantMessage
		}
		cs.StatusLine = r.StatusLine
		cs.RecentTurns = r.RecentTurns
		if cs.Status != state.StatusRunning {
			cs.CurrentTool = ""
		}
		return cs
	case BranchDetectResult:
		cs.ApplyBranchResult(r, e.Err, e.Now)
	}
	return cs
}

func (d CodexDriver) handleTranscriptChanged(cs CodexState, e state.DEvFileChanged) (CodexState, []state.Effect) {
	if cs.TranscriptPath != "" && e.Path != "" && cs.TranscriptPath != e.Path {
		return cs, nil
	}
	effs := watchCodexTranscript(&cs)
	effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	return cs, effs
}

func (d CodexDriver) startCodexTranscriptParse(cs *CodexState) []state.Effect {
	if cs.TranscriptInFlight || cs.TranscriptPath == "" {
		return nil
	}
	cs.TranscriptInFlight = true
	return []state.Effect{
		state.EffStartJob{
			Input: CodexTranscriptParseInput{
				Path: cs.TranscriptPath,
			},
		},
	}
}

func watchCodexTranscript(cs *CodexState) []state.Effect {
	if cs.TranscriptPath == "" || cs.WatchedFile == cs.TranscriptPath {
		return nil
	}
	cs.WatchedFile = cs.TranscriptPath
	return []state.Effect{state.EffWatchFile{Path: cs.TranscriptPath, Kind: "transcript"}}
}
