package driver

import (
	"time"

	"github.com/takezoh/agent-roost/state"
)

// CommonState contains the shared fields and logic used by multiple
// agent drivers (Claude, Codex, Gemini, Generic). Embedding this struct
// ensures consistent state management across different driver implementations.
type CommonState struct {
	state.DriverStateBase

	// Identity & Context
	RoostSessionID string
	// Project mirrors Session.Project / SessionFrame.Project. Updated on
	// every tick via DEvTick.Project; used as the tool-log project slug.
	Project        string
	StartDir       string
	TranscriptPath string
	WorktreeName   string

	// Status bookkeeping
	Status          state.Status
	StatusChangedAt time.Time

	// Branch & Git context
	BranchTag          string
	BranchBG           string
	BranchFG           string
	BranchTarget       string
	BranchAt           time.Time
	BranchIsWorktree   bool
	BranchParentBranch string
	BranchInFlight     bool

	// Event & Prompt history
	LastPrompt           string
	LastAssistantMessage string
	LastHookEvent        string
	LastHookAt           time.Time

	// Summary & Display
	Summary         string
	SummaryInFlight bool
	Title           string
}

const (
	// commonBranchRefreshInterval is the minimum time between VCS branch
	// detections for an active session.
	commonBranchRefreshInterval = 30 * time.Second
)

// HandleTick common implementation for drivers. Completes StartDir,
// skips heavy work for Idle/Stopped sessions, and refreshes branch info
// when active.
func (c *CommonState) HandleTick(e state.DEvTick, hasActiveSubagents bool) []state.Effect {
	c.Project = e.Project
	if c.StartDir == "" {
		c.StartDir = e.Project
	}

	if c.Status == state.StatusIdle || c.Status == state.StatusStopped {
		return nil
	}

	var effs []state.Effect

	// Branch refresh: only when the session is active (swapped into 0.0)
	// and the cache is stale or the working dir changed.
	if e.Active {
		target := c.StartDir
		if target == "" {
			target = e.Project
		}
		if target != "" && !c.BranchInFlight {
			if target != c.BranchTarget || e.Now.Sub(c.BranchAt) >= commonBranchRefreshInterval {
				c.BranchInFlight = true
				c.BranchTarget = target
				effs = append(effs, state.EffStartJob{
					Input: BranchDetectInput{WorkingDir: target},
				})
			}
		}
	}

	return effs
}

// ApplyBranchResult updates the branch fields from a BranchDetectResult.
// Returns false and leaves the state unchanged if the result is empty or
// carries an error (preserving the existing tag for retry on the next tick).
func (c *CommonState) ApplyBranchResult(r BranchDetectResult, err error, now time.Time) bool {
	c.BranchInFlight = false
	if err != nil || r.Branch == "" {
		return false
	}
	c.BranchTag = r.Branch
	c.BranchBG = r.Background
	c.BranchFG = r.Foreground
	c.BranchAt = now
	c.BranchIsWorktree = r.IsWorktree
	c.BranchParentBranch = r.ParentBranch
	return true
}

// hookPreamble is the common subset of hook-payload fields every driver needs
// to validate and absorb before its own logic runs.
type hookPreamble struct {
	SessionID      string
	HookEventName  string
	TranscriptPath string
}

// applyHookPreamble validates ordering and updates common fields.
// Returns false if the hook should be silently dropped.
// Callers should apply driver-specific session-ID fields themselves.
func (c *CommonState) applyHookPreamble(p hookPreamble, e state.DEvHook) bool {
	if p.SessionID == "" {
		return false
	}
	if !e.Timestamp.IsZero() && !e.Timestamp.After(c.LastHookAt) {
		return false
	}
	c.LastHookEvent = p.HookEventName
	if !e.Timestamp.IsZero() {
		c.LastHookAt = e.Timestamp
	}
	if e.RoostSessionID != "" {
		c.RoostSessionID = e.RoostSessionID
	}
	if p.TranscriptPath != "" {
		c.TranscriptPath = p.TranscriptPath
	}
	return true
}

// Common persistence keys shared across drivers.
const (
	keyRoostSessionID       = "roost_session_id"
	keyStartDir             = "start_dir"
	keyTranscriptPath       = "transcript_path"
	keyWorktreeName         = "worktree_name"
	keyStatus               = "status"
	keyStatusChangedAt      = "status_changed_at"
	keyBranchTag            = "branch_tag"
	keyBranchBG             = "branch_bg"
	keyBranchFG             = "branch_fg"
	keyBranchTarget         = "branch_target"
	keyBranchAt             = "branch_at"
	keyBranchIsWorktree     = "branch_is_worktree"
	keyBranchParentBranch   = "branch_parent_branch"
	keySummary              = "summary"
	keyTitle                = "title"
	keyLastPrompt           = "last_prompt"
	keyLastAssistantMessage = "last_assistant_message"
	keyLastHookEvent        = "last_hook_event"
	keyLastHookAt           = "last_hook_at"
)

// PersistCommon writes the shared fields of CommonState into the persistence bag.
func (c *CommonState) PersistCommon(out map[string]string) { //nolint:funlen
	if c.RoostSessionID != "" {
		out[keyRoostSessionID] = c.RoostSessionID
	}
	if c.StartDir != "" {
		out[keyStartDir] = c.StartDir
	}
	if c.TranscriptPath != "" {
		out[keyTranscriptPath] = c.TranscriptPath
	}
	if c.WorktreeName != "" {
		out[keyWorktreeName] = c.WorktreeName
	}
	out[keyStatus] = c.Status.String()
	if !c.StatusChangedAt.IsZero() {
		out[keyStatusChangedAt] = c.StatusChangedAt.UTC().Format(time.RFC3339)
	}
	if c.BranchTag != "" {
		out[keyBranchTag] = c.BranchTag
	}
	if c.BranchBG != "" {
		out[keyBranchBG] = c.BranchBG
	}
	if c.BranchFG != "" {
		out[keyBranchFG] = c.BranchFG
	}
	if c.BranchTarget != "" {
		out[keyBranchTarget] = c.BranchTarget
	}
	if !c.BranchAt.IsZero() {
		out[keyBranchAt] = c.BranchAt.UTC().Format(time.RFC3339)
	}
	if c.BranchIsWorktree {
		out[keyBranchIsWorktree] = "1"
	}
	if c.BranchParentBranch != "" {
		out[keyBranchParentBranch] = c.BranchParentBranch
	}
	if c.Summary != "" {
		out[keySummary] = c.Summary
	}
	if c.Title != "" {
		out[keyTitle] = c.Title
	}
	if c.LastPrompt != "" {
		out[keyLastPrompt] = c.LastPrompt
	}
	if c.LastAssistantMessage != "" {
		out[keyLastAssistantMessage] = c.LastAssistantMessage
	}
	if c.LastHookEvent != "" {
		out[keyLastHookEvent] = c.LastHookEvent
	}
	if !c.LastHookAt.IsZero() {
		out[keyLastHookAt] = c.LastHookAt.UTC().Format(time.RFC3339)
	}
}

// RestoreCommon rehydrates the shared fields of CommonState from the persistence bag.
func (c *CommonState) RestoreCommon(bag map[string]string) {
	if len(bag) == 0 {
		return
	}
	c.RoostSessionID = bag[keyRoostSessionID]
	c.StartDir = bag[keyStartDir]
	c.TranscriptPath = bag[keyTranscriptPath]
	c.WorktreeName = bag[keyWorktreeName]
	if v := bag[keyStatus]; v != "" {
		if status, ok := state.ParseStatus(v); ok {
			c.Status = status
		}
	}
	if v := bag[keyStatusChangedAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			c.StatusChangedAt = t
		}
	}
	c.BranchTag = bag[keyBranchTag]
	c.BranchBG = bag[keyBranchBG]
	c.BranchFG = bag[keyBranchFG]
	c.BranchTarget = bag[keyBranchTarget]
	if v := bag[keyBranchAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			c.BranchAt = t
		}
	}
	c.BranchIsWorktree = bag[keyBranchIsWorktree] == "1"
	c.BranchParentBranch = bag[keyBranchParentBranch]
	c.Summary = bag[keySummary]
	c.Title = bag[keyTitle]
	c.LastPrompt = bag[keyLastPrompt]
	c.LastAssistantMessage = bag[keyLastAssistantMessage]
	c.LastHookEvent = bag[keyLastHookEvent]
	if v := bag[keyLastHookAt]; v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			c.LastHookAt = t
		}
	}
}

// eventLogLine formats an EVENTS log line for a hook-sourced event.
// Produces "[event:<name>]" when detail is empty, or "[event:<name>] <detail>".
func eventLogLine(name, detail string) string {
	if detail == "" {
		return "[event:" + name + "]"
	}
	return "[event:" + name + "] " + detail
}
