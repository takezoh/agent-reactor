package web

import "net/http"

// contentSecurityPolicy locks the page to same-origin resources. Scripts
// (xterm.js, the app) are vendored and served from this origin, so script-src
// 'self' suffices with no CDN and no inline <script>. style-src additionally
// needs 'unsafe-inline' because both index.html's inline <style> block and
// xterm.js's runtime-injected <style> element are inline styles; this relaxes
// styles only, not script execution. connect-src 'self' confines the WebSocket
// to the same origin; frame-ancestors 'none' blocks framing (alongside the
// X-Frame-Options header, for browsers that honour only one).
const contentSecurityPolicy = "default-src 'none'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"base-uri 'none'; " +
	"form-action 'none'; " +
	"frame-ancestors 'none'"

// SecurityHeaders wraps h to set defensive response headers on every response:
//   - Referrer-Policy: no-referrer — the bearer token may sit in the page URL
//     fragment, so never leak any URL to a third party via the Referer header.
//   - Content-Security-Policy — only same-origin scripts/styles/connections,
//     closing off injected-script exfiltration and third-party script risk.
//   - X-Content-Type-Options: nosniff — honour declared content types.
//   - X-Frame-Options: DENY — disallow framing (clickjacking).
func SecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("Referrer-Policy", "no-referrer")
		hdr.Set("Content-Security-Policy", contentSecurityPolicy)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}
