// Package fake implements an in-process high-fidelity fake of the codex
// app-server plus a fake `codex --remote` CLI. The fakes reproduce the
// protocol behaviour that matters for the stream Backend's routing
// correctness — most importantly, that the app-server broadcasts thread
// notifications to *every* connected client, which is what makes the T1/T2
// coexistence bug (backend pre-creates a thread T1, CLI creates its own T2,
// events for T2 arrive on the backend's connection and get dropped) faithfully
// reproducible in tests.
package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
)

// TurnRequest carries the payload of a `turn/start` request through to a
// scripted TurnHandler.
type TurnRequest struct {
	ThreadID string
	TurnID   string
	Cwd      string
	Input    string
}

// TurnHandler is invoked from the server goroutine after a `turn/start`
// request completes. It receives an Emitter targeted at every connected
// client, so a handler that writes turn/started + turn/completed reproduces
// the real broadcast semantics observed on the wire.
type TurnHandler func(req TurnRequest, emit Emitter)

// Emitter fan-outs notifications to every connected client (broadcast). This
// mirrors the real codex-app-server, which delivers a thread/turn event to
// every subscribed client regardless of who initiated the turn.
type Emitter interface {
	Broadcast(method string, params any) error
}

// Thread captures the (small) subset of thread state the fake tracks. Kept
// separate from any wire type so tests can assert on it without importing
// codexschema.
type Thread struct {
	ID          string
	SessionID   string // real codex sets sessionId == thread id; fake mirrors that
	Cwd         string
	RolloutPath string
	Status      string // "idle" | "active"
	CreatedBy   string // label recorded from `clientInfo` if the client supplied one
	TurnCount   int
}

// AppServer is a codex-app-server look-alike. Start binds a UDS socket, serves
// WebSocket JSON-RPC connections, tracks thread state, and broadcasts
// notifications to all connected clients. Stop tears everything down.
type AppServer struct {
	sock       string
	rolloutDir string
	turnFn     TurnHandler

	mu           sync.Mutex
	seq          int
	threads      map[string]*Thread
	clients      []*serverConn
	lastRequests map[string]map[string]any // method → decoded params of the most recent invocation

	listener net.Listener
	httpSrv  *http.Server
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// Config configures the fake at construction time.
type Config struct {
	// Sock is the UDS path the fake binds to. Required.
	Sock string
	// RolloutDir, when non-empty, is where the fake writes a per-thread rollout
	// file (mirroring codex-app-server's ~/.codex/sessions/YYYY/MM/DD/…). Tests
	// that exercise recovery via `codex resume <id>` need this so the id is
	// resolvable against a real codex binary.
	RolloutDir string
	// TurnHandler runs after each `turn/start` request. When nil, a default
	// handler emits turn/started + turn/completed with an echo body — enough to
	// drive the backend's routing invariants without any turn scripting.
	TurnHandler TurnHandler
}

// New constructs an AppServer with the given config. Call Start to bind and
// serve.
func New(cfg Config) *AppServer {
	if cfg.TurnHandler == nil {
		cfg.TurnHandler = defaultTurnHandler
	}
	return &AppServer{
		sock:         cfg.Sock,
		rolloutDir:   cfg.RolloutDir,
		turnFn:       cfg.TurnHandler,
		threads:      map[string]*Thread{},
		lastRequests: map[string]map[string]any{},
		done:         make(chan struct{}),
	}
}

// LastRequestParams returns the decoded params of the most recent invocation
// of method. Returns nil, false when the method has not been called. Handy
// for white-box tests that need to assert on what the Backend actually
// sent (e.g. "thread/resume was called with path=X, threadId=Y").
func (a *AppServer) LastRequestParams(method string) (map[string]any, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, ok := a.lastRequests[method]
	if !ok {
		return nil, false
	}
	// Return a shallow clone so caller-side mutation doesn't corrupt state.
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out, true
}

// recordRequest snapshots the params of a method call for LastRequestParams.
func (a *AppServer) recordRequest(method string, raw json.RawMessage) {
	var params map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &params)
	}
	if params == nil {
		params = map[string]any{}
	}
	a.mu.Lock()
	a.lastRequests[method] = params
	a.mu.Unlock()
}

// Start binds the UDS socket and begins serving. Returns once the listener is
// accepting; the actual serve loop runs in a goroutine.
func (a *AppServer) Start() error {
	if a.sock == "" {
		return errors.New("fake app-server: Sock is required")
	}
	_ = os.Remove(a.sock)
	l, err := net.Listen("unix", a.sock)
	if err != nil {
		return fmt.Errorf("fake app-server: listen %s: %w", a.sock, err)
	}
	a.listener = l
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.httpSrv = &http.Server{Handler: http.HandlerFunc(a.serveHTTP)}
	go func() {
		defer close(a.done)
		_ = a.httpSrv.Serve(l)
	}()
	return nil
}

// Stop terminates the server and reaps its goroutines.
func (a *AppServer) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.httpSrv != nil {
		shutCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = a.httpSrv.Shutdown(shutCtx)
		cancel()
	}
	if a.listener != nil {
		_ = a.listener.Close()
	}
	<-a.done
	_ = os.Remove(a.sock)
}

// SockPath returns the UDS path the fake is bound to.
func (a *AppServer) SockPath() string { return a.sock }

// Threads returns a snapshot of the currently tracked threads (order
// unspecified). Handy for tests to assert what backend+CLI produced.
func (a *AppServer) Threads() []Thread {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Thread, 0, len(a.threads))
	for _, t := range a.threads {
		out = append(out, *t)
	}
	return out
}

// ClientCount returns the number of currently connected clients.
func (a *AppServer) ClientCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.clients)
}

func (a *AppServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	transport := &wsServerTransport{c: c}
	conn := codexclient.NewConn(transport, 30*time.Second)
	sc := &serverConn{conn: conn, server: codexclient.NewServer(conn), app: a}

	a.mu.Lock()
	a.clients = append(a.clients, sc)
	a.mu.Unlock()
	defer a.removeClient(sc)

	// Handler runs the JSON-RPC read loop synchronously; blocks until the
	// client disconnects or context is cancelled.
	_ = conn.Run(a.ctx, sc)
}

func (a *AppServer) removeClient(target *serverConn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, c := range a.clients {
		if c == target {
			a.clients = append(a.clients[:i], a.clients[i+1:]...)
			return
		}
	}
}

// Broadcast implements Emitter. Every currently connected client receives
// method+params as a JSON-RPC notification. Errors on individual clients are
// logged but do not abort the fan-out.
func (a *AppServer) Broadcast(method string, params any) error {
	a.mu.Lock()
	targets := append([]*serverConn(nil), a.clients...)
	a.mu.Unlock()
	for _, c := range targets {
		if err := c.server.EmitNotification(method, params); err != nil {
			slog.Debug("fake app-server: broadcast failed", "method", method, "err", err)
		}
	}
	return nil
}

// newThread mints a fresh thread id + session id (same UUID; matches real
// codex). Rollout path is written under RolloutDir when configured.
func (a *AppServer) newThread(cwd string) *Thread {
	a.mu.Lock()
	a.seq++
	// Deterministic id format (fake-thread-N) instead of UUIDv7 so tests can
	// pin exact ids; real codex uses UUIDv7 but tests only care about
	// uniqueness + persistence across broadcast.
	id := fmt.Sprintf("fake-thread-%03d", a.seq)
	t := &Thread{
		ID:        id,
		SessionID: id,
		Cwd:       cwd,
		Status:    "idle",
	}
	if a.rolloutDir != "" {
		path := filepath.Join(a.rolloutDir, "rollout-"+id+".jsonl")
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			slog.Debug("fake app-server: rollout create failed", "path", path, "err", err)
		} else {
			t.RolloutPath = path
		}
	}
	a.threads[id] = t
	a.mu.Unlock()
	return t
}

// resumeThread finds a thread by ID (preferred) or rollout path. Returns nil
// when the fake has never seen it.
func (a *AppServer) resumeThread(threadID, rolloutPath string) *Thread {
	a.mu.Lock()
	defer a.mu.Unlock()
	if threadID != "" {
		if t := a.threads[threadID]; t != nil {
			return t
		}
	}
	if rolloutPath != "" {
		for _, t := range a.threads {
			if t.RolloutPath == rolloutPath {
				return t
			}
		}
	}
	return nil
}

// serverConn is the per-client server-side wrapper. It implements the
// codexclient.Handler interface (OnNotification / OnServerRequest) — although
// no client of the fake currently emits its own notifications, so
// OnNotification is a no-op.
type serverConn struct {
	conn   *codexclient.Conn
	server *codexclient.Server
	app    *AppServer
}

// OnNotification handles client-initiated notifications. The only one the
// codex protocol defines client-side is turn/start, which triggers the
// scripted TurnHandler.
func (s *serverConn) OnNotification(method string, params json.RawMessage) {
	if method != codexschema.MethodTurnStart {
		return
	}
	s.app.recordRequest(method, params)
	threadID, _ := nestedString(params, "threadId")
	input, _ := extractTurnInput(params)
	s.app.mu.Lock()
	t := s.app.threads[threadID]
	turnID := ""
	if t != nil {
		t.TurnCount++
		t.Status = "active"
		turnID = fmt.Sprintf("fake-turn-%d", t.TurnCount)
	}
	s.app.mu.Unlock()
	if t == nil {
		slog.Debug("fake app-server: turn/start for unknown thread", "threadId", threadID)
		return
	}
	s.app.turnFn(TurnRequest{
		ThreadID: threadID,
		TurnID:   turnID,
		Cwd:      t.Cwd,
		Input:    input,
	}, s.app)
}

func (s *serverConn) OnServerRequest(id int64, method string, params json.RawMessage) {
	s.app.recordRequest(method, params)
	switch method {
	case codexschema.MethodInitialize:
		_ = s.conn.Reply(id, map[string]any{})
	case codexschema.MethodThreadStart:
		cwd, _ := nestedString(params, "cwd")
		t := s.app.newThread(cwd)
		_ = s.conn.Reply(id, map[string]any{
			"thread": map[string]any{
				"id":        t.ID,
				"sessionId": t.SessionID,
				"path":      t.RolloutPath,
				"cwd":       t.Cwd,
			},
		})
		_ = s.app.Broadcast(codexschema.MethodThreadStarted, map[string]any{
			"thread": map[string]any{"id": t.ID, "sessionId": t.SessionID, "path": t.RolloutPath, "cwd": t.Cwd},
		})
	case codexschema.MethodThreadResume:
		threadID, _ := nestedString(params, "threadId")
		rolloutPath, _ := nestedString(params, "path")
		t := s.app.resumeThread(threadID, rolloutPath)
		if t == nil {
			_ = s.conn.ReplyError(id, fmt.Sprintf("fake app-server: thread not found (threadId=%q rolloutPath=%q)", threadID, rolloutPath))
			return
		}
		_ = s.conn.Reply(id, map[string]any{
			"thread": map[string]any{
				"id":        t.ID,
				"sessionId": t.SessionID,
				"path":      t.RolloutPath,
				"cwd":       t.Cwd,
			},
		})
		_ = s.app.Broadcast(codexschema.MethodThreadStarted, map[string]any{
			"thread": map[string]any{"id": t.ID, "sessionId": t.SessionID, "path": t.RolloutPath, "cwd": t.Cwd},
		})
	default:
		// Unknown methods: reply with an error rather than silently succeed so
		// tests noticing an untracked call have a signal.
		_ = s.conn.ReplyError(id, "fake app-server: unhandled method "+method)
	}
}

// defaultTurnHandler emits the minimum event sequence that carries the driver
// through Idle → Running → Waiting: turn/started, thread/status active,
// turn/completed, thread/status idle. Real codex emits many more events
// (item/started, item/completed, message deltas, etc.); tests that need those
// override this via Config.TurnHandler.
func defaultTurnHandler(req TurnRequest, emit Emitter) {
	_ = emit.Broadcast(codexschema.MethodTurnStarted, map[string]any{
		"threadId": req.ThreadID,
		"turnId":   req.TurnID,
	})
	_ = emit.Broadcast(codexschema.MethodThreadStatusChanged, map[string]any{
		"threadId": req.ThreadID,
		"status":   map[string]any{"type": "active"},
	})
	_ = emit.Broadcast(codexschema.MethodTurnCompleted, map[string]any{
		"threadId": req.ThreadID,
		"turnId":   req.TurnID,
		"text":     "echo: " + req.Input,
	})
	_ = emit.Broadcast(codexschema.MethodThreadStatusChanged, map[string]any{
		"threadId": req.ThreadID,
		"status":   map[string]any{"type": "idle"},
	})
}
