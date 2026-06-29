package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
)

// --- fake daemon helpers ---

// fakeDaemon simulates the server daemon side of a proto connection. The test
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
	var tb struct {
		Ticket string `json:"ticket"`
	}
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

// TestMux_CreateForwardsWorktreeAndSandbox verifies that worktree=true and
// sandbox="host" in the REST body reach the daemon as
// CreateSessionParams.Options.Worktree.Enabled and Sandbox=SandboxOverrideHost.
func TestMux_CreateForwardsWorktreeAndSandbox(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	done := make(chan state.CreateSessionParams, 1)
	go func() {
		env := daemon.recv()
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
		strings.NewReader(`{"project":"/p","command":"sh","worktree":true,"sandbox":"host"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", w.Code, w.Body.String())
	}

	params := <-done
	if !params.Options.Worktree.Enabled {
		t.Error("Options.Worktree.Enabled = false, want true")
	}
	if params.Sandbox != state.SandboxOverrideHost {
		t.Errorf("Sandbox = %v, want SandboxOverrideHost", params.Sandbox)
	}
}

// TestMux_CreateDefaultsWorktreeAndSandbox verifies that omitting the new
// fields keeps the legacy wire shape: worktree disabled, sandbox=Auto.
func TestMux_CreateDefaultsWorktreeAndSandbox(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	done := make(chan state.CreateSessionParams, 1)
	go func() {
		env := daemon.recv()
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
		strings.NewReader(`{"project":"/p","command":"sh"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", w.Code, w.Body.String())
	}

	params := <-done
	if params.Options.Worktree.Enabled {
		t.Error("Options.Worktree.Enabled = true, want false (default)")
	}
	if params.Sandbox != state.SandboxOverrideAuto {
		t.Errorf("Sandbox = %v, want SandboxOverrideAuto", params.Sandbox)
	}
}

// TestMux_CreateRejectsUnknownSandbox verifies that a sandbox value outside
// the {"", "auto", "host"} vocabulary is rejected at the gateway with 400
// rather than silently dropped or forwarded.
func TestMux_CreateRejectsUnknownSandbox(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")

	r := httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"project":"/p","command":"sh","sandbox":"docker"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
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

// --- push command forwarding ---

// pushPathFor returns the REST path for a push command on the given session id.
func pushPathFor(id string) string {
	return "/api/sessions/" + id + "/push"
}

// drainPushRequest acts as a one-shot fake daemon for the push handler: it
// expects (1) a ListSessions request (replied with the supplied RespSessions)
// and (2) a PushDriver request (replied with RespOK). It writes the decoded
// PushDriverParams to the returned channel so the test can assert on what
// reached the daemon. If wantPush=false, the test expects ONLY the first
// ListSessions request and no second push request.
func drainPushRequest(t *testing.T, daemon *fakeDaemon, list proto.RespSessions, wantPush bool) <-chan state.PushDriverParams {
	t.Helper()
	out := make(chan state.PushDriverParams, 1)
	go func() {
		env := daemon.recv() // first envelope: ListSessions
		daemon.sendResp(env.ReqID, list)
		if !wantPush {
			close(out)
			return
		}
		env2 := daemon.recv() // second envelope: PushDriver
		var cmd struct {
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(env2.Data, &cmd); err != nil {
			t.Errorf("unmarshal push CmdEvent: %v", err)
			close(out)
			return
		}
		if cmd.Event != "push-driver" {
			t.Errorf("second event = %q, want push-driver", cmd.Event)
		}
		var params state.PushDriverParams
		if err := json.Unmarshal(cmd.Payload, &params); err != nil {
			t.Errorf("unmarshal push params: %v", err)
			close(out)
			return
		}
		out <- params
		daemon.sendResp(env2.ReqID, proto.RespOK{})
	}()
	return out
}

// TestMux_PushForwardsEventPushDriver verifies that POST /api/sessions/{id}/push
// with a matching daemon-global active session id sends a CmdEvent{
// Event:"push-driver", Payload: PushDriverParams{...}} to the daemon and
// returns 200.
func TestMux_PushForwardsEventPushDriver(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	list := proto.RespSessions{
		Sessions: []proto.SessionInfo{{ID: "s1"}},
	}
	gotCh := drainPushRequest(t, daemon, list, true)

	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"),
		strings.NewReader(`{"command":"/clear"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	params, ok := <-gotCh
	if !ok {
		t.Fatal("expected PushDriverParams on daemon channel")
	}
	if params.SessionID != "s1" {
		t.Errorf("SessionID = %q, want s1", params.SessionID)
	}
	if params.Command != "/clear" {
		t.Errorf("Command = %q, want /clear", params.Command)
	}
}

// TestMux_PushReturns404OnUnknownSession verifies that POST to a session id
// that does not exist on the daemon returns 404, without issuing a PushDriver
// RPC.
func TestMux_PushReturns404OnUnknownSession(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	list := proto.RespSessions{
		Sessions: []proto.SessionInfo{{ID: "other"}},
	}
	gotCh := drainPushRequest(t, daemon, list, false)

	r := httptest.NewRequest(http.MethodPost, pushPathFor("ghost"),
		strings.NewReader(`{"command":"/clear"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", w.Code, w.Body.String())
	}
	if _, ok := <-gotCh; ok {
		t.Fatal("PushDriver was sent on 404; want none")
	}
}

// TestMux_PushRejectsEmptyCommand verifies that a JSON body with an empty
// or whitespace-only command is rejected with 400 before any daemon RPC.
func TestMux_PushRejectsEmptyCommand(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")

	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"),
		strings.NewReader(`{"command":"   "}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
}

// TestMux_PushRejectsInvalidJSON verifies that a malformed JSON body returns
// 400, again before any daemon RPC.
func TestMux_PushRejectsInvalidJSON(t *testing.T) {
	t.Parallel()
	d, _ := newDaemonPair(t)
	mux := NewMux(d, "tok")

	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"),
		strings.NewReader(`{"command":`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
}

// TestMux_PushRequiresAuth verifies that POST /api/sessions/{id}/push without
// a bearer token returns 401 — the route is mounted under the same TokenAuth
// middleware as every other /api/* route.
func TestMux_PushRequiresAuth(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	mux := NewMux(d, "tok")
	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"),
		strings.NewReader(`{"command":"/clear"}`))
	// no Authorization header
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", w.Code, w.Body.String())
	}
}

// TestMux_PushRejectsBodyTooLarge verifies that a request body larger than
// pushBodyLimit (64KiB) is rejected with 413 Payload Too Large, not the 400
// "bad request body" you'd get if the body had simply truncated through an
// io.LimitReader (the original review note: "413 should be distinct from a
// 400 so the client can tell why the request failed").
//
// No daemon RPC should fire — the cap is enforced before the JSON Decoder
// finishes — so we use a daemon dialer that always errors and assert only
// the HTTP status code.
func TestMux_PushRejectsBodyTooLarge(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()
	// Wait until the dialer has failed at least once so d.Health() is false
	// and we'd see 503, OR until the test setup decides to skip if it ever
	// did flip healthy. (Defensive: this dialer cannot succeed.)
	_ = waitHealth(d, false, 100*time.Millisecond)

	mux := NewMux(d, "tok")
	// Build a payload that exceeds pushBodyLimit (64KiB). The body still parses
	// as JSON ({"command":"<huge string>"}) so a working json.Decoder would
	// return a value rather than a syntax error; only the size cap should
	// trigger the failure path.
	huge := strings.Repeat("A", pushBodyLimit+1)
	body := `{"command":"` + huge + `"}`
	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"), strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	// Either the cap fires (413) before any health check is reached (mux ordering
	// puts Health first), so we accept either 413 or 503 depending on which
	// gate ran first under the unhealthy dialer. The point: NEVER 400, because
	// the previous io.LimitReader pathway would have surfaced "unexpected EOF"
	// as 400 bad_request and obscured the size cause. If health gating fired
	// first this test still wouldn't show 400; if the cap fired first we get 413.
	if w.Code == http.StatusBadRequest {
		t.Fatalf("status = 400, want 413 (body too large) — body cap regressed to bad-body decode: %q",
			w.Body.String())
	}
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 413 or 503 (body %q)", w.Code, w.Body.String())
	}
}

// TestMux_PushRejectsBodyTooLargeOnHealthyDaemon is the strict variant of
// TestMux_PushRejectsBodyTooLarge: with a connected daemon (so the Health
// gate passes) the over-size body must be rejected with 413 specifically.
// No ListSessions / PushDriver RPC may be issued — the body cap fires before
// the active-session lookup.
func TestMux_PushRejectsBodyTooLargeOnHealthyDaemon(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	// If the gate fails, the test would see a ListSessions hit the fake
	// daemon. Use a closed channel so a stray recv would noticeably stall;
	// the test enforces this by NOT spawning a recv goroutine.
	_ = daemon // silence unused; the pair construction is what gives us a healthy d.

	huge := strings.Repeat("B", pushBodyLimit+1)
	body := `{"command":"` + huge + `"}`
	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"), strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "request body exceeds limit") {
		t.Errorf("body = %q, want it to mention 'request body exceeds limit'", w.Body.String())
	}
}

// TestMux_PushMapsDaemonErrorTo502 verifies that when the second daemon RPC
// (PushDriver) returns proto.ErrInternal, the gateway surfaces 502 Bad Gateway
// per handleProtoError's standard mapping.
func TestMux_PushMapsDaemonErrorTo502(t *testing.T) {
	t.Parallel()
	d, daemon := newDaemonPair(t)
	mux := NewMux(d, "tok")

	list := proto.RespSessions{
		Sessions: []proto.SessionInfo{{ID: "s1"}},
	}
	go func() {
		env := daemon.recv() // ListSessions
		daemon.sendResp(env.ReqID, list)
		env2 := daemon.recv() // PushDriver
		daemon.sendErr(env2.ReqID, proto.ErrInternal, "spawn failed")
	}()

	r := httptest.NewRequest(http.MethodPost, pushPathFor("s1"),
		strings.NewReader(`{"command":"/clear"}`))
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body %q)", w.Code, w.Body.String())
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

// --- session config ---

// TestMux_SessionConfigSurfacesConfigFields verifies that GET /api/session-config
// surfaces default_command / commands / projects sourced from settings.toml.
// The loader is stubbed so the test doesn't depend on the developer's
// ~/.agent-reactor/settings.toml.
// Not t.Parallel(): handleSessionConfig swaps a package-level loader via
// withSessionConfigLoader; two concurrent overrides would race the way they
// did when both this test and TestMux_SessionConfigSurfacesLoadError ran
// in parallel.
func TestMux_SessionConfigSurfacesConfigFields(t *testing.T) {
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	restore := withSessionConfigLoader(func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.Session.DefaultCommand = "claude"
		cfg.Session.Commands = []string{"claude", "shell", "npm run dev"}
		cfg.Session.PushCommands = []string{"/clear", "/compact"}
		cfg.Projects.ProjectPaths = []string{"/home/me/repo-a", "/home/me/repo-b"}
		cfg.Projects.ProjectRoots = []string{"/home/me/code"}
		return cfg, nil
	})
	defer restore()

	mux := NewMux(d, "tok")
	r := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	var got apiSessionConfig
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DefaultCommand != "claude" {
		t.Errorf("DefaultCommand = %q, want claude", got.DefaultCommand)
	}
	if len(got.Commands) != 3 || got.Commands[0] != "claude" {
		t.Errorf("Commands = %v, want [claude shell npm run dev]", got.Commands)
	}
	// push_commands flows straight from cfg.Session.PushCommands; the palette
	// push scope reads this to enumerate tool entries (FR-027).
	if len(got.PushCommands) != 2 || got.PushCommands[0] != "/clear" {
		t.Errorf("PushCommands = %v, want [/clear /compact]", got.PushCommands)
	}
	// project_roots / project_paths are surfaced verbatim from settings.toml
	// (forward-compat for future UI hints); Projects is the resolved list,
	// which may be empty when the referenced dirs don't exist on disk.
	if len(got.ProjectRoots) != 1 || got.ProjectRoots[0] != "/home/me/code" {
		t.Errorf("ProjectRoots = %v, want [/home/me/code]", got.ProjectRoots)
	}
	if len(got.ProjectPaths) != 2 {
		t.Errorf("ProjectPaths len = %d, want 2", len(got.ProjectPaths))
	}
	// ProjectPaths only land in Projects if the referenced dirs exist
	// (listProjectsFrom stats them); since the test paths are fake, expect an
	// empty Projects list rather than the raw config values. The point of
	// this assertion is that the field is JSON-encoded as an array (not null)
	// so the UI's projects.map(...) doesn't blow up.
	if got.Projects == nil {
		t.Errorf("Projects = nil, want non-nil array")
	}
}

// TestMux_SessionConfigProjectMetadataReflectsDiskState verifies the per-project
// metadata (path / isGit / isSandboxed) on GET /api/session-config. Two project
// dirs are laid down in a t.TempDir: one with a .git child (isGit=true), one
// without (isGit=false). The sandbox config is set to "devcontainer" so the
// resolver reports IsSandboxed=true for both; flipping to "direct" verifies
// the false branch. (FR-027 / ADR-0041.)
//
// Not t.Parallel(): handleSessionConfig swaps a package-level loader via
// withSessionConfigLoader; concurrent overrides would race.
func TestMux_SessionConfigProjectMetadataReflectsDiskState(t *testing.T) {
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	// Lay down two real project dirs so listProjectsFrom doesn't filter them
	// out as nonexistent. Only repoGit gets a .git child.
	tmp := t.TempDir()
	repoGit := filepath.Join(tmp, "repo-git")
	repoPlain := filepath.Join(tmp, "repo-plain")
	if err := os.MkdirAll(filepath.Join(repoGit, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repoGit/.git: %v", err)
	}
	if err := os.MkdirAll(repoPlain, 0o755); err != nil {
		t.Fatalf("mkdir repoPlain: %v", err)
	}

	t.Run("isSandboxed=true under sandbox.mode=devcontainer", func(t *testing.T) {
		restore := withSessionConfigLoader(func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Sandbox = platformconfig.SandboxConfig{Mode: "devcontainer"}
			cfg.Projects.ProjectPaths = []string{repoGit, repoPlain}
			cfg.Session.PushCommands = []string{"/clear"}
			return cfg, nil
		})
		defer restore()

		mux := NewMux(d, "tok")
		r := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
		r.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
		}
		var got apiSessionConfig
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Index by path so the test isn't sensitive to listProjectsFrom order.
		byPath := map[string]apiProjectMeta{}
		for _, p := range got.Projects {
			byPath[p.Path] = p
		}
		gitMeta, ok := byPath[repoGit]
		if !ok {
			t.Fatalf("Projects missing repoGit %q; got %+v", repoGit, got.Projects)
		}
		if !gitMeta.IsGit {
			t.Errorf("repoGit.IsGit = false, want true (.git was laid down)")
		}
		if !gitMeta.IsSandboxed {
			t.Errorf("repoGit.IsSandboxed = false, want true (sandbox.mode=devcontainer)")
		}
		plainMeta, ok := byPath[repoPlain]
		if !ok {
			t.Fatalf("Projects missing repoPlain %q; got %+v", repoPlain, got.Projects)
		}
		if plainMeta.IsGit {
			t.Errorf("repoPlain.IsGit = true, want false (no .git)")
		}
		if !plainMeta.IsSandboxed {
			t.Errorf("repoPlain.IsSandboxed = false, want true (sandbox.mode=devcontainer)")
		}
		if len(got.PushCommands) != 1 || got.PushCommands[0] != "/clear" {
			t.Errorf("PushCommands = %v, want [/clear]", got.PushCommands)
		}
	})

	t.Run("isSandboxed=false under sandbox.mode=direct", func(t *testing.T) {
		restore := withSessionConfigLoader(func() (*config.Config, error) {
			cfg := config.DefaultConfig()
			cfg.Sandbox = platformconfig.SandboxConfig{Mode: "direct"}
			cfg.Projects.ProjectPaths = []string{repoGit}
			return cfg, nil
		})
		defer restore()

		mux := NewMux(d, "tok")
		r := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
		r.Header.Set("Authorization", "Bearer tok")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
		}
		var got apiSessionConfig
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got.Projects) != 1 {
			t.Fatalf("Projects len = %d, want 1; got %+v", len(got.Projects), got.Projects)
		}
		if got.Projects[0].Path != repoGit {
			t.Errorf("Projects[0].Path = %q, want %q", got.Projects[0].Path, repoGit)
		}
		if got.Projects[0].IsSandboxed {
			t.Errorf("Projects[0].IsSandboxed = true, want false (sandbox.mode=direct)")
		}
		// PushCommands defaults to ["shell"] in DefaultConfig; assert it
		// surfaces (non-nil JSON array even when not overridden).
		if got.PushCommands == nil {
			t.Errorf("PushCommands = nil, want non-nil array")
		}
	})
}

// TestMux_SessionConfigSurfacesLoadError verifies that a malformed settings.toml
// surfaces as 500 with a grep-able reason, not a silent empty response.
//
// Not t.Parallel(): see the comment on TestMux_SessionConfigSurfacesConfigFields.
func TestMux_SessionConfigSurfacesLoadError(t *testing.T) {
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	restore := withSessionConfigLoader(func() (*config.Config, error) {
		return nil, errors.New("bad toml")
	})
	defer restore()

	mux := NewMux(d, "tok")
	r := httptest.NewRequest(http.MethodGet, "/api/session-config", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "settings.toml load failed") {
		t.Errorf("body = %q, want it to contain 'settings.toml load failed'", w.Body.String())
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

// TestMuxNoAuth_AllowsUnauthenticated verifies that NewMuxNoAuth lets a
// request through with no Authorization header — the local-dev contract
// scripts/run-dev.sh relies on. We exercise both the REST path (would 401
// under NewMux) and a WS-ticket consumption (would 401 under NewMux).
//
// We do NOT assert WS handshake success: a real WS upgrade needs a daemon-
// backed terminal. Reaching the daemon-health check (503 in this test, since
// the dialer always errors) instead of the 401 ticket-rejection path is
// sufficient — it proves the ticket gate was skipped.
func TestMuxNoAuth_AllowsUnauthenticated(t *testing.T) {
	t.Parallel()
	d := NewDaemonClientWithDialer(
		func() (*proto.Client, error) { return nil, errors.New("unused") },
		time.Millisecond, 2*time.Millisecond,
	)
	defer d.Close()

	mux := NewMuxNoAuth(d)

	t.Run("REST passes without bearer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		// Daemon dial fails in this test, so the handler reaches the daemon
		// path and produces a non-401 error (503/504/500). The point is that
		// 401 is NOT returned — TokenAuth was bypassed.
		if w.Code == http.StatusUnauthorized {
			t.Fatalf("no-auth mux still returned 401 on /api/sessions; body=%q", w.Body.String())
		}
	})

	t.Run("WS without ticket reaches daemon health check", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/ws", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		// Under NewMux this would be 401 (ticket missing); under NewMuxNoAuth
		// the handler proceeds to the daemon-health check, which returns 503
		// because the dialer errors.
		if w.Code == http.StatusUnauthorized {
			t.Fatalf("no-auth mux still returned 401 on /ws; body=%q", w.Body.String())
		}
	})
}

// --- gitChildExists ---
//
// gitChildExists is the helper that derives apiProjectMeta.IsGit. The review
// flagged the previous statExists as a silent failure: any os.Stat error
// (including permission denied / ENOTDIR / I/O error) collapsed to bool=false
// with no log trace, so a .git that existed-but-couldn't-be-stat'd vanished
// from the worktree toggle without diagnostic. The new contract:
//   - fs.ErrNotExist → return false silently (common non-git case)
//   - any other error → return false AND emit slog.Warn so the operator can
//     grep "isGit derivation failed"
//
// Tests cover (1) .git exists, (2) .git missing (silent false), (3) a parent
// that is a regular file producing ENOTDIR (logged false).

// syncBuffer is a bytes.Buffer protected by a mutex so it is safe for
// concurrent writes from in-flight HTTP handler goroutines and reads from the
// test goroutine that calls withCapturedSlog.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// withCapturedSlog swaps slog.Default() for the duration of fn so the test
// can assert on the log record emitted by gitChildExists. The returned string
// is the captured handler output (newline-delimited slog "key=value" records).
// A mutex-protected buffer is used because parallel tests share the global
// slog default and in-flight httptest goroutines may write concurrently.
func withCapturedSlog(t *testing.T, fn func()) string {
	t.Helper()
	prev := slog.Default()
	var buf syncBuffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

// TestGitChildExists_TrueWhenDotGitPresent verifies that gitChildExists
// returns true when "<projectDir>/.git" exists, with no warn log emitted
// (the happy path must not pollute the gateway log).
func TestGitChildExists_TrueWhenDotGitPresent(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	var got bool
	logs := withCapturedSlog(t, func() {
		got = gitChildExists(context.Background(), tmp)
	})
	if !got {
		t.Fatalf("gitChildExists = false, want true")
	}
	if strings.Contains(logs, "isGit derivation failed") {
		t.Errorf("happy path emitted a warn log: %s", logs)
	}
}

// TestGitChildExists_FalseSilentlyWhenMissing verifies that the common
// non-git project case (no .git child) returns false WITHOUT a warn log.
// Logging every non-git project would flood the gateway log on a typical
// developer's projects list.
func TestGitChildExists_FalseSilentlyWhenMissing(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir() // exists, but no .git child
	var got bool
	logs := withCapturedSlog(t, func() {
		got = gitChildExists(context.Background(), tmp)
	})
	if got {
		t.Fatalf("gitChildExists = true, want false")
	}
	if strings.Contains(logs, "isGit derivation failed") {
		t.Errorf("ErrNotExist path leaked a warn log: %s", logs)
	}
}

// TestGitChildExists_FalseWithWarnOnNonNotExistError verifies the major-flag
// contract: when os.Stat fails for any reason OTHER than "does not exist"
// (e.g. ENOTDIR because the project path is a regular file, permission
// denied, I/O error), gitChildExists returns false AND emits a slog.Warn
// with the path so the operator can grep "isGit derivation failed".
//
// The ENOTDIR variant is portable across CI (no special perms needed) and
// hits the same `errors.Is(err, fs.ErrNotExist) == false` branch as
// EACCES / EIO would on real disks.
func TestGitChildExists_FalseWithWarnOnNonNotExistError(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	notDir := filepath.Join(tmp, "regular-file")
	if err := os.WriteFile(notDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Sanity: confirm the OS yields a non-NotExist error for ".git" under a
	// regular file. If a future kernel/fs reclassifies ENOTDIR as ErrNotExist
	// this test would silently pass on the happy branch — verify upfront.
	if _, err := os.Stat(filepath.Join(notDir, ".git")); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Skipf("stat under regular file did not produce a non-NotExist error: err=%v", err)
	}
	var got bool
	logs := withCapturedSlog(t, func() {
		got = gitChildExists(context.Background(), notDir)
	})
	if got {
		t.Fatalf("gitChildExists = true, want false")
	}
	if !strings.Contains(logs, "isGit derivation failed") {
		t.Errorf("non-NotExist error did not emit warn log; logs=%q", logs)
	}
	if !strings.Contains(logs, notDir) {
		t.Errorf("warn log missing project path %q; logs=%q", notDir, logs)
	}
}
