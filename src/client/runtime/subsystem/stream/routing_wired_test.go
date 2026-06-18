package stream

// Wired routing harness: drives the backend through its real codexclient.Conn
// against an in-process fake app-server. It reuses the existing bindServer
// (launch_flow_test.go) as the handler/turn-counter and wraps its conn in a
// codexclient.Server for the emit side. Unlike the direct-drive contract
// (routing_contract_test.go), this exercises the async read loop, so it runs
// under `go test -race`, and it pins the fake's fidelity: events are produced
// via the same Emit* helpers (same wire shapes) the production server emits.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
)

type wired struct {
	t      *testing.T
	b      *Backend
	rt     *recordingRuntime
	server *bindServer         // handler side: replies to resume, counts turn/start
	emit   *codexclient.Server // emit side over the same server conn
	mu     sync.Mutex
	active state.FrameID
}

func newWired(t *testing.T) *wired {
	t.Helper()
	w := &wired{t: t, rt: &recordingRuntime{}}
	w.b = New(w.rt, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock",
		func() state.FrameID {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.active
		}, time.Second)

	clientT, serverT := streamPipe()
	w.b.conn = codexclient.NewConn(clientT, time.Second)
	serverConn := codexclient.NewConn(serverT, time.Second)
	w.server = &bindServer{conn: serverConn}
	w.emit = codexclient.NewServer(serverConn)

	ctx, cancel := context.WithCancel(context.Background())
	// The transports are not ctx-aware (bufio.Scanner reads block) and their
	// Close is a no-op, so the Run goroutines park until the test binary exits —
	// the same accepted condition as newBoundBackend.
	t.Cleanup(cancel)
	go w.b.conn.Run(ctx, w.b)        //nolint:errcheck
	go serverConn.Run(ctx, w.server) //nolint:errcheck
	return w
}

func (w *wired) setActive(frame state.FrameID) {
	w.mu.Lock()
	w.active = frame
	w.mu.Unlock()
}

// bindCold runs the real cold-start BindFrame (which fires turn/start through
// the conn) and waits for the fake server to observe it, so frame registration
// and turn ordering are settled before the test emits thread.started.
func (w *wired) bindCold(frame state.FrameID, dir string) {
	w.t.Helper()
	before := w.server.turnStartCount()
	_, err := w.b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frame,
		Plan:    state.LaunchPlan{StartDir: dir},
	})
	if err != nil {
		w.t.Fatalf("BindFrame(%s): %v", frame, err)
	}
	w.waitUntil("turn/start from "+string(frame), func() bool {
		return w.server.turnStartCount() > before
	})
}

func (w *wired) emitStarted(threadID, cwd string) {
	w.t.Helper()
	if err := w.emit.EmitThreadStarted(threadID, cwd); err != nil {
		w.t.Fatalf("EmitThreadStarted: %v", err)
	}
	w.waitUntil("thread "+threadID+" bound", func() bool {
		return w.b.frameForThread(threadID) != ""
	})
}

func (w *wired) emitMessage(threadID, delta string) {
	w.t.Helper()
	if err := w.emit.EmitAgentMessageDelta(threadID, delta); err != nil {
		w.t.Fatalf("EmitAgentMessageDelta: %v", err)
	}
	w.waitUntil("marker "+delta+" delivered", func() bool {
		return len(w.rt.framesWithMarker(delta)) > 0
	})
}

func (w *wired) waitUntil(what string, cond func() bool) {
	w.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for %s", what)
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestStreamRoutingWiredHappyPath validates the wired harness end-to-end and,
// with it, the fake server's fidelity: a cold-start frame whose thread streams a
// marker over the real conn must receive exactly that marker. Always on.
func TestStreamRoutingWiredHappyPath(t *testing.T) {
	w := newWired(t)
	w.bindCold("A", "/a")
	w.bindCold("B", "/b")
	w.emitStarted("tA", "/a")
	w.emitStarted("tB", "/b")
	w.emitMessage("tA", "MARK_A")
	w.emitMessage("tB", "MARK_B")
	assertMarkerFrames(t, w.rt, "MARK_A", "A")
	assertMarkerFrames(t, w.rt, "MARK_B", "B")
}

// TestStreamRoutingWiredCrosstalk is the async sibling of
// crosstalk_ambiguous_cwd: same root cause, but exercised through the read loop
// so -race observes the concurrent path. Gated until the demux fix lands.
func TestStreamRoutingWiredCrosstalk(t *testing.T) {
	requireRoutingPins(t)
	w := newWired(t)
	w.bindCold("A", "/work")
	w.bindCold("B", "/work")
	w.setActive("B")
	w.emitStarted("tA", "/work") // ground truth: tA is A's thread
	w.emitMessage("tA", "MARK_A")
	assertMarkerFrames(t, w.rt, "MARK_A", "A")
}

// assertMarkerFrames is the shared invariant check used by the direct-drive,
// wired, and e2e harnesses: the marker reached exactly `want` (and no others;
// pass no frames to assert it reached nobody).
func assertMarkerFrames(t *testing.T, rt *recordingRuntime, marker string, want ...state.FrameID) {
	t.Helper()
	got := rt.framesWithMarker(marker)
	if !sameFrameSet(got, want) {
		t.Errorf("routing isolation violated: marker %q delivered to %v, want %v", marker, got, want)
	}
}
