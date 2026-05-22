package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSnapshotCtx_Success(t *testing.T) {
	s := NewState()

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

func TestSnapshotCtx_Timeout(t *testing.T) {
	s := NewState()

	// Hold the lock to block SnapshotCtx.
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := s.SnapshotCtx(ctx)
	if !errors.Is(err, ErrSnapshotTimeout) {
		t.Errorf("want ErrSnapshotTimeout, got %v", err)
	}
}

func TestSnapshotCtx_Cancelled(t *testing.T) {
	s := NewState()

	// Hold the lock to block SnapshotCtx.
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately via a goroutine so the select in SnapshotCtx fires.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := s.SnapshotCtx(ctx)
		if !errors.Is(err, ErrOrchestratorUnavailable) {
			t.Errorf("want ErrOrchestratorUnavailable, got %v", err)
		}
	}()

	cancel()
	<-done
}

func TestScheduler_SnapshotCtx_Unavailable(t *testing.T) {
	s := New("", schedCfg(), minDeps(nil, nil, newFakeClock(time.Now())))
	// available is false by default (Run has not been called).

	ctx := context.Background()
	_, err := s.SnapshotCtx(ctx)
	if !errors.Is(err, ErrOrchestratorUnavailable) {
		t.Errorf("want ErrOrchestratorUnavailable before Run, got %v", err)
	}
}

func TestScheduler_SnapshotCtx_AvailableWhileRunning(t *testing.T) {
	s := New("", schedCfg(), minDeps(nil, nil, newFakeClock(time.Now())))
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
