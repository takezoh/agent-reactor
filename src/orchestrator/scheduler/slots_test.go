package scheduler

import (
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func agentCfg(global int, byState map[string]int) wfconfig.Config {
	return wfconfig.Config{
		Agent: wfconfig.AgentConfig{
			MaxConcurrentAgents:        global,
			MaxConcurrentAgentsByState: byState,
		},
	}
}

func snapWithRunning(states ...string) State {
	st := NewState()
	for i, s := range states {
		id := strings.Repeat("x", i+1)
		st.Running[id] = RunAttempt{Issue: tracker.Issue{ID: id, State: s}}
	}
	return st
}

func TestAvailableGlobalSlots(t *testing.T) {
	cases := []struct {
		name    string
		running []string
		global  int
		want    int
	}{
		{"no running", nil, 3, 3},
		{"some running", []string{"In Progress"}, 3, 2},
		{"at cap", []string{"a", "b", "c"}, 3, 0},
		{"over cap (clamp)", []string{"a", "b", "c", "d"}, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := snapWithRunning(tc.running...)
			got := availableGlobalSlots(snap, agentCfg(tc.global, nil))
			if got != tc.want {
				t.Errorf("want %d, got %d", tc.want, got)
			}
		})
	}
}

// TestAvailableGlobalSlots_IncludesRetryQueued verifies that RetryQueued issues (SPEC §7.1)
// count against the global concurrency limit to prevent over-dispatch.
func TestAvailableGlobalSlots_IncludesRetryQueued(t *testing.T) {
	// 1 running + 1 retryQueued = 2 used; with global cap 3, only 1 slot left.
	snap := snapWithRunning("In Progress")
	snap.RetryAttempts = map[string]RetryEntry{
		"retry-1": {IssueID: "retry-1"},
	}
	got := availableGlobalSlots(snap, agentCfg(3, nil))
	if got != 1 {
		t.Errorf("want 1 available slot (3 - 1 running - 1 retryQueued), got %d", got)
	}

	// At cap: 2 running + 1 retryQueued = 3; no slots available.
	snap2 := snapWithRunning("In Progress", "Todo")
	snap2.RetryAttempts = map[string]RetryEntry{
		"retry-1": {IssueID: "retry-1"},
	}
	got2 := availableGlobalSlots(snap2, agentCfg(3, nil))
	if got2 != 0 {
		t.Errorf("want 0 available slots at cap, got %d", got2)
	}
}

func TestAvailablePerStateSlots(t *testing.T) {
	byState := map[string]int{"in progress": 2}

	t.Run("per-state cap used", func(t *testing.T) {
		snap := snapWithRunning("In Progress")
		got := availablePerStateSlots("In Progress", snap, agentCfg(5, byState))
		if got != 1 {
			t.Errorf("want 1, got %d", got)
		}
	})
	t.Run("per-state cap reached", func(t *testing.T) {
		snap := snapWithRunning("In Progress", "In Progress")
		got := availablePerStateSlots("In Progress", snap, agentCfg(5, byState))
		if got != 0 {
			t.Errorf("want 0, got %d", got)
		}
	})
	t.Run("no per-state entry falls back to global", func(t *testing.T) {
		snap := snapWithRunning("Todo")
		got := availablePerStateSlots("Todo", snap, agentCfg(5, byState))
		if got != 4 {
			t.Errorf("want 4 (global 5 - 1 running todo), got %d", got)
		}
	})
	t.Run("empty running for state", func(t *testing.T) {
		snap := snapWithRunning()
		got := availablePerStateSlots("In Progress", snap, agentCfg(5, byState))
		if got != 2 {
			t.Errorf("want 2, got %d", got)
		}
	})
}
