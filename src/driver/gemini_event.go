package driver

import (
	"fmt"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/state"
)

type geminiHookPayload struct {
	SessionID        string         `json:"session_id"`
	HookEventName    string         `json:"hook_event_name"`
	NotificationType string         `json:"notification_type"`
	Cwd              string         `json:"cwd"`
	TranscriptPath   string         `json:"transcript_path"`
	Source           string         `json:"source"`
	Prompt           string         `json:"prompt"`
	ToolName         string         `json:"tool_name"`
	ToolInput        map[string]any `json:"tool_input"`
	ToolUseID        string         `json:"tool_use_id"`
	PermissionMode   string         `json:"permission_mode"`
	PromptResponse   string         `json:"prompt_response"`
	StopReason       string         `json:"stop_reason"`
}

func (hp geminiHookPayload) toolInputString(key string) string {
	if hp.ToolInput == nil {
		return ""
	}
	v, _ := hp.ToolInput[key].(string)
	return v
}

func (hp geminiHookPayload) formatLog() string {
	name := hp.HookEventName
	detail := ""
	switch hp.HookEventName {
	case "SessionStart":
		detail = hp.Source
	case "BeforeAgent":
		if hp.Prompt != "" {
			detail = fmt.Sprintf(`prompt="%s"`, previewText(hp.Prompt))
		}
	case "BeforeTool", "AfterTool":
		detail = strings.TrimSpace(hp.ToolName)
		if cmd := hp.toolInputString("command"); cmd != "" {
			detail = strings.TrimSpace(fmt.Sprintf(`%s cmd="%s"`, detail, previewText(cmd)))
		} else if path := hp.toolInputString("file_path"); path != "" {
			detail = strings.TrimSpace(fmt.Sprintf(`%s path="%s"`, detail, previewText(path)))
		}
	case "Notification":
		detail = hp.NotificationType
	case "AfterAgent":
		var parts []string
		if hp.StopReason != "" {
			parts = append(parts, "reason="+previewText(hp.StopReason))
		}
		if hp.PromptResponse != "" {
			parts = append(parts, fmt.Sprintf(`resp="%s"`, previewText(hp.PromptResponse)))
		}
		detail = strings.Join(parts, " ")
	}
	return eventLogLine(name, detail)
}

func (d GeminiDriver) handleWindowTitle(gs GeminiState, title string, now time.Time) GeminiState {
	if title == gs.LastWindowTitle {
		return gs
	}
	gs.LastWindowTitle = title
	if codexTitleNeedsUserAction(title) && gs.Status != state.StatusPending {
		gs.Status = state.StatusPending
		gs.StatusChangedAt = statusTime(now, gs.StatusChangedAt)
	}
	return gs
}

func (d GeminiDriver) handleHook(gs GeminiState, ctx state.FrameContext, e state.DEvHook) (GeminiState, []state.Effect) {
	hp := parseGeminiHookPayload(e.Payload)
	preamble := hookPreamble{
		SessionID:     hp.SessionID,
		HookEventName: hp.HookEventName,
		Cwd:           hp.Cwd,
	}
	if ctx.IsRoot {
		preamble.TranscriptPath = hp.TranscriptPath
	}
	if !gs.applyHookPreamble(preamble, e) {
		return gs, nil
	}
	gs.GeminiSessionID = hp.SessionID
	if !ctx.IsRoot {
		return gs, nil
	}

	effs := watchGeminiTranscript(&gs)

	switch hp.HookEventName {
	case "SessionStart":
		gs.PendingTools = nil
		gs.CurrentTool = ""
		gs.Status = state.StatusIdle
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		effs = append(effs, d.startGeminiTranscriptParse(&gs)...)
		if target := gs.StartDir; target != "" && !gs.BranchInFlight {
			gs.BranchInFlight = true
			gs.BranchTarget = target
			effs = append(effs, state.EffStartJob{Input: BranchDetectInput{WorkingDir: target}})
		}
	case "BeforeAgent":
		gs.LastPrompt = strings.TrimSpace(hp.Prompt)
		gs.Status = state.StatusRunning
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		turns := recentUserTurns(appendHookPromptTurn(gs.RecentTurns, hp.Prompt), 2)
		effs, gs.SummaryInFlight = enqueueSummaryJob(effs, gs.SummaryInFlight, formatSummaryPrompt(gs.Summary, turns))
	case "BeforeTool":
		gs.CurrentTool = strings.TrimSpace(hp.ToolName)
		gs.Status = state.StatusRunning
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		gs = handleGeminiPendingTool(gs, hp, e.Timestamp)
	case "AfterTool":
		gs.Status = state.StatusRunning
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		gs, effs = d.emitGeminiToolLog(gs, hp, e.Timestamp, effs)
	case "AfterAgent":
		if msg := strings.TrimSpace(hp.PromptResponse); msg != "" {
			gs.LastAssistantMessage = msg
		}
		gs.CurrentTool = ""
		gs.PendingTools = nil
		gs.Status = state.StatusWaiting
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		effs = append(effs, d.startGeminiTranscriptParse(&gs)...)
	case "Notification":
		if hp.NotificationType == "ToolPermission" {
			gs = markGeminiPermissionPrompt(gs)
			gs.Status = state.StatusPending
			gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
		}
	case "SessionEnd":
		gs.CurrentTool = ""
		gs.PendingTools = nil
		gs.Status = state.StatusStopped
		gs.StatusChangedAt = statusTime(e.Timestamp, gs.StatusChangedAt)
	}

	if line := strings.TrimSpace(hp.formatLog()); line != "" {
		effs = append(effs, state.EffEventLogAppend{Line: line})
	}
	return gs, effs
}

func handleGeminiPendingTool(gs GeminiState, hp geminiHookPayload, now time.Time) GeminiState {
	if hp.ToolUseID == "" || hp.ToolName == "" {
		return gs
	}
	if gs.PendingTools == nil {
		gs.PendingTools = make(map[string]geminiPendingTool)
	}
	gs.PendingTools[hp.ToolUseID] = geminiPendingTool{
		Name:      hp.ToolName,
		Input:     hp.ToolInput,
		StartedAt: now,
	}
	return gs
}

func markGeminiPermissionPrompt(gs GeminiState) GeminiState {
	var oldestID string
	var oldestTS time.Time
	for id, p := range gs.PendingTools {
		if p.SawPrompt {
			continue
		}
		if oldestID == "" || p.StartedAt.Before(oldestTS) {
			oldestID = id
			oldestTS = p.StartedAt
		}
	}
	if oldestID == "" {
		return gs
	}
	entry := gs.PendingTools[oldestID]
	entry.SawPrompt = true
	gs.PendingTools[oldestID] = entry
	return gs
}

func (d GeminiDriver) emitGeminiToolLog(gs GeminiState, hp geminiHookPayload, now time.Time, effs []state.Effect) (GeminiState, []state.Effect) {
	var (
		kind       string
		durationMs int64
		toolInput  map[string]any
	)
	if hp.ToolUseID == "" {
		kind = "auto"
		toolInput = hp.ToolInput
	} else if entry, ok := gs.PendingTools[hp.ToolUseID]; ok {
		delete(gs.PendingTools, hp.ToolUseID)
		if entry.SawPrompt {
			kind = "approved"
		} else {
			kind = "auto"
		}
		toolInput = entry.Input
		if !entry.StartedAt.IsZero() && !now.IsZero() {
			durationMs = now.Sub(entry.StartedAt).Milliseconds()
		}
	} else {
		kind = "orphan"
		toolInput = hp.ToolInput
	}
	project := gs.Project
	if project == "" {
		project = gs.StartDir
	}
	slug := resolveProjectSlug(project)
	if slug == "" || hp.ToolName == "" {
		return gs, effs
	}
	line := buildToolLogLine(toolLogEntry{
		TS:             now,
		RoostSessionID: gs.RoostSessionID,
		ToolUseID:      hp.ToolUseID,
		ToolName:       hp.ToolName,
		Kind:           kind,
		PermissionMode: hp.PermissionMode,
		DurationMs:     durationMs,
		ToolInput:      summariseToolInput(hp.ToolName, toolInput),
	})
	effs = append(effs, state.EffToolLogAppend{
		Namespace: GeminiDriverName,
		Project:   slug,
		Line:      line,
	})
	return gs, effs
}

func (d GeminiDriver) handleTranscriptChanged(gs GeminiState, e state.DEvFileChanged) (GeminiState, []state.Effect) {
	if gs.TranscriptPath != "" && e.Path != "" && gs.TranscriptPath != e.Path {
		return gs, nil
	}
	effs := watchGeminiTranscript(&gs)
	effs = append(effs, d.startGeminiTranscriptParse(&gs)...)
	return gs, effs
}

func (d GeminiDriver) startGeminiTranscriptParse(gs *GeminiState) []state.Effect {
	if gs.TranscriptInFlight || gs.TranscriptPath == "" {
		return nil
	}
	gs.TranscriptInFlight = true
	return []state.Effect{
		state.EffStartJob{
			Input: GeminiTranscriptParseInput{Path: gs.TranscriptPath},
		},
	}
}

func watchGeminiTranscript(gs *GeminiState) []state.Effect {
	if gs.TranscriptPath == "" || gs.WatchedFile == gs.TranscriptPath {
		return nil
	}
	gs.WatchedFile = gs.TranscriptPath
	return []state.Effect{state.EffWatchFile{Path: gs.TranscriptPath, Kind: "transcript"}}
}

func (d GeminiDriver) handleJobResult(gs GeminiState, e state.DEvJobResult) (GeminiState, []state.Effect) {
	if summary, inFlight, ok := applySummaryJobResult(gs.Summary, gs.SummaryInFlight, e); ok {
		gs.Summary = summary
		gs.SummaryInFlight = inFlight
		return gs, nil
	}
	switch r := e.Result.(type) {
	case GeminiTranscriptParseResult:
		gs.TranscriptInFlight = false
		if e.Err != nil {
			return gs, nil
		}
		if r.Title != "" {
			gs.Title = r.Title
		}
		if r.LastPrompt != "" {
			gs.LastPrompt = r.LastPrompt
		}
		if r.LastAssistantMessage != "" {
			gs.LastAssistantMessage = r.LastAssistantMessage
		}
		gs.StatusLine = r.StatusLine
		gs.RecentTurns = r.RecentTurns
		gs.CurrentTool = r.CurrentTool
	case BranchDetectResult:
		gs.BranchInFlight = false
		if e.Err != nil || r.Branch == "" {
			return gs, nil
		}
		gs.BranchTag = r.Branch
		gs.BranchBG = r.Background
		gs.BranchFG = r.Foreground
		gs.BranchAt = e.Now
		gs.BranchIsWorktree = r.IsWorktree
		gs.BranchParentBranch = r.ParentBranch
	case CapturePaneResult:
		return gs, gs.HandleCapturePaneResult(r, e.Err, e.Now)
	}
	return gs, nil
}
