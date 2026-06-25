package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// apiPushReq is the POST /api/sessions/{id}/push body. The web command palette
// uses this route to push a curated [session].push_commands entry onto the
// daemon-global active session (the only session a single-pane web UI can show
// at a time). The id in the URL path is REQUIRED to match the daemon-global
// active session id (see handlePushCommand) — a stale browser tab on a
// different session must not be allowed to silently retarget the active one.
type apiPushReq struct {
	Command string `json:"command"`
}

// pushBodyLimit caps the JSON body for /api/sessions/{id}/push. 64KiB is
// generous for any plausible /clear-style command picker entry but tight
// enough that a runaway client cannot exhaust gateway memory by streaming
// an unbounded body.
const pushBodyLimit = 64 * 1024

// handlePushCommand pushes a curated command onto the daemon-global active
// session via state.EventPushDriver, with strict active-session matching:
//   - 400 if the JSON body is malformed or command is empty
//   - 400 if the path id violates the session-id allowlist (ADR 0026)
//   - 404 if the path id is not a known session on the daemon
//   - 409 if the path id is known but does not match the daemon-global
//     ActiveSessionID (FR-026 / ADR-0046: stale-tab guard)
//   - 413 if the body exceeds pushBodyLimit (distinct from 400; see
//     decodePushBody)
//   - 502/504/503 per handleProtoError on RPC failure
//   - 200 on success
//
// Implementation uses ListSessions to source both the session table and the
// daemon-global ActiveSessionID in one RPC (RespSessions already carries both),
// rather than introducing a parallel "gateway snapshot" pathway. This keeps
// the handler aligned with the SendCommand-based shape every other write
// handler uses (ADR-0045) and avoids a new daemon-state cache on the gateway.
//
// TOCTOU note: the ListSessions → PushDriver pair is two separate RPCs, so a
// concurrent active-session switch on the daemon between them is observable.
// ADR-0046 ("TOCTOU note") documents why this is safe: reducePushDriver only
// validates s.Sessions[sid] and never retargets the active session, so a
// push that wins the 409 gate but lands after a daemon switch only appends
// commands to the (still-existing) target session — it never silently flips
// the active one.
func handlePushCommand(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		id := r.PathValue("id")
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id",
				"invalid session id", "id", id)
			return
		}
		command, ok := decodePushBody(w, r)
		if !ok {
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		activeID, found, ok := lookupActiveSession(ctx, d, w, r, id)
		if !ok {
			return
		}
		if !found {
			gatewayError(w, r, http.StatusNotFound, "session_not_found",
				"session not found", "id", id)
			return
		}
		if activeID != id {
			gatewayError(w, r, http.StatusConflict, "active_mismatch",
				"path session id does not match daemon-global active session",
				"want", activeID, "got", id)
			return
		}
		sendPushDriver(ctx, d, w, r, id, command)
	}
}

// decodePushBody reads the JSON body under pushBodyLimit and returns the
// trimmed command. Writes the appropriate 4xx via gatewayError and returns
// ok=false on any validation failure.
//
// Body too large is surfaced as 413 Payload Too Large (distinct from the 400
// returned for malformed JSON). http.MaxBytesReader is used over a plain
// io.LimitReader because the latter would let the json.Decoder fail with a
// generic "unexpected EOF" mid-stream — clients could not tell whether they
// had sent malformed JSON or a body that exceeded the cap. MaxBytesReader
// raises *http.MaxBytesError once the cap is hit, which we map to 413; any
// other decode error stays 400.
func decodePushBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, pushBodyLimit)
	var body apiPushReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			gatewayError(w, r, http.StatusRequestEntityTooLarge, "body_too_large",
				"request body exceeds limit",
				"limit_bytes", pushBodyLimit, "err", err)
			return "", false
		}
		gatewayError(w, r, http.StatusBadRequest, "bad_request",
			"bad request body", "err", err)
		return "", false
	}
	cmd := strings.TrimSpace(body.Command)
	if cmd == "" {
		gatewayError(w, r, http.StatusBadRequest, "empty_command",
			"command must be non-empty")
		return "", false
	}
	return cmd, true
}

// lookupActiveSession issues a ListSessions RPC and reports
// (activeID, sessionFound, ok). When ok=false the response has already been
// written. sessionFound is whether the path id appears in RespSessions.Sessions.
func lookupActiveSession(ctx context.Context, d *DaemonClient, w http.ResponseWriter, r *http.Request, id string) (string, bool, bool) {
	resp, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventListSessions,
		Payload: json.RawMessage("{}"),
	})
	if err != nil {
		handleProtoError(w, r, err)
		return "", false, false
	}
	rs, ok := resp.(proto.RespSessions)
	if !ok {
		gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
			"unexpected response type", "got_type", typeName(resp))
		return "", false, false
	}
	found := false
	for _, s := range rs.Sessions {
		if s.ID == id {
			found = true
			break
		}
	}
	return rs.ActiveSessionID, found, true
}

// sendPushDriver dispatches state.EventPushDriver to the daemon. Writes 502
// on RPC failure (handleProtoError) and 200 on success.
func sendPushDriver(ctx context.Context, d *DaemonClient, w http.ResponseWriter, r *http.Request, id, command string) {
	payload, err := json.Marshal(state.PushDriverParams{
		SessionID: id,
		Command:   command,
	})
	if err != nil {
		gatewayError(w, r, http.StatusInternalServerError, "marshal_error",
			"internal error", "err", err)
		return
	}
	if _, err := d.SendCommand(ctx, proto.CmdEvent{
		Event:   state.EventPushDriver,
		Payload: json.RawMessage(payload),
	}); err != nil {
		handleProtoError(w, r, err)
		return
	}
	_ = requestID(w, r)
	w.WriteHeader(http.StatusOK)
}
