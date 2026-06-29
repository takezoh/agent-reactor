package stream

// Wired routing harness: drives the backend through its real codexclient.Conn
// against an in-process fake app-server. It reuses the existing bindServer
// (launch_flow_test.go) as the handler — which now answers thread/start with a
// fresh unique id — and wraps its conn in a codexclient.Server for the emit
// side. Unlike the direct-drive contract (routing_contract_test.go), this
// exercises the async read loop, so it runs under `go test -race`, and it pins
// that a real cold BindFrame binds a distinct thread id synchronously — making
// cross-talk between same-cwd frames structurally impossible.

import (
	"context"
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
	server *bindServer         // handler: answers thread/start + thread/resume
	emit   *codexclient.Server // emit side over the same server conn
}

func newWired(t *testing.T) *wired {
	t.Helper()
	w := &wired{t: t, rt: &recordingRuntime{}}
	w.b = New(w.rt, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock", time.Second)

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

// bindCold runs the real cold-start BindFrame (thread/start request → a
// synchronously-bound id) and returns that thread id.
func (w *wired) bindCold(frame state.FrameID, dir string) string {
	w.t.Helper()
	_, err := w.b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: frame,
		Plan:    state.LaunchPlan{StartDir: dir},
	})
	if err != nil {
		w.t.Fatalf("BindFrame(%s): %v", frame, err)
	}
	w.b.mu.Lock()
	binding := w.b.frames[frame]
	w.b.mu.Unlock()
	if binding == nil || binding.threadID == "" {
		w.t.Fatalf("BindFrame(%s) did not bind a thread id", frame)
	}
	return binding.threadID
}

func (w *wired) emitMessage(threadID, delta string) {
	w.t.Helper()
	if err := w.emit.EmitAgentMessageDelta(threadID, delta); err != nil {
		w.t.Fatalf("EmitAgentMessageDelta: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(w.rt.framesWithMarker(delta)) == 0 {
		if time.Now().After(deadline) {
			w.t.Fatalf("timed out waiting for marker %s", delta)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestStreamRoutingWiredIsolation: two frames sharing a cwd each get a distinct
// thread id at creation, so the real async event stream routes each marker to
// its own frame — cross-talk is structurally impossible. Run under -race.
func TestStreamRoutingWiredIsolation(t *testing.T) {
	w := newWired(t)
	tA := w.bindCold("A", "/work")
	tB := w.bindCold("B", "/work")
	if tA == tB {
		t.Fatalf("same-cwd frames must get distinct thread ids, both = %q", tA)
	}
	w.emitMessage(tA, "MARK_A")
	w.emitMessage(tB, "MARK_B")
	assertMarkerFrames(t, w.rt, "MARK_A", "A")
	assertMarkerFrames(t, w.rt, "MARK_B", "B")
}
