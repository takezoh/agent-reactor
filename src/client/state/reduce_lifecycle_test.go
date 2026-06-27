package state

import (
	"testing"
)

// TestReduceShutdownEffects verifies the shutdown reducer persists state,
// acks the caller synchronously, and asks the runtime to release sandbox
// resources. Daemon termination itself is driven by the signal handler in
// the process entrypoint — the reducer no longer asks a backend to tear
// down a TUI session.
func TestReduceShutdownEffects(t *testing.T) {
	s := New()
	_, effects := reduceShutdown(s, 1, "req-1", struct{}{})

	var hasSync, hasPersist, hasRelease bool
	for _, eff := range effects {
		switch eff.(type) {
		case EffSendResponseSync:
			hasSync = true
		case EffPersistSnapshot:
			hasPersist = true
		case EffReleaseFrameSandboxes:
			hasRelease = true
		}
	}
	if !hasPersist {
		t.Error("expected EffPersistSnapshot in effects")
	}
	if !hasSync {
		t.Error("expected EffSendResponseSync in effects")
	}
	if !hasRelease {
		t.Error("expected EffReleaseFrameSandboxes in effects")
	}
}

// TestReduceShutdown_releaseBeforeAck verifies persist runs first, then the
// caller is acked, then sandboxes are released. Containers must NOT be
// released before the ack so the caller observes the daemon's intent before
// any sandbox tear-down side-effects fire.
func TestReduceShutdown_orderPersistAckRelease(t *testing.T) {
	s := New()
	_, effects := reduceShutdown(s, 1, "req-1", struct{}{})

	persistIdx, ackIdx, releaseIdx := -1, -1, -1
	for i, eff := range effects {
		switch eff.(type) {
		case EffPersistSnapshot:
			persistIdx = i
		case EffSendResponseSync:
			ackIdx = i
		case EffReleaseFrameSandboxes:
			releaseIdx = i
		}
	}
	if persistIdx < 0 || ackIdx < 0 || releaseIdx < 0 {
		t.Fatalf("missing effect: persist=%d ack=%d release=%d", persistIdx, ackIdx, releaseIdx)
	}
	if persistIdx >= ackIdx || ackIdx >= releaseIdx {
		t.Errorf("expected persist(%d) < ack(%d) < release(%d)", persistIdx, ackIdx, releaseIdx)
	}
}
