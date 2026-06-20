package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	wantHdr := map[string]string{
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}
	for k, v := range wantHdr {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	// Pin each load-bearing CSP directive EXACTLY: a regression that loosens
	// script-src/connect-src/default-src (adds a host or 'unsafe-inline'/
	// 'unsafe-eval') must fail. A brittle substring check would let
	// `script-src 'self' https://x` slip by.
	dirs := cspDirectives(w.Header().Get("Content-Security-Policy"))
	wantCSP := map[string]string{
		"default-src":     "'none'",
		"script-src":      "'self'",
		"style-src":       "'self' 'unsafe-inline'",
		"connect-src":     "'self'",
		"base-uri":        "'none'",
		"form-action":     "'none'",
		"frame-ancestors": "'none'",
	}
	for name, val := range wantCSP {
		if dirs[name] != val {
			t.Errorf("CSP %s = %q, want %q (full: %q)",
				name, dirs[name], val, w.Header().Get("Content-Security-Policy"))
		}
	}
}

// cspDirectives parses a CSP header into directive-name → space-joined sources.
func cspDirectives(csp string) map[string]string {
	out := map[string]string{}
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		name, val, _ := strings.Cut(d, " ")
		out[name] = strings.TrimSpace(val)
	}
	return out
}

// TestStaticServesShellHidesListing checks the embedded UI is served and that
// directory autoindex is suppressed. backend is unused for static.
func TestStaticServesShellHidesListing(t *testing.T) {
	h, err := Handler("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	if code, body := get(t, srv.URL+"/"); code != http.StatusOK || !strings.Contains(body, "<html") {
		t.Fatalf("GET / = %d body %q, want 200 HTML shell", code, body)
	}
	// Vite places bundled JS under /assets/; a trailing-slash request to that
	// directory must return 404 (no directory listing).
	if code, _ := get(t, srv.URL+"/assets/"); code != http.StatusNotFound {
		t.Fatalf("GET /assets/ = %d, want 404 (no directory listing)", code)
	}
}

// TestProxyForwardsAPI confirms /api is forwarded to the backend with the
// Authorization header preserved and the Origin header stripped.
func TestProxyForwardsAPI(t *testing.T) {
	var gotAuth, gotOrigin, gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotOrigin = r.Header.Get("Origin")
		gotPath = r.URL.Path
		io.WriteString(w, "ok")
	}))
	defer backend.Close()

	h, err := Handler(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer tkn")
	req.Header.Set("Origin", srv.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if gotPath != "/api/sessions" {
		t.Errorf("backend path = %q, want /api/sessions", gotPath)
	}
	if gotAuth != "Bearer tkn" {
		t.Errorf("backend Authorization = %q, want forwarded bearer", gotAuth)
	}
	if gotOrigin != "" {
		t.Errorf("backend Origin = %q, want stripped", gotOrigin)
	}
	// The host wraps every route (including the proxied API) in SecurityHeaders.
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("proxied /api response missing X-Content-Type-Options: nosniff")
	}
}

// TestWSProxyHandshake drives a real WebSocket upgrade through the host proxy to
// a backend ws endpoint and back — proving the core attach path survives the
// proxy hop (the Origin-stripping + Rewrite must not break the upgrade).
func TestWSProxyHandshake(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil) // Origin stripped by the proxy → allowed
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		_ = c.Write(r.Context(), websocket.MessageText, data) // echo
	}))
	defer backend.Close()

	h, err := Handler(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?session=x"
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {srv.URL}}, // same-origin → passes the host gate
	})
	if err != nil {
		t.Fatalf("ws dial through proxy: %v", err)
	}
	defer func() { _ = c.CloseNow() }()

	if err := c.Write(ctx, websocket.MessageText, []byte("ping-proxy")); err != nil {
		t.Fatal(err)
	}
	_, got, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ping-proxy" {
		t.Fatalf("echo through proxy = %q, want ping-proxy", got)
	}
}

// TestWSOriginCheck confirms a cross-origin WebSocket request is rejected at the
// host (before proxying), while a same-origin one is forwarded to the backend.
func TestWSOriginCheck(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "reached-backend")
	}))
	defer backend.Close()

	h, err := Handler(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	// Foreign Origin → rejected by the host, never proxied.
	if code, _ := getOrigin(t, srv.URL+"/ws?session=x", "http://evil.example"); code != http.StatusForbidden {
		t.Fatalf("cross-origin /ws = %d, want 403", code)
	}
	// Same Origin → proxied to the backend.
	if code, body := getOrigin(t, srv.URL+"/ws?session=x", srv.URL); code != http.StatusOK || body != "reached-backend" {
		t.Fatalf("same-origin /ws = %d body %q, want 200 reached-backend", code, body)
	}
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	return getOrigin(t, url, "")
}

func getOrigin(t *testing.T, url, origin string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}
