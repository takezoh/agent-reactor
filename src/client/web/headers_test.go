package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCSPHeaders ensures that the CSP set by SecurityHeaders keeps script-src
// 'self' and never includes 'unsafe-inline' in the script-src directive.
// Regression guard for FR-β03 / FR-β13.
func TestCSPHeaders(t *testing.T) {
	t.Parallel()

	// Wrap a no-op handler with the security middleware.
	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := SecurityHeaders(noop)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf("Content-Security-Policy header missing")
	}

	// Must contain script-src directive.
	if !strings.Contains(csp, "script-src") {
		t.Errorf("CSP must contain script-src directive, got: %q", csp)
	}

	// script-src must include 'self'.
	if !strings.Contains(csp, "'self'") {
		t.Errorf("CSP script-src must include 'self', got: %q", csp)
	}

	// script-src must NOT contain 'unsafe-inline'.
	// Note: style-src intentionally includes 'unsafe-inline' for xterm.js inline
	// styles, but that must never bleed into the script-src directive.
	scriptSrc := extractDirective(csp, "script-src")
	if strings.Contains(scriptSrc, "unsafe-inline") {
		t.Errorf("CSP script-src must not contain 'unsafe-inline' (FR-β03 violation), full CSP: %q", csp)
	}

	// script-src must NOT contain 'unsafe-eval'.
	if strings.Contains(scriptSrc, "unsafe-eval") {
		t.Errorf("CSP script-src must not contain 'unsafe-eval', full CSP: %q", csp)
	}
}

// extractDirective returns the value of the named CSP directive (everything up
// to the next semicolon), or the empty string if not found.
func extractDirective(csp, directive string) string {
	lower := strings.ToLower(csp)
	start := strings.Index(lower, directive)
	if start == -1 {
		return ""
	}
	tail := csp[start:]
	if end := strings.Index(tail, ";"); end != -1 {
		return tail[:end]
	}
	return tail
}
