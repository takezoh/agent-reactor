# ADR 0007 — Track Surface subscriptions in `State.Subscribers.Surface`

Status: Accepted

## Context

A1-α broadcasts `EvtSurfaceOutput` to a specific subset of connections — only
those that issued `CmdSurfaceSubscribe` for the matching session. We must decide
where the `(ConnID, SessionID) → subscribed?` relation lives.

If we keep it in a runtime-local in-memory map, the truth of "who is
subscribed" exists outside `state.State`. `Reduce` outputs alone no longer
determine the broadcast destinations — runtime must consult its own map. That
hollows out the reducer-purity invariant the rest of the system depends on
(table tests that assert `Reduce` is deterministic become uninformative).

## Decision

Add `Subscribers.Surface map[ConnID]map[SessionID]struct{}` as a first-class
field on `State`. `EvCmdSurfaceSubscribe` / `EvCmdSurfaceUnsubscribe` update
this map in `Reduce`; the runtime reads it to decide fan-out destinations and
holds no per-subscription state of its own beyond the resource handle
(`*termvt.Subscription`) needed to actually drive termvt.

In short: **policy lives in state, resource handles live in runtime**.

## Consequences

- Broadcast destinations are reproducible from a state snapshot. Reducer table
  tests stay meaningful.
- `State` gains one map field. Godoc must mark it as in-memory only (not
  persisted, recreated on daemon restart).
- `EvConnDisconnect` (existing or newly added) must walk
  `Subscribers.Surface[connID]` and emit `EffSurfaceSubscribeStop` for every
  entry, then clear the bucket — otherwise dangling termvt subscriptions leak.
- Tests must assert idempotency: re-subscribing an already-subscribed
  `(ConnID, SessionID)` does not double-allocate runtime resources.

## Alternatives

- **Runtime in-memory map only** — rejected because broadcast policy escapes
  state, breaking the purity invariant.
- **Mutate state directly from runtime** — rejected because it violates the
  α-4 invariant (`Reduce` is the sole mutator) and makes reducer table tests
  unable to fully model behaviour.

## Related requirements

- FR-012, FR-013, FR-017
