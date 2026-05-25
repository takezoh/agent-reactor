package scheduler

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	ptrackerv "github.com/takezoh/agent-roost/platform/tracker"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
)

// schedulerTrackerAPI is the tracker surface used by reconcile, revalidation, and startup cleanup.
// Satisfied by *orchestrator/tracker.Tracker; fakes implement it in tests.
type schedulerTrackerAPI interface {
	RefreshStates(ctx context.Context, ids []string) ([]ptrackerv.Issue, error)
	TerminalIssues(ctx context.Context) ([]ptrackerv.Issue, error)
}

// schedulerWorkspaceAPI is the workspace surface used by reconcile and startup cleanup.
type schedulerWorkspaceAPI interface {
	Remove(ctx context.Context, identifier string) error
}

// Deps bundles injectable dependencies for the Scheduler (SPEC §16.2).
// A nil Clock defaults to the real wall clock. Tracker and Spawn may be nil; dispatch is
// then skipped with a warning. These injection points are how the shell stays testable.
type Deps struct {
	Tracker        CandidateSource
	Spawn          SpawnFunc
	Clock          Clock
	RefreshTracker schedulerTrackerAPI
	Workspace      schedulerWorkspaceAPI
}

// WorkerExit is sent on the WorkerDone channel when an agent runner's turn loop ends (SPEC §16.6).
// Err == nil indicates a clean exit; non-nil indicates an abnormal exit.
type WorkerExit struct {
	IssueID string
	Err     error
	Attempt int
}

// CodexActivity is sent on the CodexActivity channel when the agent runner receives a codex
// protocol notification (SPEC §10 / §13.5).
type CodexActivity struct {
	IssueID       string
	Event         string
	Message       string
	Timestamp     time.Time
	Usage         *metrics.Usage
	RateLimit     *metrics.RateLimitSnapshot
	TurnDuration  *time.Duration
	TurnCompleted bool
}

// retryFireReq is sent by a retry timer callback to the Run loop.
type retryFireReq struct {
	IssueID    string
	Identifier string
	Attempt    int
}

// Scheduler is the imperative shell around the pure Reduce core (SPEC §16.2). The single Run
// loop owns the authoritative State and is the only writer; it interprets the Effects that
// Reduce returns into real I/O, holds live handles (workers, retry timers) in id→handle maps,
// and publishes immutable State snapshots lock-free for the observability HTTP server.
type Scheduler struct {
	workflowPath     string
	interval         time.Duration
	lastGood         wfconfig.Config
	lastGoodTemplate string
	reloadCh         chan struct{}
	degraded         bool
	available        atomic.Bool

	deps      Deps
	clock     Clock
	tracker   schedulerTrackerAPI
	workspace schedulerWorkspaceAPI

	// Loop-owned state. cur is mutated only by the Run loop; published holds an immutable
	// copy for concurrent lock-free reads (SnapshotCtx, SPEC §13.3).
	cur       State
	cfg       wfconfig.Config
	published atomic.Pointer[State]
	workers   map[string]Worker
	timers    map[string]Timer

	retryFire     chan retryFireReq
	workerDone    chan WorkerExit
	codexActivity chan CodexActivity
}

// New returns a Scheduler. cfg is the initial last-known-good config (caller must have
// validated it). tmpl is the initial prompt template body.
func New(workflowPath string, cfg wfconfig.Config, tmpl string, deps Deps) *Scheduler {
	clk := deps.Clock
	if clk == nil {
		clk = realClock{}
	}
	deps.Clock = clk
	s := &Scheduler{
		workflowPath:     workflowPath,
		interval:         time.Duration(cfg.Polling.IntervalMS) * time.Millisecond,
		lastGood:         cfg,
		lastGoodTemplate: tmpl,
		reloadCh:         make(chan struct{}, 1),
		deps:             deps,
		clock:            clk,
		tracker:          deps.RefreshTracker,
		workspace:        deps.Workspace,
		cur:              NewState(),
		cfg:              cfg,
		workers:          map[string]Worker{},
		timers:           map[string]Timer{},
		retryFire:        make(chan retryFireReq, 64),
		workerDone:       make(chan WorkerExit, 64),
		codexActivity:    make(chan CodexActivity, 64),
	}
	s.publish()
	return s
}

// LastGoodTemplate returns the most recently successfully loaded prompt template body.
func (s *Scheduler) LastGoodTemplate() string { return s.lastGoodTemplate }

// WorkerDone returns the send-side of the worker-exit channel (SPEC §16.6).
func (s *Scheduler) WorkerDone() chan<- WorkerExit { return s.workerDone }

// CodexActivity returns the send-side of the codex-activity channel.
func (s *Scheduler) CodexActivity() chan<- CodexActivity { return s.codexActivity }

// Refresh queues an immediate poll+reconcile tick. Returns true when coalesced with a
// pending request. Best-effort and non-blocking.
func (s *Scheduler) Refresh() (coalesced bool) {
	select {
	case s.reloadCh <- struct{}{}:
		return false
	default:
		return true
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	s.available.Store(true)
	defer s.available.Store(false)
	slog.Info("scheduler starting", "interval_ms", s.interval.Milliseconds())
	if s.deps.Spawn == nil {
		slog.Warn("scheduler: no spawn func wired, running in poll-only mode")
	}

	if s.workflowPath != "" {
		closer, err := watchWorkflow(ctx, s.workflowPath, s.reloadCh)
		if err != nil {
			slog.Warn("scheduler: fsnotify watch failed, falling back to poll-only", "err", err)
		} else {
			defer closer.Close()
		}
	}

	s.StartupCleanup(ctx)
	s.tick(ctx)

	ticker := time.NewTicker(s.intervalOrFallback())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return nil
		case <-ticker.C:
			s.tick(ctx)
			s.applyInterval(ticker)
		case req := <-s.retryFire:
			s.handleRetryFire(ctx, req)
		case w := <-s.workerDone:
			delete(s.workers, w.IssueID)
			s.cfg, _ = s.reloadConfig()
			s.step(ctx, EvWorkerExit(w))
		case a := <-s.codexActivity:
			s.step(ctx, codexEvent(a))
		case <-s.reloadCh:
			s.tick(ctx)
			s.applyInterval(ticker)
		}
	}
}

// tick reloads config and folds one EvTick (SPEC §8.1). Reconcile runs on last-known-good
// config even when invalid; dispatch is gated on ConfigValid (§5.5).
func (s *Scheduler) tick(ctx context.Context) {
	cfg, valid := s.reloadConfig()
	s.cfg = cfg
	s.step(ctx, EvTick{ConfigValid: valid})
}

// handleRetryFire processes a fired retry timer. While the workflow is invalid the request is
// rescheduled with a fixed delay (no attempt increment) so the issue is not lost (§5.5).
func (s *Scheduler) handleRetryFire(ctx context.Context, req retryFireReq) {
	delete(s.timers, req.IssueID)
	cfg, valid := s.reloadConfig()
	if !valid {
		// Transient infra problem, not an agent failure: re-arm without incrementing the
		// attempt. Refresh the pending entry's DueAtMS so observability reflects the new
		// fire time (the entry itself is retained from when the retry was first armed).
		if entry, ok := s.cur.RetryAttempts[req.IssueID]; ok {
			entry.DueAtMS = s.clock.Now().Add(continuationDelay).UnixMilli()
			s.cur = enqueueRetry(s.cur, entry)
			s.publish()
		}
		s.armTimer(ctx, req.IssueID, req.Identifier, req.Attempt, continuationDelay)
		return
	}
	s.cfg = cfg
	s.step(ctx, EvRetryDue(req))
}

// step folds one event through the pure Reduce, publishes the new state, and interprets the
// resulting effects (which may feed follow-up events back synchronously).
func (s *Scheduler) step(ctx context.Context, ev Event) {
	next, effs := Reduce(s.cur, ev, s.cfg, s.clock.Now())
	s.cur = next
	s.publish()
	for _, eff := range effs {
		s.exec(ctx, eff)
	}
}

// publish stores an immutable copy of the current state for lock-free observability reads.
func (s *Scheduler) publish() {
	cp := s.cur
	s.published.Store(&cp)
}

// codexEvent converts the public CodexActivity channel message into the internal event.
func codexEvent(a CodexActivity) EvCodexActivity { return EvCodexActivity(a) }

// intervalOrFallback returns the current interval, defaulting to 1s if zero.
func (s *Scheduler) intervalOrFallback() time.Duration {
	if s.interval > 0 {
		return s.interval
	}
	return time.Second
}

// applyInterval resets the ticker if the last-known-good config changed the poll interval.
func (s *Scheduler) applyInterval(ticker *time.Ticker) {
	want := time.Duration(s.lastGood.Polling.IntervalMS) * time.Millisecond
	if want > 0 && want != s.interval {
		s.interval = want
		ticker.Reset(want)
		slog.Info("scheduler: poll interval updated", "interval_ms", want.Milliseconds())
	}
}

// reloadConfig reloads WORKFLOW.md and resolves config (§6.2).
// On failure: returns last-known-good with valid=false and emits one operator-visible warn.
// On success: updates last-known-good; logs recovery if previously degraded.
func (s *Scheduler) reloadConfig() (wfconfig.Config, bool) {
	wf, err := workflowfile.Load(s.workflowPath)
	if err != nil {
		s.markDegraded(err)
		return s.lastGood, false
	}
	cfg, err := wfconfig.Resolve(wf.Config, filepath.Dir(s.workflowPath))
	if err != nil {
		s.markDegraded(err)
		return s.lastGood, false
	}
	if err := Preflight(cfg); err != nil {
		s.markDegraded(err)
		return s.lastGood, false
	}
	if s.degraded {
		slog.Info("scheduler: workflow recovered, resuming dispatch")
		s.degraded = false
	}
	s.lastGood = cfg
	s.lastGoodTemplate = wf.PromptTemplate
	return cfg, true
}

func (s *Scheduler) markDegraded(err error) {
	if !s.degraded {
		slog.Warn("scheduler: workflow reload failed, gating new dispatch", "reason", err)
		s.degraded = true
	}
}
