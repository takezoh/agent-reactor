package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Handler builds the web-client host: it serves the embedded browser UI and
// reverse-proxies the data plane (/api, /ws) to the backend at backendURL. The
// browser therefore talks only to this origin — same-origin — so the strict CSP
// and the WebSocket origin check below hold, while the backend stays a headless
// API that any client (this host, a future native client) connects to.
func Handler(backendURL string) (http.Handler, error) {
	target, err := url.Parse(backendURL)
	if err != nil || target.Host == "" {
		return nil, fmt.Errorf("web: invalid backend URL %q: %w", backendURL, err)
	}
	proxy := backendProxy(target)

	mux := http.NewServeMux()
	// Static UI shell (HTML/JS/vendored xterm), served with the page CSP.
	mux.Handle("/", SecurityHeaders(staticHandler(Assets)))
	// REST API: forwarded as-is; the backend authenticates the bearer token.
	mux.Handle("/api/", proxy)
	// WebSocket attach: enforce same-origin here (the browser-facing CSWSH
	// guard) before proxying — the backend sees a header-less, trusted hop.
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	return mux, nil
}

// backendProxy reverse-proxies to target. It routes to the backend host and
// drops the Origin header: the origin check is enforced at the /ws route, and
// the backend treats a header-less request as a trusted non-browser client.
func backendProxy(target *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Origin")
		},
	}
}

// sameOrigin reports whether r may open a WebSocket: a browser request must
// carry an Origin whose host equals this host; a non-browser client (no Origin)
// is allowed. Mirrors the backend's coder/websocket origin check.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// staticHandler serves the embedded UI assets but suppresses directory autoindex
// listings (e.g. /vendor/): only files are served, directory paths 404.
func staticHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
