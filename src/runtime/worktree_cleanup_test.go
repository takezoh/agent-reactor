package runtime

import (
	"fmt"
	"testing"
	"time"

	"github.com/takezoh/agent-roost/state"
)

type wtState struct {
	state.DriverStateBase
	Path string
}

type wtDriver struct{}

func (wtDriver) Name() string                            { return "wt-test" }
func (wtDriver) DisplayName() string                     { return "wt-test" }
func (wtDriver) Status(_ state.DriverState) state.Status { return state.StatusStopped }
func (wtDriver) NewState(_ time.Time) state.DriverState  { return wtState{} }
func (wtDriver) PrepareLaunch(s state.DriverState, _ state.LaunchMode, project, cmd string, _ state.LaunchOptions, _ bool) (state.LaunchPlan, error) {
	return state.LaunchPlan{Command: cmd, StartDir: project}, nil
}
func (wtDriver) Persist(_ state.DriverState) map[string]string { return nil }
func (wtDriver) Restore(_ map[string]string, _ time.Time) state.DriverState {
	return wtState{}
}
func (wtDriver) View(_ state.DriverState) state.View { return state.View{} }
func (wtDriver) Step(prev state.DriverState, _ state.FrameContext, _ state.DriverEvent) (state.DriverState, []state.Effect, state.View) {
	return prev, nil, state.View{}
}
func (wtDriver) ManagedWorktreePath(s state.DriverState) string {
	if ws, ok := s.(wtState); ok {
		return ws.Path
	}
	return ""
}

func init() {
	state.Register(wtDriver{})
}

func makeWTSession(id state.SessionID, path string) state.Session {
	return state.Session{
		ID:      id,
		Command: "wt-test",
		Frames: []state.SessionFrame{
			{ID: state.FrameID(fmt.Sprintf("%s-f", id)), Command: "wt-test", Driver: wtState{Path: path}},
		},
	}
}

func TestCollectUntrackedWorktrees(t *testing.T) {
	listDir := func(dir string) ([]string, error) {
		switch dir {
		case "/repo1/.roost/worktrees":
			return []string{
				"/repo1/.roost/worktrees/tracked",
				"/repo1/.roost/worktrees/orphan",
			}, nil
		case "/repo2/.roost/worktrees":
			return []string{
				"/repo2/.roost/worktrees/orphan2",
			}, nil
		}
		return nil, nil
	}

	sessions := map[state.SessionID]state.Session{
		"s1": makeWTSession("s1", "/repo1/.roost/worktrees/tracked"),
	}

	got := collectUntrackedWorktrees(sessions, []string{"/repo1", "/repo2"}, listDir)

	want := map[string]struct{}{
		"/repo1/.roost/worktrees/orphan":  {},
		"/repo2/.roost/worktrees/orphan2": {},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %d entries", got, len(want))
	}
	for _, p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("unexpected path %q", p)
		}
	}
}

func TestCollectUntrackedWorktreesNoSessions(t *testing.T) {
	listDir := func(dir string) ([]string, error) {
		return []string{"/repo/.roost/worktrees/stale"}, nil
	}
	got := collectUntrackedWorktrees(nil, []string{"/repo"}, listDir)
	if len(got) != 1 || got[0] != "/repo/.roost/worktrees/stale" {
		t.Errorf("expected stale worktree, got %v", got)
	}
}

func TestCollectUntrackedWorktreesNoProjects(t *testing.T) {
	sessions := map[state.SessionID]state.Session{
		"s1": makeWTSession("s1", "/repo/.roost/worktrees/foo"),
	}
	got := collectUntrackedWorktrees(sessions, nil, func(string) ([]string, error) {
		return nil, nil
	})
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
