package scheduler

import (
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

// availableGlobalSlots returns the number of global dispatch slots remaining (SPEC §8.3).
// RetryQueued issues are claimed (SPEC §7.1) and occupy a slot during the backoff window.
func availableGlobalSlots(snap StateSnapshot, cfg wfconfig.Config) int {
	used := len(snap.Running) + retryQueuedCount(snap)
	avail := cfg.Agent.MaxConcurrentAgents - used
	if avail < 0 {
		return 0
	}
	return avail
}

// retryQueuedCount returns the number of issues in RetryQueued state
// (in retryAttempts but not yet promoted back to running).
func retryQueuedCount(snap StateSnapshot) int {
	count := 0
	for id := range snap.RetryAttempts {
		if _, running := snap.Running[id]; !running {
			count++
		}
	}
	return count
}

// availablePerStateSlots returns dispatch slots for the given state (SPEC §8.3).
// If no per-state limit is configured, the global limit is used as the cap.
func availablePerStateSlots(state string, snap StateSnapshot, cfg wfconfig.Config) int {
	norm := strings.ToLower(state)
	cap, ok := cfg.Agent.MaxConcurrentAgentsByState[norm]
	if !ok {
		cap = cfg.Agent.MaxConcurrentAgents
	}
	used := runningInState(snap, norm)
	avail := cap - used
	if avail < 0 {
		return 0
	}
	return avail
}

// runningInState counts running attempts whose issue state matches norm (lowercase).
func runningInState(snap StateSnapshot, norm string) int {
	count := 0
	for _, run := range snap.Running {
		if strings.ToLower(run.Issue.State) == norm {
			count++
		}
	}
	return count
}
