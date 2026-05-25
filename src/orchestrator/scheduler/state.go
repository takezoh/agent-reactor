package scheduler

import (
	"maps"
	"time"

	"github.com/takezoh/agent-roost/platform/metrics"
	"github.com/takezoh/agent-roost/platform/tracker"
)

// ClaimState represents the orchestration claim state of an issue (SPEC §7.1).
type ClaimState int

const (
	Unclaimed   ClaimState = iota
	Claimed                // reserved but not yet running
	Running                // worker active
	RetryQueued            // waiting for retry timer
	Released               // removed from all tracking
)

// RunPhase represents the 11-phase lifecycle of a run attempt (SPEC §7.2).
type RunPhase int

const (
	PhasePreparingWorkspace RunPhase = iota
	PhaseBuildingPrompt
	PhaseLaunchingAgentProcess
	PhaseInitializingSession
	PhaseStreamingTurn
	PhaseFinishing
	PhaseSucceeded
	PhaseFailed
	PhaseTimedOut
	PhaseStalled
	PhaseCanceledByReconciliation
)

// RetryKind distinguishes continuation retries from failure-driven backoff retries (SPEC §8.4).
type RetryKind int

const (
	RetryContinuation RetryKind = iota // clean exit → fixed 1s delay
	RetryBackoff                       // abnormal exit → exponential backoff
)

// Worker is the handle through which the shell stops an agent process (SPEC §7.2).
// It is a live resource and therefore lives in the runtime shell's id→handle map,
// never inside the pure State. The concrete implementation is the agent runner's Worker.
type Worker interface {
	Kill(reason string) error
}

// LiveSession holds the session identity for a running attempt (SPEC §4.1.6).
// It is a pure value (identity only) — the live kill handle is held by the shell.
type LiveSession struct {
	SessionID string
	ThreadID  string
	TurnID    string
	StartedAt time.Time
}

// RunAttempt holds the runtime state of a running issue (SPEC §4.1.5 / §16.4).
type RunAttempt struct {
	Issue   tracker.Issue // snapshot at dispatch time
	Session LiveSession

	Attempt   int
	Phase     RunPhase
	TurnCount int // number of turns completed in this attempt (SPEC §4.1.6)

	StartedAt time.Time

	LastCodexMessage   string
	LastCodexEvent     string
	LastCodexTimestamp time.Time

	CodexAppServerPID int

	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalTokens       int64
	TotalRuntime      time.Duration
	RateLimit         *metrics.RateLimitSnapshot
}

// RetryEntry holds the scheduled retry for an issue (SPEC §4.1.7).
// It is a pure value: the live timer handle is held by the shell, keyed by IssueID.
type RetryEntry struct {
	IssueID    string
	Identifier string
	Attempt    int
	Kind       RetryKind
	Err        error // nil for continuation retries
	DueAtMS    int64
}

// State is the orchestrator runtime state (SPEC §4.1.8 OrchestratorRuntimeState).
//
// State is an immutable value: every transition is a pure function that returns a
// new State and never mutates the receiver (no mutex, no goroutines). The single
// scheduler loop owns the authoritative value; the observability HTTP server reads
// a published immutable copy lock-free (see snapshot.go). All maps are treated as
// copy-on-write — transition helpers clone before modifying.
type State struct {
	Running       map[string]RunAttempt
	Claimed       map[string]struct{}
	RetryAttempts map[string]RetryEntry
	Usage         map[string]metrics.Accumulator // per-issue token bookkeeping (§13.5 (b))
	Runtime       map[string]time.Duration       // per-issue cumulative runtime across retries (§13.5 B'')

	// CodexTotals / CodexRuntime accumulate ended-session contributions (§13.5 B'').
	// Live-session contributions are added at Snapshot time from Usage/Runtime.
	// Roll-up happens at releaseClaim (terminal); retry exits keep accumulators alive.
	CodexTotals  metrics.Totals
	CodexRuntime time.Duration
}

// NewState returns an initialized empty State.
func NewState() State {
	return State{
		Running:       map[string]RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]RetryEntry{},
		Usage:         map[string]metrics.Accumulator{},
		Runtime:       map[string]time.Duration{},
	}
}

// StateSnapshot is a read-only projection of State for observability (SPEC §7.3 / §13.5).
// CodexTotals and CodexSecondsRunning are lifetime cumulative values: ended-session
// contributions plus all live accumulators (§13.5 B”).
type StateSnapshot struct {
	Running       map[string]RunAttempt
	Claimed       map[string]struct{}
	RetryAttempts map[string]RetryEntry

	CodexTotals         metrics.Totals
	CodexSecondsRunning float64
}

// Snapshot projects the State into an observability StateSnapshot, folding in
// live per-issue accumulators (§13.5 B”). The returned maps are independent copies.
func (s State) Snapshot() StateSnapshot {
	totals := s.CodexTotals
	for _, acc := range s.Usage {
		t := acc.Totals()
		totals.Input += t.Input
		totals.Output += t.Output
		totals.Total += t.Total
	}
	rt := s.CodexRuntime
	for _, d := range s.Runtime {
		rt += d
	}
	return StateSnapshot{
		Running:             maps.Clone(s.Running),
		Claimed:             maps.Clone(s.Claimed),
		RetryAttempts:       maps.Clone(s.RetryAttempts),
		CodexTotals:         totals,
		CodexSecondsRunning: rt.Seconds(),
	}
}
