package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
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
	r.sandboxCleanupsMu.Lock()
	remaining := len(r.sandboxCleanups)
	r.sandboxCleanupsMu.Unlock()
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
// Containers must survive so tmux panes stay alive for warm-restart adoption.
// The explicit shutdown path drains via EffReleaseFrameSandboxes (see
// TestEffReleaseFrameSandboxes_drainsCleanups).
func TestCtxCancel_doesNotDrainCleanups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called atomic.Bool
	r := New(Config{Tmux: newFakeTmux()})
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
	r := New(Config{Tmux: newFakeTmux()})
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

// TestEffDetachClient_doesNotDrainCleanups verifies that the detach path
// does not touch frame cleanups — containers must survive for warm-restart.
func TestEffDetachClient_doesNotDrainCleanups(t *testing.T) {
	var called atomic.Bool
	r := New(Config{Tmux: newFakeTmux()})
	r.storeFrameCleanup("f-detach", func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffDetachClient{})

	if called.Load() {
		t.Error("EffDetachClient must not drain frame cleanups")
	}
}

// TestEffKillSession_doesNotDrainCleanups verifies that EffKillSession alone
// does not drain cleanups — sandbox release is a separate EffReleaseFrameSandboxes
// effect that precedes EffKillSession in the shutdown sequence.
func TestEffKillSession_doesNotDrainCleanups(t *testing.T) {
	var called atomic.Bool
	r := New(Config{Tmux: newFakeTmux()})
	r.storeFrameCleanup("f-kill-session", func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffKillSession{})

	if called.Load() {
		t.Error("EffKillSession must not drain frame cleanups; use EffReleaseFrameSandboxes for that")
	}
}

// TestSpawnFrameWindow_cleanupCalledOnSpawnError verifies that when WrapLaunch
// returns a Cleanup callback but SpawnWindow subsequently fails, the Cleanup is
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

	tmux := newFakeTmux()
	tmux.spawnErr = errors.New("tmux spawn failed")

	r := New(Config{Tmux: tmux, Launcher: fakeLauncher})
	frame := state.SessionFrame{
		ID:      "frame-spawn-err",
		Command: "minimal-test",
		Project: "/test/project",
		Driver:  state.DriverStateBase{},
	}

	err := r.spawnFrameWindow("sess-1", state.SandboxOverrideAuto, frame, paneSize{width: 120, height: 40})
	if err == nil {
		t.Fatal("expected error from spawnFrameWindow, got nil")
	}
	if !cleanupCalled.Load() {
		t.Error("Cleanup was not called after SpawnWindow failure")
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

func TestEffKillSessionWindow_invokesCleanup(t *testing.T) {
	var called atomic.Bool
	tmux := &fakeTmux{}
	r := New(Config{Tmux: tmux})

	frameID := state.FrameID("f-kill")
	r.sessionPanes[frameID] = "%42"
	r.storeFrameCleanup(frameID, func() error {
		called.Store(true)
		return nil
	})

	r.execute(state.EffKillSessionWindow{FrameID: frameID})

	deadline := time.Now().Add(200 * time.Millisecond)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !called.Load() {
		t.Error("cleanup not called after EffKillSessionWindow")
	}
}
