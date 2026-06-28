package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/pathmap"
)

// minimalDriver is a zero-behaviour driver for testing bootstrap paths.
type minimalDriver struct{}

func (minimalDriver) Name() string        { return "minimal-test" }
func (minimalDriver) DisplayName() string { return "minimal-test" }
func (minimalDriver) Status(_ state.DriverState) state.Status {
	return state.StatusIdle
}
func (minimalDriver) NewState(_ time.Time) state.DriverState        { return state.DriverStateBase{} }
func (minimalDriver) Persist(_ state.DriverState) map[string]string { return nil }
func (minimalDriver) Restore(_ map[string]string, _ time.Time) state.DriverState {
	return state.DriverStateBase{}
}
func (minimalDriver) View(_ state.DriverState) state.View { return state.View{} }
func (minimalDriver) Step(prev state.DriverState, _ state.FrameContext, _ state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	return prev, nil, state.View{}
}
func (minimalDriver) PrepareLaunch(_ state.DriverState, _ state.LaunchMode, project, command string, _ state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	return state.LaunchPlan{Command: command, StartDir: project}, nil
}
func (minimalDriver) StartDir(_ state.DriverState) string                          { return "" }
func (minimalDriver) WithStartDir(s state.DriverState, _ string) state.DriverState { return s }

// TestRegisterContainerFrame_warmSaveIsSynchronous guards the 029 F4 fix:
// warm Save runs synchronously inside registerContainerFrame so it cannot
// race a follow-up Delete from executeKillSessionWindow. Before the fix,
// Save was fired off in a goroutine and could win against a kill-path Delete
// for the same frame's warm file, leaving a stale token on disk.
func TestRegisterContainerFrame_warmSaveIsSynchronous(t *testing.T) {
	dir := t.TempDir()
	r := New(Config{Backend: newFakeBackend(), DataDir: dir})
	t.Cleanup(r.shutdownContainerEndpoints)
	if r.warmFrames == nil {
		t.Fatal("warm-frame store not initialised with DataDir set")
	}
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/p",
		Frames: []state.SessionFrame{{ID: "f1", Project: "/p", Command: "shell"}},
	}

	r.registerContainerFrame("f1", "/p", dir, "tok-1", pathmap.Mounts{{Host: "/h", Container: "/c"}})

	// Synchronous contract: by the time registerContainerFrame returns the
	// warm state for the frame must be visible to LoadAll. No sleep, no poll.
	states, err := r.warmFrames.LoadAll()
	if err != nil {
		t.Fatalf("warmFrames.LoadAll: %v", err)
	}
	var got *WarmFrameState
	for i := range states {
		if states[i].FrameID == "f1" {
			got = &states[i]
			break
		}
	}
	if got == nil {
		t.Fatal("warm save did not land synchronously: f1 missing from LoadAll")
	}
	if got.ContainerToken != "tok-1" {
		t.Errorf("warm token = %q, want tok-1", got.ContainerToken)
	}
}

func TestStoreAndInvokeFrameCleanup(t *testing.T) {
	r := New(Config{})

	var called atomic.Bool
	r.storeFrameCleanup("f1", func() error {
		called.Store(true)
		return nil
	})

	r.invokeFrameCleanup("f1")

	// invokeFrameCleanup runs the fn in a goroutine; wait briefly.
	deadline := time.Now().Add(200 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !called.Load() {
		t.Error("cleanup fn was not called after invokeFrameCleanup")
	}

	// Second invoke for same frame should be a no-op (already deleted).
	called.Store(false)
	r.invokeFrameCleanup("f1")
	time.Sleep(20 * time.Millisecond)
	if called.Load() {
		t.Error("cleanup fn called twice for same frame")
	}
}

func TestInvokeFrameCleanup_noopWhenNil(t *testing.T) {
	r := New(Config{})
	// No cleanup registered; must not panic.
	r.invokeFrameCleanup("unknown")
}

func TestDrainFrameCleanups(t *testing.T) {
	r := New(Config{})

	var count atomic.Int32
	for _, id := range []state.FrameID{"f1", "f2", "f3"} {
		r.storeFrameCleanup(id, func() error {
			count.Add(1)
			return nil
		})
	}

	r.drainFrameCleanups()

	if got := count.Load(); got != 3 {
		t.Errorf("drain called %d cleanups, want 3", got)
	}

	// Map must be empty after drain.
	remaining := len(r.sandboxCleanups)
	if remaining != 0 {
		t.Errorf("frameCleanups has %d entries after drain, want 0", remaining)
	}
}

func TestInvokeFrameCleanup_errorLogged(t *testing.T) {
	r := New(Config{})
	r.storeFrameCleanup("ferr", func() error {
		return errors.New("container stop failed")
	})
	// Must not panic; the error is logged internally.
	r.invokeFrameCleanup("ferr")
	time.Sleep(20 * time.Millisecond)
}

func TestDirectLauncher_adoptFrame_noop(t *testing.T) {
	l := DirectLauncher{}
	cleanup, _, err := l.AdoptFrame(context.Background(), state.FrameID("f1"), "/workspace/foo")
	if err != nil {
		t.Fatalf("AdoptFrame returned error: %v", err)
	}
	if cleanup != nil {
		t.Error("DirectLauncher.AdoptFrame should return nil cleanup")
	}
}

// TestCtxCancel_doesNotDrainCleanups verifies that cancelling the runtime
// context (= daemon SIGINT / detach) does not invoke frame cleanup callbacks.
// Containers must survive so backend panes stay alive for warm-restart adoption.
// The explicit shutdown path drains via EffReleaseFrameSandboxes (see
// TestEffReleaseFrameSandboxes_drainsCleanups).
func TestCtxCancel_doesNotDrainCleanups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called atomic.Bool
	r := New(Config{Backend: newFakeBackend()})
	r.storeFrameCleanup("f-shutdown", func() error {
		called.Store(true)
		return nil
	})

	go func() { _ = r.Run(ctx) }()
	cancel()
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop within timeout")
	}

	// Allow a brief window for any async goroutines to run.
	time.Sleep(50 * time.Millisecond)
	if called.Load() {
		t.Error("frame cleanup must NOT be called on ctx cancel (warm-restart requires containers to survive)")
	}
}

// TestEffReleaseFrameSandboxes_drainsCleanups verifies that executing
// EffReleaseFrameSandboxes runs all registered per-frame cleanup closures.
// This is the explicit shutdown path (reduceShutdown emits this effect).
func TestEffReleaseFrameSandboxes_drainsCleanups(t *testing.T) {
	var count atomic.Int32
	r := New(Config{Backend: newFakeBackend()})
	for _, id := range []state.FrameID{"f1", "f2", "f3"} {
		r.storeFrameCleanup(id, func() error {
			count.Add(1)
			return nil
		})
	}

	r.execute(state.EffReleaseFrameSandboxes{})

	if got := count.Load(); got != 3 {
		t.Errorf("EffReleaseFrameSandboxes called %d cleanups, want 3", got)
	}
}

// TestSpawnFrameWindow_cleanupCalledOnSpawnError verifies that when WrapLaunch
// returns a Cleanup callback but SpawnFrame subsequently fails, the Cleanup is
// still invoked — preventing sandbox resource leaks (ref-count, containers).
func TestSpawnFrameWindow_cleanupCalledOnSpawnError(t *testing.T) {
	saved := state.GetRegistry()
	t.Cleanup(func() {
		state.ClearRegistry()
		for _, d := range saved {
			state.Register(d)
		}
	})
	if _, ok := saved[minimalDriver{}.Name()]; !ok {
		state.Register(minimalDriver{})
	}

	var cleanupCalled atomic.Bool
	fakeLauncher := &testLauncher{
		cleanup: func() error {
			cleanupCalled.Store(true)
			return nil
		},
	}

	backend := newFakeBackend()
	backend.spawnErr = errors.New("backend spawn failed")

	r := New(Config{Backend: backend, Launcher: fakeLauncher})
	frame := state.SessionFrame{
		ID:      "frame-spawn-err",
		Command: "minimal-test",
		Project: "/test/project",
		Driver:  state.DriverStateBase{},
	}

	err := r.spawnFrameWindow("sess-1", state.SandboxOverrideAuto, frame)
	if err == nil {
		t.Fatal("expected error from spawnFrameWindow, got nil")
	}
	if !cleanupCalled.Load() {
		t.Error("Cleanup was not called after SpawnFrame failure")
	}
}

// testLauncher is a WrapLaunch stub that injects a caller-supplied cleanup.
type testLauncher struct {
	cleanup func() error
}

func (l *testLauncher) WrapLaunch(_ state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	return WrappedLaunch{Command: plan.Command, StartDir: plan.StartDir, Env: env, Cleanup: l.cleanup}, nil
}

func (l *testLauncher) AdoptFrame(_ context.Context, _ state.FrameID, _ string) (func() error, pathmap.Mounts, error) {
	return nil, nil, nil
}

func (l *testLauncher) EnsureProject(_ context.Context, _ string) error { return nil }

func (l *testLauncher) IsContainer(_ string) bool { return false }

// TestEffKillSessionWindow_doesNotInvokeCleanup asserts that
// EffKillFrame is responsible for pane-backend window kill only
// — sandbox cleanup (Manager.ReleaseFrame → DestroyInstance) is driven
// by EffReleaseFrameSandbox emitted from the reducer for the same frame.
// Splitting the responsibilities lets EvFrameVanished release the
// container even though it skips the pane kill (`killWindow=false`).
func TestEffKillSessionWindow_doesNotInvokeCleanup(t *testing.T) {
	var called atomic.Bool
	backend := noopBackend{}
	r := New(Config{Backend: backend})

	frameID := state.FrameID("f-kill")
	r.storeFrameCleanup(frameID, func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffKillFrame{FrameID: frameID})

	time.Sleep(50 * time.Millisecond)
	if called.Load() {
		t.Error("cleanup must not be called by EffKillFrame alone; expected EffReleaseFrameSandbox to drive cleanup")
	}
}

// TestEffReleaseFrameSandbox_invokesCleanup asserts the new effect fires
// the per-frame cleanup closure (devcontainer.makeCleanup =
// Manager.ReleaseFrame → 0 なら DestroyInstance). reducer emits this
// effect for every evicted frame regardless of killWindow so EvPane-
// WindowVanished and reduceFrameCommandExited (abnormal exit) routes
// also free the container.
func TestEffReleaseFrameSandbox_invokesCleanup(t *testing.T) {
	var called atomic.Bool
	backend := noopBackend{}
	r := New(Config{Backend: backend})

	frameID := state.FrameID("f-release")
	r.storeFrameCleanup(frameID, func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffReleaseFrameSandbox{FrameID: frameID})

	deadline := time.Now().Add(200 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !called.Load() {
		t.Error("cleanup not called after EffReleaseFrameSandbox")
	}
}

// TestRequestShutdown_returnsAfterEffReleaseFrameSandboxes asserts the
// signal-handler contract: RequestShutdown enqueues EventShutdown and
// blocks until the EffReleaseFrameSandboxes handler closes the ack
// channel. Without this signal handlers would call ctx.cancel() before
// containers had a chance to teardown.
func TestRequestShutdown_returnsAfterEffReleaseFrameSandboxes(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	done := make(chan struct{})
	go func() {
		r.RequestShutdown(time.Second)
		close(done)
	}()
	// Simulate the event loop side: drain the enqueued shutdown and
	// execute the EffReleaseFrameSandboxes handler that the reducer
	// would have emitted.
	select {
	case <-r.eventCh:
	case <-time.After(time.Second):
		t.Fatal("RequestShutdown did not enqueue EvEvent within 1s")
	}
	r.execute(state.EffReleaseFrameSandboxes{})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RequestShutdown did not return after EffReleaseFrameSandboxes")
	}
}

// TestRequestShutdown_timesOutWhenLoopNeverDrains guards against the
// reverse failure: a wedged event loop must not pin the signal handler
// forever — RequestShutdown must return after the timeout so cancel()
// still runs and systemd's TimeoutStopSec= is honoured.
func TestRequestShutdown_timesOutWhenLoopNeverDrains(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	start := time.Now()
	r.RequestShutdown(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("RequestShutdown returned after %v; want ≤500ms", elapsed)
	}
}

// TestRequestShutdown_enqueueTimeout_releasesConcurrentWaiters guards
// R2-F1: if the EventShutdown send into eventCh exceeds timeout (a
// wedged event loop with a full buffer), the first caller used to leave
// shutdownAck non-nil and uncloseable. A second caller would then park
// on that stale ack forever. The fix closes the ack and clears the slot
// before returning so retries succeed and waiters unblock.
func TestRequestShutdown_enqueueTimeout_releasesConcurrentWaiters(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})

	// Saturate eventCh so the blocking send inside RequestShutdown
	// cannot make progress within the timeout.
	for i := 0; i < cap(r.eventCh); i++ {
		r.eventCh <- state.EvEvent{Event: "filler"}
	}

	done1 := make(chan struct{})
	go func() {
		r.RequestShutdown(50 * time.Millisecond)
		close(done1)
	}()
	// A second waiter that arrives while the first is parked on the
	// timer must NOT inherit the stale ack — once the first call
	// surrenders, the second should also unblock promptly (either by
	// running its own enqueue attempt or by sharing the closed ack).
	done2 := make(chan struct{})
	time.Sleep(5 * time.Millisecond)
	go func() {
		r.RequestShutdown(50 * time.Millisecond)
		close(done2)
	}()
	for _, ch := range []chan struct{}{done1, done2} {
		select {
		case <-ch:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("RequestShutdown waiter did not unblock after enqueue timeout")
		}
	}
}

// TestRequestShutdown_secondCallSharesAck asserts that a second caller
// (e.g. SIGTERM arriving twice) does not enqueue a duplicate shutdown
// event and waits on the same ack as the first.
func TestRequestShutdown_secondCallSharesAck(t *testing.T) {
	r := New(Config{Backend: noopBackend{}})
	done1 := make(chan struct{})
	go func() {
		r.RequestShutdown(time.Second)
		close(done1)
	}()
	// Wait for first call to enqueue.
	select {
	case <-r.eventCh:
	case <-time.After(time.Second):
		t.Fatal("first RequestShutdown did not enqueue")
	}
	done2 := make(chan struct{})
	go func() {
		r.RequestShutdown(time.Second)
		close(done2)
	}()
	// Give second call a moment to attach to the same ack; verify it
	// has NOT enqueued an additional event by checking the channel is
	// still empty after a short pause.
	time.Sleep(20 * time.Millisecond)
	select {
	case <-r.eventCh:
		t.Fatal("second RequestShutdown should not enqueue a duplicate shutdown event")
	default:
	}
	r.execute(state.EffReleaseFrameSandboxes{})
	for _, ch := range []chan struct{}{done1, done2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("both RequestShutdown calls should return after the single ack")
		}
	}
}
