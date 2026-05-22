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

// schedulerTrackerAPI is the tracker surface used by reconcile and startup cleanup.
// Satisfied by *orchestrator/tracker.Tracker; fakes implement it in tests.
type schedulerTrackerAPI interface {
	RefreshStates(ctx context.Context, ids []string) ([]ptrackerv.Issue, error)
	TerminalIssues(ctx context.Context) ([]ptrackerv.Issue, error)
}

// schedulerWorkspaceAPI is the workspace surface used by reconcile and startup cleanup.
// Satisfied by *orchestrator/workspace.Manager; fakes implement it in tests.
type schedulerWorkspaceAPI interface {
	Remove(ctx context.Context, identifier string) error
}

// Deps bundles injectable dependencies for the Scheduler (SPEC §16.2).
// A nil Clock defaults to the real wall clock.
// Tracker and Spawn may be nil; in that case dispatch is skipped with a warning.
type Deps struct {
	Tracker        CandidateSource
	Spawn          SpawnFunc
	Clock          Clock
	RefreshTracker schedulerTrackerAPI
	Workspace      schedulerWorkspaceAPI
}

// WorkerExit is sent on the workerDone channel when an agent runner's turn loop ends.
// Err == nil indicates a clean exit; non-nil indicates an abnormal exit.
type WorkerExit struct {
	IssueID string
	Err     error
	Attempt int
}

// CodexActivity is sent on the codexActivity channel when the agent runner receives
// a codex protocol notification (SPEC §10 / §13.5).
type CodexActivity struct {
	IssueID      string
	Event        string // codex notification method name
	Message      string // non-empty for item/agentMessage/delta events
	Timestamp    time.Time
	Usage        *metrics.Usage             // non-nil for thread/tokenUsage/updated
	RateLimit    *metrics.RateLimitSnapshot // non-nil for account/rateLimits/updated
	TurnDuration *time.Duration             // non-nil for turn/completed (elapsed turn time)
}

// Scheduler runs the polling loop per SPEC §16.2.
type Scheduler struct {
	workflowPath  string
	interval      time.Duration
	lastGood      wfconfig.Config // last successfully resolved config; seeded from New
	reloadCh      chan struct{}   // fsnotify → loop coalesced reload signal (buffered 1)
	degraded      bool            // true while workflow is invalid; controls warn/recovery log
	available     atomic.Bool     // true while Run is executing (SPEC §13.3)
	state         *State
	deps          Deps
	clock         Clock
	retryFire     chan retryFireReq
	workerDone    chan WorkerExit
	codexActivity chan CodexActivity
	tracker       schedulerTrackerAPI
	workspace     schedulerWorkspaceAPI
}

// New returns a Scheduler. cfg.Polling.IntervalMS determines the initial tick interval.
// cfg is used as the initial last-known-good (caller must have validated it).
func New(workflowPath string, cfg wfconfig.Config, deps Deps) *Scheduler {
	clk := deps.Clock
	if clk == nil {
		clk = realClock{}
	}
	deps.Clock = clk
	return &Scheduler{
		workflowPath:  workflowPath,
		interval:      time.Duration(cfg.Polling.IntervalMS) * time.Millisecond,
		lastGood:      cfg,
		reloadCh:      make(chan struct{}, 1),
		state:         NewState(),
		deps:          deps,
		clock:         clk,
		retryFire:     make(chan retryFireReq, 64),
		workerDone:    make(chan WorkerExit, 64),
		codexActivity: make(chan CodexActivity, 64),
		tracker:       deps.RefreshTracker,
		workspace:     deps.Workspace,
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
// Startup: startup cleanup → immediate tick → poll at interval.
// WORKFLOW.md is watched via fsnotify for immediate re-apply; poll remains as a safety net.
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
	s.tickOnce(ctx)

	ticker := time.NewTicker(s.intervalOrFallback())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return nil
		case <-ticker.C:
			s.tickOnce(ctx)
			s.applyInterval(ticker)
		case req := <-s.retryFire:
			s.handleRetry(ctx, req)
		case w := <-s.workerDone:
			s.handleWorkerExit(ctx, w)
		case a := <-s.codexActivity:
			s.handleCodexActivity(a)
		case <-s.reloadCh:
			s.tickOnce(ctx)
			s.applyInterval(ticker)
		}
	}
}

// WorkerDone returns the send-side of the worker-exit channel. The agent runner
// sends a WorkerExit on this channel when its turn loop ends (SPEC §16.6).
func (s *Scheduler) WorkerDone() chan<- WorkerExit {
	return s.workerDone
}

// CodexActivity returns the send-side of the codex-activity channel.
func (s *Scheduler) CodexActivity() chan<- CodexActivity {
	return s.codexActivity
}

// Snapshot returns a read-only copy of the current scheduler state (SPEC §7.3).
// Safe to call concurrently with Run.
func (s *Scheduler) Snapshot() StateSnapshot {
	return s.state.Snapshot()
}

// SnapshotCtx returns a read-only copy of the current scheduler state with
// context-aware error handling (SPEC §13.3 RECOMMENDED).
// Returns ErrOrchestratorUnavailable when the scheduler is not running.
// Returns ErrSnapshotTimeout when the context deadline expires before the
// state lock can be acquired.
func (s *Scheduler) SnapshotCtx(ctx context.Context) (StateSnapshot, error) {
	if !s.available.Load() {
		return StateSnapshot{}, ErrOrchestratorUnavailable
	}
	return s.state.SnapshotCtx(ctx)
}

// Refresh queues an immediate poll+reconcile tick. Returns true when the request
// was coalesced with a pending one (i.e. the signal channel was already full).
// The operation is best-effort and non-blocking.
func (s *Scheduler) Refresh() (coalesced bool) {
	select {
	case s.reloadCh <- struct{}{}:
		return false
	default:
		return true
	}
}

// handleWorkerExit processes a worker-exit notification from the agent runner.
// It releases the scheduler slot and schedules a continuation or backoff retry.
func (s *Scheduler) handleWorkerExit(ctx context.Context, w WorkerExit) {
	cfg, _ := s.reloadConfig()
	if w.Err == nil {
		if entry, ok := s.state.WorkerExitNormal(w.IssueID); ok {
			scheduleRetry(s.state, s.clock, s.retryFire, ctx, entry, continuationDelay)
		}
		return
	}
	if entry, ok := s.state.WorkerExitAbnormal(w.IssueID, w.Err, w.Attempt); ok {
		scheduleRetry(s.state, s.clock, s.retryFire, ctx, entry, backoffDelay(entry.Attempt, cfg))
	}
}

// handleCodexActivity processes a codex notification event from the agent runner.
func (s *Scheduler) handleCodexActivity(a CodexActivity) {
	s.state.UpdateCodexActivity(a.IssueID, a.Event, a.Message, a.Timestamp)
	if a.Usage != nil {
		s.state.RecordUsage(a.IssueID, *a.Usage)
	}
	if a.RateLimit != nil {
		s.state.RecordRateLimit(a.IssueID, *a.RateLimit)
	}
	if a.TurnDuration != nil {
		s.state.AddRuntime(a.IssueID, *a.TurnDuration)
	}
}

// intervalOrFallback returns the current interval, defaulting to 1s if zero.
// A zero interval (e.g. in tests with empty cfg) would panic time.NewTicker.
func (s *Scheduler) intervalOrFallback() time.Duration {
	if s.interval > 0 {
		return s.interval
	}
	return time.Second
}

// applyInterval resets ticker if the last-known-good config specifies a different interval.
func (s *Scheduler) applyInterval(ticker *time.Ticker) {
	want := time.Duration(s.lastGood.Polling.IntervalMS) * time.Millisecond
	if want > 0 && want != s.interval {
		s.interval = want
		ticker.Reset(want)
		slog.Info("scheduler: poll interval updated", "interval_ms", want.Milliseconds())
	}
}

// tickOnce runs one poll cycle per SPEC §8.1.
// reconcile always runs on last-known-good cfg (§5.5: stall/terminal cleanup must not stop).
// dispatchOnce is skipped when the workflow is currently invalid (§5.5 dispatch gating).
func (s *Scheduler) tickOnce(ctx context.Context) {
	cfg, valid := s.reloadConfig()

	// §8.1 step 1: reconcile runs even when workflow is invalid (keeps running agents healthy).
	s.reconcile(ctx, cfg)

	if !valid {
		return
	}

	if s.deps.Tracker == nil || s.deps.Spawn == nil {
		slog.Info("tick: tracker or spawn not wired, skipping dispatch")
		return
	}

	cands, err := s.deps.Tracker.Candidates(ctx)
	if err != nil {
		slog.Error("tick: candidates fetch failed", "err", err)
		return
	}

	dispatchOnce(ctx, cands, s.state, s.clock, s.retryFire, s.deps.Spawn, cfg)
}

// reloadConfig reloads WORKFLOW.md and resolves config (§6.2).
// On failure: returns last-known-good with valid=false; emits one operator-visible warn.
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
	return cfg, true
}

func (s *Scheduler) markDegraded(err error) {
	if !s.degraded {
		slog.Warn("scheduler: workflow reload failed, gating new dispatch", "reason", err)
		s.degraded = true
	}
}

// handleRetry processes a retry-fire event from a timer callback.
// Gated by §5.5: spawn is skipped while the workflow is invalid.
func (s *Scheduler) handleRetry(ctx context.Context, req retryFireReq) {
	cfg, valid := s.reloadConfig()
	if !valid {
		return
	}
	if s.deps.Tracker == nil || s.deps.Spawn == nil {
		return
	}
	handleRetryFire(ctx, req, s.deps.Tracker, s.state, s.clock, s.retryFire, s.deps.Spawn, cfg)
}
