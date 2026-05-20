package scheduler

import (
	"sync"
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

// Worker is the interface through which the scheduler stops an agent process (SPEC §7.2).
// The concrete implementation lives in the codex agent runner (issue 013).
type Worker interface {
	Kill(reason string) error
}

// RetryTimer wraps a one-shot Timer scheduled by the scheduler (SPEC §8.4).
type RetryTimer struct{ t Timer }

// RunAttempt holds the runtime state of a running issue (SPEC §4.1.5 / §16.4).
type RunAttempt struct {
	Issue   tracker.Issue // snapshot at dispatch time
	Session LiveSession

	Attempt int
	Phase   RunPhase

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

// LiveSession holds the session identity for a running attempt (SPEC §4.1.6).
type LiveSession struct {
	SessionID string
	ThreadID  string
	TurnID    string
	StartedAt time.Time
	Worker    Worker
}

// RetryEntry holds the scheduled retry for an issue (SPEC §4.1.7).
// DueAtMS is set to 0 by transition functions; the caller (issue 012) fills in the delay.
type RetryEntry struct {
	IssueID    string
	Identifier string
	Attempt    int
	Kind       RetryKind
	Err        error // nil for continuation retries
	DueAtMS    int64
	Timer      RetryTimer
}

// StateSnapshot is a read-only copy of State for observability (SPEC §7.3 Snapshot).
type StateSnapshot struct {
	Running       map[string]RunAttempt
	Claimed       map[string]struct{}
	RetryAttempts map[string]RetryEntry
}

// State is the orchestrator runtime state (SPEC §4.1.8 OrchestratorRuntimeState).
// All mutations must hold mu.
type State struct {
	mu            sync.Mutex
	running       map[string]RunAttempt
	claimed       map[string]struct{}
	retryAttempts map[string]RetryEntry
	usage         map[string]*metrics.Accumulator // per-issue token bookkeeping (§13.5 (b))
}

// NewState returns an initialized State.
func NewState() *State {
	return &State{
		running:       make(map[string]RunAttempt),
		claimed:       make(map[string]struct{}),
		retryAttempts: make(map[string]RetryEntry),
		usage:         make(map[string]*metrics.Accumulator),
	}
}
