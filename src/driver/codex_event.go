package driver

import (
	"fmt"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/state"
)

func (hp codexHookPayload) toolInputString(key string) string {
	if hp.ToolInput == nil {
		return ""
	}
	v, _ := hp.ToolInput[key].(string)
	return v
}

func (hp codexHookPayload) formatLog() string {
	name := hp.HookEventName
	detail := ""
	switch hp.HookEventName {
	case "SessionStart":
		detail = hp.Source
	case "UserPromptSubmit":
		if hp.Prompt != "" {
			detail = fmt.Sprintf(`prompt="%s"`, previewText(hp.Prompt))
		}
	case "PreToolUse", "PostToolUse", "PostToolUseFailure":
		detail = strings.TrimSpace(hp.ToolName)
		if cmd := hp.toolInputString("command"); cmd != "" {
			detail = strings.TrimSpace(fmt.Sprintf(`%s cmd="%s"`, detail, previewText(cmd)))
		} else if path := hp.toolInputString("file_path"); path != "" {
			detail = strings.TrimSpace(fmt.Sprintf(`%s path="%s"`, detail, previewText(path)))
		}
	case "Stop":
		var parts []string
		if hp.StopReason != "" {
			parts = append(parts, "reason="+previewText(hp.StopReason))
		}
		if hp.LastAssistantMessage != "" {
			parts = append(parts, fmt.Sprintf(`last="%s"`, previewText(hp.LastAssistantMessage)))
		}
		detail = strings.Join(parts, " ")
	}
	return eventLogLine(name, detail)
}

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
	effs = append(effs, state.EffEventLogAppend{Line: eventLogLine("SessionStart", "startup")})
	return cs, effs
}

func codexTitleNeedsUserAction(title string) bool {
	return strings.Contains(title, "Action Required")
}

func (d CodexDriver) handleHook(cs CodexState, ctx state.FrameContext, e state.DEvHook) (CodexState, []state.Effect) {
	hp := parseCodexHookPayload(e.Payload)
	preamble := hookPreamble{
		SessionID:     hp.SessionID,
		HookEventName: hp.HookEventName,
	}
	if ctx.IsRoot {
		preamble.TranscriptPath = hp.TranscriptPath
	}
	if !cs.applyHookPreamble(preamble, e) {
		return cs, nil
	}
	cs.CodexSessionID = hp.SessionID
	if !ctx.IsRoot {
		return cs, nil
	}

	effs := watchCodexTranscript(&cs)
	cs, effs = d.applyHookEvent(cs, ctx, hp, e, effs)

	line := strings.TrimSpace(hp.formatLog())
	if line != "" {
		effs = append(effs, state.EffEventLogAppend{Line: line})
	}
	return cs, effs
}

func (d CodexDriver) applyHookEvent(cs CodexState, ctx state.FrameContext, hp codexHookPayload, e state.DEvHook, effs []state.Effect) (CodexState, []state.Effect) {
	switch hp.HookEventName {
	case "SessionStart":
		cs, effs = d.applySessionStart(cs, ctx, e.Timestamp, effs)
	case "UserPromptSubmit":
		cs.LastPrompt = strings.TrimSpace(hp.Prompt)
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		turns := recentUserTurns(appendHookPromptTurn(cs.RecentTurns, hp.Prompt), 2)
		effs, cs.SummaryInFlight = enqueueSummaryJob(effs, cs.SummaryInFlight, formatSummaryPrompt(cs.Summary, turns))
		effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	case "PreToolUse":
		cs.CurrentTool = strings.TrimSpace(hp.ToolName)
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		cs, effs = d.handleToolLog(cs, hp, e.Timestamp, effs)
	case "PostToolUse", "PostToolUseFailure":
		cs.CurrentTool = ""
		cs = applyHookStatus(cs, state.StatusRunning, e.Timestamp)
		cs, effs = d.handleToolLog(cs, hp, e.Timestamp, effs)
	case "Stop":
		cs.CurrentTool = ""
		cs.PendingTools = nil
		if msg := strings.TrimSpace(hp.LastAssistantMessage); msg != "" {
			cs.LastAssistantMessage = msg
		}
		cs = applyHookStatus(cs, state.StatusWaiting, e.Timestamp)
		effs = append(effs, d.startCodexTranscriptParse(&cs)...)
	}
	return cs, effs
}

func (d CodexDriver) applySessionStart(cs CodexState, ctx state.FrameContext, now time.Time, effs []state.Effect) (CodexState, []state.Effect) {
	cs.PendingTools = nil
	cs.CurrentTool = ""
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
