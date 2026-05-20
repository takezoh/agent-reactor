package runtime

import (
	"context"
	"sync/atomic"
	"testing"

	rsubsystem "github.com/takezoh/agent-roost/runtime/subsystem"
	"github.com/takezoh/agent-roost/state"
)

// fakeSubsystem records lifecycle calls for assertions.
type fakeSubsystem struct {
	id       state.SubsystemID
	kind     state.LaunchSubsystem
	startN   int32
	stopN    int32
	bindN    int32
	releaseN int32
}

func (f *fakeSubsystem) Kind() state.LaunchSubsystem { return f.kind }
func (f *fakeSubsystem) Start(_ context.Context) error {
	atomic.AddInt32(&f.startN, 1)
	return nil
}
func (f *fakeSubsystem) BindFrame(_ context.Context, _ rsubsystem.BindRequest) (rsubsystem.BindResult, error) {
	atomic.AddInt32(&f.bindN, 1)
	return rsubsystem.BindResult{}, nil
}
func (f *fakeSubsystem) ReleaseFrame(_ state.FrameID) { atomic.AddInt32(&f.releaseN, 1) }
func (f *fakeSubsystem) Stop(_ context.Context)       { atomic.AddInt32(&f.stopN, 1) }

// fakeFactory returns a pre-built fakeSubsystem keyed by SubsystemID.
type fakeFactory struct {
	sub *fakeSubsystem
}

func (f *fakeFactory) Ensure(_ context.Context, _ string, _ state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	return f.sub, f.sub.id, nil
}

func TestEnsureSubsystemDispatchesByKind(t *testing.T) {
	factories := map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystem("a"): &fakeFactory{sub: &fakeSubsystem{id: "fake:a", kind: state.LaunchSubsystem("a")}},
		state.LaunchSubsystem("b"): &fakeFactory{sub: &fakeSubsystem{id: "fake:b", kind: state.LaunchSubsystem("b")}},
	}
	a := factories[state.LaunchSubsystem("a")].(*fakeFactory).sub
	b := factories[state.LaunchSubsystem("b")].(*fakeFactory).sub

	sub, id, err := ensureSubsystemOnce(context.Background(), factories, state.LaunchSubsystem("a"), "/p", state.LaunchPlan{})
	if err != nil || sub != a || id != "fake:a" {
		t.Fatalf("kind a: got (%v, %q, %v), want (a, fake:a, nil)", sub, id, err)
	}
	sub, id, err = ensureSubsystemOnce(context.Background(), factories, state.LaunchSubsystem("b"), "/p", state.LaunchPlan{})
	if err != nil || sub != b || id != "fake:b" {
		t.Fatalf("kind b: got (%v, %q, %v), want (b, fake:b, nil)", sub, id, err)
	}
}

func TestEnsureSubsystemUnknownKindErrors(t *testing.T) {
	factories := map[state.LaunchSubsystem]rsubsystem.Factory{}
	_, _, err := ensureSubsystemOnce(context.Background(), factories, state.LaunchSubsystem("unknown"), "/p", state.LaunchPlan{})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestEnsureSubsystemEmptyKindDefaultsToCLI(t *testing.T) {
	a := &fakeSubsystem{id: "cli:default", kind: state.LaunchSubsystemCLI}
	factories := map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystemCLI: &fakeFactory{sub: a},
	}
	sub, _, err := ensureSubsystemOnce(context.Background(), factories, "", "/p", state.LaunchPlan{})
	if err != nil || sub != a {
		t.Fatalf("got (%v, %v), want (a, nil)", sub, err)
	}
}

func TestReleaseFrameSandboxesStopsAllSubsystems(t *testing.T) {
	a := &fakeSubsystem{id: "a", kind: state.LaunchSubsystemCLI}
	b := &fakeSubsystem{id: "b", kind: state.LaunchSubsystemStream}
	r := &Runtime{
		sandboxCleanups: map[state.FrameID]func() error{},
		subsystems: map[state.SubsystemID]rsubsystem.Subsystem{
			a.id: a,
			b.id: b,
		},
		frameSubsystems: map[state.FrameID]rsubsystem.Subsystem{},
	}

	r.execute(state.EffReleaseFrameSandboxes{})

	if atomic.LoadInt32(&a.stopN) != 1 {
		t.Errorf("subsystem a Stop calls = %d, want 1", a.stopN)
	}
	if atomic.LoadInt32(&b.stopN) != 1 {
		t.Errorf("subsystem b Stop calls = %d, want 1", b.stopN)
	}
}
