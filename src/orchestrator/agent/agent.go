// Package agent implements the SPEC §10 / §16.5 agent runner for the orchestrator.
package agent

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const (
	EventSessionStarted = "session_started"
	EventTurnCompleted  = "turn_completed"
	EventTurnFailed     = "turn_failed"
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
	PromptTemplate string
	Dispatcher     agentlaunch.Dispatcher
	proc           procFunc
}

// New returns a Runner wired with the real exec subprocess and DirectDispatcher.
func New(ws *workspace.Manager, cfg wfconfig.Config, tmpl string, d agentlaunch.Dispatcher) *Runner {
	return &Runner{
		Workspace:      ws,
		Cfg:            cfg,
		PromptTemplate: tmpl,
		Dispatcher:     d,
		proc:           realProc,
	}
}

// Spawn satisfies scheduler.SpawnFunc. Events are logged via slog.
func (r *Runner) Spawn(ctx context.Context, issue tracker.Issue, attempt int) (scheduler.LiveSession, error) {
	return r.spawnWith(ctx, issue, attempt, func(e Event) {
		slog.Info("agent event",
			"kind", e.Kind,
			"session_id", e.SessionID,
			"thread_id", e.ThreadID,
			"turn_id", e.TurnID,
			"err", e.Err,
		)
	})
}
