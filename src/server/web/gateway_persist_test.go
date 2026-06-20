package web

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
)

// TestLifecycleForwardsSessionFileLine verifies that EvtSessionFileLine with
// Kind "transcript" is forwarded as a {"k":"tt",...} frame on the lifecycle WS.
func TestLifecycleForwardsSessionFileLine(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Seed hello first so gateway progresses past helloSent=false.
	fake.events <- proto.EvtSessionsChanged{}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	// Now send the transcript line.
	fake.events <- proto.EvtSessionFileLine{SessionID: "s1", Kind: "transcript", Line: "hello"}

	m := readJSONFrame(t, c)

	if m["k"] != "tt" {
		t.Errorf("frame k = %q, want \"tt\"", m["k"])
	}
	if m["sessionId"] != "s1" {
		t.Errorf("sessionId = %q, want \"s1\"", m["sessionId"])
	}
	if m["line"] != "hello" {
		t.Errorf("line = %q, want \"hello\"", m["line"])
	}
}

// TestLifecycleForwardsEventLogTail verifies that EvtSessionFileLine with
// Kind "event-log" is forwarded as a {"k":"et",...} frame on the lifecycle WS.
func TestLifecycleForwardsEventLogTail(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Seed hello.
	fake.events <- proto.EvtSessionsChanged{}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	// Send an event-log line.
	fake.events <- proto.EvtSessionFileLine{SessionID: "s1", Kind: "event-log", Line: "log entry"}

	m := readJSONFrame(t, c)

	if m["k"] != "et" {
		t.Errorf("frame k = %q, want \"et\"", m["k"])
	}
	if m["sessionId"] != "s1" {
		t.Errorf("sessionId = %q, want \"s1\"", m["sessionId"])
	}
	if m["line"] != "log entry" {
		t.Errorf("line = %q, want \"log entry\"", m["line"])
	}
}

// TestLifecycleForwardsAgentNotification verifies that EvtAgentNotification is
// forwarded as a {"k":"n",...} frame on the lifecycle WS.
func TestLifecycleForwardsAgentNotification(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Seed hello.
	fake.events <- proto.EvtSessionsChanged{}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	// Send agent notification.
	fake.events <- proto.EvtAgentNotification{SessionID: "s1", Cmd: 9, Title: "t"}

	m := readJSONFrame(t, c)

	if m["k"] != "n" {
		t.Errorf("frame k = %q, want \"n\"", m["k"])
	}
	if m["sessionId"] != "s1" {
		t.Errorf("sessionId = %q, want \"s1\"", m["sessionId"])
	}
	if cmd, _ := m["cmd"].(float64); int(cmd) != 9 {
		t.Errorf("cmd = %v, want 9", m["cmd"])
	}
	if m["title"] != "t" {
		t.Errorf("title = %q, want \"t\"", m["title"])
	}
}

// TestSurfaceFiltersForeignSessionFileLine verifies that AttachWS does not
// forward EvtSessionFileLine events for a different sessionID.
func TestSurfaceFiltersForeignSessionFileLine(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Push a file-line event for a foreign session.
	fake.events <- proto.EvtSessionFileLine{SessionID: "OTHER", Kind: "transcript", Line: "should be dropped"}

	// The client should NOT receive this frame.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("expected timeout / no message for foreign session, but got a frame")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context") {
		t.Logf("got non-deadline error (acceptable): %v", err)
	}
}

// TestSurfaceForwardsOwnSessionFileLine verifies that AttachWS forwards
// EvtSessionFileLine events that match the subscribed sessionID.
func TestSurfaceForwardsOwnSessionFileLine(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Push a file-line event for the subscribed session.
	fake.events <- proto.EvtSessionFileLine{SessionID: "s1", Kind: "transcript", Line: "mine"}

	m := readJSONFrame(t, c)

	if m["k"] != "tt" {
		t.Errorf("frame k = %q, want \"tt\"", m["k"])
	}
	if m["line"] != "mine" {
		t.Errorf("line = %q, want \"mine\"", m["line"])
	}
}

// TestSurfaceFiltersForeignAgentNotification verifies that AttachWS does not
// forward EvtAgentNotification events for a different sessionID.
func TestSurfaceFiltersForeignAgentNotification(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Push a notification for a foreign session.
	fake.events <- proto.EvtAgentNotification{SessionID: "OTHER", Cmd: 1, Title: "foreign"}

	// The client should NOT receive this frame.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("expected timeout / no message for foreign session, but got a frame")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context") {
		t.Logf("got non-deadline error (acceptable): %v", err)
	}
}

// TestGatewayPersist is a combined smoke-test exercising lifecycle broadcast
// of EvtSessionFileLine and EvtAgentNotification end-to-end with a WS pair.
func TestGatewayPersist(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Seed hello so gateway is past helloSent=false.
	fake.events <- proto.EvtSessionsChanged{}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello, got k=%q", hello["k"])
	}

	// Broadcast a transcript line.
	fake.events <- proto.EvtSessionFileLine{SessionID: "s1", Kind: "transcript", Line: "broadcast-line"}
	ttFrame := readJSONFrame(t, c)
	if ttFrame["k"] != "tt" {
		t.Errorf("transcript frame k = %q, want \"tt\"", ttFrame["k"])
	}
	if ttFrame["line"] != "broadcast-line" {
		t.Errorf("line = %q, want \"broadcast-line\"", ttFrame["line"])
	}

	// Broadcast an agent notification.
	fake.events <- proto.EvtAgentNotification{SessionID: "s1", Cmd: 42, Title: "notify"}
	nFrame := readJSONFrame(t, c)
	if nFrame["k"] != "n" {
		t.Errorf("notification frame k = %q, want \"n\"", nFrame["k"])
	}
	if cmd, _ := nFrame["cmd"].(float64); int(cmd) != 42 {
		t.Errorf("cmd = %v, want 42", nFrame["cmd"])
	}

	// Verify websocket is still alive by closing gracefully.
	_ = c.Close(websocket.StatusNormalClosure, "done")
}
