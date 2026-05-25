package cli

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
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

// TestBindFrameColdStartOwnerReadoption verifies that a frame re-adopting its
// OWN managed worktree on cold-start (Enabled=true) is registered for cleanup.
func TestBindFrameColdStartOwnerReadoption(t *testing.T) {
	b := New("/repo")
	worktreePath := "/repo/.roost/worktrees/test-name"
	req := subsystem.BindRequest{
		FrameID: state.FrameID("f2"),
		Plan: state.LaunchPlan{
			Command:  "claude",
			StartDir: worktreePath,
			// Enabled=true: persisted from original creation, signals ownership.
			Options: state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: true}},
		},
	}
	result, err := b.BindFrame(context.Background(), req)
	if err != nil {
		t.Fatalf("BindFrame error: %v", err)
	}
	if result.WorktreeStartDir != worktreePath {
		t.Errorf("WorktreeStartDir = %q, want %q", result.WorktreeStartDir, worktreePath)
	}
	if result.Plan.StartDir != worktreePath {
		t.Errorf("Plan.StartDir = %q, want %q", result.Plan.StartDir, worktreePath)
	}
	// Owner re-adoption: tracked for cleanup.
	b.mu.Lock()
	path := b.frames["f2"]
	b.mu.Unlock()
	if path != worktreePath {
		t.Errorf("tracked path = %q, want %q (owner must be tracked)", path, worktreePath)
	}
}

// TestBindFrameBorrowerAdoption verifies that a child frame borrowing another
// frame's managed worktree (Enabled=false) is NOT registered for cleanup.
// This prevents cross-frame or cross-backend deletion of a shared worktree.
func TestBindFrameBorrowerAdoption(t *testing.T) {
	b := New("/repo")
	worktreePath := "/repo/.roost/worktrees/shared-name"
	req := subsystem.BindRequest{
		FrameID: state.FrameID("child"),
		Plan: state.LaunchPlan{
			Command:  "bash",
			StartDir: worktreePath,
			// Enabled=false: child inherited StartDir but did not create this worktree.
		},
	}
	result, err := b.BindFrame(context.Background(), req)
	if err != nil {
		t.Fatalf("BindFrame error: %v", err)
	}
	// WorktreeStartDir is still set so the process runs in the correct directory.
	if result.WorktreeStartDir != worktreePath {
		t.Errorf("WorktreeStartDir = %q, want %q", result.WorktreeStartDir, worktreePath)
	}
	if result.Plan.StartDir != worktreePath {
		t.Errorf("Plan.StartDir = %q, want %q", result.Plan.StartDir, worktreePath)
	}
	// Borrower: NOT registered for cleanup so it cannot delete a shared worktree.
	b.mu.Lock()
	path := b.frames["child"]
	b.mu.Unlock()
	if path != "" {
		t.Errorf("tracked path = %q, want empty (borrower must not be tracked for cleanup)", path)
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

// TestReleaseFrameSharedWorktreeNotRemovedUntilLast verifies that a worktree
// shared between two frames (root + child) is not deleted when the first frame
// releases — only when the last frame holding it releases.
func TestReleaseFrameSharedWorktreeNotRemovedUntilLast(t *testing.T) {
	b := New("/repo")
	sharedPath := "/repo/.roost/worktrees/shared-tree"

	// Manually register two frames pointing at the same worktree path
	// (simulating root that created it and a child that adopted it).
	b.mu.Lock()
	b.frames[state.FrameID("root")] = sharedPath
	b.frames[state.FrameID("child")] = sharedPath
	b.mu.Unlock()

	// Release the child frame. The worktree should still be tracked by root.
	b.ReleaseFrame(state.FrameID("child"))
	b.mu.Lock()
	_, childGone := b.frames[state.FrameID("child")]
	rootPath := b.frames[state.FrameID("root")]
	b.mu.Unlock()
	if childGone {
		t.Error("child frame still tracked after ReleaseFrame")
	}
	if rootPath != sharedPath {
		t.Errorf("root frame path = %q, want %q (must not be removed while root still holds it)", rootPath, sharedPath)
	}

	// Release the root frame. Now no frame holds the path; use empty path to
	// skip the async RemoveWorktree call (no real git repo in unit tests).
	b.mu.Lock()
	b.frames[state.FrameID("root")] = ""
	b.mu.Unlock()
	b.ReleaseFrame(state.FrameID("root"))
	b.mu.Lock()
	_, rootGone := b.frames[state.FrameID("root")]
	b.mu.Unlock()
	if rootGone {
		t.Error("root frame still tracked after ReleaseFrame")
	}
}
