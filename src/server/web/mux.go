package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// isAbsoluteProjectPath reports whether p is suitable as a session project
// directory. The daemon and its downstream launchers (devcontainer, direct
// fork) treat req.Project as the working directory; a non-absolute value
// reaches docker create as workdir and fails with a confusing message
// ("the working directory 'foo' is invalid, it needs to be an absolute
// path") that surfaces as an opaque 502 from inside the launcher chain.
// Guard at the gateway boundary so the operator sees a 400 stating the
// rule, not a 502 mentioning docker internals.
//
// Empty is rejected (project is required); the daemon also enforces this,
// but rejecting earlier keeps the error closer to the input.
func isAbsoluteProjectPath(p string) bool {
	if p == "" {
		return false
	}
	return filepath.IsAbs(p)
}

// daemonRPCTimeout caps how long an /api/* request waits on the daemon. A
// wedged daemon (e.g. event loop stuck) would otherwise hang the HTTP request
// forever, exhausting the gateway's worker pool and propagating the wedge to
// every browser tab. With a bound, the gateway returns 504 quickly and the
// browser can recover; without one, a single wedged daemon takes down the
// gateway too. 10s is generous for normal RPC (single-digit ms) but short
// enough that users notice and can recover.
const daemonRPCTimeout = 10 * time.Second

// rpcTimeoutOverride is set ONLY by tests (via withShortRPCTimeout) so the
// timeout assertions don't pay 10s × N wall clock. Zero means "use the const
// above". Production must never write this — there is no flag wiring.
//
// atomic.Int64 (not a plain time.Duration) because parallel timeout tests
// write to it from one goroutine while concurrent request goroutines read
// it from rpcContext on the parallel-running unit tests in the same package.
var rpcTimeoutOverride atomic.Int64

// rpcContext returns a context derived from r.Context() with daemonRPCTimeout
// applied. Callers must defer cancel().
func rpcContext(r *http.Request) (context.Context, context.CancelFunc) {
	d := daemonRPCTimeout
	if override := time.Duration(rpcTimeoutOverride.Load()); override > 0 {
		d = override
	}
	return context.WithTimeout(r.Context(), d)
}

// apiSessionInfo is the REST wire shape for one session returned by
// GET /api/sessions and POST /api/sessions. Fields: id, project (optional),
// command, created_at (RFC3339 UTC).
type apiSessionInfo struct {
	ID        string `json:"id"`
	Project   string `json:"project,omitempty"`
	Command   string `json:"command"`
	CreatedAt string `json:"created_at"`
}

// apiSessionConfig is the REST wire shape for GET /api/session-config. It
// surfaces the subset of the user's settings.toml the create-session form
// needs to render: the curated [session].commands list (the same picker the
// TUI palette uses), the [session].default_command, and the resolved
// [projects] list (project_roots fan-out + project_paths). Sourcing these
// from config rather than hardcoding driver names in the UI keeps web and
// TUI on the same source of truth and lets the user customize both without
// rebuilding.
type apiSessionConfig struct {
	DefaultCommand string   `json:"default_command"`
	Commands       []string `json:"commands"`
	Projects       []string `json:"projects"`
}

// apiCreateReq is the POST /api/sessions body. Required: project (absolute
// path) and command. Optional: terminal cols/rows hint, worktree (create a
// git worktree before launch), and sandbox ("" / "auto" → follow project
// config, "host" → force direct/host launch, same vocabulary the TUI
// palette uses).
type apiCreateReq struct {
	Project  string `json:"project"`
	Command  string `json:"command"`
	Cols     int    `json:"cols,omitempty"`
	Rows     int    `json:"rows,omitempty"`
	Worktree bool   `json:"worktree,omitempty"`
	Sandbox  string `json:"sandbox,omitempty"`
}

// parseSandbox maps the apiCreateReq sandbox field to the daemon's
// SandboxOverride enum. "" and "auto" both mean "follow project config"
// (SandboxOverrideAuto). "host" forces SandboxOverrideHost (the TUI
// palette's "host" / sandbox=direct toggle). Anything else returns ok=false
// so the gateway can 400 the request rather than silently degrading.
func parseSandbox(v string) (state.SandboxOverride, bool) {
	switch v {
	case "", "auto":
		return state.SandboxOverrideAuto, true
	case "host":
		return state.SandboxOverrideHost, true
	}
	return 0, false
}

// NewMux builds the backend HTTP handler. DaemonClient replaces the old
// Sessions interface; auth/ticket/CSP invariants are unchanged.
//
// Authority lives on the data plane: the REST API is guarded by the bearer
// token (Authorization header — never a URL query param); the WebSocket attach
// endpoint, which a browser cannot send headers on, is guarded by a short-lived
// single-use ticket minted over the token-authenticated API.
func NewMux(d *DaemonClient, token string) http.Handler {
	return newMux(d, token, false)
}

// NewMuxNoAuth builds the backend HTTP handler with bearer-token AND WS-ticket
// checks disabled. Intended for local dev only (scripts/run-dev.sh) on a
// loopback bind. Production callers MUST use NewMux.
func NewMuxNoAuth(d *DaemonClient) http.Handler {
	return newMux(d, "", true)
}

func newMux(d *DaemonClient, token string, noAuth bool) http.Handler {
	tickets := newTicketStore()
	mux := http.NewServeMux()

	// REST API: bearer token via Authorization header (never a query param).
	// In no-auth mode the API handler is mounted directly — the TokenAuth
	// "empty want rejects everything" contract is preserved on the path that
	// still uses it.
	if noAuth {
		mux.Handle("/api/", apiHandler(d, tickets))
	} else {
		mux.Handle("/api/", TokenAuth(token, apiHandler(d, tickets)))
	}

	// The WebSocket attach endpoint authenticates with a single-use ticket (a
	// browser WebSocket cannot carry an Authorization header), never the bearer
	// token, so the token never appears in a URL.
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			http.Error(w, "daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		if !noAuth && !tickets.consume(r.URL.Query().Get("ticket")) {
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
	mux.HandleFunc("GET /api/session-config", handleSessionConfig())
	mux.HandleFunc("POST /api/ws-ticket", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ticket": tickets.mint()})
	})
	return mux
}

// loadSessionConfig is the package-level hook used by handleSessionConfig.
// Tests swap it via withSessionConfigLoader so they don't have to lay down a
// real ~/.agent-reactor/settings.toml on disk. Production uses config.Load,
// which reads from $HOME/.agent-reactor/settings.toml (same file the daemon
// reads — see client/config.ConfigDirPath); missing-file is non-fatal and
// returns DefaultConfig().
var loadSessionConfig = config.Load

// withSessionConfigLoader installs a test-only loader and returns a restore
// func. Production code never touches this; declared in mux.go (not _test.go)
// because var assignment from _test.go can race the production reader on
// parallel tests when init order is non-deterministic.
func withSessionConfigLoader(loader func() (*config.Config, error)) func() {
	prev := loadSessionConfig
	loadSessionConfig = loader
	return func() { loadSessionConfig = prev }
}

// handleSessionConfig exposes the config-derived inputs the create-session
// form needs: default_command (initial select value), commands (the
// [session].commands picker entries — same source the TUI palette uses), and
// projects (the resolved [projects] list for the project directory datalist).
//
// Reads config on every request so the user can edit settings.toml and see
// updates without restarting either the daemon or the gateway. The config
// file is read by the gateway directly because (a) it's a static user-level
// file at a stable path, not daemon-managed state, and (b) sending the same
// data through the daemon RPC would duplicate work the gateway already has
// filesystem access to. The daemon-RPC boundary (ADR 0016) is about session
// state, not user config.
//
// On config-load error, returns 500 so a malformed settings.toml is visibly
// failing rather than silently surfacing as an empty form. A missing config
// file is NOT an error (LoadFrom treats it as DefaultConfig).
func handleSessionConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := loadSessionConfig()
		if err != nil {
			gatewayError(w, r, http.StatusInternalServerError, "config_load_failed",
				"settings.toml load failed", "err", err)
			return
		}
		commands := cfg.Session.Commands
		if commands == nil {
			commands = []string{}
		}
		projects := cfg.ListProjects()
		if projects == nil {
			projects = []string{}
		}
		writeJSON(w, http.StatusOK, apiSessionConfig{
			DefaultCommand: cfg.Session.DefaultCommand,
			Commands:       commands,
			Projects:       projects,
		})
	}
}

func handleListSessions(d *DaemonClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.Health() {
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		resp, err := d.SendCommand(ctx, proto.CmdEvent{
			Event:   state.EventListSessions,
			Payload: json.RawMessage("{}"),
		})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		rs, ok := resp.(proto.RespSessions)
		if !ok {
			gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
				"unexpected response type", "got_type", typeName(resp))
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
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		var req apiCreateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gatewayError(w, r, http.StatusBadRequest, "bad_request",
				"bad request body", "err", err)
			return
		}
		// Reject relative-path "projects" at the gateway boundary. The daemon
		// hands req.Project straight to drivers / devcontainer launchers as
		// the working directory; docker create rejects non-absolute workdirs
		// with "the working directory 'X' is invalid, it needs to be an
		// absolute path", which used to surface as an opaque 502 from deep
		// inside the launcher chain. Catching it here turns the operator
		// mistake into a clean 400 with the actual rule stated.
		if !isAbsoluteProjectPath(req.Project) {
			gatewayError(w, r, http.StatusBadRequest, "project_not_absolute",
				"project must be an absolute path (e.g. /home/me/myrepo); got "+strconv.Quote(req.Project),
				"project", req.Project)
			return
		}
		sandbox, ok := parseSandbox(req.Sandbox)
		if !ok {
			gatewayError(w, r, http.StatusBadRequest, "invalid_sandbox",
				"sandbox must be one of \"\"/\"auto\"/\"host\"; got "+strconv.Quote(req.Sandbox),
				"sandbox", req.Sandbox)
			return
		}
		params := state.CreateSessionParams{
			Project: req.Project,
			Command: req.Command,
			Sandbox: sandbox,
			Options: state.LaunchOptions{
				Cols:     uint16(req.Cols),
				Rows:     uint16(req.Rows),
				Worktree: state.WorktreeOption{Enabled: req.Worktree},
			},
		}
		payload, err := json.Marshal(params)
		if err != nil {
			// json.Marshal of a struct of primitives cannot fail — this is
			// defensive only. Surface as 500 with the err so a future
			// type-change that breaks marshalling is visible.
			gatewayError(w, r, http.StatusInternalServerError, "marshal_error",
				"internal error", "err", err)
			return
		}
		ctx, cancel := rpcContext(r)
		defer cancel()
		resp, err := d.SendCommand(ctx, proto.CmdEvent{
			Event:   state.EventCreateSession,
			Payload: json.RawMessage(payload),
		})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		rc, ok := resp.(proto.RespCreateSession)
		if !ok {
			gatewayError(w, r, http.StatusInternalServerError, "response_type_mismatch",
				"unexpected response type", "got_type", typeName(resp))
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
			gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
				"daemon unavailable")
			return
		}
		id := r.PathValue("id")
		// ADR 0026 mandates allowlist validation on every path-parameter
		// session ID. Reject anything outside the daemon's
		// alphanumeric/underscore/hyphen vocabulary before the daemon RPC.
		if !sessionIDPattern.MatchString(id) {
			gatewayError(w, r, http.StatusBadRequest, "invalid_session_id",
				"invalid session id", "id", id)
			return
		}
		payload, _ := json.Marshal(state.StopSessionParams{SessionID: id})
		ctx, cancel := rpcContext(r)
		defer cancel()
		_, err := d.SendCommand(ctx, proto.CmdEvent{
			Event:   state.EventStopSession,
			Payload: json.RawMessage(payload),
		})
		if err != nil {
			handleProtoError(w, r, err)
			return
		}
		_ = requestID(w, r) // set X-Request-Id on the 204 too
		w.WriteHeader(http.StatusNoContent)
	}
}

// typeName returns the Go type name of v as a string, used for diagnostic
// logging when a response type assertion fails (so we can see what the
// daemon actually returned without dumping the full payload).
func typeName(v any) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", v)
}

// requestID returns the X-Request-Id from the request if the client supplied
// one (browsers don't, but proxies and tests might); otherwise generates a new
// 16-hex-char id. The returned id is also written into the response header so
// the client sees the value that the server-side slog lines use.
func requestID(w http.ResponseWriter, r *http.Request) string {
	id := r.Header.Get("X-Request-Id")
	if id == "" {
		var b [8]byte
		_, _ = rand.Read(b[:])
		id = hex.EncodeToString(b[:])
	}
	w.Header().Set("X-Request-Id", id)
	return id
}

// gatewayError is the single 4xx/5xx exit for every gateway handler. It
// writes the response, logs context, and embeds the request-id in the
// response body so a screenshot of the browser response is enough to grep
// the server log.
//
// status: the HTTP status to return.
// reason: a short stable token used for log grouping ("daemon_timeout",
//
//	"spawn_failed", "auth_missing", …). Goes into slog as `reason=...`.
//
// detail: human-readable explanation; written to the response body, NOT
//
//	the slog message (to keep the log line short).
//
// fields: additional slog key/value pairs. Always include "err" when the
//
//	error came from upstream / SDK.
//
// 4xx is logged at Info; 5xx at Error. 5xx is what the user grep'd for in
// the original "/api/sessions returned 500" complaint — every such 500 is
// now identifiable by its (request_id, reason) tuple in server.log.
func gatewayError(w http.ResponseWriter, r *http.Request, status int, reason, detail string, fields ...any) {
	rid := requestID(w, r)
	level := slog.LevelInfo
	if status >= 500 {
		level = slog.LevelError
	}
	attrs := []any{
		"request_id", rid,
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"reason", reason,
	}
	attrs = append(attrs, fields...)
	slog.Log(r.Context(), level, "gateway: response", attrs...)
	body := detail + " (request_id=" + rid + ")"
	http.Error(w, body, status)
}

// handleProtoError translates a daemon RPC error into an HTTP response.
// Every proto.ErrCode is mapped explicitly; an unknown code logs at Error
// and returns 500 so a daemon-side code we forgot to handle is visibly
// failing rather than silently surfacing as a generic 500.
//
// Mapping rationale:
//   - context.DeadlineExceeded → 504: a wedged daemon would otherwise hang
//     the request forever. The 504 is recoverable; the browser can retry.
//   - context.Canceled → 504: client gave up, same shape for callers.
//   - ErrDaemonUnavailable → 503: socket disconnected mid-request; transient.
//   - proto.ErrNotFound → 404: the resource didn't exist on the daemon.
//   - proto.ErrInvalidArgument → 400: client sent bad data.
//   - proto.ErrUnsupported → 422: the request is structurally valid but the
//     daemon can't satisfy it (e.g. no driver registered for the command).
//     422 (Unprocessable Entity) distinguishes "wrong shape" (400) from
//     "right shape, wrong target".
//   - proto.ErrAlreadyExists → 409: standard idempotency-conflict status.
//   - proto.ErrSessionStopped → 410 Gone: the session existed but has shut
//     down; the resource is permanently unavailable in its previous form.
//   - proto.ErrFrameNotReady → 503: the surface isn't ready yet; the
//     React store retries on 503 with backoff (ADR 0018).
//   - proto.ErrInternal → 502 Bad Gateway: this is an upstream daemon
//     failure (the daemon ran but its operation failed — typically a
//     spawn failure). 502 is the precise gateway-pattern status; 500
//     would be wrong because the gateway itself didn't fail.
//   - proto.ErrUnknown → 502: unrecognized but proto-typed; still
//     upstream-attributed.
//   - non-proto, non-context error → 502 with the underlying error
//     surfaced in the body. This is also typically a transport error
//     (e.g. socket write failure mid-RPC), still upstream-attributable.
//
// reason= tokens are stable for log grouping. Do not rename them without
// updating any external log alerts that depend on them.
func handleProtoError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		gatewayError(w, r, http.StatusGatewayTimeout, "daemon_timeout",
			"daemon timeout", "err", err)
		return
	case errors.Is(err, context.Canceled):
		gatewayError(w, r, http.StatusGatewayTimeout, "client_cancelled",
			"client cancelled", "err", err)
		return
	case errors.Is(err, ErrDaemonUnavailable):
		gatewayError(w, r, http.StatusServiceUnavailable, "daemon_unavailable",
			"daemon unavailable", "err", err)
		return
	}
	var eb *proto.ErrorBody
	if errors.As(err, &eb) {
		status, reason := protoCodeToHTTP(eb.Code)
		gatewayError(w, r, status, reason, eb.Message, "err_code", string(eb.Code), "err_msg", eb.Message)
		return
	}
	gatewayError(w, r, http.StatusBadGateway, "upstream_error",
		"upstream error: "+err.Error(), "err", err)
}

// protoCodeToHTTP is the exhaustive code → (status, reason) mapping.
// Keeping it pure makes it trivially unit-testable; new ErrCode values
// require an entry here so a `default` 500 never silently swallows them.
func protoCodeToHTTP(code proto.ErrCode) (int, string) {
	switch code {
	case proto.ErrNotFound:
		return http.StatusNotFound, "not_found"
	case proto.ErrInvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case proto.ErrUnsupported:
		return http.StatusUnprocessableEntity, "unsupported"
	case proto.ErrAlreadyExists:
		return http.StatusConflict, "already_exists"
	case proto.ErrSessionStopped:
		return http.StatusGone, "session_stopped"
	case proto.ErrFrameNotReady:
		return http.StatusServiceUnavailable, "frame_not_ready"
	case proto.ErrInternal:
		// Upstream daemon error (typically "tmux spawn failed: …").
		// 502 Bad Gateway is the precise status — the gateway is healthy,
		// the upstream daemon's operation failed. The user's original
		// "POST /api/sessions returned 500" complaint was this code: a
		// daemon-side spawn failure being miscategorized as a gateway bug.
		return http.StatusBadGateway, "daemon_internal"
	case proto.ErrUnknown:
		return http.StatusBadGateway, "daemon_unknown"
	}
	// New ErrCode added in proto but not mapped here — surface it as 500
	// (gateway bug, not upstream) so the omission is visible in logs.
	return http.StatusInternalServerError, "unmapped_code"
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
