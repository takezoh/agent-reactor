package scheduler

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
)

// Deps bundles injectable dependencies for the Scheduler (SPEC §16.2).
// Nil fields receive safe defaults:
//   - Reconcile → no-op
//   - Clock     → real wall clock
//
// Tracker and Spawn may be nil; in that case dispatch is skipped with a warning.
type Deps struct {
	Tracker   CandidateSource
	Spawn     SpawnFunc
	Reconcile ReconcileFunc
	Clock     Clock
}

// Scheduler runs the polling loop per SPEC §16.2.
type Scheduler struct {
	workflowPath string
	interval     time.Duration
	state        *State
	deps         Deps
	clock        Clock
	retryFire    chan retryFireReq
}

// New returns a Scheduler. cfg.Polling.IntervalMS determines the tick interval.
func New(workflowPath string, cfg wfconfig.Config, deps Deps) *Scheduler {
	clk := deps.Clock
	if clk == nil {
		clk = realClock{}
	}
	rec := deps.Reconcile
	if rec == nil {
		rec = func(context.Context) error { return nil }
	}
	deps.Clock = clk
	deps.Reconcile = rec
	return &Scheduler{
		workflowPath: workflowPath,
		interval:     time.Duration(cfg.Polling.IntervalMS) * time.Millisecond,
		state:        NewState(),
		deps:         deps,
		clock:        clk,
		retryFire:    make(chan retryFireReq, 64),
	}
}

// Run starts the scheduler loop and blocks until ctx is cancelled.
// Startup: validate config → reconcile → immediate tick → poll at interval.
func (s *Scheduler) Run(ctx context.Context) error {
	slog.Info("scheduler starting", "interval_ms", s.interval.Milliseconds())
	if s.deps.Spawn == nil {
		slog.Warn("scheduler: no spawn func wired, running in poll-only mode")
	}

	// Startup: reconcile then immediate tick.
	s.runReconcile(ctx)
	s.tickOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return nil
		case <-ticker.C:
			s.tickOnce(ctx)
		case req := <-s.retryFire:
			s.handleRetry(ctx, req)
		}
	}
}

// runReconcile calls the injected reconcile function; errors are logged but do not stop the loop.
func (s *Scheduler) runReconcile(ctx context.Context) {
	if err := s.deps.Reconcile(ctx); err != nil {
		slog.Error("reconcile error", "err", err)
	}
}

// tickOnce runs one poll cycle per SPEC §8.1.
func (s *Scheduler) tickOnce(ctx context.Context) {
	// §8.1: reconcile always runs.
	s.runReconcile(ctx)

	cfg, ok := s.reloadConfig()
	if !ok {
		// dispatch skipped; reconcile already ran above.
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

// reloadConfig reloads the workflow and resolves config; returns false if preflight fails.
func (s *Scheduler) reloadConfig() (wfconfig.Config, bool) {
	wf, err := workflowfile.Load(s.workflowPath)
	if err != nil {
		slog.Error("dispatch skipped", "reason", err)
		return wfconfig.Config{}, false
	}
	cfg, err := wfconfig.Resolve(wf.Config, filepath.Dir(s.workflowPath))
	if err != nil {
		slog.Error("dispatch skipped", "reason", err)
		return wfconfig.Config{}, false
	}
	if err := Preflight(cfg); err != nil {
		slog.Error("dispatch skipped", "reason", err)
		return wfconfig.Config{}, false
	}
	return cfg, true
}

// handleRetry processes a retry-fire event from a timer callback.
func (s *Scheduler) handleRetry(ctx context.Context, req retryFireReq) {
	cfg, ok := s.reloadConfig()
	if !ok {
		return
	}
	if s.deps.Tracker == nil || s.deps.Spawn == nil {
		return
	}
	handleRetryFire(ctx, req, s.deps.Tracker, s.state, s.clock, s.retryFire, s.deps.Spawn, cfg)
}
