package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/driver"
	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

func TestMain(m *testing.M) {
	// Register drivers so reducers can resolve commands. The runtime
	// tests don't exercise driver-specific behaviour — they just need
	// SOMETHING in the registry.
	state.Register(driver.NewGenericDriver("", "", 0))
	state.Register(driver.NewGenericDriver("shell", "shell", 0))
	state.Register(driver.NewCodexDriver(""))
	os.Exit(m.Run())
}

// === Fake backends for runtime tests ===

type fakeBackend struct {
	mu               sync.Mutex
	spawnCalls       int
	spawnCmds        []string
	spawnEnvs        []map[string]string
	spawnFrameIDs    []string
	killCalls        int
	sessionKillCalls int
	killedFrames      []string
	breakCalls       int
	breakTargets     []string
	breakNewCalls    int
	breakNewNames    []string
	joinCalls        int
	joinSources      []string
	joinTargets      []string
	swapCalls        int
	swapSources      []string
	swapTargets      []string
	callLog          []string // records "swap"/"kill" in order, for ordering assertions
	resizeCalls      int
	resizeTargets    []string
	resizeWidths     []int
	resizeHeights    []int
	respawnCmds      []string
	statusLines      []string
	envs             map[string]string
	popups           []string
	alive            map[string]bool
	exitStatusErr    map[string]error // frame target → error from FrameExitStatus
	exitStatus       map[string]int   // frame target → exit code (when dead)
	captured         string
	breakNewWID      string
	spawnErr         error
	swapErr          error
	envOutput        string
	frameWidth        int
	frameHeight       int
	frameIDs          map[string]string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		alive:         map[string]bool{},
		exitStatusErr: map[string]error{},
		exitStatus:    map[string]int{},
		envs:          map[string]string{},
		frameIDs:       map[string]string{},
		breakNewWID:   "9",
		frameWidth:     120,
		frameHeight:    40,
	}
}

func (f *fakeBackend) SpawnFrame(frameID, name, command, startDir string, env map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spawnCalls++
	f.spawnCmds = append(f.spawnCmds, command)
	f.spawnEnvs = append(f.spawnEnvs, cloneEnvMap(env, 0))
	f.spawnFrameIDs = append(f.spawnFrameIDs, frameID)
	return f.spawnErr
}

func (f *fakeBackend) ShowEnvironment() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.envOutput, nil
}

func (f *fakeBackend) KillFrame(frameID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killCalls++
	f.killedFrames = append(f.killedFrames, frameID)
	f.callLog = append(f.callLog, "kill")
	return nil
}

func (f *fakeBackend) ResolveID(target string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lookup := strings.Replace(target, ":=", ":", 1)
	if id, ok := f.frameIDs[lookup]; ok {
		if id == "error" {
			return "", fmt.Errorf("backend error for %s", target)
		}
		return id, nil
	}
	return "%main", nil
}
func (f *fakeBackend) FrameSize(string) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frameWidth, f.frameHeight, nil
}
func (f *fakeBackend) SetStatusLine(line string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusLines = append(f.statusLines, line)
	return nil
}
func (f *fakeBackend) SetEnv(k, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envs[k] = v
	return nil
}
func (f *fakeBackend) UnsetEnv(k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.envs, k)
	return nil
}
func (f *fakeBackend) FrameExitStatus(target string) (bool, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.exitStatusErr[target]; ok {
		return false, -1, err
	}
	alive, known := f.alive[target]
	if !known || alive {
		return false, -1, nil
	}
	code, has := f.exitStatus[target]
	if !has {
		return false, -1, nil
	}
	return true, code, nil
}
func (f *fakeBackend) RespawnFrame(target, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.respawnCmds = append(f.respawnCmds, cmd)
	return nil
}
func (f *fakeBackend) CaptureFrame(string, int) (string, error) {
	return f.captured, nil
}
func (f *fakeBackend) DetachClient() error { return nil }
func (f *fakeBackend) KillSession() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionKillCalls++
	return nil
}
func (f *fakeBackend) DisplayPopup(w, h, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.popups = append(f.popups, cmd)
	return nil
}
func (f *fakeBackend) PipeFrame(string, string) error   { return nil }
func (f *fakeBackend) SendKeys(string, string) error    { return nil }
func (f *fakeBackend) SendKey(string, string) error     { return nil }
func (f *fakeBackend) LoadBuffer(string, string) error  { return nil }
func (f *fakeBackend) PasteBuffer(string, string) error { return nil }
func (f *fakeBackend) SendEnter(string) error           { return nil }

type recordingPersist struct {
	mu      sync.Mutex
	saves   int
	last    []SessionSnapshot
	deletes []string
}

func (r *recordingPersist) Save(s []SessionSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saves++
	r.last = s
	return nil
}
func (r *recordingPersist) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletes = append(r.deletes, id)
	return nil
}
func (r *recordingPersist) Load() ([]SessionSnapshot, error) { return nil, nil }

type recordingWatcher struct {
	mu      sync.Mutex
	watches map[state.FrameID]string
}

func (r *recordingWatcher) Watch(sessionID state.FrameID, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.watches == nil {
		r.watches = map[state.FrameID]string{}
	}
	r.watches[sessionID] = path
	return nil
}

func (r *recordingWatcher) Unwatch(sessionID state.FrameID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.watches, sessionID)
	return nil
}

func (r *recordingWatcher) Events() <-chan FSEvent { return nil }
func (r *recordingWatcher) Close() error           { return nil }

// === Tests ===

func TestRuntimeStartsAndShutsDown(t *testing.T) {
	r := New(Config{
		TickInterval: 50 * time.Millisecond,
		Backend:      newFakeBackend(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = r.Run(ctx)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("Run did not exit")
	}
}

func TestSendResponseSyncFlushesImmediately(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	r := New(Config{})
	cc := newIPCConn(1, server)
	r.conns[1] = cc

	done := make(chan []byte, 1)
	go func() {
		reader := bufio.NewReader(client)
		line, _ := reader.ReadBytes('\n')
		done <- line
	}()

	r.execute(state.EffSendResponseSync{
		ConnID: 1,
		ReqID:  "req-1",
		Body:   nil,
	})

	select {
	case line := <-done:
		env, err := proto.DecodeEnvelope(line)
		if err != nil {
			t.Fatalf("DecodeEnvelope: %v", err)
		}
		if env.Type != proto.TypeResponse {
			t.Fatalf("type = %q, want %q", env.Type, proto.TypeResponse)
		}
		if env.ReqID != "req-1" {
			t.Fatalf("req_id = %q, want req-1", env.ReqID)
		}
		if env.Status != proto.StatusOK {
			t.Fatalf("status = %q, want %q", env.Status, proto.StatusOK)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sync response")
	}
}

func TestRuntimeCreateSessionFlow(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		TickInterval: 10 * time.Second, // suppress periodic ticks
		Backend:      backend,
		Persist:      persist,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	r.Enqueue(state.EvEvent{
		ConnID: 1, ReqID: "r1", Event: "create-session",
		Payload: json.RawMessage(`{"project":"/tmp/test","command":"stub-fallback"}`),
	})

	// Wait for the spawn callback to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		backend.mu.Lock()
		spawned := backend.spawnCalls
		backend.mu.Unlock()
		persist.mu.Lock()
		saved := persist.saves
		persist.mu.Unlock()
		if spawned >= 1 && saved >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.spawnCalls != 1 {
		t.Errorf("spawnCalls = %d, want 1", backend.spawnCalls)
	}
	// With the TUI gone there is no "main frame" to resize new spawns to;
	// spawnDeps no longer carries a frameSize provider, so ResizeWindow stays
	// at zero calls on this path.
	if backend.resizeCalls != 0 {
		t.Errorf("resizeCalls = %d, want 0 (no main frame to size to)", backend.resizeCalls)
	}
	persist.mu.Lock()
	defer persist.mu.Unlock()
	if persist.saves < 1 {
		t.Errorf("persist saves = %d, want ≥1", persist.saves)
	}
	if len(persist.last) != 1 {
		t.Errorf("last snapshot len = %d, want 1", len(persist.last))
	}
}

func TestWindowName(t *testing.T) {
	if got := windowName("/foo/bar", "abc"); got != "bar:abc" {
		t.Errorf("got %q, want bar:abc", got)
	}
	if got := windowName("", "abc"); got != "session:abc" {
		t.Errorf("got %q, want session:abc", got)
	}
}

func TestCommandToStateEvent(t *testing.T) {
	cases := []struct {
		cmd  proto.Command
		want string
	}{
		{proto.CmdSubscribe{}, "EvCmdSubscribe"},
		{proto.CmdEvent{Event: "test"}, "EvEvent"},
	}
	for _, c := range cases {
		ev := commandToStateEvent(state.ConnID(1), "r1", c.cmd)
		if ev == nil {
			t.Errorf("nil event for %T", c.cmd)
		}
	}
}

func TestEventTypeName(t *testing.T) {
	cases := []struct {
		ev   state.Event
		want string
	}{
		{state.EvTick{}, "EvTick"},
		{state.EvEvent{}, "EvEvent"},
	}
	for _, c := range cases {
		if got := eventTypeName(c.ev); got != c.want {
			t.Errorf("eventTypeName = %q, want %q", got, c.want)
		}
	}
}

// stop-session immediately kills the session window (no SIGINT).
func TestRuntimeStopSession(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      backend,
	})
	r.state.Sessions["abc"] = state.Session{
		ID:      "abc",
		Command: "stub-x",
		Driver:  driver.NewGenericDriver("", "", 0).NewState(time.Now()),
		Frames:  []state.SessionFrame{{ID: "abc-frame", Command: "stub-x", Driver: driver.NewGenericDriver("", "", 0).NewState(time.Now())}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	r.Enqueue(state.EvEvent{ConnID: 1, ReqID: "r", Event: "stop-session", Payload: json.RawMessage(`{"session_id":"abc"}`)})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		backend.mu.Lock()
		n := backend.killCalls
		backend.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.killCalls != 1 {
		t.Errorf("killCalls = %d, want 1 (kill-window should fire)", backend.killCalls)
	}
}

// reconcileWindows must distinguish a vanished frame from a transient query
// failure: only errors that wrap ErrFrameMissing should evict a frame.
func TestReconcileWindowsTransientErrorKeepsFrame(t *testing.T) {
	backend := newFakeBackend()
	backend.exitStatusErr["inactive-frame"] = fmt.Errorf("backend display-message -t %%7 -p ...: %w", context.DeadlineExceeded)
	r := New(Config{
		Backend: backend,
	})
	r.state.Sessions["inactive-session"] = state.Session{
		ID:     "inactive-session",
		Frames: []state.SessionFrame{{ID: "inactive-frame", Command: "stub"}},
	}

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		t.Fatalf("transient FrameExitStatus error must not vanish a frame, got %T", ev)
	case <-time.After(200 * time.Millisecond):
		// OK: transient error ignored.
	}
}

func TestReconcileWindowsMissingFrameVanishesFrame(t *testing.T) {
	backend := newFakeBackend()
	backend.exitStatusErr["inactive-frame"] = fmt.Errorf("runtime: unknown frame %q: %w", "inactive-frame", ErrFrameMissing)
	r := New(Config{
		Backend: backend,
	})
	r.state.Sessions["inactive-session"] = state.Session{
		ID:     "inactive-session",
		Frames: []state.SessionFrame{{ID: "inactive-frame", Command: "stub"}},
	}

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		vanished, ok := ev.(state.EvFrameVanished)
		if !ok {
			t.Fatalf("expected EvFrameVanished, got %T", ev)
		}
		if vanished.FrameID != "inactive-frame" {
			t.Errorf("FrameID = %q, want inactive-frame", vanished.FrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvFrameVanished for a genuinely missing frame")
	}
}

func TestIsShellCommand(t *testing.T) {
	if !isShellCommand("shell") {
		t.Error("expected true for 'shell'")
	}
	if isShellCommand("claude") {
		t.Error("expected false for 'claude'")
	}
	if isShellCommand("") {
		t.Error("expected false for empty")
	}
}

func TestRuntimeShellSessionSpawnsLoginShell(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      persist,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	r.Enqueue(state.EvEvent{
		ConnID: 1, ReqID: "r1", Event: "create-session",
		Payload: json.RawMessage(`{"project":"/tmp/test","command":"shell"}`),
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		backend.mu.Lock()
		spawned := backend.spawnCalls
		backend.mu.Unlock()
		if spawned >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.spawnCalls != 1 {
		t.Fatalf("spawnCalls = %d, want 1", backend.spawnCalls)
	}
	if want := buildSpawnCommand("shell", nil); backend.spawnCmds[0] != want {
		t.Errorf("spawn command = %q, want %q (explicit passwd login shell)", backend.spawnCmds[0], want)
	}
}

func TestRecreateAllUsesPrepareLaunch(t *testing.T) {
	t.Skip("shared codex backend is runtime-managed; helper command assertions are obsolete")
}

func TestSpawnFrameWindowAsyncUsesPrepareLaunch(t *testing.T) {
	t.Skip("shared codex backend is runtime-managed; direct remote command is covered by codex backend tests")
}

func TestSpawnFrameWindowAsyncInjectsStreamPolicyEnv(t *testing.T) {
	t.Skip("stream policy is applied via runtime-owned codex backend, not helper env")
}

func TestReconcileDetectsVanishedFrame(t *testing.T) {
	fbackend := newFakeBackend()
	fbackend.exitStatusErr["tracked1"] = fmt.Errorf("runtime: unknown frame %q: %w", "tracked1", ErrFrameMissing)
	r := New(Config{
		Backend: fbackend,
	})
	drv := state.GetDriver("shell")
	r.state.Sessions[state.SessionID("tracked1")] = state.Session{
		ID:      state.SessionID("tracked1"),
		Command: "shell",
		Driver:  drv.NewState(time.Now()),
		Frames:  []state.SessionFrame{{ID: "tracked1", Command: "shell", Driver: drv.NewState(time.Now())}},
	}

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		vanished, ok := ev.(state.EvFrameVanished)
		if !ok {
			t.Fatalf("expected EvFrameVanished, got %T", ev)
		}
		if vanished.FrameID != "tracked1" {
			t.Errorf("FrameID = %q, want tracked1", vanished.FrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvFrameVanished for the missing frame")
	}
}

func TestReconcileSkipsWithoutTrackedFrames(t *testing.T) {
	fbackend := newFakeBackend()
	r := New(Config{
		TickInterval: 20 * time.Millisecond,
		Backend:      fbackend,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	time.Sleep(60 * time.Millisecond)
	cancel()
	<-r.Done()

	fbackend.mu.Lock()
	defer fbackend.mu.Unlock()
	if fbackend.killCalls != 0 {
		t.Errorf("killCalls = %d, want 0 (no orphans)", fbackend.killCalls)
	}
}

func TestRuntimeEnqueueDoesNotBlock(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      backend,
	})
	// Don't start Run — just check Enqueue doesn't deadlock when no
	// reader is active.
	var n atomic.Int32
	for i := 0; i < 100; i++ {
		r.Enqueue(state.EvTick{Now: time.Now()})
		n.Add(1)
	}
	// Channel buffer is 256 so 100 fits without dropping.
	if n.Load() != 100 {
		t.Errorf("enqueued %d, want 100", n.Load())
	}
}
