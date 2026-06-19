package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
)

// fakeSessionAttacher is a test double for Attacher that records calls and
// streams events from a channel the test controls. All recorded-call fields
// are protected by mu so the test goroutine can read them safely.
type fakeSessionAttacher struct {
	events chan proto.ServerEvent

	mu            sync.Mutex
	writeRawCalls []writeRawCall
	resizeCalls   []resizeCall
	subscribeErr  error
}

type writeRawCall struct {
	sessionID string
	data      []byte
}

type resizeCall struct {
	sessionID  string
	cols, rows uint16
}

func newFakeAttacher() *fakeSessionAttacher {
	return &fakeSessionAttacher{
		events: make(chan proto.ServerEvent, 16),
	}
}

func (f *fakeSessionAttacher) SubscribeSurface(_ context.Context, _ string) (<-chan proto.ServerEvent, error) {
	f.mu.Lock()
	err := f.subscribeErr
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.events, nil
}

func (f *fakeSessionAttacher) UnsubscribeSurface(_ context.Context, _ string) error {
	return nil
}

func (f *fakeSessionAttacher) WriteRaw(_ context.Context, sessionID string, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	f.mu.Lock()
	f.writeRawCalls = append(f.writeRawCalls, writeRawCall{sessionID: sessionID, data: cp})
	f.mu.Unlock()
	return nil
}

func (f *fakeSessionAttacher) Resize(_ context.Context, sessionID string, cols, rows uint16) error {
	f.mu.Lock()
	f.resizeCalls = append(f.resizeCalls, resizeCall{sessionID: sessionID, cols: cols, rows: rows})
	f.mu.Unlock()
	return nil
}

func (f *fakeSessionAttacher) writeRawSnapshot() []writeRawCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]writeRawCall, len(f.writeRawCalls))
	copy(out, f.writeRawCalls)
	return out
}

func (f *fakeSessionAttacher) resizeSnapshot() []resizeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]resizeCall, len(f.resizeCalls))
	copy(out, f.resizeCalls)
	return out
}

// startGatewayServer starts an httptest server that attaches a fakeSessionAttacher
// for sessionID over WS, returning the server, attacher, and a dial function.
func startGatewayServer(t *testing.T, sess Attacher, sessionID string) *httptest.Server { //nolint:unparam
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = AttachWS(r.Context(), sess, sessionID, c)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func dialWsGateway(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// TestGatewayAttachWS_SubscribeOutput verifies that EvtSurfaceOutput events
// are forwarded to the WS client as asciicast v2 arrays.
func TestGatewayAttachWS_SubscribeOutput(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Push an event from the fake daemon.
	encoded := base64.StdEncoding.EncodeToString([]byte("hi"))
	fake.events <- proto.EvtSurfaceOutput{SessionID: "s1", TimeSec: 0.5, DataB64: encoded}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var arr []any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if len(arr) != 3 || arr[1] != "o" {
		t.Fatalf("unexpected frame: %s", data)
	}
	if arr[0].(float64) != 0.5 {
		t.Errorf("TimeSec = %v, want 0.5", arr[0])
	}
	if arr[2].(string) != "hi" {
		t.Errorf("data = %q, want \"hi\"", arr[2])
	}
}

// TestGatewayAttachWS_InboundInputTranslatesToCmdSurfaceWriteRaw verifies that
// {"k":"i","d":"abc"} frames from the browser become WriteRaw calls.
func TestGatewayAttachWS_InboundInputTranslatesToCmdSurfaceWriteRaw(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"i","d":"abc"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Give the reader goroutine time to process.
	deadline := time.Now().Add(time.Second)
	var calls []writeRawCall
	for time.Now().Before(deadline) {
		calls = fake.writeRawSnapshot()
		if len(calls) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(calls) == 0 {
		t.Fatal("no WriteRaw call observed")
	}
	call := calls[0]
	if call.sessionID != "s1" {
		t.Errorf("sessionID = %q, want \"s1\"", call.sessionID)
	}
	if string(call.data) != "abc" {
		t.Errorf("data = %q, want \"abc\"", call.data)
	}
}

// TestGatewayAttachWS_InboundResizeTranslatesToCmdSurfaceResize verifies that
// {"k":"r","cols":120,"rows":40} frames become Resize calls.
func TestGatewayAttachWS_InboundResizeTranslatesToCmdSurfaceResize(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"k":"r","cols":120,"rows":40}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	var rcalls []resizeCall
	for time.Now().Before(deadline) {
		rcalls = fake.resizeSnapshot()
		if len(rcalls) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(rcalls) == 0 {
		t.Fatal("no Resize call observed")
	}
	call := rcalls[0]
	if call.sessionID != "s1" {
		t.Errorf("sessionID = %q, want \"s1\"", call.sessionID)
	}
	if call.cols != 120 || call.rows != 40 {
		t.Errorf("size = %dx%d, want 120x40", call.cols, call.rows)
	}
}

// TestGatewayAttachWS_TwoStepCloseOnDaemonDisconnect verifies ADR 0011:
// when the daemon events channel closes, the gateway sends a control frame
// then a StatusGoingAway typed close.
func TestGatewayAttachWS_TwoStepCloseOnDaemonDisconnect(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Simulate daemon disconnect by closing the events channel.
	close(fake.events)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First message must be the control frame.
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("expected control frame before close, got read error: %v", err)
	}
	var msg controlMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal control frame: %v", err)
	}
	if msg.K != "c" || msg.Data != "daemon-disconnected" {
		t.Errorf("control frame = %+v, want {K:\"c\", Data:\"daemon-disconnected\"}", msg)
	}

	// Next read must produce a close error with StatusGoingAway.
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

// TestGatewayAttachWS_SubscribeError verifies that when SubscribeSurface
// returns an error, the gateway performs a 2-step close (control + StatusGoingAway).
func TestGatewayAttachWS_SubscribeError(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	fake.subscribeErr = &proto.ErrorBody{Code: "frame-not-ready", Message: "not ready"}
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First message: control frame with error code.
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("expected control frame, got: %v", err)
	}
	var msg controlMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal control: %v", err)
	}
	if msg.K != "c" {
		t.Errorf("control kind = %q, want \"c\"", msg.K)
	}

	// Next read: StatusGoingAway close.
	_, _, err = c.Read(ctx)
	if err == nil {
		t.Fatal("expected close error")
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CloseError, got %T: %v", err, err)
	}
	if ce.Code != websocket.StatusGoingAway {
		t.Errorf("close code = %v, want StatusGoingAway", ce.Code)
	}
}

// TestGatewayAttachWS_FilterByOtherSessionDropped verifies that events for a
// different sessionID are not forwarded to the WS client.
func TestGatewayAttachWS_FilterByOtherSessionDropped(t *testing.T) {
	t.Parallel()

	fake := newFakeAttacher()
	srv := startGatewayServer(t, fake, "s1")
	c := dialWsGateway(t, srv)

	// Push an event for a different session.
	encoded := base64.StdEncoding.EncodeToString([]byte("other"))
	fake.events <- proto.EvtSurfaceOutput{SessionID: "other-session", TimeSec: 1.0, DataB64: encoded}

	// The client should NOT receive this event — verify with a short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("expected timeout / no message for other session, but got a frame")
	}
	// context.DeadlineExceeded is the expected outcome.
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context") {
		t.Logf("got non-deadline error (acceptable): %v", err)
	}
}
