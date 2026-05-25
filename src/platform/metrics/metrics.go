// Package metrics provides agent-agnostic token/runtime accounting (SPEC §13.5).
//
// All callers supply absolute cumulative token counts per thread.
// Accumulator prevents double-counting via last-reported-absolute difference
// bookkeeping: each report contributes only the delta since the prior report from
// the same thread (§13.5 (b)).
// Delta-style payloads (§13.5 "Ignore delta-style payloads") must not be passed here.
package metrics

import (
	"maps"
	"time"
)

// Totals holds aggregated token counts across all threads for one run attempt.
type Totals struct {
	Input  int64
	Output int64
	Total  int64
}

// Usage is one absolute cumulative token report for a single thread.
// Input/Output/Total are the thread's running totals at the time of reporting.
type Usage struct {
	ThreadID string
	Input    int64
	Output   int64
	Total    int64
}

// Accumulator aggregates absolute token reports across threads (§13.5).
// It tracks the last-reported absolute per thread so successive reports
// from the same thread are not double-counted.
//
// Accumulator is an immutable value type: Observe returns a new Accumulator and
// never mutates the receiver, so it can be folded into a pure state machine. The
// zero value is a valid empty accumulator.
type Accumulator struct {
	lastSeen map[string]Usage
	totals   Totals
}

// NewAccumulator returns an empty Accumulator. The zero value is equivalent.
func NewAccumulator() Accumulator { return Accumulator{} }

// Observe returns a new Accumulator incorporating an absolute cumulative report.
// The receiver is not modified. Negative deltas (monotonic violation) are clamped
// to zero and ignored.
func (a Accumulator) Observe(u Usage) Accumulator {
	prev := a.lastSeen[u.ThreadID]
	totals := Totals{
		Input:  a.totals.Input + clampPos(u.Input-prev.Input),
		Output: a.totals.Output + clampPos(u.Output-prev.Output),
		Total:  a.totals.Total + clampPos(u.Total-prev.Total),
	}
	lastSeen := make(map[string]Usage, len(a.lastSeen)+1)
	maps.Copy(lastSeen, a.lastSeen)
	lastSeen[u.ThreadID] = u
	return Accumulator{lastSeen: lastSeen, totals: totals}
}

// Totals returns the current aggregated totals.
func (a Accumulator) Totals() Totals { return a.totals }

func clampPos(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

// RuntimeAccumulator sums per-turn durations for §13.5 runtime-seconds aggregation.
type RuntimeAccumulator struct {
	total time.Duration
}

// AddTurn adds one completed turn's duration.
func (r *RuntimeAccumulator) AddTurn(d time.Duration) {
	if d > 0 {
		r.total += d
	}
}

// Total returns the accumulated runtime.
func (r *RuntimeAccumulator) Total() time.Duration { return r.total }

// RateLimitSnapshot holds the most recent rate-limit state reported by the agent (§13.5).
// Fields are zero-valued when not reported.
type RateLimitSnapshot struct {
	PrimaryUsedPercent   int64
	PrimaryResetsAt      int64 // Unix ms; 0 = unknown
	SecondaryUsedPercent int64
	SecondaryResetsAt    int64 // Unix ms; 0 = unknown
}
