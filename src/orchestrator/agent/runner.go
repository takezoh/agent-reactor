package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/prompt"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	"github.com/takezoh/agent-roost/platform/tracker"
)

const (
	spawnTimeout       = 30 * time.Second
	continuationPrompt = "Continue working on the issue."
)

type launchResult struct {
	conn         *codexclient.Conn
	startDir     string // container-translated working dir (or host wsPath for direct mode)
	sessionReady <-chan sessionIDs
	turnDone     <-chan turnResult
	doneCh       <-chan struct{}
	cleanup      func(context.Context) error
}

// workerParams bundles the per-session state for the multi-turn loop goroutine.
type workerParams struct {
	issue        tracker.Issue
	attempt      int
	conn         *codexclient.Conn
	startDir     string
	ids          sessionIDs
	sessionReady <-chan sessionIDs
	turnDone     <-chan turnResult
	doneCh       <-chan struct{}
	cancel       context.CancelFunc
	worker       *Worker
	emit         func(Event)
}

func (r *Runner) spawnWith(ctx context.Context, issue tracker.Issue, attempt int, emit func(Event)) (scheduler.LiveSession, error) {
	wsPath, err := r.ensureWorkspace(ctx, issue.Identifier)
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	// Workspace exists from here — after_run must execute on every exit path (SPEC §9.4).
	// committed is set true when runLoop takes ownership of teardown.
	committed := false
	defer func() {
		if !committed {
			r.Workspace.AfterRun(context.Background(), issue.Identifier)
		}
	}()

	if err := r.Workspace.BeforeRun(ctx, issue.Identifier); err != nil {
		return scheduler.LiveSession{}, fmt.Errorf("agent: before run: %w", err)
	}

	rendered, err := r.renderPrompt(issue, attempt)
	if err != nil {
		return scheduler.LiveSession{}, err
	}

	frameID := fmt.Sprintf("%s#%d", issue.Identifier, attempt)

	workerCtx, cancel := context.WithCancel(ctx)
	lr, err := r.launchConn(workerCtx, frameID, wsPath, issue.ID)
	if err != nil {
		cancel()
		return scheduler.LiveSession{}, err
	}

	ids, err := initSession(lr.conn, lr.startDir, rendered, r.dynamicToolSpecs(), lr.sessionReady, lr.doneCh)
	if err != nil {
		cancel()
		<-lr.doneCh
		return scheduler.LiveSession{}, err
	}

	worker := &Worker{cancel: cancel, done: lr.doneCh, cleanup: lr.cleanup}

	wp := workerParams{
		issue:        issue,
		attempt:      attempt,
		conn:         lr.conn,
		startDir:     lr.startDir,
		ids:          ids,
		sessionReady: lr.sessionReady,
		turnDone:     lr.turnDone,
		doneCh:       lr.doneCh,
		cancel:       cancel,
		worker:       worker,
		emit:         emit,
	}
	committed = true
	go r.runLoop(workerCtx, wp)

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

// runLoop runs the §16.5 multi-turn while-loop in a goroutine.
// It signals worker exit via sendWorkerExit after teardown completes.
func (r *Runner) runLoop(ctx context.Context, wp workerParams) {
	var loopErr error
	for turn := 1; ; turn++ {
		result := r.awaitTurn(wp)
		if result.failed {
			wp.emit(r.turnFailedEvent(wp.ids, result.err))
			loopErr = result.err
			break
		}
		wp.emit(r.turnCompletedEvent(wp.ids))

		if !r.shouldContinue(ctx, wp.issue, turn) {
			break
		}
		if err := r.nextTurn(ctx, &wp, turn+1); err != nil {
			loopErr = err
			break
		}
	}
	r.teardown(wp)
	r.sendWorkerExit(wp.issue.ID, wp.attempt, loopErr)
}

// awaitTurn waits for the current turn to complete, applying per-turn timeout.
// Timeout and unexpected process exit are mapped to turnResult{failed:true}.
func (r *Runner) awaitTurn(wp workerParams) turnResult {
	var timeoutCh <-chan time.Time
	if r.Cfg.Codex.TurnTimeoutMS > 0 {
		timer := time.NewTimer(time.Duration(r.Cfg.Codex.TurnTimeoutMS) * time.Millisecond)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case result := <-wp.turnDone:
		return result
	case <-timeoutCh:
		wp.cancel()
		<-wp.doneCh
		return turnResult{failed: true, err: fmt.Errorf("turn timeout exceeded (%dms)", r.Cfg.Codex.TurnTimeoutMS)}
	case <-wp.doneCh:
		select {
		case result := <-wp.turnDone:
			return result
		default:
			return turnResult{failed: true, err: errors.New("codex process exited unexpectedly")}
		}
	}
}

// shouldContinue re-fetches issue state and checks continuation criteria (§16.5).
// Returns false when the issue is no longer active or max_turns is reached.
func (r *Runner) shouldContinue(ctx context.Context, issue tracker.Issue, turn int) bool {
	if turn >= r.Cfg.Agent.MaxTurns {
		return false
	}
	if r.Tracker == nil {
		return false
	}
	issues, err := r.Tracker.RefreshStates(ctx, []string{issue.ID})
	if err != nil {
		return false
	}
	if len(issues) == 0 {
		return false
	}
	norm := strings.ToLower(issues[0].State)
	for _, s := range r.Cfg.Tracker.ActiveStates {
		if strings.ToLower(s) == norm {
			return true
		}
	}
	return false
}

// nextTurn issues the next turn on the same thread and awaits turn/started.
func (r *Runner) nextTurn(workerCtx context.Context, wp *workerParams, turn int) error {
	if err := codexclient.StartTurn(wp.conn, wp.ids.threadID, wp.startDir, []byte(continuationPrompt)); err != nil {
		return fmt.Errorf("agent: start turn %d: %w", turn, err)
	}
	timer := time.NewTimer(spawnTimeout)
	defer timer.Stop()
	select {
	case ids := <-wp.sessionReady:
		wp.ids = ids
		return nil
	case <-timer.C:
		return fmt.Errorf("agent: timeout waiting for turn %d to start", turn)
	case <-wp.doneCh:
		return fmt.Errorf("agent: codex exited before turn %d started", turn)
	case <-workerCtx.Done():
		return fmt.Errorf("agent: context cancelled waiting for turn %d", turn)
	}
}

// teardown cancels the worker context, waits for process exit, and runs cleanup.
func (r *Runner) teardown(wp workerParams) {
	wp.cancel()
	<-wp.doneCh
	wp.worker.runCleanup()
	r.Workspace.AfterRun(context.Background(), wp.issue.Identifier)
}

// sendWorkerExit delivers the worker exit signal to the scheduler (§16.6).
// Uses non-blocking send because the channel is buffered (cap 64) and teardown
// must complete before the scheduler can re-dispatch to the same workspace.
func (r *Runner) sendWorkerExit(issueID string, attempt int, exitErr error) {
	if r.WorkerDone == nil {
		return
	}
	select {
	case r.WorkerDone <- scheduler.WorkerExit{IssueID: issueID, Err: exitErr, Attempt: attempt}:
	default:
	}
}

// ensureWorkspace idempotently creates the workspace directory and verifies the
// cwd invariant (§9.5).  After this call succeeds the workspace is guaranteed
// to exist, so the caller must arrange for AfterRun on any subsequent failure.
func (r *Runner) ensureWorkspace(ctx context.Context, identifier string) (string, error) {
	wsPath, err := r.Workspace.Ensure(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("agent: workspace ensure: %w", err)
	}
	if err := r.Workspace.VerifyCWD(identifier, wsPath); err != nil {
		return "", fmt.Errorf("agent: verify cwd: %w", err)
	}
	return wsPath, nil
}

func (r *Runner) renderPrompt(issue tracker.Issue, attempt int) (string, error) {
	rendered, err := prompt.Render(r.PromptTemplate, prompt.Vars{Issue: issue, Attempt: attempt})
	if err != nil {
		return "", fmt.Errorf("agent: render prompt: %w", err)
	}
	return rendered, nil
}

func (r *Runner) launchConn(ctx context.Context, frameID, wsPath, issueID string) (*launchResult, error) {
	plan := agentlaunch.LaunchPlan{
		Command: r.Cfg.Codex.Command,
		Env:     map[string]string{},
		// StartDir is the per-issue workspace (agent cwd); Project is the
		// workspace root so every issue shares one per-project devcontainer.
		// pathmap translates StartDir to <WorkspaceTarget>/<id> inside it.
		StartDir: wsPath,
		Project:  r.Workspace.Root(),
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
	activity := r.CodexActivity
	report := func(a scheduler.CodexActivity) {
		if activity == nil {
			return
		}
		select {
		case activity <- a:
		default:
		}
	}
	h := &turnHandler{
		conn:         conn,
		linearClient: r.LinearClient,
		sessionReady: sessionReady,
		turnDone:     turnDone,
		issueID:      issueID,
		report:       report,
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		_ = conn.Run(ctx, h)
		wait() // reap the subprocess after the read loop drains stdout
	}()

	return &launchResult{
		conn:         conn,
		startDir:     wrapped.StartDir,
		sessionReady: sessionReady,
		turnDone:     turnDone,
		doneCh:       doneCh,
		cleanup:      wrapped.Cleanup,
	}, nil
}

func initSession(conn *codexclient.Conn, wsPath, rendered string, dynamicTools []any, sessionReady <-chan sessionIDs, doneCh <-chan struct{}) (sessionIDs, error) {
	if err := codexclient.Initialize(conn); err != nil {
		return sessionIDs{}, fmt.Errorf("agent: initialize: %w", err)
	}
	threadID, err := codexclient.StartThread(conn, wsPath, dynamicTools)
	if err != nil {
		return sessionIDs{}, fmt.Errorf("agent: start thread: %w", err)
	}
	if err := codexclient.StartTurn(conn, threadID, wsPath, []byte(rendered)); err != nil {
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

func (r *Runner) turnCompletedEvent(ids sessionIDs) Event {
	return Event{
		Kind:      EventTurnCompleted,
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		Timestamp: time.Now(),
	}
}

func (r *Runner) turnFailedEvent(ids sessionIDs, err error) Event {
	return Event{
		Kind:      EventTurnFailed,
		SessionID: ids.sessionID(),
		ThreadID:  ids.threadID,
		TurnID:    ids.turnID,
		Timestamp: time.Now(),
		Err:       err,
	}
}
