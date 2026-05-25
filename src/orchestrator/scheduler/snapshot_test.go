package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSnapshotCtx_Success(t *testing.T) {
	s := New("", schedCfg(), "", minDeps(nil, nil, newFakeClock(time.Now())))
	s.available.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snap, err := s.SnapshotCtx(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Running == nil {
		t.Error("Running map should be non-nil")
	}
}

// TestSnapshotCtx_Cancelled verifies a cancelled context maps to ErrOrchestratorUnavailable.
// (The lock-free snapshot cannot time out, so there is no separate timeout error.)
func TestSnapshotCtx_Cancelled(t *testing.T) {
	s := New("", schedCfg(), "", minDeps(nil, nil, newFakeClock(time.Now())))
	s.available.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.SnapshotCtx(ctx); !errors.Is(err, ErrOrchestratorUnavailable) {
		t.Errorf("want ErrOrchestratorUnavailable for cancelled ctx, got %v", err)
	}
}

func TestScheduler_SnapshotCtx_Unavailable(t *testing.T) {
	s := New("", schedCfg(), "", minDeps(nil, nil, newFakeClock(time.Now())))
	// available is false by default (Run has not been called).

	if _, err := s.SnapshotCtx(context.Background()); !errors.Is(err, ErrOrchestratorUnavailable) {
		t.Errorf("want ErrOrchestratorUnavailable before Run, got %v", err)
	}
}

func TestScheduler_SnapshotCtx_AvailableWhileRunning(t *testing.T) {
	s := New("", schedCfg(), "", minDeps(nil, nil, newFakeClock(time.Now())))
	s.available.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	snap, err := s.SnapshotCtx(ctx)
	if err != nil {
		t.Fatalf("want nil error while available, got %v", err)
	}
	if snap.Running == nil {
		t.Error("Running map should be non-nil")
	}
}
