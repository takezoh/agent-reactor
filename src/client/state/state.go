// Package state holds the pure functional core of the client. State is a plain
// data type, Reduce is a pure function, Event and Effect are closed sum
// types. No goroutines, no I/O, no globals (except the driver registry and
// the default-driver factory, both set once at init time).
//
// The runtime package interprets effects and feeds events back into Reduce.
// All concurrency lives in runtime; state is single-threaded by construction.
package state

import (
	"time"

	"github.com/takezoh/agent-reactor/platform/features"
)

// Identifier types. Distinct named types prevent accidental mix-up at the
// type level instead of at runtime.
type (
	SessionID   string
	FrameID     string
	SubsystemID string
	TargetID    string
	ConnID      uint64
	JobID       uint64
)

// State is the entire client domain state at one point in time. Reduce
// produces a new State value from an existing State + an Event; the
// runtime swaps its single in-memory copy each tick of the event loop.
//
// Maps are owned by the state and updated copy-on-write inside Reduce —
// callers must not mutate a State they did not produce.
type State struct {
	Sessions    map[SessionID]Session
	Subscribers map[ConnID]Subscriber
	// SurfaceSubs records which (ConnID, SessionID) pairs are streaming
	// pane output via the surface.subscribe RPC. The outer map is keyed by
	// ConnID so connection close can drop all subscriptions in one step
	// (see reduceConnClosed). The inner set keeps lookup O(1).
	//
	// In-memory only: SurfaceSubs is NOT persisted to sessions.json on
	// purpose. Subscriptions reset on daemon restart so clients must
	// re-subscribe (matches the runtime's relay-goroutine lifecycle).
	//
	// Per-ConnID cap: the reducer enforces len(SurfaceSubs[ConnID]) <= 8
	// (ADR 0007); excess subscribe attempts get RespErr(resource_exhausted).
	SurfaceSubs      map[ConnID]map[SessionID]struct{}
	Jobs             map[JobID]JobMeta
	NextJobID        JobID
	NextConnID       ConnID
	Now              time.Time         // last tick timestamp; deterministic in tests
	Aliases          map[string]string // command alias expansion (e.g. "cw" → "<tool> --workspace")
	DefaultCommand   string            // fallback when session command is empty
	SandboxedProject func(string) bool // nil = not configured; true when project runs in sandbox

	// Features is the set of enabled runtime flags, built once at startup
	// from the config file and never mutated. Reduce reads it as a
	// read-only value, so it does not break pure-function semantics.
	Features features.Set

	// ActiveSession is the logically focused session. The web client tracks
	// its own active-session-per-tab; the daemon-side ActiveSession is only
	// used as a fallback for surface RPCs that omit SessionID and for the
	// session-tagging in reduceFrame/reduceFrameEvict. Cleared when the
	// session is removed.
	ActiveSession SessionID
}

// Session is the static metadata + driver state of one client session.
// All dynamic per-session data lives in Driver (a sum-typed value), which
// each driver impl returns from its Step method.
type Session struct {
	ID            SessionID
	Project       string
	CreatedAt     time.Time
	Frames        []SessionFrame
	ActiveFrameID FrameID   // explicit active frame; empty = use Frames[len-1]
	MRUFrameIDs   []FrameID // MRU stack for fallback on active-frame death
	Command       string
	Sandbox       SandboxOverride // session-scoped sandbox mode, set at creation, applies to all frames
	LaunchOptions LaunchOptions
	Driver        DriverState
}

type SessionFrame struct {
	ID            FrameID
	SubsystemID   SubsystemID
	TargetID      TargetID
	Project       string
	Command       string
	LaunchOptions LaunchOptions
	CreatedAt     time.Time
	Driver        DriverState
}

// Subscriber tracks a connected IPC client that has opted into broadcasts.
// Filters is the set of event names the client wants to receive; an empty
// list means "all events".
type Subscriber struct {
	ConnID  ConnID
	Filters []string
}

// JobMeta is the in-flight worker bookkeeping for one async job. The
// runtime worker pool reports back via EvJobResult, which the reducer
// looks up here to find which session the result belongs to.
type JobMeta struct {
	SessionID SessionID
	FrameID   FrameID
	StartedAt time.Time
}

// New returns an empty State suitable for a fresh daemon start. Maps
// are initialised so callers can write into them without nil checks.
func New() State {
	return State{
		Sessions:    map[SessionID]Session{},
		Subscribers: map[ConnID]Subscriber{},
		SurfaceSubs: map[ConnID]map[SessionID]struct{}{},
		Jobs:        map[JobID]JobMeta{},
	}
}
