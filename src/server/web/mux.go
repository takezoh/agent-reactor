//go:build legacy_session

package web

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/termvt"
	"github.com/takezoh/agent-reactor/server/session"
)

// Sessions is the session-service surface the HTTP API needs (satisfied by
// *session.Service; a fake is used in tests).
type Sessions interface {
	Create(ctx context.Context, spec session.Spec) (session.Info, error)
	List() []session.Info
	Stop(ctx context.Context, id string) error
	Session(id string) (*termvt.Session, bool)
}

// NewMux builds the backend HTTP handler: the session REST API and the
// per-session WebSocket attach endpoint. It is a headless API — it serves no
// HTML. Any client (the web-client host, a future native client) connects here;
// the web UI is served and proxied by a separate process (client/web.Handler).
//
// Authority lives on the data plane: the REST API is guarded by the bearer
// token (Authorization header — never a URL query param); the WebSocket attach
// endpoint, which a browser cannot send headers on, is guarded by a short-lived
// single-use ticket minted over the token-authenticated API.
func NewMux(svc Sessions, token string) http.Handler {
	tickets := newTicketStore()
	mux := http.NewServeMux()

	// REST API: bearer token via Authorization header (never a query param).
	mux.Handle("/api/", TokenAuth(token, apiHandler(svc, tickets)))

	// The WebSocket attach endpoint authenticates with a single-use ticket (a
	// browser WebSocket cannot carry an Authorization header), never the bearer
	// token, so the token never appears in a URL.
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if !tickets.consume(r.URL.Query().Get("ticket")) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		serveAttach(svc, w, r)
	})
	return mux
}

// apiHandler builds the header-authenticated REST routes (session CRUD and
// WebSocket-ticket minting). Mount it under /api/ wrapped with TokenAuth.
func apiHandler(svc Sessions, tickets *ticketStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.List())
	})
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		var spec session.Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		info, err := svc.Create(r.Context(), spec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, info)
	})
	mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Stop(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ticket": tickets.mint()})
	})
	return mux
}

func serveAttach(svc Sessions, w http.ResponseWriter, r *http.Request) {
	sess, ok := svc.Session(r.URL.Query().Get("session"))
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	// Leaving InsecureSkipVerify unset enforces the default origin check (the
	// request Origin host must equal Host), which blocks cross-site WebSocket
	// hijacking from a browser. Non-browser clients send no Origin and pass.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	_ = AttachWS(r.Context(), sess, c)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
