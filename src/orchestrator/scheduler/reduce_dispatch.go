package scheduler

import (
	"strings"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// reduceCandidates implements one dispatch pass (SPEC §8.1–§8.3): filter eligible, sort,
// allocate up to the available global and per-state slots, and claim each winner. Each
// claimed issue emits an EffRevalidate (re-verify immediately before spawn, §16.4); the
// spawn itself follows once EvRevalidated confirms the issue is still active.
func reduceCandidates(s State, cands []tracker.Issue, cfg wfconfig.Config) (State, []Effect) {
	eligibles := sortCandidates(filterEligible(cands, s, cfg))

	globalAvail := availableGlobalSlots(s, cfg)
	perStateUsed := make(map[string]int)
	for _, run := range s.Running {
		perStateUsed[strings.ToLower(run.Issue.State)]++
	}

	var effs []Effect
	for _, iss := range eligibles {
		if globalAvail <= 0 {
			break
		}
		norm := strings.ToLower(iss.State)
		capacity, ok := cfg.Agent.MaxConcurrentAgentsByState[norm]
		if !ok {
			capacity = cfg.Agent.MaxConcurrentAgents
		}
		if perStateUsed[norm] >= capacity {
			continue
		}
		ns, err := claim(s, iss)
		if err != nil {
			continue // duplicate claim — already reserved elsewhere.
		}
		s = ns
		effs = append(effs, EffRevalidate{Issue: iss, Attempt: 0})
		globalAvail--
		perStateUsed[norm]++
	}
	return s, effs
}

// reduceRevalidated acts on a pre-spawn revalidation result (SPEC §16.4): a missing issue
// or one that left the active states releases the claim; an issue still active proceeds to
// spawn (with the originally-claimed issue snapshot, matching the pre-refactor behavior).
func reduceRevalidated(s State, e EvRevalidated, cfg wfconfig.Config) (State, []Effect) {
	if e.Fresh == nil {
		return releaseClaim(s, e.Issue.ID), nil
	}
	active := normSet(cfg.Tracker.ActiveStates)
	terminal := normSet(cfg.Tracker.TerminalStates)
	norm := strings.ToLower(e.Fresh.State)
	if !active[norm] || terminal[norm] {
		return releaseClaim(s, e.Issue.ID), nil
	}
	return s, []Effect{EffSpawn{Issue: e.Issue, Attempt: e.Attempt}}
}
