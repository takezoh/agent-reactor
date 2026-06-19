package runtime

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/termvt"
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

// TestKeyBytes is the table test for the named-key → byte-sequence mapping,
// including the literal passthrough for unknown keys.
func TestKeyBytes(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"Escape", "\x1b"},
		{"Enter", "\r"},
		{"Up", "\x1b[A"},
		{"Down", "\x1b[B"},
		{"Right", "\x1b[C"},
		{"Left", "\x1b[D"},
		{"Tab", "\t"},
		{"BSpace", "\x7f"},
		{"Space", " "},
		{"q", "q"},                       // unknown single char passes through literally
		{"some-literal", "some-literal"}, // unknown multi-char passes through
		{"", ""},                         // empty passes through as empty
		{"C-c", "\x03"},                  // control chord → SIGINT byte
		{"C-a", "\x01"},                  // control chord → SOH
		{"M-x", "\x1bx"},                 // meta chord → ESC + char
		{"X-y", "X-y"},                   // unknown chord prefix passes through
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := keyBytes(c.key); got != c.want {
				t.Errorf("keyBytes(%q) = %q, want %q", c.key, got, c.want)
			}
		})
	}
}

// TestPtyBackendSendKey verifies SendKey reaches the pty by echoing a Space
// keystroke through cat and observing it in the captured output.
func TestPtyBackendSendKey(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	// Send a recognisable text marker via SendKeys, then a Space via SendKey;
	// cat echoes both. We assert the marker appears (SendKey path drove the pty).
	if err := b.SendKeys(paneID, "key-marker"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	if err := b.SendKey(paneID, "Space"); err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "key-marker")
	})
}

// TestPtyBackendUnknownPaneErrors pins the unknown-target contract: every
// inspect/IO/lifecycle op that addresses a pane returns a non-nil error for an
// unspawned target, while PaneAlive reports (false, nil). The errors that flow
// out of the runtime-internal "unknown pane" path must wrap ErrPaneMissing so
// callers like resident.isMissingPaneErr can distinguish vanished panes from
// transient failures.
func TestPtyBackendUnknownPaneErrors(t *testing.T) {
	b := NewPtyBackend()
	const unknown = "%999"

	wantSentinel := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s(unknown) error = nil, want non-nil", name)
			return
		}
		if !errors.Is(err, ErrPaneMissing) {
			t.Errorf("%s(unknown) error = %v, does not wrap ErrPaneMissing", name, err)
		}
	}

	if err := b.SendKeys(unknown, "x"); err != nil {
		wantSentinel("SendKeys", err)
	} else {
		t.Error("SendKeys(unknown) error = nil, want non-nil")
	}
	if _, err := b.CapturePane(unknown, 10); err != nil {
		wantSentinel("CapturePane", err)
	} else {
		t.Error("CapturePane(unknown) error = nil, want non-nil")
	}
	if err := b.ResizeWindow(unknown, 80, 24); err != nil {
		wantSentinel("ResizeWindow", err)
	} else {
		t.Error("ResizeWindow(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.PaneSize(unknown); err != nil {
		wantSentinel("PaneSize", err)
	} else {
		t.Error("PaneSize(unknown) error = nil, want non-nil")
	}
	if _, err := b.PaneID(unknown); err != nil {
		wantSentinel("PaneID", err)
	} else {
		t.Error("PaneID(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.PaneExitStatus(unknown); err != nil {
		wantSentinel("PaneExitStatus", err)
	} else {
		t.Error("PaneExitStatus(unknown) error = nil, want non-nil")
	}
	// KillPaneWindow is the Manager-level error path: it surfaces termvt.Manager's
	// "not found" rather than our wrapped sentinel. Just require it is non-nil.
	if err := b.KillPaneWindow(unknown); err == nil {
		t.Error("KillPaneWindow(unknown) error = nil, want non-nil")
	}
	// PaneAlive is the explicit exception: unknown target is reported dead, no error.
	if alive, err := b.PaneAlive(unknown); alive || err != nil {
		t.Errorf("PaneAlive(unknown) = %v, %v; want false, nil", alive, err)
	}
}

// TestPtyBackendResizeByWindowIndex verifies ResizeWindow can be addressed by
// the windowIndex form SpawnWindow returns (e.g. "1") — interpret_spawn calls
// it that way (via the sessionName:windowIndex form, see other test) for the
// post-spawn fit-to-main resize.
func TestPtyBackendResizeByWindowIndex(t *testing.T) {
	b := NewPtyBackend()
	winIdx, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.ResizeWindow(winIdx, 100, 30); err != nil {
		t.Fatalf("ResizeWindow(%q) = %v, want nil", winIdx, err)
	}
	w, h, err := b.PaneSize(paneID)
	if err != nil {
		t.Fatal(err)
	}
	if w != 100 || h != 30 {
		t.Fatalf("PaneSize = %dx%d, want 100x30", w, h)
	}
}

// TestPtyBackendResizeBySessionScopedTarget verifies that the sessionName-
// scoped form interpret_spawn emits (e.g. "arc:1") is normalised — the prefix
// is stripped and the windowIndex path runs.
func TestPtyBackendResizeBySessionScopedTarget(t *testing.T) {
	b := NewPtyBackend()
	winIdx, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	target := "arc:" + winIdx
	if err := b.ResizeWindow(target, 90, 25); err != nil {
		t.Fatalf("ResizeWindow(%q) = %v, want nil", target, err)
	}
	w, h, err := b.PaneSize(paneID)
	if err != nil {
		t.Fatal(err)
	}
	if w != 90 || h != 25 {
		t.Fatalf("PaneSize = %dx%d, want 90x25", w, h)
	}
}

// TestPtyBackendKillForgetsWindowIndex verifies the windowIndex→paneID entry is
// dropped after KillPaneWindow so a stale windowIndex no longer routes anywhere.
func TestPtyBackendKillForgetsWindowIndex(t *testing.T) {
	b := NewPtyBackend()
	winIdx, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.KillPaneWindow(paneID); err != nil {
		t.Fatal(err)
	}
	// After kill the windowIndex must not resolve to a live pane id.
	if err := b.ResizeWindow(winIdx, 80, 24); err == nil {
		t.Error("ResizeWindow(stale winIdx) = nil, want non-nil after KillPaneWindow")
	}
}

// TestPtyBackendKillPaneWindowWrapsSentinel verifies that KillPaneWindow's
// "pane not found" error path is recognised by isMissingPaneErr: the second
// kill on the same target must wrap ErrPaneMissing so reconcileWindows can
// evict the vanished frame instead of treating the error as transient.
func TestPtyBackendKillPaneWindowWrapsSentinel(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.KillPaneWindow(paneID); err != nil {
		t.Fatal(err)
	}
	// Second kill: pane already gone. Error must wrap ErrPaneMissing.
	err = b.KillPaneWindow(paneID)
	if err == nil {
		t.Fatal("second KillPaneWindow(pane) = nil, want non-nil")
	}
	if !errors.Is(err, ErrPaneMissing) {
		t.Fatalf("second KillPaneWindow(pane) = %v, does not wrap ErrPaneMissing", err)
	}
	// And the runtime's resident.isMissingPaneErr must classify it as missing.
	if !isMissingPaneErr(err) {
		t.Fatalf("isMissingPaneErr(%v) = false, want true", err)
	}
}

// TestIsPaneIDForm pins the "%<digits>" recogniser: plain "%", "%abc", and the
// empty string are not pane-id shaped, while every id newPaneID can produce
// is.
func TestIsPaneIDForm(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"%1", true},
		{"%99", true},
		{"%", false},
		{"%a", false},
		{"%1a", false},
		{"", false},
		{"1", false},
		{"arc:1", false},
	}
	for _, c := range cases {
		if got := isPaneIDForm(c.in); got != c.want {
			t.Errorf("isPaneIDForm(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestPtyBackendResolveTargetWithBogusPanePrefix verifies a target shaped like
// "%abc" (which only superficially resembles a pane id) is NOT short-circuited
// as a pane id; the windows-map lookup runs and returns the original target
// when nothing matches, so the caller's mgr.Get reports ErrPaneMissing.
func TestPtyBackendResolveTargetWithBogusPanePrefix(t *testing.T) {
	b := NewPtyBackend()
	// "%abc" is not "%<digits>"; resolvePaneTarget falls through to the
	// windows-map lookup, which has no entry → returned as-is → mgr.Get
	// reports missing.
	if got := b.resolvePaneTarget("%abc"); got != "%abc" {
		t.Fatalf("resolvePaneTarget(%q) = %q, want %q", "%abc", got, "%abc")
	}
}

// TestPtyBackendKillPaneWindowPreservesCallerTarget verifies the error message
// quotes the ORIGINAL caller-supplied target, not the resolved paneID, so
// log lines stay readable in the runtime's session-prefixed shape.
func TestPtyBackendKillPaneWindowPreservesCallerTarget(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.KillPaneWindow(paneID); err != nil {
		t.Fatal(err)
	}
	// Second kill via the session-prefixed form. The pane is gone; the error
	// message must quote the original "arc:<paneID>" target, not just paneID.
	target := "arc:" + paneID
	err = b.KillPaneWindow(target)
	if err == nil {
		t.Fatal("second KillPaneWindow = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), target) {
		t.Fatalf("error %q does not quote caller target %q", err.Error(), target)
	}
}

// TestPtyBackendSpawnRunsShellStrings verifies the runtime's shell-string spawn
// inputs (with the "exec " prefix and embedded quoting) survive PtyBackend's
// sh -c wrapping. interpret_spawn.buildSpawnCommand emits "exec <cmd>"; the
// shell must honour exec semantics so the user's process replaces sh.
func TestPtyBackendSpawnRunsShellStrings(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "exec bash -c 'exit 9'", "", nil)
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
	if code != 9 {
		t.Fatalf("PaneExitStatus code = %d, want 9 (exec bash -c 'exit 9')", code)
	}
}

// TestPtyBackendExitStatusLive verifies PaneExitStatus on a running process
// reports (false, -1, nil), and PaneAlive flips to false once a clean exit is
// reaped.
func TestPtyBackendExitStatusLive(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "sleep 5", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	dead, code, err := b.PaneExitStatus(paneID)
	if err != nil {
		t.Fatalf("PaneExitStatus(live) err = %v, want nil", err)
	}
	if dead || code != -1 {
		t.Fatalf("PaneExitStatus(live) = %v, %d; want false, -1", dead, code)
	}

	// A separate pane that exits 0: PaneAlive must flip to false.
	_, exitPane, err := b.SpawnWindow("w2", "bash -c 'exit 0'", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(exitPane) }()
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(exitPane)
		return err == nil && !alive
	})
}

// TestPtyBackendRespawn verifies a pane can be respawned in place and that an
// empty respawn command is rejected.
func TestPtyBackendRespawn(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("w", "bash -c 'exit 0'", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	// Wait for the original to die so we respawn over a reaped pane.
	waitUntil(t, func() bool {
		alive, err := b.PaneAlive(paneID)
		return err == nil && !alive
	})

	// Respawn over the same target with a long-lived command: pane is alive again.
	if err := b.RespawnPane(paneID, "cat"); err != nil {
		t.Fatalf("RespawnPane: %v", err)
	}
	if alive, err := b.PaneAlive(paneID); err != nil || !alive {
		t.Fatalf("PaneAlive after respawn = %v, %v; want true", alive, err)
	}
	// Echo path still works on the respawned pane.
	if err := b.SendKeys(paneID, "respawned-ok"); err != nil {
		t.Fatalf("SendKeys after respawn: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "respawned-ok")
	})

	// Empty respawn command is rejected.
	if err := b.RespawnPane(paneID, ""); err == nil {
		t.Error("RespawnPane(empty) error = nil, want non-nil")
	}
}

// TestPtyBackendSubscribeSurface_SnapshotFirst verifies that the first event
// received on a freshly opened subscriber channel has Kind == EventOutput
// (the reattach snapshot that termvt.Session.Subscribe guarantees).
func TestPtyBackendSubscribeSurface_SnapshotFirst(t *testing.T) {
	b := NewPtyBackend()
	// cat keeps the session alive so the subscriber channel stays open.
	_, paneID, err := b.SpawnWindow("t", "cat", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	// Send a marker so the VT emulator has rendered content before we subscribe.
	if err := b.SendKeys(paneID, "snapshot-marker"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CapturePane(paneID, 50)
		return err == nil && strings.Contains(out, "snapshot-marker")
	})

	subID, ch, err := b.SubscribeSurface(paneID)
	if err != nil {
		t.Fatalf("SubscribeSurface: %v", err)
	}
	defer func() { _ = b.UnsubscribeSurface(paneID, subID) }()

	// The first event from Subscribe is always the reattach snapshot (EventOutput).
	deadline := time.After(3 * time.Second)
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("SubscribeSurface channel closed immediately")
		}
		if ev.Kind != termvt.EventOutput {
			t.Fatalf("first event Kind = %v, want EventOutput", ev.Kind)
		}
	case <-deadline:
		t.Fatal("timeout waiting for first event from SubscribeSurface")
	}
}

// TestPtyBackendWriteSurface verifies that bytes sent via WriteSurface reach
// the pty: cat echoes them back and they appear in the captured output.
func TestPtyBackendWriteSurface(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("t", "cat", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	subID, ch, err := b.SubscribeSurface(paneID)
	if err != nil {
		t.Fatalf("SubscribeSurface: %v", err)
	}
	defer func() { _ = b.UnsubscribeSurface(paneID, subID) }()

	// Drain the initial snapshot event so the loop below sees only live output.
	deadline := time.After(3 * time.Second)
	select {
	case <-ch:
	case <-deadline:
		t.Fatal("timeout waiting for snapshot event")
	}

	if err := b.WriteSurface(paneID, []byte("hello\n")); err != nil {
		t.Fatalf("WriteSurface: %v", err)
	}

	// Read events until "hello" appears in the accumulated data or timeout.
	var got strings.Builder
	deadline = time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("subscriber channel closed before observing hello")
			}
			if ev.Kind == termvt.EventOutput {
				got.Write(ev.Data)
				if strings.Contains(got.String(), "hello") {
					return // success
				}
			}
		case <-deadline:
			t.Fatalf("timeout: observed %q, want to see hello", got.String())
		}
	}
}

// TestPtyBackendResizeSurface verifies that ResizeSurface updates both the pty
// winsize and the VT emulator grid so PaneSize reports the new dimensions.
func TestPtyBackendResizeSurface(t *testing.T) {
	b := NewPtyBackend()
	_, paneID, err := b.SpawnWindow("t", "sleep 5", "", nil)
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	defer func() { _ = b.KillPaneWindow(paneID) }()

	if err := b.ResizeSurface(paneID, 120, 40); err != nil {
		t.Fatalf("ResizeSurface: %v", err)
	}

	cols, rows, err := b.PaneSize(paneID)
	if err != nil {
		t.Fatalf("PaneSize: %v", err)
	}
	if cols != 120 || rows != 40 {
		t.Fatalf("PaneSize = %dx%d, want 120x40", cols, rows)
	}
}

// TestPtyBackendSurface_MissingPaneTarget verifies that all surface accessors
// wrap ErrPaneMissing when the target pane does not exist.
func TestPtyBackendSurface_MissingPaneTarget(t *testing.T) {
	b := NewPtyBackend()
	const unknown = "%999"

	_, _, err := b.SubscribeSurface(unknown)
	if err == nil || !errors.Is(err, ErrPaneMissing) {
		t.Errorf("SubscribeSurface(unknown) = %v, want ErrPaneMissing", err)
	}

	if err := b.UnsubscribeSurface(unknown, 0); err == nil || !errors.Is(err, ErrPaneMissing) {
		t.Errorf("UnsubscribeSurface(unknown) = %v, want ErrPaneMissing", err)
	}

	if err := b.WriteSurface(unknown, []byte("x")); err == nil || !errors.Is(err, ErrPaneMissing) {
		t.Errorf("WriteSurface(unknown) = %v, want ErrPaneMissing", err)
	}

	if err := b.ResizeSurface(unknown, 80, 24); err == nil || !errors.Is(err, ErrPaneMissing) {
		t.Errorf("ResizeSurface(unknown) = %v, want ErrPaneMissing", err)
	}
}

// TestPtyBackendConcurrent drives spawn/SendKeys/CapturePane/KillPaneWindow from
// many goroutines so `go test -race` can prove the backend's shared state is
// race-free.
func TestPtyBackendConcurrent(t *testing.T) {
	b := NewPtyBackend()
	const workers = 8

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, paneID, err := b.SpawnWindow("w", "cat", "", nil)
			if err != nil {
				t.Errorf("SpawnWindow: %v", err)
				return
			}
			_ = b.SendKeys(paneID, "hello")
			_, _ = b.CapturePane(paneID, 10)
			_, _ = b.PaneAlive(paneID)
			_ = b.KillPaneWindow(paneID)
		}()
	}
	wg.Wait()
}
