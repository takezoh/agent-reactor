# ADR 0010 — `EvtSurfaceOutput.Sequence` resets per subscribe, not per session

Status: Accepted

## Context

`termvt.Session.Subscribe` has a documented side effect: the first event
delivered to a new subscriber is a **reattach snapshot** — the full current
screen state — emitted before any live `EventOutput` chunks. This is essential
for cleanly restarting a viewer.

We need a `Sequence` field for browsers to detect dropped output. The initial
draft defined Sequence as "session-scoped monotonic, reset on session restart".
But under that rule, two scenarios break:

1. A slow subscriber gets dropped by termvt, then re-subscribes. It receives
   a fresh snapshot + Sequence=0. Meanwhile other subscribers are at
   Sequence=N. The "session-scope monotonic" invariant has no consistent
   meaning across subscribers.
2. The semantics of "what does Sequence=0 mean" depend on history rather than
   the subscribe contract.

## Decision

Redefine `Sequence` as **subscribe-scoped monotonic, reset to 0 on each
subscribe**. The reattach snapshot is delivered as a single
`EvtSurfaceOutput{Sequence:0, DataB64:<snapshot>}` frame. Subsequent live
chunks increment per-subscriber.

Drop detection becomes a per-subscriber concern, which matches what UIs
actually want.

Document this in the `proto.EvtSurfaceOutput` godoc and in the wire reference.

## Consequences

- Implementation and contract align — testable via fake `PtyBackend` with
  table tests.
- Each subscriber can detect drops independently of others.
- "Session-scoped absolute ordering" is no longer derivable from Sequence
  alone; consumers needing it must use `TimeSec` (which is daemon clock-based
  and monotonic per session).
- β's UI must read this contract carefully when implementing gap detection.

## Alternatives

- **Keep session-scoped sequence** — rejected because a fresh snapshot at
  Sequence=0 to a re-subscribing connection cannot coexist with continuous
  numbering for other subscribers.
- **Re-number at the gateway** — rejected because the gateway then cannot
  detect daemon-side publish drops.

## Related requirements

- FR-018, FR-019
