package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// apiSessionInfo is the REST wire shape for one session returned by
// GET /api/sessions and POST /api/sessions. Fields: id, project (optional),
// command, created_at (RFC3339 UTC).
type apiSessionInfo struct {
	ID        string `json:"id"`
	Project   string `json:"project,omitempty"`
	Command   string `json:"command"`
	CreatedAt string `json:"created_at"`
}

// apiCreateReq is the POST /api/sessions body: project (optional), command,
// and optional terminal cols/rows packed into state.LaunchOptions.
type apiCreateReq struct {
	Project string `json:"project"`
	Command string `json:"command"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
}

// NewMux builds the backend HTTP handler. DaemonClient replaces the old
// Sessions interface; auth/ticket/CSP invariants are unchanged.
//
// Authority lives on the data plane: the REST API is guarded by the bearer
// token (Authorization header — never a URL query param); the WebSocket attach
// endpoint, which a browser cannot send headers on, is guarded by a short-lived
// single-use ticket minted over the token-authenticated API.
func NewMux(d *DaemonClient, token string) http.Handler {
	tickets := newTicketStore()
	mux := http.NewServeMux()

	// REST API: bearer token via Authorization header (never a query param).
	mux.Handle("/api/", TokenAuth(token, apiHandler(d, tickets)))

	// The WebSocket attach endpoint authenticates with a single-use ticket (a
	// browser WebSocket cannot carry an Authorization header), never the bearer
	// token, so the token never appears in a URL.
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		if !tickets.consume(r.URL.Query().Get("ticket")) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		serveAttach(d, w, r)
	})
	return mux
}

// apiHandler builds the header-authenticated REST routes (session CRUD and
// WebSocket-ticket minting). Mount it under /api/ wrapped with TokenAuth.
func apiHandler(d *DaemonClient, tickets *ticketStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", handleListSessions(d))
	mux.HandleFunc("POST /api/sessions", handleCreateSession(d))
	mux.HandleFunc("DELETE /api/sessions/{id}", handleDeleteSession(d))
	mux.HandleFunc("GET /api/sessions/{id}/transcript", handleGetTranscript(d))
	mux.HandleFunc("GET /api/sessions/{id}/event-log", handleGetEventLog(d))
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ticket": tickets.mint()})
	})
	return mux
}

func handleListSessions(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		resp, err := d.SendCommand(r.Context(), proto.CmdEvent{
			Event:   state.EventListSessions,
			Payload: json.RawMessage("{}"),
		})
		if err != nil {
			handleProtoError(w, err)
			return
		}
		rs, ok := resp.(proto.RespSessions)
		if !ok {
			http.Error(w, "unexpected response type", http.StatusInternalServerError)
			return
		}
		out := make([]apiSessionInfo, len(rs.Sessions))
		for i, s := range rs.Sessions {
			out[i] = apiSessionInfo{
				ID:        s.ID,
				Project:   s.Project,
				Command:   s.Command,
				CreatedAt: s.CreatedAt,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func handleCreateSession(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		var req apiCreateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		params := state.CreateSessionParams{
			Project: req.Project,
			Command: req.Command,
			Options: state.LaunchOptions{
				Cols: uint16(req.Cols),
				Rows: uint16(req.Rows),
			},
		}
		payload, err := json.Marshal(params)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp, err := d.SendCommand(r.Context(), proto.CmdEvent{
			Event:   state.EventCreateSession,
			Payload: json.RawMessage(payload),
		})
		if err != nil {
			handleProtoError(w, err)
			return
		}
		rc, ok := resp.(proto.RespCreateSession)
		if !ok {
			http.Error(w, "unexpected response type", http.StatusInternalServerError)
			return
		}
		info := apiSessionInfo{
			ID:        rc.SessionID,
			Project:   req.Project,
			Command:   req.Command,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		writeJSON(w, http.StatusCreated, info)
	}
}

func handleDeleteSession(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		id := r.PathValue("id")
		// ADR 0026 mandates allowlist validation on every path-parameter
		// session ID. Reject anything outside the daemon's
		// alphanumeric/underscore/hyphen vocabulary before the daemon RPC.
		if !sessionIDPattern.MatchString(id) {
			http.Error(w, "invalid session id", http.StatusBadRequest)
			return
		}
		payload, _ := json.Marshal(state.StopSessionParams{SessionID: id})
		_, err := d.SendCommand(r.Context(), proto.CmdEvent{
			Event:   state.EventStopSession,
			Payload: json.RawMessage(payload),
		})
		if err != nil {
			handleProtoError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleProtoError maps proto.ErrorBody codes to HTTP status codes.
func handleProtoError(w http.ResponseWriter, err error) {
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		switch eb.Code {
		case proto.ErrNotFound:
			http.Error(w, eb.Message, http.StatusNotFound)
		case proto.ErrInvalidArgument:
			http.Error(w, eb.Message, http.StatusBadRequest)
		default:
			http.Error(w, eb.Message, http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func serveAttach(d *DaemonClient, w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	// Leaving InsecureSkipVerify unset enforces the default origin check (the
	// request Origin host must equal Host), which blocks cross-site WebSocket
	// hijacking from a browser. Non-browser clients send no Origin and pass.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	if sessionID == "" {
		_ = AttachLifecycleWS(r.Context(), NewDaemonAdapter(d), c)
		return
	}
	_ = AttachWS(r.Context(), NewDaemonAdapter(d), sessionID, c)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
