package cli

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/runtime/subsystem"
	"github.com/takezoh/agent-roost/state"
)

func TestBackendKind(t *testing.T) {
	b := New("/repo")
	if b.Kind() != state.LaunchSubsystemCLI {
		t.Fatalf("Kind = %v, want LaunchSubsystemCLI", b.Kind())
	}
}

func TestBindFrameNoWorktree(t *testing.T) {
	b := New("/repo")
	req := subsystem.BindRequest{
		FrameID: state.FrameID("f1"),
		Plan: state.LaunchPlan{
			Command:  "claude",
			StartDir: "/repo",
		},
	}
	result, err := b.BindFrame(context.Background(), req)
	if err != nil {
		t.Fatalf("BindFrame error: %v", err)
	}
	if result.Plan.StartDir != "/repo" {
		t.Errorf("StartDir = %q, want /repo", result.Plan.StartDir)
	}
	if result.WorktreeStartDir != "" {
		t.Errorf("WorktreeStartDir = %q, want empty", result.WorktreeStartDir)
	}
	// Frame tracked with empty worktree path.
	b.mu.Lock()
	path := b.frames["f1"]
	b.mu.Unlock()
	if path != "" {
		t.Errorf("tracked path = %q, want empty", path)
	}
}

func TestBindFrameColdStartAdoption(t *testing.T) {
	b := New("/repo")
	worktreePath := "/repo/.roost/worktrees/test-name"
	req := subsystem.BindRequest{
		FrameID: state.FrameID("f2"),
		Plan: state.LaunchPlan{
			Command:  "claude",
			StartDir: worktreePath,
		},
	}
	result, err := b.BindFrame(context.Background(), req)
	if err != nil {
		t.Fatalf("BindFrame error: %v", err)
	}
	if result.WorktreeStartDir != worktreePath {
		t.Errorf("WorktreeStartDir = %q, want %q", result.WorktreeStartDir, worktreePath)
	}
	// StartDir unchanged (already pointing at worktree).
	if result.Plan.StartDir != worktreePath {
		t.Errorf("Plan.StartDir = %q, want %q", result.Plan.StartDir, worktreePath)
	}
	// Tracked.
	b.mu.Lock()
	path := b.frames["f2"]
	b.mu.Unlock()
	if path != worktreePath {
		t.Errorf("tracked path = %q, want %q", path, worktreePath)
	}
}

func TestReleaseFrameRemovesTracking(t *testing.T) {
	b := New("/repo")
	worktreePath := "/repo/.roost/worktrees/some-name"
	b.mu.Lock()
	b.frames["f3"] = worktreePath
	b.mu.Unlock()

	// RemoveWorktree is async and tries to call git — we can't run it in a unit
	// test without a real repo. Override with an empty path to avoid the call.
	b.mu.Lock()
	b.frames["f3"] = ""
	b.mu.Unlock()

	b.ReleaseFrame("f3")
	b.mu.Lock()
	_, ok := b.frames["f3"]
	b.mu.Unlock()
	if ok {
		t.Error("frame still tracked after ReleaseFrame")
	}
}

func TestCleanupUntrackedNoop(t *testing.T) {
	b := New(t.TempDir())
	// No .roost/worktrees/ dir exists — should not panic or error.
	b.CleanupUntracked(context.Background())
}

func TestIsManagedWorktreePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/repo/.roost/worktrees/alpha-beta", true},
		{"/repo/.roost/worktrees/x-y-z", true},
		{"/repo/main", false},
		{"/repo/.roost/alpha", false},
		{"", false},
	}
	for _, tc := range cases {
		got := subsystem.IsManagedWorktreePath(tc.path)
		if got != tc.want {
			t.Errorf("IsManagedWorktreePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGenerateWorktreeNames(t *testing.T) {
	names := subsystem.GenerateWorktreeNames(subsystem.WorktreeNameAttempts)
	if len(names) != subsystem.WorktreeNameAttempts {
		t.Fatalf("len(names) = %d, want %d", len(names), subsystem.WorktreeNameAttempts)
	}
	for _, name := range names {
		if name == "" {
			t.Fatal("got empty name")
		}
	}
}
