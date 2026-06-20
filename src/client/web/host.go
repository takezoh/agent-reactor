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
	if p := target.Path; p != "" && p != "/" {
		// SetURL would prefix this path onto every proxied request, 404-ing the
		// whole API silently; reject it at startup instead.
		return nil, fmt.Errorf("web: backend URL must not include a path: %q", backendURL)
	}
	proxy := backendProxy(target)

	distFS, err := DistFS()
	if err != nil {
		return nil, fmt.Errorf("web: embed sub-fs: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler(distFS)) // static UI shell
	mux.Handle("/api/", proxy)             // REST: the backend authenticates the bearer token
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		// This host is the browser-facing CSWSH gate: enforce same-origin before
		// proxying. The backend then sees a header-less, trusted hop (backendProxy
		// strips Origin), so this check is the effective anti-CSWSH defense.
		if !sameOrigin(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	// Defensive headers on every response: the page needs the CSP, and the
	// proxied /api responses still want X-Content-Type-Options / Referrer-Policy.
	return SecurityHeaders(mux), nil
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
// is allowed (it still faces the backend's ticket gate). Same rule the backend's
// coder/websocket check uses, applied here because this host fronts the browser.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// staticHandler serves the embedded UI assets but suppresses directory autoindex
// listings (e.g. /vendor/): a bare http.FileServer would list the embedded
// directory, which is needless attack surface, so directory paths 404 and only
// files are served. The trailing-slash branch is the deliberate guard, not boilerplate.
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
