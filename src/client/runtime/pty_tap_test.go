package runtime

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// readUntilClose drains ch until it closes, collecting the chunks. Each chunk
// must arrive within budget; missing chunks fail the test rather than hanging.
func readUntilClose(t *testing.T, ch <-chan []byte, budget time.Duration) [][]byte {
	t.Helper()
	var chunks [][]byte
	deadline := time.After(budget)
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return chunks
			}
			chunks = append(chunks, data)
		case <-deadline:
			t.Fatalf("readUntilClose: timeout after %v, got %d chunks", budget, len(chunks))
		}
	}
}

// waitForChunkContaining drains ch until any chunk includes sub or the deadline
// fires. Returns the concatenated payload seen so far for diagnostic messages.
func waitForChunkContaining(t *testing.T, ch <-chan []byte, sub []byte, budget time.Duration) {
	t.Helper()
	var seen bytes.Buffer
	deadline := time.After(budget)
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before chunk containing %q (saw %q)", sub, seen.Bytes())
			}
			seen.Write(data)
			if bytes.Contains(seen.Bytes(), sub) {
				return
			}
		case <-deadline:
			t.Fatalf("waitForChunkContaining: timeout after %v (saw %q)", budget, seen.Bytes())
		}
	}
}

// waitChannelClosed asserts ch closes within budget. Drains any chunks in the
// meantime so a session that exits after some output still satisfies the test.
func waitChannelClosed(t *testing.T, ch <-chan []byte, budget time.Duration) {
	t.Helper()
	deadline := time.After(budget)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatalf("waitChannelClosed: channel still open after %v", budget)
		}
	}
}

// spawnPaneSeq lets each spawnPane invocation pick a unique frame id so
// multiple tests in the same test binary cannot collide on the Manager.
var spawnPaneSeq int

// spawnPane spawns a frame under backend and returns its pane id. The
// caller is responsible for backend.KillFrame on cleanup.
func spawnPane(t *testing.T, backend *PtyBackend, command string) string {
	t.Helper()
	spawnPaneSeq++
	paneID := "tap-frame-" + string(rune('a'+spawnPaneSeq%26))
	if err := backend.SpawnFrame(paneID, "test", command, "", nil); err != nil {
		t.Fatalf("SpawnFrame: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.KillFrame(paneID)
	})
	return paneID
}

func TestPtyPaneTap_Start_UnknownPaneReturnsMissing(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })
	tap := NewPtyFrameTap(backend)

	_, err := tap.Start(context.Background(), "unknown-frame")
	if err == nil {
		t.Fatal("expected error for unknown pane")
	}
	if !errors.Is(err, ErrFrameMissing) {
		t.Fatalf("err = %v, want errors.Is ErrFrameMissing", err)
	}
}

func TestPtyPaneTap_Start_DeliversSnapshotFirst(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	pane := spawnPane(t, backend, "sleep 1")
	tap := NewPtyFrameTap(backend)

	ch, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = tap.Stop(pane) })

	// Subscribe always seeds the channel with a snapshot EventOutput, so the
	// first chunk must be non-nil bytes (possibly empty if the screen is
	// blank, but the chunk itself arrives).
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot chunk within 1s")
	}
}

func TestPtyPaneTap_ForwardsOutputChunks(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	// printf emits an OSC 9 escape; termvt fans the raw bytes out as an
	// EventOutput in addition to surfacing the structured Control. PtyFrameTap
	// drops the Control side and forwards the raw bytes, which is exactly what
	// tap_manager's vt.Terminal then re-parses to fire EvFrameOsc.
	pane := spawnPane(t, backend, `printf '\033]9;tap-test\a'; sleep 0.5`)
	tap := NewPtyFrameTap(backend)

	ch, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = tap.Stop(pane) })

	waitForChunkContaining(t, ch, []byte("\x1b]9;tap-test"), 2*time.Second)
}

func TestPtyPaneTap_Stop_ClosesChannel(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	pane := spawnPane(t, backend, "sleep 5")
	tap := NewPtyFrameTap(backend)

	ch, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := tap.Stop(pane); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	waitChannelClosed(t, ch, time.Second)

	tap.mu.Lock()
	_, stillThere := tap.subs[pane]
	tap.mu.Unlock()
	if stillThere {
		t.Fatal("subs entry was not removed after Stop")
	}
}

func TestPtyPaneTap_ContextCancelClosesChannel(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	pane := spawnPane(t, backend, "sleep 5")
	tap := NewPtyFrameTap(backend)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := tap.Start(ctx, pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	cancel()
	waitChannelClosed(t, ch, time.Second)
}

func TestPtyPaneTap_SessionExitClosesChannel(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	// printf + a short sleep keeps the session alive long enough for Start to
	// subscribe before the process exits. The Start-side ExitCode guard turns
	// an already-reaped Session into ErrFrameMissing, so a bare `echo bye`
	// would race the reaper and intermittently exercise the missing-pane
	// path instead of the EventExit → channel-close path this test pins.
	pane := spawnPane(t, backend, `printf 'bye'; sleep 0.3`)
	tap := NewPtyFrameTap(backend)

	ch, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = tap.Stop(pane) })

	chunks := readUntilClose(t, ch, 3*time.Second)
	if len(chunks) == 0 {
		t.Fatal("expected at least the snapshot chunk before close")
	}
}

func TestPtyPaneTap_RespawnSamePane(t *testing.T) {
	backend := NewPtyBackend(0)
	t.Cleanup(func() { backend.mgr.CloseAll() })

	pane := spawnPane(t, backend, "sleep 5")
	tap := NewPtyFrameTap(backend)

	firstCh, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// RespawnFrame closes the previous session, which closes firstCh through
	// EventExit. A subsequent Start must subscribe to the new session.
	if err := backend.RespawnFrame(pane, `printf 'after-respawn'; sleep 0.5`); err != nil {
		t.Fatalf("RespawnFrame: %v", err)
	}
	waitChannelClosed(t, firstCh, 2*time.Second)

	secondCh, err := tap.Start(context.Background(), pane)
	if err != nil {
		t.Fatalf("second Start: %v", err)
	}
	t.Cleanup(func() { _ = tap.Stop(pane) })

	waitForChunkContaining(t, secondCh, []byte("after-respawn"), 2*time.Second)
}
