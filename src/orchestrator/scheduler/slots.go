package scheduler

import (
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

// retryReclaimHasSlot reports whether a RetryQueued issue may re-dispatch into a slot when
// its backoff timer fires (SPEC §8.3 / §8.4). The issue already holds a claim (RetryQueued ⊆
// claimed, §7.1), so promoting it back to Running does not consume a *new* global slot — its
// own reservation must therefore be excluded from the global count, otherwise an issue would
// count against itself and starve (e.g. a single issue under max_concurrent_agents=1 could
// never re-dispatch). The per-state limit still gates on the number actually Running in that
// state, which already excludes the (not-yet-running) firing issue.
func retryReclaimHasSlot(s State, cfg wfconfig.Config, issueID, state string) bool {
	globalUsed := len(s.Running) + retryQueuedCount(s)
	if _, queued := s.RetryAttempts[issueID]; queued {
		globalUsed-- // do not count our own held reservation against ourselves
	}
	if globalUsed >= cfg.Agent.MaxConcurrentAgents {
		return false
	}
	return availablePerStateSlots(state, s, cfg) > 0
}

// availableGlobalSlots returns the number of global dispatch slots remaining (SPEC §8.3).
// RetryQueued issues are claimed (SPEC §7.1) and occupy a slot during the backoff window.
func availableGlobalSlots(s State, cfg wfconfig.Config) int {
	used := len(s.Running) + retryQueuedCount(s)
	avail := cfg.Agent.MaxConcurrentAgents - used
	if avail < 0 {
		return 0
	}
	return avail
}

// retryQueuedCount returns the number of issues in RetryQueued state
// (in RetryAttempts but not yet promoted back to running).
func retryQueuedCount(s State) int {
	count := 0
	for id := range s.RetryAttempts {
		if _, running := s.Running[id]; !running {
			count++
		}
	}
	return count
}

// availablePerStateSlots returns dispatch slots for the given state (SPEC §8.3).
// If no per-state limit is configured, the global limit is used as the cap.
func availablePerStateSlots(state string, s State, cfg wfconfig.Config) int {
	norm := strings.ToLower(state)
	capacity, ok := cfg.Agent.MaxConcurrentAgentsByState[norm]
	if !ok {
		capacity = cfg.Agent.MaxConcurrentAgents
	}
	used := runningInState(s, norm)
	avail := capacity - used
	if avail < 0 {
		return 0
	}
	return avail
}

// runningInState counts running attempts whose issue state matches norm (lowercase).
func runningInState(s State, norm string) int {
	count := 0
	for _, run := range s.Running {
		if strings.ToLower(run.Issue.State) == norm {
			count++
		}
	}
	return count
}
