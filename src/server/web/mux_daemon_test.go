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

	"github.com/takezoh/agent-reactor/client/config"
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
// CreateSessionParams.Options.Worktree.Enabled and Sandbox=SandboxOverrideHost
// (the same vocabulary the TUI palette's worktree/host toggles use).
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
		cfg.Projects.ProjectPaths = []string{"/home/me/repo-a", "/home/me/repo-b"}
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
	// ProjectPaths only land if the referenced dirs exist (listProjectsFrom
	// stats them); since the test paths are fake, expect an empty list rather
	// than the raw config values. The point of this assertion is that the
	// field is JSON-encoded as an array (not null) so the UI's
	// projects.map(...) doesn't blow up.
	if got.Projects == nil {
		t.Errorf("Projects = nil, want non-nil array")
	}
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
