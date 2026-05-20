package stream

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

type fakeRuntime struct {
	events []state.Event
}

func (f *fakeRuntime) Enqueue(e state.Event) { f.events = append(f.events, e) }
func (f *fakeRuntime) ContainerExecConfig(context.Context, string) (*ContainerExecConfig, error) {
	return nil, nil
}

func TestBackendKindAndBridgePort(t *testing.T) {
	b := New(&fakeRuntime{}, "sid", "/p", "codex", nil, "", false, false, "/sock", "/csock", 1234, nil, 0)
	if b.Kind() != state.LaunchSubsystemStream {
		t.Errorf("Kind = %v", b.Kind())
	}
	if b.BridgePort() != 1234 {
		t.Errorf("BridgePort = %d", b.BridgePort())
	}
}

func TestReleaseFrameAndLookup(t *testing.T) {
	b := New(&fakeRuntime{}, "sid", "/p", "codex", nil, "", false, false, "/sock", "/csock", 0, nil, 0)
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

func TestIsContainerProject(t *testing.T) {
	b := New(&fakeRuntime{}, "sid", "/p", "codex", nil, "", false, false, "/sock", "/csock", 0, nil, 0)
	ok, err := b.isContainerProject(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("nil container config should be host mode")
	}
}
