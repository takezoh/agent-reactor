package runtime

import (
	"context"
	"strings"
	"testing"

	rsubsystem "github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
)

// panickingFactory is a subsystem factory whose Ensure() panics. Used to
// simulate a defect deep in spawn-pipeline code (devcontainer manager,
// launcher wrapper, …) panicking on a malformed input.
type panickingFactory struct {
	msg string
}

func (p *panickingFactory) Ensure(_ context.Context, _ state.SessionID, _ string, _ state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	panic(p.msg)
}

// TestSpawnPaneWindow_recoversFromPanicAndEmitsSpawnFailed verifies that a
// panic inside the spawn pipeline does NOT propagate out of the spawn
// goroutine (which would crash the daemon and kill every session inside —
// including the agent session that issued the POST /api/sessions). Instead,
// the panic is converted to EvSpawnFailed scoped to the one frame being
// spawned, and the daemon survives.
//
// Why this test exists: the user reported "POST /api/sessions returns 500
// and the server session that started this conversation terminates." Without
// the defer recover() in spawnPaneWindow, an upstream panic during
// ensureSubsystem / BindFrame / wrapLaunchForSpawn / backend SpawnFrame would
// unwind out of the goroutine and Go's runtime would kill the process.
// With it, the daemon stays up.
func TestSpawnPaneWindow_recoversFromPanicAndEmitsSpawnFailed(t *testing.T) {
	backend := newFakeBackend()
	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)

	deps := spawnDeps{
		backend:  backend,
		launcher: DirectLauncher{},
		factories: map[state.LaunchSubsystem]rsubsystem.Factory{
			state.LaunchSubsystemCLI: &panickingFactory{msg: "synthetic panic from test"},
		},
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}

	// This call MUST NOT panic. Without defer recover() in spawnPaneWindow
	// it would propagate the panic out of the goroutine; in a goroutine
	// that would crash the process, but synchronously here `testing` would
	// fail the test with the panic surface. Either way, panic = test fail.
	spawnPaneWindow(deps, state.EffSpawnFrame{
		SessionID: "s-survives", FrameID: "f-survives",
		Project: "/p", Command: "minimal-test",
	})

	select {
	case ev := <-eventCh:
		failed, ok := ev.(state.EvSpawnFailed)
		if !ok {
			t.Fatalf("expected EvSpawnFailed, got %T", ev)
		}
		if !strings.Contains(failed.Err, "spawn panicked") {
			t.Errorf("Err should mention 'spawn panicked' so operators can grep for it; got %q", failed.Err)
		}
		if !strings.Contains(failed.Err, "synthetic panic from test") {
			t.Errorf("Err should include the original panic message; got %q", failed.Err)
		}
		if failed.FrameID != "f-survives" {
			t.Errorf("FrameID = %q, want f-survives (scoped to the spawning frame)", failed.FrameID)
		}
	default:
		t.Fatal("no EvSpawnFailed emitted — the panic was swallowed silently and the reply will never be sent")
	}

	// No internalSpawnComplete on the failure path.
	select {
	case ev := <-internalCh:
		t.Fatalf("unexpected internal event on panic-recover path: %T", ev)
	default:
	}
}
