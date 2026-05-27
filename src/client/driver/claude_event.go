package driver

import (
	"encoding/json"
	"log/slog"
	"time"
	"unicode"

	"github.com/takezoh/agent-roost/client/state"
)

// Hook event handling for the Claude driver. The hook bridge sends the
// raw JSON payload via DEvHook{Event: hookEventName, Payload: {"raw": ...}}.
// This file parses the raw JSON into a hookPayload struct and dispatches
// by hook_event_name. All field extraction lives here — the bridge is
// a thin relay.

// hookPayload is the minimal subset of the Claude hook JSON the driver
// needs. Parsed from the "raw" key in DEvHook.Payload. Defined here
// (not in lib/claude/hookevent) so state/driver stays a leaf package.
type hookPayload struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	Prompt         string `json:"prompt"`
	TranscriptPath string `json:"transcript_path"`

	NotificationType string         `json:"notification_type"`
	ToolName         string         `json:"tool_name"`
	ToolInput        map[string]any `json:"tool_input"`
	Source           string         `json:"source"`

	// Tool correlation and permission fields (PreToolUse / PostToolUse /
	// PostToolUseFailure). tool_use_id links Pre to its matching Post.
	ToolUseID      string `json:"tool_use_id"`
	PermissionMode string `json:"permission_mode"`
	Error          string `json:"error"`
	IsInterrupt    bool   `json:"is_interrupt"`
}

// deriveState maps the hook_event_name to a roost status string.
// Must stay in sync with lib/claude/hookevent.HookEvent.DeriveState.
func (hp hookPayload) deriveState() string {
	switch hp.HookEventName {
	case "UserPromptSubmit", "PreToolUse", "PostToolUse":
		return "running"
	case "Stop":
		return "waiting"
	case "StopFailure":
		return "stopped"
	case "SessionEnd":
		return "stopped"
	case "SessionStart":
		return "idle"
	case "Notification":
		switch hp.NotificationType {
		case "permission_prompt":
			return "pending"
		case "idle_prompt", "elicitation_dialog":
			return "waiting"
		}
	}
	return ""
}

func (hp hookPayload) formatLog() string {
	if hp.HookEventName == "" {
		return ""
	}
	detail := ""
	switch hp.HookEventName {
	case "PreToolUse", "PostToolUse", "PostToolUseFailure":
		if hp.ToolName == "" {
			break
		}
		detail = hp.ToolName
		switch hp.ToolName {
		case "Bash":
			if cmd := toolInputString(hp.ToolInput, "command"); cmd != "" {
				detail += " " + previewText(cmd)
			}
		case "Read", "Write", "Edit", "Glob":
			if fp := toolInputString(hp.ToolInput, "file_path"); fp != "" {
				detail += " " + fp
			} else if p := toolInputString(hp.ToolInput, "pattern"); p != "" {
				detail += " " + p
			}
		}
	case "Notification":
		detail = hp.NotificationType
	case "SessionStart":
		detail = hp.Source
	}
	return eventLogLine(hp.HookEventName, detail)
}

func (hp hookPayload) logEffects() []state.Effect {
	if line := hp.formatLog(); line != "" {
		return []state.Effect{state.EffEventLogAppend{Line: line}}
	}
	return nil
}

func parseHookPayload(payload json.RawMessage) hookPayload {
	return parsePayload[hookPayload](payload)
}

// handleHook parses the raw JSON from the bridge and dispatches by
// hook_event_name.
func (d ClaudeDriver) handleHook(cs ClaudeState, ctx state.FrameContext, e state.DEvHook) (ClaudeState, []state.Effect) {
	hp := parseHookPayload(e.Payload)
	if hp.SessionID == "" {
		return cs, nil
	}

	if e.RoostSessionID != "" {
		cs.RoostSessionID = e.RoostSessionID
	}

	ts := e.Timestamp
	if !ctx.IsRoot {
		if !ts.IsZero() && !ts.After(cs.LastBridgeTS) {
			return cs, nil
		}
		if hp.SessionID != "" {
			cs.ClaudeSessionID = hp.SessionID
		}
		if !ts.IsZero() {
			cs.LastBridgeTS = ts
		}
		return cs, nil
	}

	if hp.HookEventName == "SessionStart" {
		cs.LastBridgeTS = ts
		cs.PendingTools = nil // discard any stale pending tools from prior session
		return d.handleSessionStart(cs, ctx, hp, ts)
	}

	if !ts.IsZero() && !ts.After(cs.LastBridgeTS) {
		slog.Warn("claude: dropping out-of-order hook",
			"event", hp.HookEventName, "ts", ts, "last", cs.LastBridgeTS)
		return cs, nil
	}
	if !ts.IsZero() {
		cs.LastBridgeTS = ts
	}

	if hp.HookEventName == "UserPromptSubmit" {
		return d.handleUserPromptSubmit(cs, hp, e.Timestamp)
	}

	// Agent tool events track subagent lifecycle, not main-agent
	// activity — log only, no status change.
	if hp.ToolName == "Agent" {
		return cs, hp.logEffects()
	}

	switch hp.HookEventName {
	case "SubagentStart", "SubagentStop":
		return cs, hp.logEffects()
	}

	// Tool log side-channel: update PendingTools and emit EffToolLogAppend
	// for Pre/Post/Notification. Must run before handleStateChange so that
	// any PendingTools mutations carry forward.
	cs, toolLogEffs := d.handleToolLog(cs, hp, ts)

	// SessionEnd: discard pending tools — no Post will arrive for them.
	if hp.HookEventName == "SessionEnd" {
		cs.PendingTools = nil
	}

	// All other hook events (PreToolUse, PostToolUse, Stop, etc.)
	// go through the state-change path if they map to a status.
	status := hp.deriveState()
	if status == "" {
		return cs, append(hp.logEffects(), toolLogEffs...)
	}

	next, effs := d.handleStateChange(cs, hp, status, e.Timestamp)
	return next, append(effs, toolLogEffs...)
}

// handleSessionStart absorbs identity and kicks initial transcript
// watch + parse + event log.
func (d ClaudeDriver) handleSessionStart(cs ClaudeState, ctx state.FrameContext, hp hookPayload, now time.Time) (ClaudeState, []state.Effect) {
	cs = absorbIdentityFromHP(cs, hp)
	if now.IsZero() {
		now = cs.StatusChangedAt
	}
	// Reset to Idle. A SessionStart fires on fresh launch, --resume,
	// /resume, and /clear. In every case the session is freshly
	// initialized. This also clears the Stopped that a preceding
	// SessionEnd wrote — without it a resumed session would stick at
	// Stopped until the user typed something.
	cs.Status = state.StatusIdle
	cs.StatusChangedAt = now

	var effs []state.Effect
	if path := d.resolveTranscriptPath(cs); path != "" && cs.WatchedFile != path {
		cs.WatchedFile = path
		effs = append(effs, state.EffWatchFile{Path: path, Kind: "transcript"})
		if !cs.TranscriptInFlight {
			cs.TranscriptInFlight = true
			effs = append(effs, state.EffStartJob{
				Input: TranscriptParseInput{
					ClaudeUUID: cs.ClaudeSessionID,
					Path:       path,
				},
			})
		}
	}
	if line := hp.formatLog(); line != "" {
		effs = append(effs, state.EffEventLogAppend{Line: line})
	}

	// Trigger branch detection immediately so the tag appears before
	// the user types anything (Idle sessions are skipped by tick).
	// Only root frame runs branch-detect; non-root frames share the same
	// repo and the UI displays root's branch info only.
	if ctx.IsRoot {
		target := cs.StartDir
		if target != "" && !cs.BranchInFlight {
			cs.BranchInFlight = true
			cs.BranchTarget = target
			effs = append(effs, state.EffStartJob{
				Input: BranchDetectInput{WorkingDir: target},
			})
		}
	}

	return cs, effs
}

// handleStateChange advances the status machine and emits an event log.
func (d ClaudeDriver) handleStateChange(cs ClaudeState, hp hookPayload, statusStr string, now time.Time) (ClaudeState, []state.Effect) {
	cs = absorbIdentityFromHP(cs, hp)
	if now.IsZero() {
		now = cs.StatusChangedAt
	}

	if status, ok := state.ParseStatus(statusStr); ok {
		cs.Status = status
		cs.StatusChangedAt = now
	}

	var effs []state.Effect
	logLine := hp.formatLog()
	if logLine != "" {
		effs = append(effs, state.EffEventLogAppend{Line: logLine})
	}

	if !cs.TranscriptInFlight {
		if path := d.resolveTranscriptPath(cs); path != "" {
			cs.TranscriptInFlight = true
			effs = append(effs, state.EffStartJob{
				Input: TranscriptParseInput{
					ClaudeUUID: cs.ClaudeSessionID,
					Path:       path,
				},
			})
		}
	}

	return cs, effs
}

// handleUserPromptSubmit seeds LastPrompt, triggers haiku summary,
// and also runs the state-change logic (UserPromptSubmit → "running").
func (d ClaudeDriver) handleUserPromptSubmit(cs ClaudeState, hp hookPayload, now time.Time) (ClaudeState, []state.Effect) {
	cs = absorbIdentityFromHP(cs, hp)
	if !now.IsZero() {
		cs.StatusChangedAt = now
	}
	cs.Status = state.StatusRunning
	cs.StatusChangedAt = now

	if hp.Prompt != "" {
		cs.LastPrompt = hp.Prompt
	}

	var effs []state.Effect
	if line := hp.formatLog(); line != "" {
		effs = append(effs, state.EffEventLogAppend{Line: line})
	}

	turns := appendHookPromptTurn(cs.RecentTurns, hp.Prompt)
	prompt := formatSummaryPrompt(cs.Summary, turns)
	effs, cs.SummaryInFlight = enqueueSummaryJob(effs, cs.SummaryInFlight, prompt)

	if !cs.TranscriptInFlight {
		if path := d.resolveTranscriptPath(cs); path != "" {
			cs.TranscriptInFlight = true
			effs = append(effs, state.EffStartJob{
				Input: TranscriptParseInput{
					ClaudeUUID: cs.ClaudeSessionID,
					Path:       path,
				},
			})
		}
	}

	return cs, effs
}

// handleWindowTitle interprets an OSC 0 window-title update from the pane.
// Claude Code uses the title to advertise its working state:
//   - ✳ (U+2733) prefix → agent is idle, waiting for user input
//   - Braille spinner (U+2800–U+28FF) prefix → agent is working
//
// Status transitions trigger EffNotify{NotifyKindDone} automatically via
// stepDriver's ClassifyStatusTransition diff, so this function returns no
// effects of its own.
func (d ClaudeDriver) handleWindowTitle(cs ClaudeState, title string, now time.Time) ClaudeState {
	if title == cs.LastWindowTitle {
		return cs
	}
	cs.LastWindowTitle = title

	r, _ := firstRune(title)
	switch {
	case r == '✳':
		if cs.Status != state.StatusWaiting {
			cs.Status = state.StatusWaiting
			cs.StatusChangedAt = now
		}
	case unicode.In(r, unicode.Braille):
		if cs.Status != state.StatusRunning {
			cs.Status = state.StatusRunning
			cs.StatusChangedAt = now
		}
	}
	return cs
}

// firstRune returns the first rune in s and whether s was non-empty.
func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func absorbIdentityFromHP(cs ClaudeState, hp hookPayload) ClaudeState {
	if hp.SessionID != "" {
		if cs.ForkParentID != "" && hp.SessionID == cs.ForkParentID {
			// The hook carries the parent session id, which arrives first when
			// `--fork-session` is launched via `--resume <parent>`. Skip the
			// overwrite to avoid poisoning ClaudeSessionID with the parent id.
			// Claude will emit the real fork id in a subsequent hook.
			slog.Debug("claude: fork: dropping parent session_id, waiting for fork id",
				"parent_id", hp.SessionID)
		} else {
			cs.ClaudeSessionID = hp.SessionID
			cs.ForkParentID = "" // fork id confirmed; lineage no longer needed
		}
	}
	if hp.TranscriptPath != "" {
		cs.TranscriptPath = hp.TranscriptPath
	}
	return cs
}
