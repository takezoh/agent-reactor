// Package metrics provides agent-agnostic token/runtime accounting (SPEC §13.5).
//
// All callers supply absolute cumulative token counts per thread.
// Accumulator prevents double-counting via last-reported-absolute difference
// bookkeeping: each report contributes only the delta since the prior report from
// the same thread (§13.5 (b)).
// Delta-style payloads (§13.5 "Ignore delta-style payloads") must not be passed here.
package metrics

import "time"

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
type Accumulator struct {
	lastSeen map[string]Usage
	totals   Totals
}

// NewAccumulator returns an initialized Accumulator.
func NewAccumulator() *Accumulator {
	return &Accumulator{lastSeen: make(map[string]Usage)}
}

// Observe incorporates an absolute cumulative report and returns updated Totals.
// Negative deltas (monotonic violation) are clamped to zero and ignored.
func (a *Accumulator) Observe(u Usage) Totals {
	prev := a.lastSeen[u.ThreadID]
	a.totals.Input += clampPos(u.Input - prev.Input)
	a.totals.Output += clampPos(u.Output - prev.Output)
	a.totals.Total += clampPos(u.Total - prev.Total)
	a.lastSeen[u.ThreadID] = u
	return a.totals
}

// Snapshot returns the current Totals without modifying state.
func (a *Accumulator) Snapshot() Totals { return a.totals }

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
