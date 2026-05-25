package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatchWorkflowSignalsOnWrite verifies that writing to WORKFLOW.md sends a signal.
func TestWatchWorkflowSignalsOnWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(path, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer, err := watchWorkflow(ctx, path, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	if err := os.WriteFile(path, []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ch:
		// signal received
	case <-time.After(3 * time.Second):
		t.Error("no reload signal within 3s after file write")
	}
}

// TestWatchWorkflowCoalesces verifies that rapid writes produce at most one pending signal.
func TestWatchWorkflowCoalesces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WORKFLOW.md")
	if err := os.WriteFile(path, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer, err := watchWorkflow(ctx, path, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	// Rapid writes — channel capacity is 1, so at most one signal should be buffered.
	for i := range 5 {
		if err := os.WriteFile(path, []byte("update"), 0o644); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond) // let fsnotify deliver events

	if len(ch) > 1 {
		t.Errorf("want at most 1 pending signal, got %d", len(ch))
	}
	if len(ch) == 0 {
		t.Error("want at least 1 signal after writes")
	}
}

// TestWatchWorkflowIgnoresUnrelatedFiles verifies that changes to unrelated files
// in the same directory do not trigger a reload signal.
func TestWatchWorkflowIgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer, err := watchWorkflow(ctx, path, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	// Write to a different file in the same directory.
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("irrelevant"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	if len(ch) != 0 {
		t.Error("want no signal for unrelated file write")
	}
}
