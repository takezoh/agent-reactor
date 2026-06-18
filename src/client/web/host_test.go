package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	want := map[string]string{
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}
	for k, v := range want {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "cdn.") {
		t.Errorf("CSP must pin scripts to 'self' with no CDN: %q", csp)
	}
}

// TestStaticServesShellHidesListing checks the embedded UI is served and the
// /vendor/ directory autoindex is suppressed. backend is unused for static.
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
	if code, _ := get(t, srv.URL+"/app.js"); code != http.StatusOK {
		t.Fatalf("GET /app.js = %d, want 200", code)
	}
	if code, _ := get(t, srv.URL+"/vendor/"); code != http.StatusNotFound {
		t.Fatalf("GET /vendor/ = %d, want 404 (no directory listing)", code)
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
