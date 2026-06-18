package stream

import (
	"context"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
)

type fakeRuntime struct {
	events []state.Event
}

func (f *fakeRuntime) Enqueue(e state.Event) { f.events = append(f.events, e) }

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

func TestBackendKind(t *testing.T) {
	b := New(&fakeRuntime{}, nil, "sid", "sess1", "/p", "codex", nil, "", false, false, "/sock", 0)
	if b.Kind() != state.LaunchSubsystemStream {
		t.Errorf("Kind = %v", b.Kind())
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

func TestResolveDialSock(t *testing.T) {
	t.Run("host mode (no mounts) dials the listen path", func(t *testing.T) {
		const listen = "/host/run/codex/codex-x.sock"
		if got := resolveDialSock(listen, agentlaunch.WrappedLaunch{}); got != listen {
			t.Errorf("resolveDialSock() = %q, want %q", got, listen)
		}
	})

	t.Run("container mode maps the listen path to its bind-mount host path", func(t *testing.T) {
		got := resolveDialSock("/opt/agent-reactor/run/codex-x.sock", agentlaunch.WrappedLaunch{
			Mounts: []agentlaunch.Mount{{Host: "/home/u/.agent-reactor/run/4342aed7adbf", Container: "/opt/agent-reactor/run"}},
		})
		if want := "/home/u/.agent-reactor/run/4342aed7adbf/codex-x.sock"; got != want {
			t.Errorf("resolveDialSock() = %q, want %q", got, want)
		}
	})

	t.Run("unmapped listen path falls back unchanged", func(t *testing.T) {
		const listen = "/elsewhere/codex-x.sock"
		got := resolveDialSock(listen, agentlaunch.WrappedLaunch{
			Mounts: []agentlaunch.Mount{{Host: "/h/run", Container: "/opt/agent-reactor/run"}},
		})
		if got != listen {
			t.Errorf("resolveDialSock() = %q, want %q", got, listen)
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
