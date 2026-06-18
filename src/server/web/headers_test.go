package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (headers must not block the response)", w.Code)
	}
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

	// Pin the load-bearing CSP directives EXACTLY (parse, don't substring-match):
	// a regression that loosens script-src/connect-src/default-src — e.g. adds a
	// host or 'unsafe-inline'/'unsafe-eval' — must fail this test. A brittle
	// adjacency substring check would let `script-src 'self' https://x` slip by.
	dirs := cspDirectives(w.Header().Get("Content-Security-Policy"))
	want := map[string]string{
		"default-src":     "'none'",
		"script-src":      "'self'", // exactly self: no CDN, no inline, no eval
		"style-src":       "'self' 'unsafe-inline'",
		"connect-src":     "'self'",
		"base-uri":        "'none'",
		"form-action":     "'none'",
		"frame-ancestors": "'none'",
	}
	for name, val := range want {
		if dirs[name] != val {
			t.Errorf("CSP directive %s = %q, want %q (full: %q)",
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
