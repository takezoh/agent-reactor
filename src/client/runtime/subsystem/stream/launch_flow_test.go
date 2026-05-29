package stream

import (
	"context"
	"encoding/json"
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

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agent/codexschema"
	libcodex "github.com/takezoh/agent-roost/platform/lib/codex"
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
// to thread/resume (empty result → backend keeps the requested thread id) and
// records turn/start notifications (cold-start path).
type bindServer struct {
	conn  *codexclient.Conn
	mu    sync.Mutex
	turns int
}

func (s *bindServer) OnNotification(method string, _ json.RawMessage) {
	if method == codexschema.MethodTurnStart {
		s.mu.Lock()
		s.turns++
		s.mu.Unlock()
	}
}

func (s *bindServer) OnServerRequest(id int64, _ string, _ json.RawMessage) {
	_ = s.conn.Reply(id, map[string]any{})
}

func newBoundBackend(t *testing.T, listenSock string) (*Backend, *bindServer) {
	t.Helper()
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", false, false,
		listenSock, func() state.FrameID { return "" }, time.Second)
	ta, tb := streamPipe()
	b.conn = codexclient.NewConn(ta, time.Second)
	srv := &bindServer{conn: codexclient.NewConn(tb, time.Second)}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go b.conn.Run(ctx, b)     //nolint:errcheck
	go srv.conn.Run(ctx, srv) //nolint:errcheck
	return b, srv
}

func TestBackendBindFrameColdStartRemoteCommand(t *testing.T) {
	const listen = "/opt/roost/run/codex-sess1.sock"
	b, srv := newBoundBackend(t, listen)

	res, err := b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan:    state.LaunchPlan{StartDir: "/repo"},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	want := strings.Join(libcodex.RemoteAttachArgs(listen, "", "/repo"), " ")
	if res.Plan.Command != want {
		t.Fatalf("Command = %q, want %q", res.Plan.Command, want)
	}
	if !strings.Contains(res.Plan.Command, "--remote unix://"+listen) {
		t.Errorf("command must attach to the container-absolute socket unix://%s: %q", listen, res.Plan.Command)
	}
	if res.Plan.Stream.ResumeThreadID != "" {
		t.Errorf("cold start ResumeThreadID = %q, want empty", res.Plan.Stream.ResumeThreadID)
	}
	// turn/start is a fire-and-forget notification; poll briefly for the server
	// to observe it rather than racing the async read loop.
	deadline := time.Now().Add(time.Second)
	for {
		srv.mu.Lock()
		turns := srv.turns
		srv.mu.Unlock()
		if turns == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cold start must issue exactly one turn/start, observed %d", turns)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestBackendBindFrameResumeRemoteCommand(t *testing.T) {
	const listen = "/opt/roost/run/codex-sess2.sock"
	const thread = "thread-abc"
	b, _ := newBoundBackend(t, listen)

	res, err := b.BindFrame(context.Background(), subsystem.BindRequest{
		FrameID: "f1",
		Plan:    state.LaunchPlan{StartDir: "/repo", Stream: state.StreamLaunchOptions{ResumeThreadID: thread}},
	})
	if err != nil {
		t.Fatalf("BindFrame: %v", err)
	}

	want := strings.Join(libcodex.RemoteAttachArgs(listen, thread, "/repo"), " ")
	if res.Plan.Command != want {
		t.Fatalf("Command = %q, want %q", res.Plan.Command, want)
	}
	if res.Plan.Stream.ResumeThreadID != thread {
		t.Errorf("ResumeThreadID = %q, want %q", res.Plan.Stream.ResumeThreadID, thread)
	}
	b.mu.Lock()
	binding := b.frames["f1"]
	b.mu.Unlock()
	if binding == nil || binding.resumePhase != resumePhasePending {
		t.Errorf("resume must register a pending binding, got %+v", binding)
	}
}

// === Start: spawn → dial → initialize (helper sub-process) ===

func newHelperBackend(t *testing.T, mode string) *Backend {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "codex-x.sock")
	return New(&fakeRuntime{}, nil, "sid", "sess1", "/p",
		os.Args[0], []string{"--mode", mode}, "", false, false,
		sock, func() state.FrameID { return "" }, 3*time.Second)
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
