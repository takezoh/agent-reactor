package scheduler

import (
	"context"
	"errors"
)

// ErrSnapshotTimeout is returned by SnapshotCtx when the state lock cannot be acquired before the deadline (SPEC §13.3).
var ErrSnapshotTimeout = errors.New("snapshot timeout")

// ErrOrchestratorUnavailable is returned by SnapshotCtx when the scheduler is not running (SPEC §13.3).
var ErrOrchestratorUnavailable = errors.New("orchestrator unavailable")

// SnapshotCtx acquires the state lock via a goroutine; buffered chan(1) prevents goroutine leak on timeout (SPEC §13.3).
func (s *State) SnapshotCtx(ctx context.Context) (StateSnapshot, error) {
	ch := make(chan StateSnapshot, 1)
	go func() {
		ch <- s.Snapshot()
	}()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return StateSnapshot{}, ErrSnapshotTimeout
		}
		return StateSnapshot{}, ErrOrchestratorUnavailable
	case snap := <-ch:
		return snap, nil
	}
}
