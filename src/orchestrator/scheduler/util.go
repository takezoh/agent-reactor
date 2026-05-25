package scheduler

import "sort"

// sortedRunningIDs returns the running issue IDs in deterministic (sorted) order.
// Reduce must iterate maps deterministically so its emitted Effect ordering is a
// pure function of its inputs (Go map iteration order is randomized).
func sortedRunningIDs(s State) []string {
	ids := make([]string, 0, len(s.Running))
	for id := range s.Running {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// sortedIDs returns a sorted copy of ids (deterministic iteration).
func sortedIDs(ids []string) []string {
	out := make([]string, len(ids))
	copy(out, ids)
	sort.Strings(out)
	return out
}
