package stream

import (
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
)

type fakeRuntime struct {
	events []state.Event
}

func (f *fakeRuntime) Enqueue(e state.Event) { f.events = append(f.events, e) }

// fakeDispatcher is a stub agentlaunch.Dispatcher for unit tests.
type fakeDispatcher struct {
	container bool
}

func (d *fakeDispatcher) IsContainer(_ string) bool { return d.container }
func (d *fakeDispatcher) Wrap(_ context.Context, _ string, plan agentlaunch.LaunchPlan) (agentlaunch.WrappedLaunch, error) {
	return agentlaunch.WrappedLaunch{Argv: plan.Argv}, nil
}
func (d *fakeDispatcher) AdoptFrame(_ context.Context, _, _ string) (func(context.Context) error, []agentlaunch.Mount, error) {
	return nil, nil, nil
}
func (d *fakeDispatcher) EnsureProject(_ context.Context, _ string) error { return nil }

func TestStopBeforeStartIsNoop(t *testing.T) {
	b, _ := newTestBackend()
	// Never Started: cancel and done are nil. Stop must not panic or block.
	b.Stop(context.Background())
}

func TestStopCancelsAndWaitsForReap(t *testing.T) {
	b, _ := newTestBackend()
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.done = make(chan struct{})
	// Emulate waitProcess: closes done once the subsystem ctx is cancelled.
	go func() {
		<-b.ctx.Done()
		close(b.done)
	}()

	start := time.Now()
	b.Stop(context.Background())
	if elapsed := time.Since(start); elapsed >= stopGrace {
		t.Fatalf("Stop blocked %v (>= grace %v); did not observe reap", elapsed, stopGrace)
	}
	select {
	case <-b.done:
	default:
		t.Fatal("Stop returned before done was closed")
	}
}

func TestBackendKindAndBridgePort(t *testing.T) {
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock", "/csock", 1234, nil, 0)
	if b.Kind() != state.LaunchSubsystemStream {
		t.Errorf("Kind = %v", b.Kind())
	}
	if b.BridgePort() != 1234 {
		t.Errorf("BridgePort = %d", b.BridgePort())
	}
}

func TestReleaseFrameAndLookup(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	if got := b.frameForThread("t1"); got != "f1" {
		t.Errorf("frameForThread = %q", got)
	}
	if got := b.frameForThread(""); got != "" {
		t.Errorf("empty threadID should return empty FrameID, got %q", got)
	}
	if got := b.frameForThread("unknown"); got != "" {
		t.Errorf("unknown should return empty, got %q", got)
	}

	b.ReleaseFrame("f1")
	b.mu.Lock()
	_, frameOK := b.frames["f1"]
	_, threadOK := b.threads["t1"]
	b.mu.Unlock()
	if frameOK || threadOK {
		t.Errorf("ReleaseFrame did not clean up: frames=%v threads=%v", frameOK, threadOK)
	}

	// idempotent
	b.ReleaseFrame("nonexistent")
}

func TestChooseSockPath(t *testing.T) {
	const hostSock = "/host/codex.sock"
	const ctrSock = "/container/codex.sock"

	t.Run("nil dispatcher uses host sock", func(t *testing.T) {
		b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, hostSock, ctrSock, 0, nil, 0)
		if got := b.chooseSockPath(); got != hostSock {
			t.Errorf("chooseSockPath() = %q, want host sock %q", got, hostSock)
		}
	})

	t.Run("dispatcher IsContainer=false uses host sock", func(t *testing.T) {
		b := New(&fakeRuntime{}, &fakeDispatcher{container: false}, "sid", "sess1", "/p", "codex", nil, "", false, false, hostSock, ctrSock, 0, nil, 0)
		if got := b.chooseSockPath(); got != hostSock {
			t.Errorf("chooseSockPath() = %q, want host sock %q", got, hostSock)
		}
	})

	t.Run("dispatcher IsContainer=true uses container sock", func(t *testing.T) {
		b := New(&fakeRuntime{}, &fakeDispatcher{container: true}, "sid", "sess1", "/p", "codex", nil, "", false, false, hostSock, ctrSock, 0, nil, 0)
		if got := b.chooseSockPath(); got != ctrSock {
			t.Errorf("chooseSockPath() = %q, want container sock %q", got, ctrSock)
		}
	})
}

func TestFactoryRange(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	f.backends["a"] = &Backend{subsystemID: "a"}
	f.backends["b"] = &Backend{subsystemID: "b"}
	seen := map[state.SubsystemID]bool{}
	f.Range(func(b *Backend) bool {
		seen[b.subsystemID] = true
		return true
	})
	if len(seen) != 2 {
		t.Errorf("Range visited %d, want 2", len(seen))
	}
	// early termination
	count := 0
	f.Range(func(*Backend) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("early-stop visited %d, want 1", count)
	}
}
