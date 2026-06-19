# ADR 0018 — Defer the subscribe / ActiveFrame race to A1-β

Status: Accepted

## Context

When the browser sends `CmdSurfaceSubscribe{SessionID}` immediately after
`POST /api/sessions` returns, the daemon's `EffSpawnFrame` may not yet have
run. `Sessions[sid].ActiveFrame()` is `nil`, and `terminal_relay` cannot
resolve the paneID.

Three responses were considered:

1. **`RespErr(NotReady)` immediately, client retries.** Reducer stays pure
   (no state extension), but the vanilla JS UI has no retry logic — the
   user must reload after a failed first attach.
2. **State pending entry, Reduce drains on `EvFrameReady`.** Adds a `pending`
   field to `Subscribers.Surface`, plus driver wiring to emit `EvFrameReady`.
   State purity preserved but state schema and reducer logic grow.
3. **Gateway waits.** `cmd/server` blocks on a `EvtSessionsChanged` /
   `EvtFrameReady` event before issuing subscribe. Reducer stays pure,
   state unchanged, but gateway gains a new "wait" phase and a fresh proto
   event.

All three add scope. The Master Plan also schedules the React frontend for
β, which is where retry logic naturally lives anyway.

## Decision

In α, the reducer returns `RespErr(Code:'frame-not-ready')` immediately when
`Sessions[sid].ActiveFrame() == nil`. The gateway translates this to a
two-step typed close (see [ADR 0011](./0011-two-step-ws-close-on-daemon-disconnect.md))
with `code:'frame-not-ready'` on the control frame. No retry happens in α.

Retry logic is implemented in β alongside the React store, where exponential
backoff in `useEffect` is a few lines.

For α development and smoke testing, the operator sequences `POST` and `WS
attach` with sufficient delay between them, or reloads manually if `attach`
races the frame spawn.

## Consequences

- Reducer stays minimal; no state schema growth in α.
- No new proto events needed.
- α development workflow tolerates the race because the operator controls
  the sequencing manually.
- β must add client-side retry as part of the React migration — documented
  in the Master Plan β scope.
- Throwaway code in α is essentially zero (the `frame-not-ready` RespErr is
  reusable in β unchanged).

## Alternatives

- **State pending + `EvFrameReady`** — rejected for α; state schema growth
  and driver wiring that retry-on-client subsumes more cleanly.
- **Gateway wait** — rejected for α; adds a new "wait" phase to gateway and
  a fresh proto event for no UX gain over client retry.
- **Vanilla JS retry shim in α** — rejected; ~30 lines that would be
  discarded entirely when β rewrites `app.js` in React, and complicates the
  "α does not touch the UI" boundary.

## Related requirements

- FR-024
