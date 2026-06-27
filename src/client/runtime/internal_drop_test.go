package runtime

import (
	"sync"
	"testing"
)

// TestInternalDropCounter_incPerType asserts per-event-type drops increment
// the right bucket and are reflected in snapshot.
func TestInternalDropCounter_incPerType(t *testing.T) {
	c := newInternalDropCounter()
	c.inc(internalEventBroadcastWire)
	c.inc(internalEventBroadcastWire)
	c.inc(internalEventBroadcastSurface)

	snap := c.snapshot()
	if got := snap[internalEventBroadcastWire]; got != 2 {
		t.Errorf("broadcast-wire = %d, want 2", got)
	}
	if got := snap[internalEventBroadcastSurface]; got != 1 {
		t.Errorf("broadcast-surface = %d, want 1", got)
	}
	if _, ok := snap[internalEventConnOpen]; ok {
		t.Errorf("conn-open should be absent (zero) in snapshot, got %d", snap[internalEventConnOpen])
	}
}

// TestInternalDropCounter_unknownBucket asserts an unrecognised label falls
// back to the "unknown" bucket so we never silently lose a drop signal.
func TestInternalDropCounter_unknownBucket(t *testing.T) {
	c := newInternalDropCounter()
	c.inc("not-a-real-event")
	c.inc("also-fake")

	snap := c.snapshot()
	if got := snap[internalEventUnknown]; got != 2 {
		t.Errorf("unknown bucket = %d, want 2", got)
	}
}

// TestInternalDropCounter_concurrent asserts inc is safe under concurrent
// callers (the hot path runs on every enqueueInternal goroutine).
func TestInternalDropCounter_concurrent(t *testing.T) {
	c := newInternalDropCounter()
	const goroutines = 16
	const perG = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				c.inc(internalEventBroadcastWire)
			}
		}()
	}
	wg.Wait()

	snap := c.snapshot()
	if got := snap[internalEventBroadcastWire]; got != goroutines*perG {
		t.Errorf("broadcast-wire = %d, want %d", got, goroutines*perG)
	}
}

// TestEnqueueInternal_dropsCountAndReturnsFalse asserts enqueueInternal returns
// false on a saturated channel and bumps the per-type counter.
func TestEnqueueInternal_dropsCountAndReturnsFalse(t *testing.T) {
	r := New(Config{Backend: newFakeBackend()})
	// Saturate internalCh without ever draining it (no Run loop): fill to cap.
	for i := 0; i < cap(r.internalCh); i++ {
		if !r.enqueueInternal(internalStartRestoredTaps{}) {
			t.Fatalf("unexpected drop while filling (iteration %d)", i)
		}
	}
	// The next enqueue must drop.
	if r.enqueueInternal(internalStartRestoredTaps{}) {
		t.Error("expected drop on saturated channel, got delivered=true")
	}
	stats := r.InternalDropStats()
	if got := stats[internalEventStartRestoredTaps]; got != 1 {
		t.Errorf("start-restored-taps drops = %d, want 1", got)
	}
}
