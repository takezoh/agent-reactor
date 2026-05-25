package cli

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

func TestFactoryEnsureSameProjectSameInstance(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	sub1, id1, err := f.Ensure(ctx, "", "/repo", state.LaunchPlan{})
	if err != nil {
		t.Fatalf("Ensure 1: %v", err)
	}
	sub2, id2, err := f.Ensure(ctx, "", "/repo", state.LaunchPlan{})
	if err != nil {
		t.Fatalf("Ensure 2: %v", err)
	}
	if sub1 != sub2 {
		t.Error("expected same Backend instance for same project")
	}
	if id1 != id2 {
		t.Errorf("ids differ: %q vs %q", id1, id2)
	}
	if id1 != state.SubsystemID("cli:/repo") {
		t.Errorf("id = %q, want cli:/repo", id1)
	}
}

func TestFactoryEnsureDifferentProjectsDifferentInstances(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	sub1, id1, _ := f.Ensure(ctx, "", "/repo-a", state.LaunchPlan{})
	sub2, id2, _ := f.Ensure(ctx, "", "/repo-b", state.LaunchPlan{})
	if sub1 == sub2 {
		t.Error("expected distinct Backend instances per project")
	}
	if id1 == id2 {
		t.Errorf("ids collided: %q", id1)
	}
}

func TestFactoryRangeVisitsAllBackends(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	_, _, _ = f.Ensure(ctx, "", "/repo-a", state.LaunchPlan{})
	_, _, _ = f.Ensure(ctx, "", "/repo-b", state.LaunchPlan{})
	visited := 0
	f.Range(func(_ *Backend) bool {
		visited++
		return true
	})
	if visited != 2 {
		t.Errorf("visited = %d, want 2", visited)
	}
}
