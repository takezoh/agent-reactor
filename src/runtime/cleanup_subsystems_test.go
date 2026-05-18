package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-roost/driver"
	"github.com/takezoh/agent-roost/state"
)

// These tests use drivers registered in TestMain (runtime_test.go):
// NewGenericDriver("", ""), NewGenericDriver("shell", "shell"), NewCodexDriver("").

func TestCollectTrackedWorktreesEmpty(t *testing.T) {
	s := state.State{Sessions: map[state.SessionID]state.Session{}}
	tracked := collectTrackedWorktrees(s)
	if len(tracked) != 0 {
		t.Errorf("got %v, want empty", tracked)
	}
}

func TestCollectTrackedWorktreesCodex(t *testing.T) {
	// Codex frame with a managed worktree path stored in driver state.
	worktreePath := "/repo/.roost/worktrees/alpha-beta"
	codexDrv := driver.NewCodexDriver("")
	ds := codexDrv.WithStartDir(codexDrv.NewState(time.Now()), worktreePath)

	s := state.State{
		Sessions: map[state.SessionID]state.Session{
			"s1": {
				Project: "/repo",
				Frames: []state.SessionFrame{
					{ID: "f1", Command: "codex", Driver: ds},
				},
			},
		},
	}
	tracked := collectTrackedWorktrees(s)
	if _, ok := tracked[worktreePath]; !ok {
		t.Errorf("tracked = %v, want %q", tracked, worktreePath)
	}
	if len(tracked) != 1 {
		t.Errorf("len(tracked) = %d, want 1", len(tracked))
	}
}

func TestCollectTrackedWorktreesNonWorktreeExcluded(t *testing.T) {
	// Frame whose StartDir is the plain project dir — must not appear in tracked.
	codexDrv := driver.NewCodexDriver("")
	ds := codexDrv.WithStartDir(codexDrv.NewState(time.Now()), "/repo")

	s := state.State{
		Sessions: map[state.SessionID]state.Session{
			"s1": {
				Project: "/repo",
				Frames:  []state.SessionFrame{{ID: "f1", Command: "codex", Driver: ds}},
			},
		},
	}
	tracked := collectTrackedWorktrees(s)
	if len(tracked) != 0 {
		t.Errorf("got %v, want empty", tracked)
	}
}

func TestCollectTrackedWorktreesMultipleFrames(t *testing.T) {
	// shell frame (generic driver) + codex frame, each with a managed worktree.
	shellDrv := driver.NewGenericDriver("shell", "shell", 0)
	shellDS := shellDrv.WithStartDir(shellDrv.NewState(time.Now()), "/repo/.roost/worktrees/shell-wt")

	codexDrv := driver.NewCodexDriver("")
	codexDS := codexDrv.WithStartDir(codexDrv.NewState(time.Now()), "/repo/.roost/worktrees/codex-wt")

	s := state.State{
		Sessions: map[state.SessionID]state.Session{
			"s1": {
				Project: "/repo",
				Frames: []state.SessionFrame{
					{ID: "f1", Command: "shell", Driver: shellDS},
					{ID: "f2", Command: "codex", Driver: codexDS},
				},
			},
		},
	}
	tracked := collectTrackedWorktrees(s)
	if len(tracked) != 2 {
		t.Fatalf("len(tracked) = %d, want 2", len(tracked))
	}
	for _, p := range []string{"/repo/.roost/worktrees/shell-wt", "/repo/.roost/worktrees/codex-wt"} {
		if _, ok := tracked[p]; !ok {
			t.Errorf("missing %q in tracked %v", p, tracked)
		}
	}
}
