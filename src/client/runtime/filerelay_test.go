package runtime

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/takezoh/agent-roost/client/proto"
	"github.com/takezoh/agent-roost/client/state"
)

func newTestFileRelay(t *testing.T) *FileRelay {
	t.Helper()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return &FileRelay{
		watcher: w,
		files:   map[string]*relayFile{},
	}
}

func TestUnwatchFile(t *testing.T) {
	fr := newTestFileRelay(t)

	sid := state.FrameID("sess-1")
	fr.files["/tmp/a.log"] = &relayFile{path: "/tmp/a.log", frameID: sid, kind: "transcript"}
	fr.files["/tmp/b.log"] = &relayFile{path: "/tmp/b.log", frameID: sid, kind: "log"}
	fr.files["/tmp/c.log"] = &relayFile{path: "/tmp/c.log", frameID: "other", kind: "log"}

	fr.UnwatchFile(sid)

	if _, ok := fr.files["/tmp/a.log"]; ok {
		t.Error("expected /tmp/a.log to be removed")
	}
	if _, ok := fr.files["/tmp/b.log"]; ok {
		t.Error("expected /tmp/b.log to be removed")
	}
	if _, ok := fr.files["/tmp/c.log"]; !ok {
		t.Error("expected /tmp/c.log to remain")
	}
}

func TestUnwatchFileNoMatch(t *testing.T) {
	fr := newTestFileRelay(t)
	fr.files["/tmp/x.log"] = &relayFile{path: "/tmp/x.log", frameID: "keep", kind: "log"}

	fr.UnwatchFile("nonexistent")

	if len(fr.files) != 1 {
		t.Errorf("expected 1 file remaining, got %d", len(fr.files))
	}
}

func TestUnwatch(t *testing.T) {
	fr := newTestFileRelay(t)
	fr.files["/tmp/d.log"] = &relayFile{path: "/tmp/d.log", frameID: "s1", kind: "log"}
	fr.files["/tmp/e.log"] = &relayFile{path: "/tmp/e.log", frameID: "s2", kind: "log"}

	fr.Unwatch("/tmp/d.log")

	if _, ok := fr.files["/tmp/d.log"]; ok {
		t.Error("expected /tmp/d.log to be removed")
	}
	if _, ok := fr.files["/tmp/e.log"]; !ok {
		t.Error("expected /tmp/e.log to remain")
	}
}

func TestWatchFileCreatesMissingPath(t *testing.T) {
	dir := t.TempDir()
	fr := newTestFileRelay(t)

	// Register a path whose parent dir does not exist yet.
	// The touch inside add() will fail silently, but the entry is still recorded.
	missingDir := filepath.Join(dir, "events", "sess-1.log")
	fr.WatchFile("sess-1", missingDir, "text")
	if _, ok := fr.files[missingDir]; !ok {
		t.Fatalf("expected %s to be registered in fr.files", missingDir)
	}

	// With a pre-existing parent dir, touch creates the file and the fsnotify
	// watch succeeds.
	eventsDir := filepath.Join(dir, "events2")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(eventsDir, "sess-2.log")
	fr.WatchFile("sess-2", path, "text")
	if _, ok := fr.files[path]; !ok {
		t.Fatalf("expected %s to be registered in fr.files", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to be created by touch: %v", path, err)
	}
}

func TestWatchFileIdempotent(t *testing.T) {
	dir := t.TempDir()
	fr := newTestFileRelay(t)

	path := filepath.Join(dir, "roost.log")
	if err := os.WriteFile(path, []byte("line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fr.WatchFile("frame-1", path, "log")
	fr.WatchFile("frame-1", path, "log")

	if len(fr.files) != 1 {
		t.Errorf("fr.files len = %d, want 1 (idempotent)", len(fr.files))
	}
}

// TestBroadcastEnqueuesInternalEvent asserts that broadcast does not call
// broadcastWire synchronously (which would touch loop-owned state from the
// sweep goroutine) but instead posts an internalBroadcastWire onto the loop.
func TestBroadcastEnqueuesInternalEvent(t *testing.T) {
	var got []internalEvent
	fr := &FileRelay{
		files: map[string]*relayFile{},
		send:  func(ev internalEvent) { got = append(got, ev) },
	}

	fr.broadcast(&relayFile{path: "/tmp/app.log", kind: "log"}, "hello\n")

	if len(got) != 1 {
		t.Fatalf("expected 1 internal event, got %d", len(got))
	}
	bw, ok := got[0].(internalBroadcastWire)
	if !ok {
		t.Fatalf("expected internalBroadcastWire, got %T", got[0])
	}
	if bw.eventName != proto.EvtNameLogLine {
		t.Errorf("eventName = %q, want %q", bw.eventName, proto.EvtNameLogLine)
	}
	if len(bw.wire) == 0 {
		t.Error("expected non-empty wire bytes")
	}
}

// TestBroadcastRaceWithConnChurn drives a real event loop while the FileRelay
// sweep goroutine broadcasts log lines and connections are opened/closed on
// the loop. Before the fix (broadcast → fr.rt.broadcastWire on the sweep
// goroutine) this races the loop-owned conns / state maps and `go test -race`
// fatals; after routing broadcasts through internalCh it is clean.
func TestBroadcastRaceWithConnChurn(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	r := New(Config{
		SessionName: "roost-test",
		RoostExe:    "/usr/bin/roost",
		Tmux:        newFakeTmux(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = r.Run(ctx) }()

	fr, err := NewFileRelay(r)
	if err != nil {
		t.Fatal(err)
	}
	r.SetRelay(fr)
	fr.WatchLog(logPath)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer goroutine: append to the watched file so the sweep goroutine
	// keeps broadcasting.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
				if err == nil {
					_, _ = f.WriteString("log line\n")
					_ = f.Close()
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Churn goroutine: open and close connections so the event loop keeps
	// writing the conns map and reassigning state.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c1, c2 := net.Pipe()
				r.enqueueInternal(connOpen{conn: c1})
				_ = c1.Close()
				_ = c2.Close()
				time.Sleep(time.Millisecond)
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	fr.Close()
	cancel()
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop within timeout")
	}
}
