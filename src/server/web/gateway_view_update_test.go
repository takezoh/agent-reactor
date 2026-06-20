package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
	stateview "github.com/takezoh/agent-reactor/client/state/view"
)

// fakeLifecycleAttacher is a test double for Attacher that supports
// lifecycle subscriptions. SubscribeLifecycle returns a channel the
// test controls; all surface/input methods are no-ops.
type fakeLifecycleAttacher struct {
	events       chan proto.ServerEvent
	subscribeErr error
}

func newFakeLifecycleAttacher() *fakeLifecycleAttacher {
	return &fakeLifecycleAttacher{
		events: make(chan proto.ServerEvent, 16),
	}
}

func (f *fakeLifecycleAttacher) SubscribeLifecycle(_ context.Context) (<-chan proto.ServerEvent, error) {
	if f.subscribeErr != nil {
		return nil, f.subscribeErr
	}
	return f.events, nil
}

func (f *fakeLifecycleAttacher) SubscribeSurface(_ context.Context, _ string) (<-chan proto.ServerEvent, error) {
	return nil, errors.New("not implemented in lifecycle fake")
}

func (f *fakeLifecycleAttacher) UnsubscribeSurface(_ context.Context, _ string) error { return nil }

func (f *fakeLifecycleAttacher) SendSurfaceSubscribe(_ context.Context, _ string) error { return nil }

func (f *fakeLifecycleAttacher) WriteRaw(_ context.Context, _ string, _ []byte) error { return nil }

func (f *fakeLifecycleAttacher) Resize(_ context.Context, _ string, _ uint16, _ uint16) error {
	return nil
}

// startLifecycleServer starts an httptest server that calls AttachLifecycleWS
// directly (no ticket check) so tests can dial without auth plumbing.
func startLifecycleServer(t *testing.T, sess Attacher) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = AttachLifecycleWS(r.Context(), sess, c)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func dialLifecycleWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial lifecycle WS: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// readJSONFrame reads one WS text frame and unmarshals it as a map.
// Uses a fixed 3-second deadline appropriate for protofake-driven tests.
func readJSONFrame(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read WS frame: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal frame %q: %v", data, err)
	}
	return m
}

// sampleSession builds a SessionInfo with the given ID and title for tests.
func sampleSession(id, title string, status stateview.Status) proto.SessionInfo {
	return proto.SessionInfo{
		ID:      id,
		Project: "proj",
		Command: "claude",
		View: stateview.View{
			Card:   stateview.Card{Title: title},
			Status: status,
		},
	}
}

// TestGatewayLifecycle_EmitsHelloFirst verifies that the first frame sent on a
// lifecycle WebSocket has k:"h" and contains the seeded session data.
func TestGatewayLifecycle_EmitsHelloFirst(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	fake.events <- proto.EvtSessionsChanged{
		Sessions:        []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
		ActiveSessionID: "s1",
		Features:        []string{"surface"},
	}

	m := readJSONFrame(t, c)

	if m["k"] != "h" {
		t.Errorf("first frame k = %q, want \"h\"", m["k"])
	}
	if m["activeSessionID"] != "s1" {
		t.Errorf("activeSessionID = %q, want \"s1\"", m["activeSessionID"])
	}
	sessions, ok := m["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Fatalf("sessions missing or empty: %v", m["sessions"])
	}
	sess0, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0] is not object: %T", sessions[0])
	}
	viewObj, ok := sess0["view"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view is not object: %T", sess0["view"])
	}
	cardObj, ok := viewObj["card"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view.card is not object: %T", viewObj["card"])
	}
	if cardObj["title"] != "T" {
		t.Errorf("sessions[0].view.card.title = %q, want \"T\"", cardObj["title"])
	}
}

// TestGatewayLifecycle_BroadcastsViewUpdate verifies that the second event is
// sent as a view-update frame (k:"v") after the hello frame.
func TestGatewayLifecycle_BroadcastsViewUpdate(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// First event → hello frame.
	fake.events <- proto.EvtSessionsChanged{
		Sessions:        []proto.SessionInfo{sampleSession("s1", "T", stateview.StatusRunning)},
		ActiveSessionID: "s1",
		Features:        []string{"surface"},
	}
	hello := readJSONFrame(t, c)
	if hello["k"] != "h" {
		t.Fatalf("expected hello frame, got k=%q", hello["k"])
	}

	// Second event → view-update frame.
	fake.events <- proto.EvtSessionsChanged{
		Sessions:        []proto.SessionInfo{sampleSession("s2", "U", stateview.StatusIdle)},
		ActiveSessionID: "s2",
	}
	m := readJSONFrame(t, c)

	if m["k"] != "v" {
		t.Errorf("second frame k = %q, want \"v\"", m["k"])
	}
	if m["activeSessionID"] != "s2" {
		t.Errorf("activeSessionID = %q, want \"s2\"", m["activeSessionID"])
	}
	sessions, ok := m["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Fatalf("sessions missing or empty: %v", m["sessions"])
	}
	sess0, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0] is not object: %T", sessions[0])
	}
	viewObj, ok := sess0["view"].(map[string]any)
	if !ok {
		t.Fatalf("sessions[0].view is not object: %T", sess0["view"])
	}
	if viewObj["status"] != "idle" {
		t.Errorf("sessions[0].view.status = %q, want \"idle\"", viewObj["status"])
	}
}

// TestGatewayLifecycle_DaemonDisconnect verifies the 2-step close protocol
// (ADR 0011) when the daemon events channel closes on a lifecycle WS.
func TestGatewayLifecycle_DaemonDisconnect(t *testing.T) {
	t.Parallel()

	fake := newFakeLifecycleAttacher()
	srv := startLifecycleServer(t, fake)
	c := dialLifecycleWS(t, srv)

	// Simulate daemon disconnect.
	close(fake.events)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First message: control frame {"k":"c","data":"daemon-disconnected"}.
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("expected control frame before close, got: %v", err)
	}
	var msg controlMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal control frame: %v", err)
	}
	if msg.K != "c" || msg.Data != "daemon-disconnected" {
		t.Errorf("control frame = %+v, want {K:\"c\", Data:\"daemon-disconnected\"}", msg)
	}

	// Next read: StatusGoingAway typed close.
	_, _, err = c.Read(ctx)
	if err == nil {
		t.Fatal("expected close error, got nil")
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CloseError, got %T: %v", err, err)
	}
	if ce.Code != websocket.StatusGoingAway {
		t.Errorf("close code = %v, want StatusGoingAway", ce.Code)
	}
}
