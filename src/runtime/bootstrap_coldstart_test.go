package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/lib/pathmap"
	"github.com/takezoh/agent-roost/state"
)

type trackingLauncher struct {
	mu          sync.Mutex
	calls       map[string]int
	delay       time.Duration
	failOn      string
	wrapCalled  bool
	lastSandbox state.SandboxOverride
}

func (l *trackingLauncher) WrapLaunch(_ state.FrameID, plan state.LaunchPlan, env map[string]string) (WrappedLaunch, error) {
	l.mu.Lock()
	l.wrapCalled = true
	l.lastSandbox = plan.Sandbox
	l.mu.Unlock()
	return WrappedLaunch{Command: plan.Command, StartDir: plan.StartDir, Env: env}, nil
}

func (l *trackingLauncher) AdoptFrame(_ context.Context, _ state.FrameID, _ string) (func() error, pathmap.Mounts, error) {
	return nil, nil, nil
}

func (l *trackingLauncher) IsContainer(_ string) bool { return false }

func (l *trackingLauncher) EnsureProject(_ context.Context, projectPath string) error {
	if l.delay > 0 {
		time.Sleep(l.delay)
	}
	l.mu.Lock()
	l.calls[projectPath]++
	l.mu.Unlock()
	if projectPath == l.failOn {
		return errors.New("injected failure")
	}
	return nil
}

func makeRuntimeWithProjects(projects []string, launcher AgentLauncher) *Runtime {
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         noopTmux{},
		Launcher:     launcher,
	})
	r.SetSandboxedProjectResolver(func(string) bool { return true })
	for i, p := range projects {
		sid := state.SessionID("s" + string(rune('1'+i)))
		fid := state.FrameID("f" + string(rune('1'+i)))
		r.state.Sessions[sid] = state.Session{
			ID:      sid,
			Project: p,
			Frames: []state.SessionFrame{{
				ID:      fid,
				Project: p,
				Command: "shell",
			}},
		}
	}
	return r
}

func TestPrewarmContainers_Parallel(t *testing.T) {
	const delay = 100 * time.Millisecond
	const n = 3
	projects := []string{"/proj/a", "/proj/b", "/proj/c"}
	l := &trackingLauncher{calls: make(map[string]int), delay: delay}
	r := makeRuntimeWithProjects(projects, l)

	start := time.Now()
	r.PrewarmContainers(context.Background())
	elapsed := time.Since(start)

	// Parallel execution should finish well under n*delay
	if elapsed >= time.Duration(n)*delay {
		t.Errorf("elapsed %v >= %v; containers were not started in parallel", elapsed, time.Duration(n)*delay)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	for _, p := range projects {
		if l.calls[p] != 1 {
			t.Errorf("EnsureProject(%q) called %d times, want 1", p, l.calls[p])
		}
	}
}

func TestPrewarmContainers_DeduplicatesProject(t *testing.T) {
	project := "/proj/shared"
	l := &trackingLauncher{calls: make(map[string]int)}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         noopTmux{},
		Launcher:     l,
	})
	r.SetSandboxedProjectResolver(func(string) bool { return true })
	// Two frames pointing at the same project
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: project,
		Frames: []state.SessionFrame{
			{ID: "f1", Project: project, Command: "shell"},
			{ID: "f2", Project: project, Command: "shell"},
		},
	}

	r.PrewarmContainers(context.Background())

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls[project] != 1 {
		t.Errorf("EnsureProject called %d times for shared project, want 1", l.calls[project])
	}
}

func TestPrewarmContainers_FailureDoesNotAbort(t *testing.T) {
	projects := []string{"/proj/ok", "/proj/fail"}
	l := &trackingLauncher{calls: make(map[string]int), failOn: "/proj/fail"}
	r := makeRuntimeWithProjects(projects, l)

	r.PrewarmContainers(context.Background()) // must not panic or block

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.calls["/proj/ok"] != 1 {
		t.Errorf("ok project not warmed, calls=%d", l.calls["/proj/ok"])
	}
	if l.calls["/proj/fail"] != 1 {
		t.Errorf("failing project EnsureProject not called, calls=%d", l.calls["/proj/fail"])
	}
}

func TestPrewarmContainers_SkipsNonSandboxed(t *testing.T) {
	l := &trackingLauncher{calls: make(map[string]int)}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         noopTmux{},
		Launcher:     l,
	})
	r.SetSandboxedProjectResolver(func(string) bool { return false })
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/proj/local",
		Frames: []state.SessionFrame{{ID: "f1", Project: "/proj/local", Command: "shell"}},
	}

	r.PrewarmContainers(context.Background())

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) != 0 {
		t.Errorf("EnsureProject called for non-sandboxed project: %v", l.calls)
	}
}

func TestPrewarmContainers_NoSessionsIsNoop(t *testing.T) {
	l := &trackingLauncher{calls: make(map[string]int)}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         noopTmux{},
		Launcher:     l,
	})
	r.SetSandboxedProjectResolver(func(string) bool { return true })

	r.PrewarmContainers(context.Background()) // must not panic

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) != 0 {
		t.Errorf("expected no calls with empty sessions, got %v", l.calls)
	}
}

func TestPrewarmContainers_SkipsHostOnlyProject(t *testing.T) {
	l := &trackingLauncher{calls: make(map[string]int)}
	r := New(Config{
		SessionName:  "roost-test",
		TickInterval: 10 * time.Second,
		Tmux:         noopTmux{},
		Launcher:     l,
	})
	r.SetSandboxedProjectResolver(func(string) bool { return true })
	r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/proj/host",
		Sandbox: state.SandboxOverrideHost,
		Frames: []state.SessionFrame{
			{ID: "f1", Project: "/proj/host", Command: "shell"},
			{ID: "f2", Project: "/proj/host", Command: "shell"},
		},
	}

	r.PrewarmContainers(context.Background())

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) != 0 {
		t.Errorf("EnsureProject called for host-only project: %v", l.calls)
	}
}

func registerMinimalDriver(t *testing.T) {
	t.Helper()
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
}

func TestSpawnFrameWindow_StreamSubsystemInjectsEndpointDir(t *testing.T) {
	t.Skip("endpoint-dir injection was removed with the codex helper")
}

func TestSpawnFrameWindow_SandboxOptionOnColdStart(t *testing.T) {
	tests := []struct {
		name        string
		sandbox     state.SandboxOverride
		wantSandbox state.SandboxOverride
	}{
		{"host override propagates", state.SandboxOverrideHost, state.SandboxOverrideHost},
		{"auto does not become host", state.SandboxOverrideAuto, state.SandboxOverrideAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registerMinimalDriver(t)
			l := &trackingLauncher{calls: make(map[string]int)}
			r := New(Config{
				SessionName: "roost-test",
				Tmux:        newFakeTmux(),
				Launcher:    l,
			})
			r.SetSandboxedProjectResolver(func(string) bool { return true })
			frame := state.SessionFrame{
				ID:      "f1",
				Project: "/proj/sandboxed",
				Command: "minimal-test",
				Driver:  state.DriverStateBase{},
			}
			if err := r.spawnFrameWindow("s1", tt.sandbox, frame, paneSize{width: 120, height: 40}); err != nil {
				t.Fatalf("spawnFrameWindow: %v", err)
			}
			l.mu.Lock()
			defer l.mu.Unlock()
			if !l.wrapCalled {
				t.Fatal("WrapLaunch was not called")
			}
			if l.lastSandbox != tt.wantSandbox {
				t.Errorf("WrapLaunch received Sandbox=%v, want %v", l.lastSandbox, tt.wantSandbox)
			}
		})
	}
}
