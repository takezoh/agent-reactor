package runtime

import (
	"strings"
	"testing"
	"time"
)

// Compile-time proof that PtyBackend satisfies the full TmuxBackend role set.
var _ TmuxBackend = (*PtyBackend)(nil)

// waitUntil polls pred until it returns true or the deadline elapses.
func waitUntil(t *testing.T, pred func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if pred() {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for condition")
		case <-tick.C:
		}
	}
}

// TestPtyBackendSpawnEchoCaptureKill exercises the full data-plane flow:
// spawn a cat pty, send a line, capture the echoed output, then kill and
// observe the exit status.
func TestPtyBackendSpawnEchoCaptureKill(t *testing.T) {
	b := NewPtyBackend()

	winIdx, paneID, err := b.SpawnWindow("w1", "cat", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	if winIdx == "" || paneID == "" {
		t.Fatalf("SpawnWindow returned empty ids: win=%q pane=%q", winIdx, paneID)
	}

	// PaneID echoes the synthetic id back.
	if got, err := b.PaneID(paneID); err != nil || got != paneID {
		t.Fatalf("PaneID(%q) = %q, %v; want %q", paneID, got, err, paneID)
	}

	// Alive before kill.
	if alive, err := b.PaneAlive(paneID); err != nil || !alive {
		t.Fatalf("PaneAlive(%q) = %v, %v; want true", paneID, alive, err)
	}

	// SendKeys appends Enter; cat echoes it back.
	if err := b.SendKeys(paneID, "echo-marker-xyz"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var captured string
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		if err != nil {
			return false
		}
		captured = out
		return strings.Contains(out, "echo-marker-xyz")
	})
	if strings.Contains(captured, "\x1b[") {
		t.Fatalf("CapturePane output still contains SGR escapes: %q", captured)
	}

	if err := b.KillPaneWindow(paneID); err != nil {
		t.Fatalf("KillPaneWindow: %v", err)
	}

	// After kill the pane is no longer alive.
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(paneID)
		return err == nil && !alive
	})
}

// TestPtyBackendExitStatus verifies a process that exits non-zero reports its
// code via PaneExitStatus.
func TestPtyBackendExitStatus(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "bash -c 'exit 7'", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}

	var code int
	waitUntil(t, func() bool {
		dead, c, err := b.PaneExitStatus(paneID)
		if err != nil || !dead {
			return false
		}
		code = c
		return true
	})
	if code != 7 {
		t.Fatalf("PaneExitStatus code = %d, want 7", code)
	}
}

// TestPtyBackendEnvStore verifies the in-process session env store backing
// SetEnv/UnsetEnv/ShowEnvironment.
func TestPtyBackendEnvStore(t *testing.T) {
	b := NewPtyBackend()
	if err := b.SetEnv("BETA", "2"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetEnv("ALPHA", "1"); err != nil {
		t.Fatal(err)
	}
	out, err := b.ShowEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	// Sorted ascending by key.
	if out != "ALPHA=1\nBETA=2\n" {
		t.Fatalf("ShowEnvironment() = %q, want sorted KEY=VALUE lines", out)
	}
	if err := b.UnsetEnv("ALPHA"); err != nil {
		t.Fatal(err)
	}
	out, _ = b.ShowEnvironment()
	if out != "BETA=2\n" {
		t.Fatalf("after UnsetEnv = %q, want %q", out, "BETA=2\n")
	}
}

// TestPtyBackendBufferRoundTrip verifies LoadBuffer holds text and PasteBuffer
// writes it to the pane then drops the buffer.
func TestPtyBackendBufferRoundTrip(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.LoadBuffer("buf1", "pasted-text-abc\n"); err != nil {
		t.Fatal(err)
	}
	if err := b.PasteBuffer("buf1", paneID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "pasted-text-abc")
	})
	// Buffer consumed: a second paste is a no-op error (buffer gone).
	if err := b.PasteBuffer("buf1", paneID); err == nil {
		t.Fatal("PasteBuffer on consumed buffer should error")
	}
}

// TestPtyBackendResize verifies ResizeWindow is delegated to the session and
// PaneSize reflects the new dimensions.
func TestPtyBackendResize(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.ResizeWindow(paneID, 120, 40); err != nil {
		t.Fatal(err)
	}
	w, h, err := b.PaneSize(paneID)
	if err != nil {
		t.Fatal(err)
	}
	if w != 120 || h != 40 {
		t.Fatalf("PaneSize = %dx%d, want 120x40", w, h)
	}
}

// TestPtyBackendSpawnSyntheticIDs verifies synthetic id allocation increments.
func TestPtyBackendSpawnSyntheticIDs(t *testing.T) {
	b := NewPtyBackend()
	win1, pane1, err := b.SpawnWindow("a", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(pane1) }()
	win2, pane2, err := b.SpawnWindow("b", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(pane2) }()

	if pane1 != "%1" || pane2 != "%2" {
		t.Fatalf("pane ids = %q,%q; want %%1,%%2", pane1, pane2)
	}
	if win1 != "1" || win2 != "2" {
		t.Fatalf("window indexes = %q,%q; want 1,2", win1, win2)
	}
}
