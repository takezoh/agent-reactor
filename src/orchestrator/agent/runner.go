package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/prompt"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const spawnTimeout = 30 * time.Second

type launchResult struct {
	conn         *codexclient.Conn
	sessionReady <-chan sessionIDs
	turnDone     <-chan turnResult
	doneCh       <-chan struct{}
	cleanup      func(context.Context) error
}

func (r *Runner) spawnWith(ctx context.Context, issue tracker.Issue, attempt int, emit func(Event)) (scheduler.LiveSession, error) {
	wsPath, err := r.prepareWorkspace(ctx, issue.Identifier)
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	rendered, err := prompt.Render(r.PromptTemplate, prompt.Vars{Issue: issue, Attempt: attempt})
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	frameID := fmt.Sprintf("%s#%d", issue.Identifier, attempt)

	workerCtx, cancel := context.WithCancel(ctx)
	lr, err := r.launchConn(workerCtx, frameID, wsPath)
	if err != nil {
		cancel()
		return scheduler.LiveSession{}, err
	}

	ids, err := initSession(lr.conn, wsPath, rendered, lr.sessionReady, lr.doneCh)
	if err != nil {
		cancel()
		<-lr.doneCh
		return scheduler.LiveSession{}, err
	}

	worker := &Worker{cancel: cancel, done: lr.doneCh, cleanup: lr.cleanup}
	go r.runMonitor(issue.Identifier, ids, lr.turnDone, lr.doneCh, cancel, worker, emit)

	emit(Event{
		Kind:      EventSessionStarted,
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		Timestamp: time.Now(),
	})

	return scheduler.LiveSession{
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		StartedAt: time.Now(),
		Worker:    worker,
	}, nil
}

func (r *Runner) prepareWorkspace(ctx context.Context, identifier string) (string, error) {
	wsPath, err := r.Workspace.Ensure(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("agent: workspace ensure: %w", err)
	}
	if err := r.Workspace.VerifyCWD(identifier, wsPath); err != nil {
		return "", fmt.Errorf("agent: verify cwd: %w", err)
	}
	if err := r.Workspace.BeforeRun(ctx, identifier); err != nil {
		return "", fmt.Errorf("agent: before run: %w", err)
	}
	return wsPath, nil
}

func (r *Runner) launchConn(ctx context.Context, frameID, wsPath string) (*launchResult, error) {
	plan := agentlaunch.LaunchPlan{
		Command:  r.Cfg.Codex.Command,
		Env:      map[string]string{},
		StartDir: wsPath,
		// TODO(016): use actual project root for devcontainer/sandbox routing.
		Project: r.Cfg.Workspace.Root,
	}
	wrapped, err := r.Dispatcher.Wrap(ctx, frameID, plan)
	if err != nil {
		return nil, fmt.Errorf("agent: dispatch wrap: %w", err)
	}

	stdout, stdin, wait, err := r.proc(ctx, wrapped.StartDir, wrapped.Env, wrapped.Command)
	if err != nil {
		return nil, err
	}

	readTimeout := time.Duration(r.Cfg.Codex.ReadTimeoutMS) * time.Millisecond
	tr := codexclient.StdioTransport(stdout, stdin)
	conn := codexclient.NewConn(tr, readTimeout)

	sessionReady := make(chan sessionIDs, 1)
	turnDone := make(chan turnResult, 1)
	h := &turnHandler{
		conn:         conn,
		sessionReady: sessionReady,
		turnDone:     turnDone,
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		_ = conn.Run(ctx, h)
		wait() // reap the subprocess after the read loop drains stdout
	}()

	return &launchResult{
		conn:         conn,
		sessionReady: sessionReady,
		turnDone:     turnDone,
		doneCh:       doneCh,
		cleanup:      wrapped.Cleanup,
	}, nil
}

func initSession(conn *codexclient.Conn, wsPath, rendered string, sessionReady <-chan sessionIDs, doneCh <-chan struct{}) (sessionIDs, error) {
	if err := codexclient.Initialize(conn); err != nil {
		return sessionIDs{}, fmt.Errorf("agent: initialize: %w", err)
	}
	if err := codexclient.StartTurn(conn, "", wsPath, []byte(rendered)); err != nil {
		return sessionIDs{}, fmt.Errorf("agent: start turn: %w", err)
	}

	timer := time.NewTimer(spawnTimeout)
	defer timer.Stop()

	select {
	case ids := <-sessionReady:
		return ids, nil
	case <-timer.C:
		return sessionIDs{}, errors.New("agent: timeout waiting for session start")
	case <-doneCh:
		return sessionIDs{}, errors.New("agent: codex exited before session started")
	}
}

func (r *Runner) runMonitor(identifier string, ids sessionIDs, turnDone <-chan turnResult, doneCh <-chan struct{}, cancel context.CancelFunc, worker *Worker, emit func(Event)) {
	var result turnResult

	// Enforce codex.turn_timeout_ms (§10.3): a turn that neither completes nor
	// fails within the budget is killed and reported as a failure.
	var timeoutCh <-chan time.Time
	if r.Cfg.Codex.TurnTimeoutMS > 0 {
		timer := time.NewTimer(time.Duration(r.Cfg.Codex.TurnTimeoutMS) * time.Millisecond)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case result = <-turnDone:
	case <-timeoutCh:
		cancel()
		<-doneCh
		result = turnResult{failed: true, err: fmt.Errorf("turn timeout exceeded (%dms)", r.Cfg.Codex.TurnTimeoutMS)}
	case <-doneCh:
		select {
		case result = <-turnDone:
		default:
			result = turnResult{failed: true, err: errors.New("codex process exited unexpectedly")}
		}
	}

	if result.failed {
		emit(Event{
			Kind:      EventTurnFailed,
			SessionID: ids.sessionID(),
			ThreadID:  ids.threadID,
			TurnID:    ids.turnID,
			Timestamp: time.Now(),
			Err:       result.err,
		})
	} else {
		emit(Event{
			Kind:      EventTurnCompleted,
			SessionID: ids.sessionID(),
			ThreadID:  ids.threadID,
			TurnID:    ids.turnID,
			Timestamp: time.Now(),
		})
	}

	// Single-turn (§16.5): stop the session/subprocess, then run after_run
	// best-effort. cancel triggers conn.Run to return, which reaps the process.
	cancel()
	<-doneCh
	worker.runCleanup()
	r.Workspace.AfterRun(context.Background(), identifier)
}
