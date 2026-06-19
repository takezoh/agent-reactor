package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// --- fake daemon helpers ---

// fakeDaemon simulates the arc daemon side of a proto connection. The test
// controls what it sends back via the sendResp / sendErr helpers.
type fakeDaemon struct {
	t      *testing.T
	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
}

// newDaemonPair creates a connected (DaemonClient, fakeDaemon) pair using
// net.Pipe. The DaemonClient is immediately healthy.
func newDaemonPair(t *testing.T) (*DaemonClient, *fakeDaemon) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close(); serverConn.Close() })

	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return proto.DialConn(clientConn), nil
	}, time.Millisecond, 2*time.Millisecond)
	t.Cleanup(func() { d.Close() })

	if !waitHealth(d, true, time.Second) {
		t.Fatal("DaemonClient did not become healthy")
	}

	fd := &fakeDaemon{
		t:      t,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
		conn:   serverConn,
	}
	return d, fd
}

// recv reads the next newline-delimited envelope from the client.
func (f *fakeDaemon) recv() proto.Envelope {
	f.t.Helper()
	line, err := f.reader.ReadBytes('\n')
	if err != nil {
		f.t.Fatalf("fakeDaemon.recv: %v", err)
	}
	env, err := proto.DecodeEnvelope(line)
	if err != nil {
		f.t.Fatalf("fakeDaemon.decode: %v", err)
	}
	return env
}

// sendResp writes a successful response for the given reqID.
func (f *fakeDaemon) sendResp(reqID string, r proto.Response) {
	f.t.Helper()
	wire, err := proto.EncodeResponse(reqID, r)
	if err != nil {
		f.t.Fatalf("fakeDaemon.encodeResp: %v", err)
	}
	f.write(wire)
}

// sendErr writes an error response for the given reqID.
func (f *fakeDaemon) sendErr(reqID string, code proto.ErrCode, msg string) {
	f.t.Helper()
	wire, err := proto.EncodeError(reqID, code, msg, nil)
	if err != nil {
		f.t.Fatalf("fakeDaemon.encodeErr: %v", err)
	}
	f.write(wire)
}

func (f *fakeDaemon) write(b []byte) {
	_, err := f.writer.Write(b)
	if err == nil {
		err = f.writer.WriteByte('\n')
	}
	if err == nil {
		err = f.writer.Flush()
	}
	if err != nil {
		// Connection already closed (test cleanup). Log, don't fatal.
		f.t.Logf("fakeDaemon.write: %v", err)
	}
}

// --- health 503 tests ---

// TestMux_Health503OnRest verifies that GET /api/sessions returns 503 when
// the DaemonClient has never connected.
func TestMux_Health503OnRest(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("no daemon") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()
	if waitHealth(d, true, 50*time.Millisecond) {
		t.Skip("daemon became healthy unexpectedly")
	}

	mux := NewMux(d, "tok")
	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

// TestMux_Health503OnWS verifies that /ws returns 503 when the daemon is down.
func TestMux_Health503OnWS(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("no daemon") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()
	if waitHealth(d, true, 50*time.Millisecond) {
		t.Skip("daemon became healthy unexpectedly")
	}

	srv := httptest.NewServer(NewMux(d, "tok"))
	defer srv.Close()

	// Mint a ticket (requires auth, but server is not a daemon call — tickets
	// are minted in-process; only the WS upgrade itself checks d.Health()).
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("mint ticket: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mint ticket status = %d", resp.StatusCode)
	}
	var ticketBody struct {
		Ticket string `json:"ticket"`
	}
	// Re-mint because resp body already closed above — do it properly.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ws-ticket", nil)
	req2.Header.Set("Authorization", "Bearer tok")
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("mint ticket2: %v", err)
	}
	if err := json.NewDecoder(resp2.Body).Decode(&ticketBody); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}
	_ = resp2.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?session=s1&ticket=" + ticketBody.Ticket
	_, httpResp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected ws dial to fail with 503, but succeeded")
	}
	if httpResp == nil || httpResp.StatusCode != http.StatusServiceUnavailable {
		got := 0
		if httpResp != nil {
			got = httpResp.StatusCode
		}
		t.Fatalf("want 503, got %d (err %v)", got, err)
	}
}

// --- ticket flow ---

// TestMux_TicketFlowUnchanged exercises the ticket-gated WebSocket flow: mint,
// connect once (101), re-use same ticket (401).
func TestMux_TicketFlowUnchanged(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	srv := httptest.NewServer(NewMux(d, "tok"))
	defer srv.Close()

	// Mint ticket.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := srv.Client().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("mint: status=%d err=%v", resp.StatusCode, err)
	}
	var tb struct{ Ticket string `json:"ticket"` }
	if err := json.NewDecoder(resp.Body).Decode(&tb); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}
	_ = resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First dial — daemon receives subscribe; respond with empty RespSessions to
	// let SubscribeSurface complete. The test only checks the upgrade succeeds.
	go func() {
		env := daemon.recv() // CmdSurfaceSubscribe
		daemon.sendResp(env.ReqID, proto.RespOK{})
	}()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?session=s1&ticket=" + tb.Ticket
	c, wsr, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first dial: %v (status %v)", err, wsr)
	}
	_ = c.CloseNow()

	// Re-use: same ticket must be rejected.
	_, httpResp, err2 := websocket.Dial(ctx, wsURL, nil)
	if err2 == nil {
		t.Fatal("second dial: expected 401, got success")
	}
	if httpResp == nil || httpResp.StatusCode != http.StatusUnauthorized {
		got := 0
		if httpResp != nil {
			got = httpResp.StatusCode
		}
		t.Fatalf("second dial: want 401, got %d (err %v)", got, err2)
	}
}

// --- create session forwarding ---

// TestMux_CreateForwardsCmdEvent verifies that POST /api/sessions sends a
// CmdEvent{Event:"create-session"} with the correct params to the daemon, and
// maps RespCreateSession.SessionID into the REST response.
func TestMux_CreateForwardsCmdEvent(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	done := make(chan state.CreateSessionParams, 1)
	go func() {
		env := daemon.recv()
		// env.Data is the CmdEvent JSON; unwrap to get the nested payload.
		var cmd struct {
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(env.Data, &cmd); err != nil {
			t.Errorf("unmarshal CmdEvent: %v", err)
			close(done)
			return
		}
		var params state.CreateSessionParams
		if err := json.Unmarshal(cmd.Payload, &params); err != nil {
			t.Errorf("unmarshal params: %v", err)
			close(done)
			return
		}
		done <- params
		daemon.sendResp(env.ReqID, proto.RespCreateSession{SessionID: "abc"})
	}()

	r := httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"project":"/p","command":"sh","cols":120,"rows":40}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", w.Code, w.Body.String())
	}

	params := <-done
	if params.Project != "/p" {
		t.Errorf("Project = %q, want /p", params.Project)
	}
	if params.Command != "sh" {
		t.Errorf("Command = %q, want sh", params.Command)
	}
	if params.Options.Cols != 120 {
		t.Errorf("Cols = %d, want 120", params.Options.Cols)
	}
	if params.Options.Rows != 40 {
		t.Errorf("Rows = %d, want 40", params.Options.Rows)
	}

	var info apiSessionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if info.ID != "abc" {
		t.Errorf("response ID = %q, want abc", info.ID)
	}
}

// --- list sessions ---

// TestMux_ListMapsRespSessions verifies that GET /api/sessions maps
// RespSessions.Sessions into the REST response array.
func TestMux_ListMapsRespSessions(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	go func() {
		env := daemon.recv()
		daemon.sendResp(env.ReqID, proto.RespSessions{
			Sessions: []proto.SessionInfo{
				{ID: "s1", Command: "sh"},
			},
		})
	}()

	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	var sessions []apiSessionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "s1" {
		t.Errorf("ID = %q, want s1", sessions[0].ID)
	}
	if sessions[0].Command != "sh" {
		t.Errorf("Command = %q, want sh", sessions[0].Command)
	}
}

// --- delete error mapping ---

// TestMux_DeleteMapsErrorToHTTPCode verifies that proto ErrorBody codes are
// mapped to the expected HTTP status codes by DELETE /api/sessions/{id}.
func TestMux_DeleteMapsErrorToHTTPCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		errCode  proto.ErrCode
		wantHTTP int
	}{
		{"not_found", proto.ErrNotFound, http.StatusNotFound},
		{"invalid_argument", proto.ErrInvalidArgument, http.StatusBadRequest},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, daemon := newDaemonPair(t)
			mux := NewMux(d, "tok")

			go func() {
				env := daemon.recv()
				daemon.sendErr(env.ReqID, tc.errCode, "test error")
			}()

			r := httptest.NewRequest(http.MethodDelete, "/api/sessions/s1", nil)
			r.Header.Set("Authorization", "Bearer tok")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			if w.Code != tc.wantHTTP {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.wantHTTP, w.Body.String())
			}
		})
	}
}

// --- auth invariant ---

// TestMux_AuthInvariant verifies that GET /api/sessions without a bearer
// token returns 401.
func TestMux_AuthInvariant(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	mux := NewMux(d, "tok")
	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil) // no auth
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
