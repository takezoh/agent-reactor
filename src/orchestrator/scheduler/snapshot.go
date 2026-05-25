package scheduler

import (
	"context"
	"errors"
)

// ErrOrchestratorUnavailable is returned by SnapshotCtx when the scheduler is not running (SPEC §13.3).
var ErrOrchestratorUnavailable = errors.New("orchestrator unavailable")

// Snapshot returns a read-only projection of the current published state (SPEC §7.3).
// Safe to call concurrently with Run: the loop publishes an immutable State value after each
// reduce, and this reads it lock-free via an atomic pointer.
func (s *Scheduler) Snapshot() StateSnapshot {
	st := s.published.Load()
	if st == nil {
		return StateSnapshot{}
	}
	return st.Snapshot()
}

// SnapshotCtx returns the current state for observability (SPEC §13.3).
// Returns ErrOrchestratorUnavailable when the scheduler is not running. Because the read is
// lock-free against an immutable published snapshot, it cannot block — the §13.3 RECOMMENDED
// timeout path is therefore not applicable (see symphony-conformance.md).
func (s *Scheduler) SnapshotCtx(ctx context.Context) (StateSnapshot, error) {
	if !s.available.Load() || ctx.Err() != nil {
		return StateSnapshot{}, ErrOrchestratorUnavailable
	}
	return s.Snapshot(), nil
}
