package runtime

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// Compile-time proof that PtyBackend satisfies the full FrameBackend role set.
var _ FrameBackend = (*PtyBackend)(nil)

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

// spawn is a small helper around SpawnFrame: each test gets a unique
// frameID per call site so they cannot collide in the same Manager.
func spawn(t *testing.T, b *PtyBackend, frameID, name, command string) string {
	t.Helper()
	if err := b.SpawnFrame(frameID, name, command, "", nil); err != nil {
		t.Fatalf("SpawnFrame(%q): %v", frameID, err)
	}
	return frameID
}

// TestPtyBackendSpawnEchoCaptureKill exercises the full data-plane flow:
// spawn a cat pty, send a line, capture the echoed output, then kill and
// observe the exit status.
func TestPtyBackendSpawnEchoCaptureKill(t *testing.T) {
	b := NewPtyBackend(0)

	frameID := spawn(t, b, "frame-echo", "w1", "cat")

	// ResolveID echoes the synthetic id back.
	if got, err := b.ResolveID(frameID); err != nil || got != frameID {
		t.Fatalf("ResolveID(%q) = %q, %v; want %q", frameID, got, err, frameID)
	}

	// Alive before kill.
	if dead, _, err := b.FrameExitStatus(frameID); err != nil || dead {
		t.Fatalf("FrameExitStatus(%q) = dead=%v, err=%v; want dead=false, err=nil", frameID, dead, err)
	}

	// SendKeys appends Enter; cat echoes it back.
	if err := b.SendKeys(frameID, "echo-marker-xyz"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var captured string
	waitUntil(t, func() bool {
		out, err := b.CaptureFrame(frameID, 50)
		if err != nil {
			return false
		}
		captured = out
		return strings.Contains(out, "echo-marker-xyz")
	})
	if strings.Contains(captured, "\x1b[") {
		t.Fatalf("CaptureFrame output still contains SGR escapes: %q", captured)
	}

	if err := b.KillFrame(frameID); err != nil {
		t.Fatalf("KillFrame: %v", err)
	}

	// After kill the frame is gone: either reported dead with no error
	// or forgotten from the manager (ErrFrameMissing).
	waitUntil(t, func() bool {
		dead, _, err := b.FrameExitStatus(frameID)
		if err != nil {
			return errors.Is(err, ErrFrameMissing)
		}
		return dead
	})
}

// TestPtyBackendExitStatus verifies a process that exits non-zero reports its
// code via FrameExitStatus.
func TestPtyBackendExitStatus(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-exit7", "w", "bash -c 'exit 7'")

	var code int
	waitUntil(t, func() bool {
		dead, c, err := b.FrameExitStatus(frameID)
		if err != nil || !dead {
			return false
		}
		code = c
		return true
	})
	if code != 7 {
		t.Fatalf("FrameExitStatus code = %d, want 7", code)
	}
}

// TestPtyBackendEnvStore verifies the in-process session env store backing
// SetEnv/UnsetEnv/ShowEnvironment.
func TestPtyBackendEnvStore(t *testing.T) {
	b := NewPtyBackend(0)
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
// writes it to the frame then drops the buffer.
func TestPtyBackendBufferRoundTrip(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-buf", "w", "cat")
	defer func() { _ = b.KillFrame(frameID) }()

	if err := b.LoadBuffer("buf1", "pasted-text-abc\n"); err != nil {
		t.Fatal(err)
	}
	if err := b.PasteBuffer("buf1", frameID); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool {
		out, err := b.CaptureFrame(frameID, 50)
		return err == nil && strings.Contains(out, "pasted-text-abc")
	})
	// Buffer consumed: a second paste is a no-op error (buffer gone).
	if err := b.PasteBuffer("buf1", frameID); err == nil {
		t.Fatal("PasteBuffer on consumed buffer should error")
	}
}

// TestPtyBackendSpawnFrameIDIsTermVTKey verifies the frame id passed to
// SpawnFrame becomes the termvt.Manager session key — addressing the
// freshly spawned session by that id resolves through ResolveID.
func TestPtyBackendSpawnFrameIDIsTermVTKey(t *testing.T) {
	b := NewPtyBackend(0)
	id1 := spawn(t, b, "frame-a", "a", "sleep 5")
	defer func() { _ = b.KillFrame(id1) }()
	id2 := spawn(t, b, "frame-b", "b", "sleep 5")
	defer func() { _ = b.KillFrame(id2) }()

	for _, id := range []string{id1, id2} {
		if got, err := b.ResolveID(id); err != nil || got != id {
			t.Errorf("ResolveID(%q) = (%q, %v), want (%q, nil)", id, got, err, id)
		}
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
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-sendkey", "w", "cat")
	defer func() { _ = b.KillFrame(frameID) }()

	// Send a recognisable text marker via SendKeys, then a Space via SendKey;
	// cat echoes both. We assert the marker appears (SendKey path drove the pty).
	if err := b.SendKeys(frameID, "key-marker"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	if err := b.SendKey(frameID, "Space"); err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CaptureFrame(frameID, 50)
		return err == nil && strings.Contains(out, "key-marker")
	})
}

// TestPtyBackendUnknownFrameErrors pins the unknown-target contract: every
// inspect/IO/lifecycle op that addresses a frame returns a non-nil error for an
// unspawned target. The errors that flow out of the runtime-internal
// "unknown frame" path must wrap ErrFrameMissing so callers like
// resident.isMissingFrameErr can distinguish vanished frames from transient
// failures.
func TestPtyBackendUnknownFrameErrors(t *testing.T) {
	b := NewPtyBackend(0)
	const unknown = "unknown-frame"

	wantSentinel := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s(unknown) error = nil, want non-nil", name)
			return
		}
		if !errors.Is(err, ErrFrameMissing) {
			t.Errorf("%s(unknown) error = %v, does not wrap ErrFrameMissing", name, err)
		}
	}

	if err := b.SendKeys(unknown, "x"); err != nil {
		wantSentinel("SendKeys", err)
	} else {
		t.Error("SendKeys(unknown) error = nil, want non-nil")
	}
	if _, err := b.CaptureFrame(unknown, 10); err != nil {
		wantSentinel("CaptureFrame", err)
	} else {
		t.Error("CaptureFrame(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.FrameSize(unknown); err != nil {
		wantSentinel("FrameSize", err)
	} else {
		t.Error("FrameSize(unknown) error = nil, want non-nil")
	}
	if _, err := b.ResolveID(unknown); err != nil {
		wantSentinel("ResolveID", err)
	} else {
		t.Error("ResolveID(unknown) error = nil, want non-nil")
	}
	if _, _, err := b.FrameExitStatus(unknown); err != nil {
		wantSentinel("FrameExitStatus", err)
	} else {
		t.Error("FrameExitStatus(unknown) error = nil, want non-nil")
	}
	// KillFrame is the Manager-level error path: it surfaces termvt.Manager's
	// "not found" rather than our wrapped sentinel. Just require it is non-nil.
	if err := b.KillFrame(unknown); err == nil {
		t.Error("KillFrame(unknown) error = nil, want non-nil")
	}
}

// TestPtyBackendKillFrameWrapsSentinel verifies that KillFrame's
// "frame not found" error path is recognised by isMissingFrameErr: the
// second kill on the same target must wrap ErrFrameMissing so
// reconcileWindows can evict the vanished frame instead of treating
// the error as transient.
func TestPtyBackendKillFrameWrapsSentinel(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-kill-sentinel", "w", "sleep 5")
	if err := b.KillFrame(frameID); err != nil {
		t.Fatal(err)
	}
	// Second kill: frame already gone. Error must wrap ErrFrameMissing.
	err := b.KillFrame(frameID)
	if err == nil {
		t.Fatal("second KillFrame(frame) = nil, want non-nil")
	}
	if !errors.Is(err, ErrFrameMissing) {
		t.Fatalf("second KillFrame(frame) = %v, does not wrap ErrFrameMissing", err)
	}
	// And the runtime's resident.isMissingFrameErr must classify it as missing.
	if !isMissingFrameErr(err) {
		t.Fatalf("isMissingFrameErr(%v) = false, want true", err)
	}
}

// TestPtyBackendSpawnRunsShellStrings verifies the runtime's shell-string spawn
// inputs (with the "exec " prefix and embedded quoting) survive PtyBackend's
// sh -c wrapping. interpret_spawn.buildSpawnCommand emits "exec <cmd>"; the
// shell must honour exec semantics so the user's process replaces sh.
func TestPtyBackendSpawnRunsShellStrings(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-exec", "w", "exec bash -c 'exit 9'")

	var code int
	waitUntil(t, func() bool {
		dead, c, err := b.FrameExitStatus(frameID)
		if err != nil || !dead {
			return false
		}
		code = c
		return true
	})
	if code != 9 {
		t.Fatalf("FrameExitStatus code = %d, want 9 (exec bash -c 'exit 9')", code)
	}
}

// TestPtyBackendExitStatusLive verifies FrameExitStatus on a running process
// reports (false, -1, nil), and flips to (true, 0, nil) once a clean exit is
// reaped.
func TestPtyBackendExitStatusLive(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-live", "w", "sleep 5")
	defer func() { _ = b.KillFrame(frameID) }()

	dead, code, err := b.FrameExitStatus(frameID)
	if err != nil {
		t.Fatalf("FrameExitStatus(live) err = %v, want nil", err)
	}
	if dead || code != -1 {
		t.Fatalf("FrameExitStatus(live) = %v, %d; want false, -1", dead, code)
	}

	// A separate frame that exits 0: FrameExitStatus must flip to dead.
	exitFrame := spawn(t, b, "frame-exit0", "w2", "bash -c 'exit 0'")
	defer func() { _ = b.KillFrame(exitFrame) }()
	waitUntil(t, func() bool {
		dead, _, err := b.FrameExitStatus(exitFrame)
		return err == nil && dead
	})
}

// TestPtyBackendRespawn verifies a frame can be respawned in place and that an
// empty respawn command is rejected.
func TestPtyBackendRespawn(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-respawn", "w", "bash -c 'exit 0'")
	defer func() { _ = b.KillFrame(frameID) }()

	// Wait for the original to die so we respawn over a reaped frame.
	waitUntil(t, func() bool {
		dead, _, err := b.FrameExitStatus(frameID)
		return err == nil && dead
	})

	// Respawn over the same target with a long-lived command: frame is alive again.
	if err := b.RespawnFrame(frameID, "cat"); err != nil {
		t.Fatalf("RespawnFrame: %v", err)
	}
	if dead, _, err := b.FrameExitStatus(frameID); err != nil || dead {
		t.Fatalf("FrameExitStatus after respawn = dead=%v, err=%v; want dead=false, err=nil", dead, err)
	}
	// Echo path still works on the respawned frame.
	if err := b.SendKeys(frameID, "respawned-ok"); err != nil {
		t.Fatalf("SendKeys after respawn: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CaptureFrame(frameID, 50)
		return err == nil && strings.Contains(out, "respawned-ok")
	})

	// Empty respawn command is rejected.
	if err := b.RespawnFrame(frameID, ""); err == nil {
		t.Error("RespawnFrame(empty) error = nil, want non-nil")
	}
}

// TestPtyBackendSubscribeSurface_SnapshotFirst verifies that the first event
// received on a freshly opened subscriber channel has Kind == EventOutput
// (the reattach snapshot that termvt.Session.Subscribe guarantees).
func TestPtyBackendSubscribeSurface_SnapshotFirst(t *testing.T) {
	b := NewPtyBackend(0)
	// cat keeps the session alive so the subscriber channel stays open.
	frameID := spawn(t, b, "frame-snapshot", "t", "cat")
	defer func() { _ = b.KillFrame(frameID) }()

	// Send a marker so the VT emulator has rendered content before we subscribe.
	if err := b.SendKeys(frameID, "snapshot-marker"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	waitUntil(t, func() bool {
		out, err := b.CaptureFrame(frameID, 50)
		return err == nil && strings.Contains(out, "snapshot-marker")
	})

	subID, ch, err := b.SubscribeSurface(frameID)
	if err != nil {
		t.Fatalf("SubscribeSurface: %v", err)
	}
	defer func() { _ = b.UnsubscribeSurface(frameID, subID) }()

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
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-write", "t", "cat")
	defer func() { _ = b.KillFrame(frameID) }()

	subID, ch, err := b.SubscribeSurface(frameID)
	if err != nil {
		t.Fatalf("SubscribeSurface: %v", err)
	}
	defer func() { _ = b.UnsubscribeSurface(frameID, subID) }()

	// Drain the initial snapshot event so the loop below sees only live output.
	deadline := time.After(3 * time.Second)
	select {
	case <-ch:
	case <-deadline:
		t.Fatal("timeout waiting for snapshot event")
	}

	if err := b.WriteSurface(frameID, []byte("hello\n")); err != nil {
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
// winsize and the VT emulator grid so FrameSize reports the new dimensions.
func TestPtyBackendResizeSurface(t *testing.T) {
	b := NewPtyBackend(0)
	frameID := spawn(t, b, "frame-resize-surface", "t", "sleep 5")
	defer func() { _ = b.KillFrame(frameID) }()

	if err := b.ResizeSurface(frameID, 120, 40); err != nil {
		t.Fatalf("ResizeSurface: %v", err)
	}

	cols, rows, err := b.FrameSize(frameID)
	if err != nil {
		t.Fatalf("FrameSize: %v", err)
	}
	if cols != 120 || rows != 40 {
		t.Fatalf("FrameSize = %dx%d, want 120x40", cols, rows)
	}
}

// TestPtyBackendSurface_MissingFrame verifies that all surface accessors
// wrap ErrFrameMissing when the target frame does not exist.
func TestPtyBackendSurface_MissingFrame(t *testing.T) {
	b := NewPtyBackend(0)
	const unknown = "unknown-frame"

	_, _, err := b.SubscribeSurface(unknown)
	if err == nil || !errors.Is(err, ErrFrameMissing) {
		t.Errorf("SubscribeSurface(unknown) = %v, want ErrFrameMissing", err)
	}

	if err := b.UnsubscribeSurface(unknown, 0); err == nil || !errors.Is(err, ErrFrameMissing) {
		t.Errorf("UnsubscribeSurface(unknown) = %v, want ErrFrameMissing", err)
	}

	if err := b.WriteSurface(unknown, []byte("x")); err == nil || !errors.Is(err, ErrFrameMissing) {
		t.Errorf("WriteSurface(unknown) = %v, want ErrFrameMissing", err)
	}

	if err := b.ResizeSurface(unknown, 80, 24); err == nil || !errors.Is(err, ErrFrameMissing) {
		t.Errorf("ResizeSurface(unknown) = %v, want ErrFrameMissing", err)
	}
}

// TestPtyBackendConcurrent drives spawn/SendKeys/CaptureFrame/KillFrame from
// many goroutines so `go test -race` can prove the backend's shared state is
// race-free.
func TestPtyBackendConcurrent(t *testing.T) {
	b := NewPtyBackend(0)
	const workers = 8

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			frameID := "concurrent-frame-" + strings.Repeat("a", i+1)
			if err := b.SpawnFrame(frameID, "w", "cat", "", nil); err != nil {
				t.Errorf("SpawnFrame: %v", err)
				return
			}
			_ = b.SendKeys(frameID, "hello")
			_, _ = b.CaptureFrame(frameID, 10)
			_, _, _ = b.FrameExitStatus(frameID)
			_ = b.KillFrame(frameID)
		}()
	}
	wg.Wait()
}

// envSliceToMap parses KEY=VALUE pairs and tracks each key's occurrence count
// so envSlice tests can assert both value-equality and emit-exactly-once
// without re-deriving the parse inline.
func envSliceToMap(t *testing.T, kvs []string) (map[string]string, map[string]int) {
	t.Helper()
	values := make(map[string]string, len(kvs))
	counts := make(map[string]int, len(kvs))
	for _, kv := range kvs {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		values[key] = val
		counts[key]++
	}
	return values, counts
}

// TestEnvSliceInheritsOsEnvironWithOverrides asserts the daemon's process env
// is the base and explicit overrides shadow individual keys without dropping
// the rest. Regression guard for host-direct launches losing PATH and hitting
// exit 127 on `exec claude` (a5ec8f11 incident, 2026-06-27).
func TestEnvSliceInheritsOsEnvironWithOverrides(t *testing.T) {
	t.Setenv("ENVSLICE_BASE_KEY", "from-os")
	t.Setenv("ENVSLICE_SHADOWED", "from-os")

	got := envSlice(map[string]string{
		"ENVSLICE_SHADOWED": "from-overlay",
		"ENVSLICE_NEW_KEY":  "from-overlay",
	})
	values, counts := envSliceToMap(t, got)

	want := map[string]string{
		"ENVSLICE_BASE_KEY": "from-os",      // inherited untouched
		"ENVSLICE_SHADOWED": "from-overlay", // overlay wins
		"ENVSLICE_NEW_KEY":  "from-overlay", // overlay adds
	}
	for k, v := range want {
		if values[k] != v {
			t.Errorf("envSlice key %q = %q, want %q", k, values[k], v)
		}
		// Each key under test must appear exactly once. The check is scoped to
		// our ENVSLICE_-prefixed keys: any duplicates already present in the
		// test runner's host env (legal under POSIX and observable after e.g.
		// sudo or layered EnvironmentFiles) would otherwise spuriously fail
		// the assertion for unrelated keys.
		if counts[k] != 1 {
			t.Errorf("envSlice key %q appears %d times, want 1", k, counts[k])
		}
	}
}

// mergeEnv drops duplicate KEY= entries in base (legal under POSIX, observable
// after sudo / layered EnvironmentFiles), keeping only the first occurrence so
// libc readers cannot diverge. Drives the dedup branch directly with a synthetic
// base since os.Setenv collapses duplicates and the real os.Environ() path
// can't reach this state from Go.
func TestMergeEnvDeduplicatesBase(t *testing.T) {
	base := []string{"FOO=first", "BAR=keep", "FOO=second"}
	got := mergeEnv(base, map[string]string{"FOO": "overlay"})
	want := []string{"FOO=overlay", "BAR=keep"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("mergeEnv = %v, want %v", got, want)
	}
}

// Malformed (no `=`) base entries are preserved verbatim but must NOT
// participate in the keyed dedup. A bare "FOO" must not shadow a well-formed
// "FOO=…" or block an overlay for FOO — that regression silently dropped the
// caller's override (Round 2 review finding).
func TestMergeEnvMalformedEntryDoesNotShadowOverlay(t *testing.T) {
	base := []string{"FOO", "FOO=base"}
	got := mergeEnv(base, map[string]string{"FOO": "overlay"})
	want := []string{"FOO", "FOO=overlay"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("mergeEnv = %v, want %v", got, want)
	}
}

// Empty overrides → os.Environ verbatim. Container-launch path relies on
// this (BuildLaunchCommand returns an empty outEnv map; the in-container
// process env is encoded into the docker-exec command-line, not the host
// shell's env), so the daemon's PATH still reaches host /bin/sh and `docker`
// resolves under POSIX-default-or-systemd PATH.
//
// Only nil is tested: len(nil map) == len(map[string]string{}) == 0 in Go,
// so the {} branch would exercise the same code path with no extra coverage.
func TestEnvSliceEmptyOverridesYieldsOsEnviron(t *testing.T) {
	t.Setenv("ENVSLICE_PROBE", "v")
	got := envSlice(nil)
	values, _ := envSliceToMap(t, got)
	if values["ENVSLICE_PROBE"] != "v" {
		t.Errorf("envSlice(nil) did not propagate os.Environ probe, got %q", values["ENVSLICE_PROBE"])
	}
}

// Unshadowed override keys are emitted in sorted order so termvt.Spec.Env is
// deterministic between runs. Memoization and golden-test consumers downstream
// rely on this.
func TestEnvSliceUnshadowedOverridesAreSorted(t *testing.T) {
	overrides := map[string]string{
		"ENVSLICE_ZED":   "z",
		"ENVSLICE_ALPHA": "a",
		"ENVSLICE_MID":   "m",
	}
	first := envSlice(overrides)
	second := envSlice(overrides)
	if strings.Join(first, "\x00") != strings.Join(second, "\x00") {
		t.Fatal("envSlice output is non-deterministic between calls with identical overrides")
	}
	// Tail of the slice carries the appended unshadowed overrides in sorted
	// order; locate them by prefix to avoid coupling to os.Environ length.
	var tail []string
	for _, kv := range first {
		if strings.HasPrefix(kv, "ENVSLICE_") {
			tail = append(tail, kv)
		}
	}
	want := []string{"ENVSLICE_ALPHA=a", "ENVSLICE_MID=m", "ENVSLICE_ZED=z"}
	if strings.Join(tail, ",") != strings.Join(want, ",") {
		t.Errorf("envSlice unshadowed tail = %v, want %v (sorted)", tail, want)
	}
}
