package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agent/codexschema"
	libcodex "github.com/takezoh/agent-reactor/platform/lib/codex"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// Stream subsystem launch-flow tests: the codex app-server is faked so no real
// codex binary runs. Two altitudes:
//
//   - BindFrame command/socket rewrite — white-box: b.conn is paired with an
//     in-process fake server over a pipe, BindFrame's thread/resume + turn/start
//     RPCs are answered, and the resolved launch command is asserted.
//   - Start ordering — a helper sub-process (this test binary re-invoked with a
//     leading "app-server" arg) binds a real WebSocket-over-UDS server, so the
//     full spawn → dial → initialize sequence runs. The Initialize-failure path
//     pins the e41ab1c regression where a failed handshake must reap the process.

// TestMain doubles as the fake app-server entry point. The backend spawns this
// binary with argv [<bin> app-server --listen unix://<sock> --mode <mode>]
// (AppServerListenArgs shape); the leading "app-server" token selects helper
// mode before the test flag parser runs.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "app-server" {
		runFakeAppServer(os.Args[2:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeAppServer binds a WebSocket-over-UDS server at the --listen socket and
// answers `initialize` per --mode ("ok" → success, "initfail" → JSON-RPC error).
// It blocks serving until the parent SIGKILLs it (subsystem ctx cancel).
func runFakeAppServer(args []string) {
	var sock, mode string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 < len(args) {
				sock = strings.TrimPrefix(args[i+1], "unix://")
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				mode = args[i+1]
				i++
			}
		}
	}
	if sock == "" {
		os.Exit(2)
	}
	l, err := net.Listen("unix", sock)
	if err != nil {
		os.Exit(3)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg struct {
				ID     *int64 `json:"id"`
				Method string `json:"method"`
			}
			if json.Unmarshal(data, &msg) != nil || msg.ID == nil || msg.Method != codexschema.MethodInitialize {
				continue
			}
			reply := map[string]any{"id": *msg.ID, "result": map[string]any{}}
			if mode == "initfail" {
				reply = map[string]any{"id": *msg.ID, "error": map[string]any{"message": "boom"}}
			}
			b, _ := json.Marshal(reply)
			_ = c.Write(ctx, websocket.MessageText, b)
		}
	})}
	_ = srv.Serve(l)
}

// === BindFrame command / socket rewrite (white-box, in-process server) ===

// streamPipe wires two StdioTransports back-to-back (mirrors codexclient's own
// test helper, which is package-private).
func streamPipe() (codexclient.Transport, codexclient.Transport) {
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	return codexclient.StdioTransport(pr1, pw2), codexclient.StdioTransport(pr2, pw1)
}

// bindServer is the in-process fake app-server for BindFrame tests. It replies
// to thread/start with a fresh unique thread id (cold start creates the thread
// synchronously) and to thread/resume with an empty result (backend keeps the
// requested id).
type bindServer struct {
	conn       *codexclient.Conn
	mu         sync.Mutex
	threadSeq  int
	lastResume map[string]any
	omitPath   bool
	customPath string
}

func (s *bindServer) OnNotification(string, json.RawMessage) {}

func (s *bindServer) OnServerRequest(id int64, method string, params json.RawMessage) {
	if method == codexschema.MethodThreadStart {
		s.mu.Lock()
		s.threadSeq++
		tid := fmt.Sprintf("thread-%d", s.threadSeq)
		path := s.customPath
		omitPath := s.omitPath
		s.mu.Unlock()
		thread := map[string]any{"id": tid, "sessionId": "sess-" + tid}
		if !omitPath {
			if path == "" {
				path = "/repo/.codex/rollout.jsonl"
			}
			thread["path"] = path
		}
		_ = s.conn.Reply(id, map[string]any{"thread": thread})
		return
	}
	if method == codexschema.MethodThreadResume {
		var decoded map[string]any
		_ = json.Unmarshal(params, &decoded)
		s.mu.Lock()
		s.lastResume = decoded
		path := s.customPath
		omitPath := s.omitPath
		s.mu.Unlock()
		thread := map[string]any{"id": "thread-resumed", "sessionId": "sess-resumed"}
		if !omitPath {
			if path == "" {
				if p, ok := decoded["path"].(string); ok {
					path = p
				}
			}
			if path != "" {
				thread["path"] = path
			}
		}
		_ = s.conn.Reply(id, map[string]any{"thread": thread})
		return
	}
	_ = s.conn.Reply(id, map[string]any{})
}

type boundHarness struct {
	backend *Backend
	server  *bindServer
}

func newBoundBackend(t *testing.T, listenSock string) *boundHarness {
	t.Helper()
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", false, false,
		listenSock, time.Second)
	ta, tb := streamPipe()
	b.conn = codexclient.NewConn(ta, time.Second)
	srv := &bindServer{conn: codexclient.NewConn(tb, time.Second)}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go b.conn.Run(ctx, b)     //nolint:errcheck
	go srv.conn.Run(ctx, srv) //nolint:errcheck
	return &boundHarness{backend: b, server: srv}
}

func newBoundBackendNoPath(t *testing.T, listenSock string) *boundHarness {
	t.Helper()
	h := newBoundBackend(t, listenSock)
	h.server.mu.Lock()
	h.server.omitPath = true
	h.server.mu.Unlock()
	return h
}

func newBoundBackendCustomPath(t *testing.T, listenSock, path string) *boundHarness {
	t.Helper()
	h := newBoundBackend(t, listenSock)
	h.server.mu.Lock()
	h.server.customPath = path
	h.server.mu.Unlock()
	return h
}

func writeRollout(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBackendBindFrameColdStartRemoteCommand(t *testing.T) {
	const listen = "/opt/agent-reactor/run/codex-sess1.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan:    state.LaunchPlan{StartDir: "/repo"},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	// Cold start now creates the thread synchronously (thread/start) and binds
	// it, so the frame resumes that id — same command shape as a warm start.
	h.backend.mu.Lock()
	binding := h.backend.frames["f1"]
	h.backend.mu.Unlock()
	if binding == nil || binding.threadID == "" {
		t.Fatal("cold start must bind a synchronously-created thread id")
	}
	want := strings.Join(libcodex.RemoteAttachArgs(listen, "/repo"), " ")
	if res.Plan.Command != want {
		t.Fatalf("Command = %q, want %q", res.Plan.Command, want)
	}
	if strings.Contains(res.Plan.Command, "resume ") {
		t.Errorf("cold start command must not route through codex resume: %q", res.Plan.Command)
	}
	if res.Plan.Stream.ColdStartSessionID != "sess-"+binding.threadID {
		t.Fatalf("ColdStartSessionID = %q, want stable session id %q", res.Plan.Stream.ColdStartSessionID, "sess-"+binding.threadID)
	}
	if binding.rolloutPath != "/repo/.codex/rollout.jsonl" {
		t.Fatalf("binding.rolloutPath = %q", binding.rolloutPath)
	}
	if binding.resumePhase != resumePhaseAttached {
		t.Errorf("cold start must bind+attach the frame, got %+v", binding)
	}
}

func TestBackendBindFrameResumeUsesRolloutPathForRPCAndCLI(t *testing.T) {
	const listen = "/opt/agent-reactor/run/codex-sess2.sock"
	h := newBoundBackend(t, listen)
	rollout := writeRollout(t)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget:       state.ResumeTarget{ThreadID: "thread-abc", RolloutPath: rollout},
			ColdStartSessionID: "019e727e-fde4-7432-9036-ae6604ce1b27",
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	want := strings.Join(libcodex.RemoteAttachArgs(listen, "/repo"), " ")
	if res.Plan.Command != want {
		t.Fatalf("Command = %q, want %q", res.Plan.Command, want)
	}
	if res.Plan.Stream.ColdStartSessionID != "019e727e-fde4-7432-9036-ae6604ce1b27" {
		t.Fatalf("ColdStartSessionID = %q", res.Plan.Stream.ColdStartSessionID)
	}
	if res.Plan.Stream.ResumeTarget != (state.ResumeTarget{ThreadID: "thread-resumed", RolloutPath: rollout}) {
		t.Fatalf("ResumeTarget = %+v", res.Plan.Stream.ResumeTarget)
	}
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	if h.server.lastResume["path"] != rollout {
		t.Fatalf("thread/resume path = %v, want %q", h.server.lastResume["path"], rollout)
	}
}

func TestBackendBindFrameResumeAllowsThreadIDOnly(t *testing.T) {
	const listen = "/opt/agent-reactor/run/codex-sess3.sock"
	h := newBoundBackend(t, listen)

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget: state.ResumeTarget{ThreadID: "thread-abc"},
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	if strings.Contains(res.Plan.Command, "resume ") {
		t.Fatalf("Command = %q, want plain remote attach", res.Plan.Command)
	}
}

func TestBackendBindFrameResumeTranslatesHostRolloutPathForSandboxedRPC(t *testing.T) {
	const listen = "/opt/agent-reactor/run/codex-sess4.sock"
	h := newBoundBackend(t, listen)
	h.backend.mounts = pathmap.Mounts{{Host: "/host/work", Container: "/work"}}
	h.backend.sandboxed = true
	rollout := "/host/work/.codex/rollout.jsonl"

	res, err := h.backend.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan: state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{
			ResumeTarget:       state.ResumeTarget{ThreadID: "thread-abc", RolloutPath: rollout},
			ColdStartSessionID: "019e727e-fde4-7432-9036-ae6604ce1b27",
		}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	if h.server.lastResume["path"] != "/work/.codex/rollout.jsonl" {
		t.Fatalf("thread/resume path = %v, want translated container path", h.server.lastResume["path"])
	}
	if strings.Contains(res.Plan.Command, "resume ") {
		t.Fatalf("remote attach must not use codex resume, got %q", res.Plan.Command)
	}
	h.backend.mu.Lock()
	binding := h.backend.frames["f1"]
	h.backend.mu.Unlock()
	if binding == nil || binding.rolloutPath != rollout {
		t.Fatalf("binding rolloutPath = %v, want host path %q", binding, rollout)
	}
}

// === Start: spawn → dial → initialize (helper sub-process) ===

func newHelperBackend(t *testing.T, mode string) *Backend {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "codex-x.sock")
	return New(&fakeRuntime{}, nil, "sid", "sess1", "/p",
		os.Args[0], []string{"--mode", mode}, "", false, false,
		sock, 3*time.Second)
}

func TestBackendStartDialsAndInitializes(t *testing.T) {
	b := newHelperBackend(t, "ok")
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { b.Stop(context.Background()) })
	if b.conn == nil {
		t.Error("conn not established after successful Start")
	}
}

// TestBackendStartReapsOnInitializeFailure pins the e41ab1c robustness fix: when
// the app-server dials successfully but rejects `initialize`, Start must surface
// the error (after reaping the process) rather than leaving it orphaned.
func TestBackendStartReapsOnInitializeFailure(t *testing.T) {
	b := newHelperBackend(t, "initfail")
	err := b.Start(context.Background())
	if err == nil {
		t.Fatal("Start must fail when the app-server rejects initialize")
	}
	if !strings.Contains(err.Error(), codexschema.MethodInitialize) {
		t.Errorf("error should come from the initialize handshake (dial succeeded), got: %v", err)
	}
}
