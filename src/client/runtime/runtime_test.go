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
	killCalls        int
	sessionKillCalls int
	killedPanes      []string
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
	aliveErr         map[string]error // pane target → transient error from PaneAlive
	exitStatusErr    map[string]error // pane target → error from PaneExitStatus
	exitStatus       map[string]int   // pane target → exit code (when dead)
	captured         string
	spawnWID         string
	spawnPane        string
	breakNewWID      string
	spawnErr         error
	swapErr          error
	envOutput        string
	paneWidth        int
	paneHeight       int
	paneIDs          map[string]string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		alive:         map[string]bool{},
		aliveErr:      map[string]error{},
		exitStatusErr: map[string]error{},
		exitStatus:    map[string]int{},
		envs:          map[string]string{},
		paneIDs:       map[string]string{},
		spawnWID:      "1",
		spawnPane:     "%1",
		breakNewWID:   "9",
		paneWidth:     120,
		paneHeight:    40,
	}
}

func (f *fakeBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.spawnCalls++
	f.spawnCmds = append(f.spawnCmds, command)
	f.spawnEnvs = append(f.spawnEnvs, cloneEnvMap(env, 0))
	if f.spawnErr != nil {
		return "", "", f.spawnErr
	}
	return f.spawnWID, f.spawnPane, nil
}

func (f *fakeBackend) ShowEnvironment() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.envOutput, nil
}

func (f *fakeBackend) KillPaneWindow(paneID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killCalls++
	f.killedPanes = append(f.killedPanes, paneID)
	f.callLog = append(f.callLog, "kill")
	return nil
}

func (f *fakeBackend) RunChain(ops ...[]string) error {
	return nil
}
func (f *fakeBackend) BreakPane(srcPane, dstWindow string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.breakCalls++
	f.breakTargets = append(f.breakTargets, dstWindow)
	return nil
}
func (f *fakeBackend) SwapPane(srcPane, dstPane string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.swapCalls++
	f.swapSources = append(f.swapSources, srcPane)
	f.swapTargets = append(f.swapTargets, dstPane)
	f.callLog = append(f.callLog, "swap")
	return f.swapErr
}
func (f *fakeBackend) BreakPaneToNewWindow(srcPane, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.breakNewCalls++
	f.breakNewNames = append(f.breakNewNames, name)
	return f.breakNewWID, nil
}
func (f *fakeBackend) JoinPane(srcPane, dstPane string, before bool, sizePct int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.joinCalls++
	f.joinSources = append(f.joinSources, srcPane)
	f.joinTargets = append(f.joinTargets, dstPane)
	return nil
}
func (f *fakeBackend) PaneID(target string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lookup := strings.Replace(target, ":=", ":", 1)
	if id, ok := f.paneIDs[lookup]; ok {
		if id == "error" {
			return "", fmt.Errorf("backend error for %s", target)
		}
		return id, nil
	}
	if target == "reactor-test:0.1" && f.spawnPane != "" {
		return f.spawnPane, nil
	}
	return "%main", nil
}
func (f *fakeBackend) PaneSize(string) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paneWidth, f.paneHeight, nil
}
func (f *fakeBackend) SelectPane(string) error { return nil }
func (f *fakeBackend) ResizeWindow(target string, width, height int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizeCalls++
	f.resizeTargets = append(f.resizeTargets, target)
	f.resizeWidths = append(f.resizeWidths, width)
	f.resizeHeights = append(f.resizeHeights, height)
	return nil
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
func (f *fakeBackend) PaneAlive(target string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.aliveErr[target]; ok {
		return false, err
	}
	v, ok := f.alive[target]
	if !ok {
		return true, nil
	}
	return v, nil
}
func (f *fakeBackend) PaneExitStatus(target string) (bool, int, error) {
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
func (f *fakeBackend) RespawnPane(target, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.respawnCmds = append(f.respawnCmds, cmd)
	return nil
}
func (f *fakeBackend) CapturePane(string, int) (string, error) {
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
func (f *fakeBackend) PipePane(string, string) error    { return nil }
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
		SessionName:  "reactor-test",
		RoostExe:     "/usr/local/bin/roost",
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

func TestExecuteKillSession(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		SessionName: "reactor-test",
		RoostExe:    "/usr/local/bin/roost",
		Backend:     backend,
	})

	r.execute(state.EffKillSession{})

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.sessionKillCalls != 1 {
		t.Fatalf("sessionKillCalls = %d, want 1", backend.sessionKillCalls)
	}
}

func TestSendResponseSyncFlushesImmediately(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	r := New(Config{
		SessionName: "reactor-test",
		RoostExe:    "/usr/local/bin/roost",
	})
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
		SessionName:  "reactor-test",
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
	if backend.resizeCalls == 0 {
		t.Error("expected spawned window to be resized to main pane size")
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

func TestRuntimeTickFiresHealthChecks(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Millisecond,
		Backend:      backend,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	// Wait for several ticks
	time.Sleep(40 * time.Millisecond)
	cancel()
	<-r.Done()
	// Health checks call PaneAlive on the control panes; with our
	// noop default returning alive=true, no respawns should fire.
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.respawnCmds) != 0 {
		t.Errorf("expected 0 respawns when panes are alive, got %d", len(backend.respawnCmds))
	}
}

func TestRuntimeRespawnsDeadPane(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["reactor-test:0.2"] = false // pane 0.2 is the sessions pane
	r := New(Config{
		SessionName:  "reactor-test",
		RoostExe:     "/usr/bin/roost",
		TickInterval: 10 * time.Millisecond,
		Backend:      backend,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		backend.mu.Lock()
		n := len(backend.respawnCmds)
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
	if len(backend.respawnCmds) == 0 {
		t.Fatal("expected respawn for dead pane")
	}
	if backend.respawnCmds[0] != "'/usr/bin/roost' --tui sessions" {
		t.Errorf("respawn cmd = %q", backend.respawnCmds[0])
	}
}

func TestActivateSessionInitializesMainPaneIDOnDemand(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		SessionName:       "reactor-test",
		MainPaneHeightPct: 70,
		Backend:           backend,
	})
	r.state.Sessions["sess-1"] = state.Session{
		ID:      "sess-1",
		Command: "shell",
		Driver:  driver.NewGenericDriver("shell", "shell", 0).NewState(time.Now()),
		Frames:  []state.SessionFrame{{ID: "frame-1", Command: "shell", Driver: driver.NewGenericDriver("shell", "shell", 0).NewState(time.Now())}},
	}
	r.sessionPanes["frame-1"] = "%3"

	r.execute(state.EffActivateSession{
		SessionID: "sess-1",
		Reason:    state.EventPreviewSession,
	})

	if r.sessionPanes["_main"] != "%1" {
		t.Fatalf("sessionPanes[_main] = %q, want %%1", r.sessionPanes["_main"])
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.envs["ROOST_FRAME__main"] != "%1" {
		t.Fatalf("ROOST_FRAME__main = %q, want %%1", backend.envs["ROOST_FRAME__main"])
	}
	if backend.swapCalls != 1 {
		t.Fatalf("swapCalls = %d, want 1", backend.swapCalls)
	}
}

func TestActivateSessionMissingPaneEnqueuesWindowVanished(t *testing.T) {
	backend := newFakeBackend()
	backend.swapErr = fmt.Errorf("runtime: unknown pane %q: %w", "%3", ErrPaneMissing)
	r := New(Config{
		SessionName:       "reactor-test",
		MainPaneHeightPct: 70,
		Backend:           backend,
	})
	r.sessionPanes["_main"] = "%main"
	r.state.Sessions["sess-1"] = state.Session{
		ID:      "sess-1",
		Frames:  []state.SessionFrame{{ID: "frame-1", Command: "shell", Driver: driver.NewGenericDriver("shell", "shell", 0).NewState(time.Now())}},
		Command: "shell",
		Driver:  driver.NewGenericDriver("shell", "shell", 0).NewState(time.Now()),
	}
	r.sessionPanes["frame-1"] = "%3"

	r.execute(state.EffActivateSession{
		SessionID: "sess-1",
		Reason:    state.EventPreviewSession,
	})

	select {
	case ev := <-r.eventCh:
		v, ok := ev.(state.EvPaneWindowVanished)
		if !ok {
			t.Fatalf("event type = %T, want EvPaneWindowVanished", ev)
		}
		if v.FrameID != "frame-1" {
			t.Fatalf("FrameID = %q, want frame-1", v.FrameID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected EvPaneWindowVanished")
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	got := substitutePlaceholdersString("{sessionName}:0.1", "myroost", "/r")
	if got != "myroost:0.1" {
		t.Errorf("got %q", got)
	}
	got2 := substitutePlaceholdersString("{roostExe} --tui log", "x", "/r")
	if got2 != "/r --tui log" {
		t.Errorf("got %q", got2)
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
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
	})
	r.state.Sessions["abc"] = state.Session{
		ID:      "abc",
		Command: "stub-x",
		Driver:  driver.NewGenericDriver("", "", 0).NewState(time.Now()),
		Frames:  []state.SessionFrame{{ID: "abc-frame", Command: "stub-x", Driver: driver.NewGenericDriver("", "", 0).NewState(time.Now())}},
	}
	r.sessionPanes["abc-frame"] = "%5"
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

func TestFastTickDetectsActivePaneDeath(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["%42"] = false // frame pane destroyed
	r := New(Config{
		SessionName:      "reactor-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Backend:          backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.scheduleActiveFramePaneProbe()

	select {
	case ev := <-r.eventCh:
		pd, ok := ev.(state.EvPaneDied)
		if !ok {
			t.Fatalf("expected EvPaneDied, got %T", ev)
		}
		if pd.OwnerFrameID != "frame-1" {
			t.Errorf("OwnerFrameID = %q, want frame-1", pd.OwnerFrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvPaneDied to be enqueued within 500ms")
	}
}

func TestFastTickSkipsWhenNoActiveFrame(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		SessionName:      "reactor-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Backend:          backend,
	})
	// activeFrameID remains empty

	r.scheduleActiveFramePaneProbe()

	select {
	case ev := <-r.eventCh:
		t.Fatalf("expected no event, got %T", ev)
	case <-time.After(50 * time.Millisecond):
		// OK: no-op
	}
}

func TestFastTickIgnoresAliveActivePane(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["%42"] = true
	r := New(Config{
		SessionName:      "reactor-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Backend:          backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.scheduleActiveFramePaneProbe()

	select {
	case ev := <-r.eventCh:
		t.Fatalf("expected no event for alive pane, got %T", ev)
	case <-time.After(100 * time.Millisecond):
		// OK: no-op
	}
}

// Regression: when an active frame's program exits, the backend destroys the pane
// (remain-on-exit off) and the layout reflows — so positional target
// "{sessionName}:0.1" then resolves to a different, alive pane. The probe
// must target the frame's pane_id, not the positional slot.
func TestFastTickDetectsActivePaneDeathByPaneID(t *testing.T) {
	backend := newFakeBackend()
	// Frame pane is destroyed → dead at its pane_id.
	backend.alive["%42"] = false
	// Positional 0.1 now points at a different, alive pane (shifted up).
	backend.alive["reactor-test:0.1"] = true
	r := New(Config{
		SessionName:      "reactor-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Backend:          backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.scheduleActiveFramePaneProbe()

	select {
	case ev := <-r.eventCh:
		pd, ok := ev.(state.EvPaneDied)
		if !ok {
			t.Fatalf("expected EvPaneDied, got %T", ev)
		}
		if pd.OwnerFrameID != "frame-1" {
			t.Errorf("OwnerFrameID = %q, want frame-1", pd.OwnerFrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvPaneDied to be enqueued within 500ms")
	}
}

// Same scenario via the tick-driven EffCheckPaneAlive path.
func TestExecuteCheckPaneAliveResolvesActiveFramePaneID(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["%42"] = false
	backend.alive["reactor-test:0.1"] = true
	r := New(Config{
		SessionName: "reactor-test",
		Backend:     backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.executeCheckPaneAlive(state.EffCheckPaneAlive{Pane: "{sessionName}:0.1"})

	select {
	case ev := <-r.eventCh:
		pd, ok := ev.(state.EvPaneDied)
		if !ok {
			t.Fatalf("expected EvPaneDied, got %T", ev)
		}
		if pd.OwnerFrameID != "frame-1" {
			t.Errorf("OwnerFrameID = %q, want frame-1", pd.OwnerFrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvPaneDied within 500ms")
	}
}

// Repro for "Claude Driver session suddenly disappears": under load the backend
// `display-message` shell-out times out (context deadline exceeded). The active
// frame probe at interpret.go:240-247 treats ANY PaneAlive error as death and
// enqueues EvPaneDied, which evicts the still-alive session. A transient probe
// error must NOT be interpreted as pane death.
//
// Observed in ~/.agent-reactor/arc.log:
//
//	msg="runtime: active frame pane alive check failed" target=%5
//	  err="backend display-message -t %5 -p #{pane_dead}: context deadline exceeded: "
//	-> state: reducePaneDied entry -> evictFrame ok
func TestExecuteCheckPaneAliveTransientErrorDoesNotKillActiveFrame(t *testing.T) {
	backend := newFakeBackend()
	// The pane is actually alive, but the probe shell-out times out.
	backend.alive["%42"] = true
	backend.aliveErr["%42"] = fmt.Errorf("backend display-message -t %%42 -p #{pane_dead}: %w", context.DeadlineExceeded)
	r := New(Config{
		SessionName: "reactor-test",
		Backend:     backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.executeCheckPaneAlive(state.EffCheckPaneAlive{Pane: "{sessionName}:0.1"})

	select {
	case ev := <-r.eventCh:
		if pd, ok := ev.(state.EvPaneDied); ok {
			t.Fatalf("transient PaneAlive timeout must not kill the active frame, "+
				"but EvPaneDied was enqueued for owner %q", pd.OwnerFrameID)
		}
		t.Fatalf("unexpected event enqueued on transient probe error: %T", ev)
	case <-time.After(200 * time.Millisecond):
		// OK: a transient error is ignored; no eviction.
	}
}

// Guards against over-suppression: a genuine "can't find pane" error (the
// pane_id vanished with the process) must still be treated as death.
func TestExecuteCheckPaneAliveMissingPaneKillsActiveFrame(t *testing.T) {
	backend := newFakeBackend()
	backend.aliveErr["%42"] = fmt.Errorf("runtime: unknown pane %q: %w", "%42", ErrPaneMissing)
	r := New(Config{
		SessionName: "reactor-test",
		Backend:     backend,
	})
	r.activeFrameID = "frame-1"
	r.sessionPanes["frame-1"] = "%42"

	r.executeCheckPaneAlive(state.EffCheckPaneAlive{Pane: "{sessionName}:0.1"})

	select {
	case ev := <-r.eventCh:
		pd, ok := ev.(state.EvPaneDied)
		if !ok {
			t.Fatalf("expected EvPaneDied, got %T", ev)
		}
		if pd.OwnerFrameID != "frame-1" {
			t.Errorf("OwnerFrameID = %q, want frame-1", pd.OwnerFrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvPaneDied for a genuinely missing pane")
	}
}

// reconcileWindows must distinguish a vanished pane from a transient query
// failure: only errors that wrap ErrPaneMissing should evict an inactive frame.
func TestReconcileWindowsTransientErrorKeepsFrame(t *testing.T) {
	backend := newFakeBackend()
	backend.exitStatusErr["%7"] = fmt.Errorf("backend display-message -t %%7 -p ...: %w", context.DeadlineExceeded)
	r := New(Config{
		SessionName: "reactor-test",
		Backend:     backend,
	})
	r.activeFrameID = "active-frame"
	r.sessionPanes["inactive-frame"] = "%7"

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		t.Fatalf("transient PaneExitStatus error must not vanish a frame, got %T", ev)
	case <-time.After(200 * time.Millisecond):
		// OK: transient error ignored.
	}
}

func TestReconcileWindowsMissingPaneVanishesFrame(t *testing.T) {
	backend := newFakeBackend()
	backend.exitStatusErr["%7"] = fmt.Errorf("runtime: unknown pane %q: %w", "%7", ErrPaneMissing)
	r := New(Config{
		SessionName: "reactor-test",
		Backend:     backend,
	})
	r.activeFrameID = "active-frame"
	r.sessionPanes["inactive-frame"] = "%7"

	r.reconcileWindows()

	select {
	case ev := <-r.eventCh:
		vanished, ok := ev.(state.EvPaneWindowVanished)
		if !ok {
			t.Fatalf("expected EvPaneWindowVanished, got %T", ev)
		}
		if vanished.FrameID != "inactive-frame" {
			t.Errorf("FrameID = %q, want inactive-frame", vanished.FrameID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected EvPaneWindowVanished for a genuinely missing pane")
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
		SessionName:  "reactor-test",
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

func TestSpawnPaneWindowAsyncUsesPrepareLaunch(t *testing.T) {
	t.Skip("shared codex backend is runtime-managed; direct remote command is covered by codex backend tests")
}

func TestSpawnPaneWindowAsyncInjectsStreamPolicyEnv(t *testing.T) {
	t.Skip("stream policy is applied via runtime-owned codex backend, not helper env")
}

func TestReconcileDetectsVanishedPane(t *testing.T) {
	fbackend := newFakeBackend()
	fbackend.alive["%3"] = false
	fbackend.envs["ROOST_FRAME_tracked1"] = "%3"
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 20 * time.Millisecond,
		Backend:      fbackend,
	})
	drv := state.GetDriver("shell")
	r.state.Sessions[state.SessionID("tracked1")] = state.Session{
		ID:      state.SessionID("tracked1"),
		Command: "shell",
		Driver:  drv.NewState(time.Now()),
		Frames:  []state.SessionFrame{{ID: "tracked1", Command: "shell", Driver: drv.NewState(time.Now())}},
	}
	r.sessionPanes[state.FrameID("tracked1")] = "%3"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		fbackend.mu.Lock()
		_, stillSet := fbackend.envs["ROOST_FRAME_tracked1"]
		fbackend.mu.Unlock()
		if !stillSet {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	fbackend.mu.Lock()
	defer fbackend.mu.Unlock()
	if _, ok := fbackend.envs["ROOST_SESSION_tracked1"]; ok {
		t.Error("ROOST_SESSION_tracked1 should be unset after pane vanished")
	}
}

func TestReconcileSkipsWithoutTrackedPanes(t *testing.T) {
	fbackend := newFakeBackend()
	r := New(Config{
		SessionName:  "reactor-test",
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
		SessionName:  "reactor-test",
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

func TestActivateSessionSwapsOnFrameChange(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["%0"] = true // main pane
	backend.alive["%1"] = true // root frame pane
	backend.alive["%2"] = true // pushed frame pane
	backend.envs["ROOST_FRAME__main"] = "%0"

	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      &recordingPersist{},
	})

	drv := state.GetDriver("shell")
	sid := state.SessionID("sess-swap")
	rootFrameID := state.FrameID("frame-root")
	newFrameID := state.FrameID("frame-new")

	r.state.Sessions[sid] = state.Session{
		ID:      sid,
		Project: "/project",
		Command: "shell",
		Driver:  drv.NewState(time.Now()),
		Frames: []state.SessionFrame{
			{ID: rootFrameID, Project: "/project", Command: "shell", Driver: drv.NewState(time.Now())},
			{ID: newFrameID, Project: "/project", Command: "shell", Driver: drv.NewState(time.Now())},
		},
	}
	// Root frame is currently active in 0.0.
	r.sessionPanes[rootFrameID] = "%1"
	r.sessionPanes[newFrameID] = "%2"
	r.sessionPanes["_main"] = "%0"
	r.mainPaneSession = sid
	r.activeFrameID = rootFrameID // old frame — different from top-of-stack

	r.activateSession(sid)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.swapCalls != 1 {
		t.Fatalf("swapCalls = %d, want 1 (new frame should be swapped into 0.0)", backend.swapCalls)
	}
	// Swap source should be the new frame's pane.
	if backend.swapSources[0] != "%2" {
		t.Errorf("swap source = %q, want %%2 (new frame pane)", backend.swapSources[0])
	}
	if r.activeFrameID != newFrameID {
		t.Errorf("activeFrameID = %q, want %q", r.activeFrameID, newFrameID)
	}
}

func TestActivateSessionNoopWhenFrameUnchanged(t *testing.T) {
	backend := newFakeBackend()
	backend.alive["%0"] = true
	backend.alive["%1"] = true
	backend.envs["ROOST_FRAME__main"] = "%0"

	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      &recordingPersist{},
	})

	drv := state.GetDriver("shell")
	sid := state.SessionID("sess-noop")
	frameID := state.FrameID("frame-only")

	r.state.Sessions[sid] = state.Session{
		ID:      sid,
		Project: "/project",
		Command: "shell",
		Driver:  drv.NewState(time.Now()),
		Frames: []state.SessionFrame{
			{ID: frameID, Project: "/project", Command: "shell", Driver: drv.NewState(time.Now())},
		},
	}
	r.sessionPanes[frameID] = "%1"
	r.sessionPanes["_main"] = "%0"
	r.mainPaneSession = sid
	r.activeFrameID = frameID // already on the active frame

	r.activateSession(sid)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.swapCalls != 0 {
		t.Fatalf("swapCalls = %d, want 0 (same frame, no swap needed)", backend.swapCalls)
	}
}

// TestPopTopFrameSwapBeforeKill verifies Fix A: when the active top frame's pane
// dies, SwapPane (restoring the parent pane to 0.1) is called before
// KillPaneWindow (tearing down the top frame's window).
func TestPopTopFrameSwapBeforeKill(t *testing.T) {
	backend := newFakeBackend()
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      &recordingPersist{},
	})

	drv := state.GetDriver("shell")
	sid := state.SessionID("sess-pop")
	rootFrameID := state.FrameID("frame-root")
	topFrameID := state.FrameID("frame-top")

	r.state.Sessions[sid] = state.Session{
		ID:            sid,
		Project:       "/project",
		Command:       "shell",
		Driver:        drv.NewState(time.Now()),
		ActiveFrameID: topFrameID,
		Frames: []state.SessionFrame{
			{ID: rootFrameID, Project: "/project", Command: "shell", Driver: drv.NewState(time.Now())},
			{ID: topFrameID, Project: "/project", Command: "shell", Driver: drv.NewState(time.Now())},
		},
	}
	r.state.ActiveOccupant = state.OccupantFrame
	r.state.ActiveSession = sid
	r.sessionPanes[rootFrameID] = "%A"
	r.sessionPanes[topFrameID] = "%B"
	r.sessionPanes["_main"] = "%main"
	r.mainPaneSession = sid
	r.activeFrameID = topFrameID

	// Drive the pane-died event directly (no goroutines needed).
	next, effs := state.Reduce(r.state, state.EvPaneDied{
		Pane:         "{sessionName}:0.1",
		OwnerFrameID: topFrameID,
	})
	r.state = next
	for _, eff := range effs {
		r.execute(eff)
	}

	backend.mu.Lock()
	defer backend.mu.Unlock()

	swapIdx := -1
	killIdx := -1
	for i, call := range backend.callLog {
		switch call {
		case "swap":
			swapIdx = i
		case "kill":
			killIdx = i
		}
	}
	if swapIdx < 0 {
		t.Fatal("SwapPane was not called")
	}
	if killIdx < 0 {
		t.Fatal("KillPaneWindow was not called")
	}
	if swapIdx > killIdx {
		t.Errorf("SwapPane (callLog[%d]) must precede KillPaneWindow (callLog[%d])", swapIdx, killIdx)
	}
	if r.activeFrameID != rootFrameID {
		t.Errorf("activeFrameID = %q, want root %q", r.activeFrameID, rootFrameID)
	}
}

// TestSwapHiddenUsesPositionalTargets verifies that swapHidden() uses positional
// targets on every call so repeated toggles never produce a self-swap.
func TestSwapHiddenUsesPositionalTargets(t *testing.T) {
	fake := newFakeBackend()
	r := New(Config{SessionName: "reactor-test", Backend: fake})
	r.sessionPanes["_log"] = "%42" // must not appear as a SwapPane arg

	r.swapHidden()
	r.swapHidden()

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if fake.swapCalls != 2 {
		t.Fatalf("SwapPane call count = %d, want 2", fake.swapCalls)
	}
	wantSrc := r.hiddenPaneTarget()
	wantDst := r.mainPaneTarget()
	for i, src := range fake.swapSources {
		dst := fake.swapTargets[i]
		if src != wantSrc || dst != wantDst {
			t.Errorf("call %d: SwapPane(%q, %q), want (%q, %q)", i, src, dst, wantSrc, wantDst)
		}
	}
}
