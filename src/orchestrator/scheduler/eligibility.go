package scheduler

import (
	"slices"
	"sort"
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// filterEligible returns the subset of cands eligible for dispatch per SPEC §8.2.
func filterEligible(cands []tracker.Issue, s State, cfg wfconfig.Config) []tracker.Issue {
	active := normSet(cfg.Tracker.ActiveStates)
	terminal := normSet(cfg.Tracker.TerminalStates)

	var out []tracker.Issue
	for _, iss := range cands {
		if !eligible(iss, s, active, terminal) {
			continue
		}
		out = append(out, iss)
	}
	return out
}

func eligible(iss tracker.Issue, s State, active, terminal map[string]bool) bool {
	if iss.ID == "" || iss.Identifier == "" || iss.Title == "" || iss.State == "" {
		return false
	}
	norm := strings.ToLower(iss.State)
	if !active[norm] || terminal[norm] {
		return false
	}
	if _, ok := s.Running[iss.ID]; ok {
		return false
	}
	if _, ok := s.Claimed[iss.ID]; ok {
		return false
	}
	// Defense-in-depth: RetryQueued issues are also in claimed (SPEC §7.1), but guard
	// explicitly here in case a future refactor breaks the claimed-retention invariant.
	if _, ok := s.RetryAttempts[iss.ID]; ok {
		return false
	}
	if strings.ToLower(iss.State) == "todo" && hasActiveBlocker(iss.BlockedBy, terminal) {
		return false
	}
	return true
}

// hasActiveBlocker reports whether any blocker is not in a terminal state.
func hasActiveBlocker(blockers []tracker.Blocker, terminal map[string]bool) bool {
	return slices.ContainsFunc(blockers, func(b tracker.Blocker) bool {
		return !terminal[strings.ToLower(b.State)]
	})
}

// sortCandidates sorts issues per SPEC §8.2: priority asc (nil last), created_at asc, identifier asc.
// The sort is stable to preserve tie-breaking order.
func sortCandidates(in []tracker.Issue) []tracker.Issue {
	out := make([]tracker.Issue, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		// priority: nil is last
		switch {
		case a.Priority == nil && b.Priority != nil:
			return false
		case a.Priority != nil && b.Priority == nil:
			return true
		case a.Priority != nil && b.Priority != nil:
			if *a.Priority != *b.Priority {
				return *a.Priority < *b.Priority
			}
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.Identifier < b.Identifier
	})
	return out
}

// normSet builds a lowercase lookup set from a slice of state names.
func normSet(states []string) map[string]bool {
	m := make(map[string]bool, len(states))
	for _, s := range states {
		m[strings.ToLower(s)] = true
	}
	return m
}
