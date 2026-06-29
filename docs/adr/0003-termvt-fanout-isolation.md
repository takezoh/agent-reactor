# ADR 0003 — The termvt multiplexer is verified by a fan-out isolation contract

Status: Accepted

## Context

`platform/termvt` is the pty-multiplexer primitive behind the web
client⇄server: one pty-backed `Session` parses its program's output through a
server-side VT emulator and fans typed `Event`s out to N subscribers (browser
tabs attached over WebSocket); a `Manager` holds many such sessions.

This is the same *class* of component as the stream subsystem
([ADR 0001](0001-multiplexed-backends-shared-routing-contract.md)): a single
source multiplexed to many consumers. It has the same safety-critical failure
mode — **cross-talk** — in two shapes:

- **Manager cross-talk**: a subscriber attached to session A receives session
  B's bytes. One agent's terminal would leak into another's.
- **Back-pressure cross-talk**: a subscriber that stops draining (a hung or slow
  browser tab) fills its channel; if fan-out blocked on it, the read loop would
  stall and *every other* subscriber of that session would freeze, or — if the
  loop dropped events to avoid blocking — the survivors' byte streams would be
  silently corrupted (a terminal desync).

The original tests covered single-session behaviours (echo, OSC capture, resize,
exit) but never pinned the multi-subscriber and cross-session invariants.

## Decision

Pin the **Fan-out Isolation Invariant** with a contract analogous to the stream
subsystem's routing-isolation contract:

> Every `Event` a `Session` produces reaches exactly the live subscribers of
> that session — all of them, in order, control-before-output within a chunk —
> and no subscriber of any other session. A subscriber that cannot keep up is
> severed (channel closed), never dropped silently and never allowed to block or
> corrupt the others.

Mechanics (all in `platform/termvt`):

- **`fanout` runs inside the sole owner of session state** and does a
  non-blocking send per subscriber, closing any whose buffer is full. A slow
  consumer is severed by construction; it cannot stall the loop or steal
  another subscriber's stream. (Originally this was "fanout holds the
  single-writer lock"; [ADR 0028](0028-termvt-session-actor-model.md) upgraded
  the discipline from a mutex to single-goroutine ownership when the
  mutex-based design deadlocked on undrained VT reply pipes — the contract is
  unchanged, the property is now structural.)
- **`fanout_contract_test.go`** drives a real pty (markers via `cat` echo) and
  asserts: every subscriber receives the output; `Manager` sessions never
  cross-talk; a slow subscriber is severed while a fast one runs to `EventExit`;
  control events precede the chunk's output.
- The **`make test-race`** target guards the single-writer discipline against a
  stray goroutine touching session state.

### No in-process fake; no opt-in e2e tier

ADR 0002 added an opt-in real-app-server backstop for the stream subsystem
*because* it tests against an in-process fake that must be proven faithful.
termvt has no fake — its only backend is a real pty, always present in CI — so
the contract drives the real backend directly. The wired and fidelity layers
coalesce; there is no build-tagged e2e tier to maintain. This is the deliberate
adaptation of the harness to this subsystem's shape, not an omission.

## Consequences

- Cross-talk (Manager and back-pressure) is caught by a named, load-bearing
  invariant rather than by chance, matching the enforcement posture of the
  stream subsystem.
- The enforcement is **test-pinned** (a runtime fan-out property, not statically
  lint-able); it is catalogued in
  [code-enforcement.md §7](../technical/code-enforcement.md). Full guide:
  [termvt-multiplexer-testing.md](../technical/platform/termvt-multiplexer-testing.md).
- The untrusted client→server frame decode (the web gateway's `/ws` data plane)
  is fuzzed in `server/web` (`FuzzInbound`) so malformed frames can neither panic
  the reader nor resize the pty to a non-positive size.
