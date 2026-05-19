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

	"github.com/takezoh/agent-roost/driver"
	"github.com/takezoh/agent-roost/proto"
	"github.com/takezoh/agent-roost/state"
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

type fakeTmuxBackend struct {
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
	exitStatus       map[string]int // pane target → exit code (when dead)
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

func newFakeTmux() *fakeTmuxBackend {
	return &fakeTmuxBackend{
		alive:       map[string]bool{},
		exitStatus:  map[string]int{},
		envs:        map[string]string{},
		paneIDs:     map[string]string{},
		spawnWID:    "1",
		spawnPane:   "%1",
		breakNewWID: "9",
		paneWidth:   120,
		paneHeight:  40,
	}
}

func (f *fakeTmuxBackend) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
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

func (f *fakeTmuxBackend) ShowEnvironment() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.envOutput, nil
}

func (f *fakeTmuxBackend) KillPaneWindow(paneID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killCalls++
	f.killedPanes = append(f.killedPanes, paneID)
	f.callLog = append(f.callLog, "kill")
	return nil
}

func (f *fakeTmuxBackend) RunChain(ops ...[]string) error {
	return nil
}
func (f *fakeTmuxBackend) BreakPane(srcPane, dstWindow string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.breakCalls++
	f.breakTargets = append(f.breakTargets, dstWindow)
	return nil
}
func (f *fakeTmuxBackend) SwapPane(srcPane, dstPane string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.swapCalls++
	f.swapSources = append(f.swapSources, srcPane)
	f.swapTargets = append(f.swapTargets, dstPane)
	f.callLog = append(f.callLog, "swap")
	return f.swapErr
}
func (f *fakeTmuxBackend) BreakPaneToNewWindow(srcPane, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.breakNewCalls++
	f.breakNewNames = append(f.breakNewNames, name)
	return f.breakNewWID, nil
}
func (f *fakeTmuxBackend) JoinPane(srcPane, dstPane string, before bool, sizePct int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.joinCalls++
	f.joinSources = append(f.joinSources, srcPane)
	f.joinTargets = append(f.joinTargets, dstPane)
	return nil
}
func (f *fakeTmuxBackend) PaneID(target string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lookup := strings.Replace(target, ":=", ":", 1)
	if id, ok := f.paneIDs[lookup]; ok {
		if id == "error" {
			return "", fmt.Errorf("tmux error for %s", target)
		}
		return id, nil
	}
	if target == "roost-test:0.1" && f.spawnPane != "" {
		return f.spawnPane, nil
	}
	return "%main", nil
}
func (f *fakeTmuxBackend) PaneSize(string) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paneWidth, f.paneHeight, nil
}
func (f *fakeTmuxBackend) SelectPane(string) error { return nil }
func (f *fakeTmuxBackend) ResizeWindow(target string, width, height int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizeCalls++
	f.resizeTargets = append(f.resizeTargets, target)
	f.resizeWidths = append(f.resizeWidths, width)
	f.resizeHeights = append(f.resizeHeights, height)
	return nil
}
func (f *fakeTmuxBackend) SetStatusLine(line string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusLines = append(f.statusLines, line)
	return nil
}
func (f *fakeTmuxBackend) SetEnv(k, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envs[k] = v
	return nil
}
func (f *fakeTmuxBackend) UnsetEnv(k string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.envs, k)
	return nil
}
func (f *fakeTmuxBackend) PaneAlive(target string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.alive[target]
	if !ok {
		return true, nil
	}
	return v, nil
}
func (f *fakeTmuxBackend) PaneExitStatus(target string) (bool, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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
func (f *fakeTmuxBackend) RespawnPane(target, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.respawnCmds = append(f.respawnCmds, cmd)
	return nil
}
func (f *fakeTmuxBackend) CapturePane(string, int) (string, error) {
	return f.captured, nil
}
func (f *fakeTmuxBackend) DetachClient() error { return nil }
func (f *fakeTmuxBackend) KillSession() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionKillCalls++
	return nil
}
func (f *fakeTmuxBackend) DisplayPopup(w, h, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.popups = append(f.popups, cmd)
	return nil
}
func (f *fakeTmuxBackend) PipePane(string, string) error    { return nil }
func (f *fakeTmuxBackend) SendKeys(string, string) error    { return nil }
func (f *fakeTmuxBackend) SendKey(string, string) error     { return nil }
func (f *fakeTmuxBackend) LoadBuffer(string, string) error  { return nil }
func (f *fakeTmuxBackend) PasteBuffer(string, string) error { return nil }
func (f *fakeTmuxBackend) SendEnter(string) error           { return nil }

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
		SessionName:  "roost-test",
		RoostExe:     "/usr/local/bin/roost",
		TickInterval: 50 * time.Millisecond,
		Tmux:         newFakeTmux(),
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
	tmux := newFakeTmux()
	r := New(Config{
		SessionName: "roost-test",
		RoostExe:    "/usr/local/bin/roost",
		Tmux:        tmux,
	})

	r.execute(state.EffKillSession{})

	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.sessionKillCalls != 1 {
		t.Fatalf("sessionKillCalls = %d, want 1", tmux.sessionKillCalls)
	}
}

func TestSendResponseSyncFlushesImmediately(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	r := New(Config{
		SessionName: "roost-test",
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
	tmux := newFakeTmux()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second, // suppress periodic ticks
		Tmux:         tmux,
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
		tmux.mu.Lock()
		spawned := tmux.spawnCalls
		tmux.mu.Unlock()
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

	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.spawnCalls != 1 {
		t.Errorf("spawnCalls = %d, want 1", tmux.spawnCalls)
	}
	if tmux.resizeCalls == 0 {
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
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Millisecond,
		Tmux:         tmux,
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
	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if len(tmux.respawnCmds) != 0 {
		t.Errorf("expected 0 respawns when panes are alive, got %d", len(tmux.respawnCmds))
	}
}

func TestRuntimeRespawnsDeadPane(t *testing.T) {
	tmux := newFakeTmux()
	tmux.alive["roost-test:0.2"] = false // pane 0.2 is the sessions pane
	r := New(Config{
		SessionName:  "roost-test",
		RoostExe:     "/usr/bin/roost",
		TickInterval: 10 * time.Millisecond,
		Tmux:         tmux,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		tmux.mu.Lock()
		n := len(tmux.respawnCmds)
		tmux.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()
	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if len(tmux.respawnCmds) == 0 {
		t.Fatal("expected respawn for dead pane")
	}
	if tmux.respawnCmds[0] != "'/usr/bin/roost' --tui sessions" {
		t.Errorf("respawn cmd = %q", tmux.respawnCmds[0])
	}
}

func TestActivateSessionInitializesMainPaneIDOnDemand(t *testing.T) {
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:       "roost-test",
		MainPaneHeightPct: 70,
		Tmux:              tmux,
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
	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.envs["ROOST_FRAME__main"] != "%1" {
		t.Fatalf("ROOST_FRAME__main = %q, want %%1", tmux.envs["ROOST_FRAME__main"])
	}
	if tmux.swapCalls != 1 {
		t.Fatalf("swapCalls = %d, want 1", tmux.swapCalls)
	}
}

func TestActivateSessionMissingPaneEnqueuesWindowVanished(t *testing.T) {
	tmux := newFakeTmux()
	tmux.swapErr = fmt.Errorf("tmux swap-pane -d -s %%3 -t roost-test:0.0: exit status 1: can't find pane: %%3")
	r := New(Config{
		SessionName:       "roost-test",
		MainPaneHeightPct: 70,
		Tmux:              tmux,
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
		v, ok := ev.(state.EvTmuxWindowVanished)
		if !ok {
			t.Fatalf("event type = %T, want EvTmuxWindowVanished", ev)
		}
		if v.FrameID != "frame-1" {
			t.Fatalf("FrameID = %q, want frame-1", v.FrameID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected EvTmuxWindowVanished")
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
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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
		tmux.mu.Lock()
		n := tmux.killCalls
		tmux.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()
	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.killCalls != 1 {
		t.Errorf("killCalls = %d, want 1 (kill-window should fire)", tmux.killCalls)
	}
}

func TestFastTickDetectsActivePaneDeath(t *testing.T) {
	tmux := newFakeTmux()
	tmux.alive["%42"] = false // frame pane destroyed
	r := New(Config{
		SessionName:      "roost-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Tmux:             tmux,
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
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:      "roost-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Tmux:             tmux,
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
	tmux := newFakeTmux()
	tmux.alive["%42"] = true
	r := New(Config{
		SessionName:      "roost-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Tmux:             tmux,
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

// Regression: when an active frame's program exits, tmux destroys the pane
// (remain-on-exit off) and the layout reflows — so positional target
// "{sessionName}:0.1" then resolves to a different, alive pane. The probe
// must target the frame's pane_id, not the positional slot.
func TestFastTickDetectsActivePaneDeathByPaneID(t *testing.T) {
	tmux := newFakeTmux()
	// Frame pane is destroyed → dead at its pane_id.
	tmux.alive["%42"] = false
	// Positional 0.1 now points at a different, alive pane (shifted up).
	tmux.alive["roost-test:0.1"] = true
	r := New(Config{
		SessionName:      "roost-test",
		TickInterval:     10 * time.Second,
		FastTickInterval: 10 * time.Millisecond,
		Tmux:             tmux,
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
	tmux := newFakeTmux()
	tmux.alive["%42"] = false
	tmux.alive["roost-test:0.1"] = true
	r := New(Config{
		SessionName: "roost-test",
		Tmux:        tmux,
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

func TestRuntimeShellSessionSpawnsWithoutCommand(t *testing.T) {
	tmux := newFakeTmux()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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
		tmux.mu.Lock()
		spawned := tmux.spawnCalls
		tmux.mu.Unlock()
		if spawned >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.spawnCalls != 1 {
		t.Fatalf("spawnCalls = %d, want 1", tmux.spawnCalls)
	}
	if tmux.spawnCmds[0] != "" {
		t.Errorf("spawn command = %q, want empty (login shell)", tmux.spawnCmds[0])
	}
}

func TestRecreateAllUsesPrepareLaunch(t *testing.T) {
	t.Skip("shared codex backend is runtime-managed; helper command assertions are obsolete")
}

func TestSpawnTmuxWindowAsyncUsesPrepareLaunch(t *testing.T) {
	t.Skip("shared codex backend is runtime-managed; direct remote command is covered by codex backend tests")
}

func TestSpawnTmuxWindowAsyncInjectsStreamPolicyEnv(t *testing.T) {
	t.Skip("stream policy is applied via runtime-owned codex backend, not helper env")
}

func TestReconcileDetectsVanishedPane(t *testing.T) {
	ftmux := newFakeTmux()
	ftmux.alive["%3"] = false
	ftmux.envs["ROOST_FRAME_tracked1"] = "%3"
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 20 * time.Millisecond,
		Tmux:         ftmux,
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
		ftmux.mu.Lock()
		_, stillSet := ftmux.envs["ROOST_FRAME_tracked1"]
		ftmux.mu.Unlock()
		if !stillSet {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	ftmux.mu.Lock()
	defer ftmux.mu.Unlock()
	if _, ok := ftmux.envs["ROOST_SESSION_tracked1"]; ok {
		t.Error("ROOST_SESSION_tracked1 should be unset after pane vanished")
	}
}

func TestReconcileSkipsWithoutTrackedPanes(t *testing.T) {
	ftmux := newFakeTmux()
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 20 * time.Millisecond,
		Tmux:         ftmux,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	time.Sleep(60 * time.Millisecond)
	cancel()
	<-r.Done()

	ftmux.mu.Lock()
	defer ftmux.mu.Unlock()
	if ftmux.killCalls != 0 {
		t.Errorf("killCalls = %d, want 0 (no orphans)", ftmux.killCalls)
	}
}

func TestRuntimeEnqueueDoesNotBlock(t *testing.T) {
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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
	tmux := newFakeTmux()
	tmux.alive["%0"] = true // main pane
	tmux.alive["%1"] = true // root frame pane
	tmux.alive["%2"] = true // pushed frame pane
	tmux.envs["ROOST_FRAME__main"] = "%0"

	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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

	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.swapCalls != 1 {
		t.Fatalf("swapCalls = %d, want 1 (new frame should be swapped into 0.0)", tmux.swapCalls)
	}
	// Swap source should be the new frame's pane.
	if tmux.swapSources[0] != "%2" {
		t.Errorf("swap source = %q, want %%2 (new frame pane)", tmux.swapSources[0])
	}
	if r.activeFrameID != newFrameID {
		t.Errorf("activeFrameID = %q, want %q", r.activeFrameID, newFrameID)
	}
}

func TestActivateSessionNoopWhenFrameUnchanged(t *testing.T) {
	tmux := newFakeTmux()
	tmux.alive["%0"] = true
	tmux.alive["%1"] = true
	tmux.envs["ROOST_FRAME__main"] = "%0"

	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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

	tmux.mu.Lock()
	defer tmux.mu.Unlock()
	if tmux.swapCalls != 0 {
		t.Fatalf("swapCalls = %d, want 0 (same frame, no swap needed)", tmux.swapCalls)
	}
}

// TestPopTopFrameSwapBeforeKill verifies Fix A: when the active top frame's pane
// dies, SwapPane (restoring the parent pane to 0.1) is called before
// KillPaneWindow (tearing down the top frame's window).
func TestPopTopFrameSwapBeforeKill(t *testing.T) {
	tmux := newFakeTmux()
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         tmux,
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

	tmux.mu.Lock()
	defer tmux.mu.Unlock()

	swapIdx := -1
	killIdx := -1
	for i, call := range tmux.callLog {
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
	fake := newFakeTmux()
	r := New(Config{SessionName: "roost-test", Tmux: fake})
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
