package httpserver

import (
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/scheduler"
)

// projectState projects a StateSnapshot into a stateResponse DTO.
func projectState(snap scheduler.StateSnapshot, now time.Time) stateResponse {
	running := make([]runningEntry, 0, len(snap.Running))
	for _, run := range snap.Running {
		running = append(running, projectRunningEntry(run))
	}

	retrying := make([]retryingEntry, 0, len(snap.RetryAttempts))
	for _, re := range snap.RetryAttempts {
		retrying = append(retrying, projectRetryingEntry(re))
	}

	totals := &codexTotals{
		InputTokens:    snap.CodexTotals.Input,
		OutputTokens:   snap.CodexTotals.Output,
		TotalTokens:    snap.CodexTotals.Total,
		SecondsRunning: snap.CodexSecondsRunning,
	}

	// Collect the most recently seen rate-limit from any running issue.
	var rl *rateLimits
	for _, run := range snap.Running {
		if run.RateLimit != nil {
			rl = &rateLimits{
				PrimaryUsedPercent:   run.RateLimit.PrimaryUsedPercent,
				PrimaryResetsAt:      run.RateLimit.PrimaryResetsAt,
				SecondaryUsedPercent: run.RateLimit.SecondaryUsedPercent,
				SecondaryResetsAt:    run.RateLimit.SecondaryResetsAt,
			}
			break
		}
	}

	return stateResponse{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Counts: stateCounts{
			Running:  len(snap.Running),
			Retrying: len(snap.RetryAttempts),
		},
		Running:     running,
		Retrying:    retrying,
		CodexTotals: totals,
		RateLimits:  rl,
	}
}

func projectRunningEntry(run scheduler.RunAttempt) runningEntry {
	lastEventAt := run.LastCodexTimestamp
	if lastEventAt.IsZero() {
		lastEventAt = run.StartedAt
	}
	return runningEntry{
		IssueID:         run.Issue.ID,
		IssueIdentifier: run.Issue.Identifier,
		State:           run.Issue.State,
		SessionID:       run.Session.SessionID,
		TurnCount:       0, // turn counter not tracked; shape maintained
		LastEvent:       run.LastCodexEvent,
		LastMessage:     run.LastCodexMessage,
		StartedAt:       formatTime(run.StartedAt),
		LastEventAt:     formatTime(lastEventAt),
		Tokens: tokenCounts{
			InputTokens:  run.TotalInputTokens,
			OutputTokens: run.TotalOutputTokens,
			TotalTokens:  run.TotalTokens,
		},
	}
}

func projectRetryingEntry(re scheduler.RetryEntry) retryingEntry {
	errMsg := ""
	if re.Err != nil {
		errMsg = re.Err.Error()
	}
	return retryingEntry{
		IssueID:         re.IssueID,
		IssueIdentifier: re.Identifier,
		Attempt:         re.Attempt,
		DueAt:           formatUnixMS(re.DueAtMS),
		Error:           errMsg,
	}
}

// projectIssue finds an issue by identifier across running/retrying and returns a detail response.
// Returns nil when not found.
func projectIssue(snap scheduler.StateSnapshot, identifier string, workspaceRoot string) *issueResponse {
	// Check running first.
	for _, run := range snap.Running {
		if run.Issue.Identifier == identifier {
			return projectRunningIssue(run, workspaceRoot)
		}
	}
	// Check retrying.
	for _, re := range snap.RetryAttempts {
		if re.Identifier == identifier {
			return projectRetryingIssue(re, workspaceRoot)
		}
	}
	return nil
}

func projectRunningIssue(run scheduler.RunAttempt, workspaceRoot string) *issueResponse {
	lastEventAt := run.LastCodexTimestamp
	if lastEventAt.IsZero() {
		lastEventAt = run.StartedAt
	}

	var events []recentEvent
	if !run.LastCodexTimestamp.IsZero() && run.LastCodexEvent != "" {
		events = []recentEvent{{
			At:      formatTime(run.LastCodexTimestamp),
			Event:   run.LastCodexEvent,
			Message: run.LastCodexMessage,
		}}
	}

	resp := &issueResponse{
		IssueIdentifier: run.Issue.Identifier,
		IssueID:         run.Issue.ID,
		Status:          "running",
		Workspace:       issueWorkspace{Path: workspacePath(workspaceRoot, run.Issue.Identifier)},
		Attempts: issueAttempts{
			RestartCount:        run.Attempt - 1,
			CurrentRetryAttempt: run.Attempt,
		},
		Running: &runningDetail{
			SessionID:   run.Session.SessionID,
			TurnCount:   0,
			State:       run.Issue.State,
			StartedAt:   formatTime(run.StartedAt),
			LastEvent:   run.LastCodexEvent,
			LastMessage: run.LastCodexMessage,
			LastEventAt: formatTime(lastEventAt),
			Tokens: tokenCounts{
				InputTokens:  run.TotalInputTokens,
				OutputTokens: run.TotalOutputTokens,
				TotalTokens:  run.TotalTokens,
			},
		},
		Retry:        nil,
		Logs:         issueLogs{CodexSessionLogs: []codexLogEntry{}},
		RecentEvents: events,
		LastError:    nil,
		Tracked:      []byte("{}"),
	}
	return resp
}

func projectRetryingIssue(re scheduler.RetryEntry, workspaceRoot string) *issueResponse {
	errMsg := ""
	if re.Err != nil {
		errMsg = re.Err.Error()
	}
	var lastErr *string
	if errMsg != "" {
		lastErr = &errMsg
	}

	resp := &issueResponse{
		IssueIdentifier: re.Identifier,
		IssueID:         re.IssueID,
		Status:          "retrying",
		Workspace:       issueWorkspace{Path: workspacePath(workspaceRoot, re.Identifier)},
		Attempts: issueAttempts{
			RestartCount:        re.Attempt - 1,
			CurrentRetryAttempt: re.Attempt,
		},
		Running: nil,
		Retry: &retryDetail{
			Attempt: re.Attempt,
			DueAt:   formatUnixMS(re.DueAtMS),
			Error:   errMsg,
		},
		Logs:         issueLogs{CodexSessionLogs: []codexLogEntry{}},
		RecentEvents: []recentEvent{},
		LastError:    lastErr,
		Tracked:      []byte("{}"),
	}
	return resp
}

func workspacePath(root, identifier string) string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, identifier)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatUnixMS(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
