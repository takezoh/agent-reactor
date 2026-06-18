package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/server/session"
)

const testToken = "test-token"

func TestMuxCreateListStop(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	mux := NewMux(svc, fstest.MapFS{}, testToken)

	info := decodeInfo(t, doReq(t, mux, http.MethodPost, "/api/sessions",
		`{"command":"sleep 5"}`, http.StatusCreated))
	if info.ID == "" {
		t.Fatal("create returned empty id")
	}

	listBody := doReq(t, mux, http.MethodGet, "/api/sessions", "", http.StatusOK)
	var list []session.Info
	if err := json.Unmarshal([]byte(listBody), &list); err != nil || len(list) != 1 {
		t.Fatalf("list = %q (err %v)", listBody, err)
	}

	doReq(t, mux, http.MethodDelete, "/api/sessions/"+info.ID, "", http.StatusNoContent)
	doReq(t, mux, http.MethodDelete, "/api/sessions/"+info.ID, "", http.StatusNotFound)
}

func TestMuxCreateBadCommand(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	mux := NewMux(svc, fstest.MapFS{}, testToken)
	doReq(t, mux, http.MethodPost, "/api/sessions", `{"command":""}`, http.StatusBadRequest)
}

// TestMuxRequiresToken confirms the REST API is unreachable without the bearer
// header.
func TestMuxRequiresToken(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	mux := NewMux(svc, fstest.MapFS{}, testToken)

	r := httptest.NewRequest(http.MethodGet, "/api/sessions", nil) // no Authorization
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list = %d, want 401", w.Code)
	}
}

// TestMuxStaticPublic confirms the static shell loads without auth: a browser
// navigating to the page cannot send an Authorization header (the token lives
// in the fragment), so gating the shell would deadlock the bootstrap. The shell
// holds no secrets; authority is on /api and /ws.
func TestMuxStaticPublic(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	assets := fstest.MapFS{
		"index.html":      {Data: []byte("<html>shell</html>")},
		"vendor/xterm.js": {Data: []byte("//js")},
	}
	mux := NewMux(svc, assets, testToken)

	r := httptest.NewRequest(http.MethodGet, "/", nil) // no Authorization
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("static shell GET / = %d, want 200 (must serve without auth)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "shell") {
		t.Fatalf("static shell body = %q", w.Body.String())
	}

	// A vendored file is served, but the directory autoindex is suppressed.
	if got := staticGet(t, mux, "/vendor/xterm.js"); got != http.StatusOK {
		t.Fatalf("GET /vendor/xterm.js = %d, want 200", got)
	}
	if got := staticGet(t, mux, "/vendor/"); got != http.StatusNotFound {
		t.Fatalf("GET /vendor/ = %d, want 404 (no directory listing)", got)
	}
}

func staticGet(t *testing.T, h http.Handler, path string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// TestMuxWSTicketAuth exercises the ticket-gated WebSocket attach: no ticket is
// rejected, a minted ticket attaches once, the ticket cannot be reused, and a
// cross-origin handshake is rejected even with a fresh ticket (CSWSH defense).
func TestMuxWSTicketAuth(t *testing.T) {
	svc := session.NewService(agentlaunch.DirectDispatcher{})
	defer svc.CloseAll(context.Background())
	srv := httptest.NewServer(NewMux(svc, fstest.MapFS{}, testToken))
	defer srv.Close()

	info := createSession(t, srv, "cat")
	q := "session=" + info.ID

	// Each row pins a specific defense via its rejection status: missing/reused
	// ticket → 401 (our gate), foreign Origin → 403 (websocket origin check).
	dialWS(t, srv, q, nil, http.StatusUnauthorized)                         // no ticket
	ticket := mintTicket(t, srv)                                            //
	dialWS(t, srv, q+"&ticket="+ticket, nil, http.StatusSwitchingProtocols) // fresh → attach
	dialWS(t, srv, q+"&ticket="+ticket, nil, http.StatusUnauthorized)       // reused → reject
	dialWS(t, srv, q+"&ticket="+mintTicket(t, srv),                         // foreign Origin → reject
		http.Header{"Origin": {"http://evil.example"}}, http.StatusForbidden)
}

func createSession(t *testing.T, srv *httptest.Server, command string) session.Info {
	t.Helper()
	body := strings.NewReader(`{"command":"` + command + `"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/sessions", body)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session status = %d", resp.StatusCode)
	}
	var info session.Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode session info: %v", err)
	}
	return info
}

func mintTicket(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("mint ticket: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mint ticket status = %d", resp.StatusCode)
	}
	var out struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Ticket == "" {
		t.Fatalf("decode ticket: %v (ticket %q)", err, out.Ticket)
	}
	return out.Ticket
}

// dialWS dials /ws and asserts the handshake outcome. wantStatus ==
// http.StatusSwitchingProtocols (101) means the upgrade must succeed; any other
// status means the handshake must be rejected with exactly that HTTP status.
func dialWS(t *testing.T, srv *httptest.Server, query string, hdr http.Header, wantStatus int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?" + query
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if wantStatus == http.StatusSwitchingProtocols {
		if err != nil {
			t.Fatalf("dial %q: want success, got %v", query, err)
		}
		_ = c.CloseNow()
		return
	}
	if err == nil {
		_ = c.CloseNow()
		t.Fatalf("dial %q: want rejection %d, got success", query, wantStatus)
	}
	if resp == nil {
		t.Fatalf("dial %q: want rejection %d, got error without response: %v", query, wantStatus, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("dial %q: rejection status = %d, want %d", query, resp.StatusCode, wantStatus)
	}
}

func doReq(t *testing.T, h http.Handler, method, path, body string, want int) string {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: status = %d, want %d (body %q)", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.String()
}

func decodeInfo(t *testing.T, body string) session.Info {
	t.Helper()
	var info session.Info
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("decode info: %v (body %q)", err, body)
	}
	return info
}
