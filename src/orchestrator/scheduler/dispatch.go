package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// CandidateSource abstracts the tracker for testability.
type CandidateSource interface {
	Candidates(ctx context.Context) ([]tracker.Issue, error)
}

// SpawnFunc spawns a worker for the given issue and returns its session (injected by issue 013).
type SpawnFunc func(ctx context.Context, issue tracker.Issue, attempt int) (LiveSession, error)

// dispatchOnce performs one dispatch pass: filter eligible, sort, allocate slots (SPEC §8.1–§8.3).
// It consumes up to the available global and per-state slot counts.
func dispatchOnce(ctx context.Context, cands []tracker.Issue, st *State, clk Clock, fireCh chan<- retryFireReq, spawn SpawnFunc, cfg wfconfig.Config) {
	snap := st.Snapshot()
	active := normSet(cfg.Tracker.ActiveStates)
	terminal := normSet(cfg.Tracker.TerminalStates)

	eligibles := make([]tracker.Issue, 0, len(cands))
	for _, iss := range cands {
		if eligible(iss, snap, active, terminal) {
			eligibles = append(eligibles, iss)
		}
	}
	eligibles = sortCandidates(eligibles)

	globalAvail := availableGlobalSlots(snap, cfg)
	// Track per-state counts locally to avoid re-snapshotting after each dispatch.
	perStateUsed := make(map[string]int)
	for _, run := range snap.Running {
		perStateUsed[strings.ToLower(run.Issue.State)]++
	}

	for _, iss := range eligibles {
		if globalAvail <= 0 {
			break
		}
		norm := strings.ToLower(iss.State)
		cap, ok := cfg.Agent.MaxConcurrentAgentsByState[norm]
		if !ok {
			cap = cfg.Agent.MaxConcurrentAgents
		}
		if perStateUsed[norm] >= cap {
			continue
		}

		if err := st.Claim(iss, 1); err != nil {
			// Duplicate claim — already claimed elsewhere; skip.
			continue
		}

		session, err := spawn(ctx, iss, 1)
		if err != nil {
			slog.Error("spawn failed", "issue_id", iss.ID, "identifier", iss.Identifier, "err", err)
			st.ReleaseClaim(iss.ID)
			entry := RetryEntry{IssueID: iss.ID, Identifier: iss.Identifier, Attempt: 2, Kind: RetryBackoff, Err: err}
			scheduleRetry(st, clk, fireCh, ctx, entry, backoffDelay(2, cfg))
			continue
		}

		st.MarkRunning(iss.ID, iss, 1, session, time.Now())
		globalAvail--
		perStateUsed[norm]++
		slog.Info("dispatched", "issue_id", iss.ID, "identifier", iss.Identifier)
	}
}

// handleRetryFire processes a timer-fired retry request per SPEC §8.4.
func handleRetryFire(ctx context.Context, req retryFireReq, tr CandidateSource, st *State, clk Clock, fireCh chan<- retryFireReq, spawn SpawnFunc, cfg wfconfig.Config) {
	cands, err := tr.Candidates(ctx)
	if err != nil {
		slog.Error("retry-fire: candidates fetch failed", "issue_id", req.IssueID, "err", err)
		return
	}

	var found *tracker.Issue
	for i := range cands {
		if cands[i].ID == req.IssueID {
			found = &cands[i]
			break
		}
	}
	if found == nil {
		slog.Info("retry-fire: issue not found, releasing", "issue_id", req.IssueID)
		st.ReleaseClaim(req.IssueID)
		return
	}

	// RetryQueued issues are in snap.Claimed and snap.RetryAttempts, so eligible() correctly
	// excludes them. Skip it: check only active/terminal state, then use ClaimFromRetry
	// (not Claim) because claimed is already retained by WorkerExit* (SPEC §7.1).
	terminal := normSet(cfg.Tracker.TerminalStates)
	active := normSet(cfg.Tracker.ActiveStates)
	norm := strings.ToLower(found.State)
	if terminal[norm] || !active[norm] {
		slog.Info("retry-fire: issue not active, releasing", "issue_id", req.IssueID, "state", found.State)
		st.ReleaseClaim(req.IssueID)
		return
	}

	snap := st.Snapshot()
	if availableGlobalSlots(snap, cfg) <= 0 || availablePerStateSlots(found.State, snap, cfg) <= 0 {
		slog.Info("retry-fire: no available orchestrator slots, requeuing", "issue_id", req.IssueID)
		entry := RetryEntry{IssueID: req.IssueID, Identifier: found.Identifier, Attempt: req.Attempt, Kind: RetryBackoff}
		scheduleRetry(st, clk, fireCh, ctx, entry, continuationDelay)
		return
	}

	if err := st.ClaimFromRetry(req.IssueID, req.Attempt); err != nil {
		slog.Info("retry-fire: claim rejected", "issue_id", req.IssueID, "err", err)
		return
	}

	session, err := spawn(ctx, *found, req.Attempt)
	if err != nil {
		slog.Error("retry-fire: spawn failed", "issue_id", req.IssueID, "err", err)
		st.ReleaseClaim(req.IssueID)
		entry := RetryEntry{IssueID: req.IssueID, Identifier: found.Identifier, Attempt: req.Attempt + 1, Kind: RetryBackoff, Err: err}
		scheduleRetry(st, clk, fireCh, ctx, entry, backoffDelay(req.Attempt+1, cfg))
		return
	}

	st.MarkRunning(req.IssueID, *found, req.Attempt, session, time.Now())
	slog.Info("retry-fire: dispatched", "issue_id", req.IssueID, "identifier", found.Identifier, "attempt", req.Attempt)
}
