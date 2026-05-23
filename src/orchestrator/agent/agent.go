// Package agent implements the SPEC §10 / §16.5 agent runner for the orchestrator.
package agent

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/lineargql"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// stateRefresher fetches current issue states between turns (SPEC §16.5).
// Satisfied by *orchestrator/tracker.Tracker; fakes implement it in tests.
type stateRefresher interface {
	RefreshStates(ctx context.Context, ids []string) ([]tracker.Issue, error)
}

const (
	EventSessionStarted      = "session_started"
	EventTurnCompleted       = "turn_completed"
	EventTurnFailed          = "turn_failed"
	EventTurnCancelled       = "turn_cancelled"
	EventStartupFailed       = "startup_failed"
	EventUnsupportedToolCall = "unsupported_tool_call"
	EventTurnInputRequired   = "turn_input_required"
)

// Event carries a single agent lifecycle notification.
type Event struct {
	Kind      string
	SessionID string
	ThreadID  string
	TurnID    string
	Timestamp time.Time
	Err       error
}

// procFunc launches the agent subprocess and returns its stdout/stdin plus a
// wait func that reaps the process. wait must be called once the read loop has
// drained stdout (i.e. after conn.Run returns) to avoid leaking a zombie.
type procFunc func(ctx context.Context, dir string, env map[string]string, command string) (stdout io.ReadCloser, stdin io.WriteCloser, wait func(), err error)

// Runner builds scheduler.SpawnFunc-compatible spawn calls for the orchestrator.
type Runner struct {
	Workspace      *workspace.Manager
	Cfg            wfconfig.Config
	PromptTemplate string        // static fallback; used when PromptLoader is nil
	PromptLoader   func() string // called per-dispatch to pick up live WORKFLOW.md edits (SPEC §6.2)
	Dispatcher     agentlaunch.Dispatcher
	Tracker        stateRefresher
	WorkerDone     chan<- scheduler.WorkerExit
	CodexActivity  chan<- scheduler.CodexActivity
	LinearClient   *lineargql.Client // nil disables the linear_graphql tool (SPEC §10.5)
	proc           procFunc
}

// New returns a Runner wired with the real exec subprocess.
// If cfg.Tracker.APIKey and cfg.Tracker.Endpoint are set, the linear_graphql
// agent tool (SPEC §10.5) is enabled using those credentials.
func New(ws *workspace.Manager, cfg wfconfig.Config, tmpl string, d agentlaunch.Dispatcher, tr stateRefresher) *Runner {
	var lc *lineargql.Client
	if cfg.Tracker.APIKey != "" && cfg.Tracker.Endpoint != "" {
		lc = lineargql.New(cfg.Tracker.Endpoint, cfg.Tracker.APIKey)
	}
	return &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: tmpl,
		Dispatcher:     d,
		Tracker:        tr,
		LinearClient:   lc,
		proc:           realProc,
	}
}

// Spawn satisfies scheduler.SpawnFunc. Events are logged via slog.
func (r *Runner) Spawn(ctx context.Context, issue tracker.Issue, attempt int) (scheduler.LiveSession, error) {
	return r.spawnWith(ctx, issue, attempt, func(e Event) {
		slog.Info("agent event",
			"kind", e.Kind,
			"issue_id", issue.ID,
			"issue_identifier", issue.Identifier,
			"session_id", e.SessionID,
			"thread_id", e.ThreadID,
			"turn_id", e.TurnID,
			"err", e.Err,
		)
	})
}
