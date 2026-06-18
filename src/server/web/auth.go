package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// TokenAuth wraps h, requiring a bearer token equal to want, supplied as
// "Authorization: Bearer <token>". An empty want rejects every request, so a
// token must always be configured.
//
// The token is deliberately NOT accepted via a URL query parameter: query
// strings leak into server access logs, browser history, and Referer headers.
// Browser WebSocket connections — which cannot set request headers — instead
// authenticate with a short-lived, single-use ticket (see ticketStore) minted
// over this header-authenticated API.
func TokenAuth(want string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want == "" || !tokenOK(r, want) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func tokenOK(r *http.Request, want string) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	got := strings.TrimPrefix(h, "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
