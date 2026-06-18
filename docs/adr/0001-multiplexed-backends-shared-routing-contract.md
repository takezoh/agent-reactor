# ADR 0001 — Multiplexed backends are verified by a shared routing-isolation contract

Status: Accepted

## Context

The stream subsystem backend (`client/runtime/subsystem/stream`) fronts **one**
codex app-server connection but multiplexes **many** frames (agents) over it.
Inbound server events are demultiplexed by `threadID → frameID` (`b.threads`).

A new turn's thread id is **not** returned synchronously by `turn/start`; it
arrives later in an async `thread.started`. The backend then guesses which frame
the thread belongs to (`resolveFrameForStartedThread`): it matches the start
`cwd` against unbound frames and, when that is ambiguous, **falls back to the
currently active frame** (`activeLookup`). Once a thread is bound to the wrong
frame, every subsequent event for it — assistant text, tool output — is routed
there. The result is **cross-talk**: one agent's output (including tool results
like `ssh`/file reads) surfaces in another agent's session, and the receiving
model confabulates around the foreign input.

This became systemic once shared-container isolation made multiple frames share
one `cwd` (the ambiguous case became the norm). The existing tests missed it:
they covered single-frame flows and *structural* multi-frame binding (map
integrity), never the invariant that an event reaches **only** the frame that
started its thread.

The `activeLookup` fallback is a concrete violation of the cross-layer **"No
fabricated fallbacks"** principle ([ARCHITECTURE.md](../../ARCHITECTURE.md#design-principles)):
it invents ownership truth when the real answer is unknown.

## Decision

Pin the **Routing Isolation Invariant** with a shared, reusable contract before
fixing the demux:

> Every `state.EvSubsystem` emitted from a thread T carries `FrameID == owner(T)`,
> where `owner(T)` is the frame whose `BindFrame` started/resumed T. No event
> reaches any other frame.

Mechanics (all in `client/runtime/subsystem/stream`):

- **`recordingRuntime`** captures every emitted `EvSubsystem` by `FrameID`;
  unique marker strings (carried in `LastAssistantMessage`) tie an event back to
  the thread that produced it. `assertMarkerFrames` is the single invariant check.
- **Direct-drive contract** (`routing_contract_test.go`) feeds the event handlers
  synchronously — deterministic, no goroutines.
- **Wired harness** (`routing_wired_test.go`) drives the real `codexclient.Conn`
  against an in-process fake app-server built on `codexclient.Server`, so the
  async read loop is exercised under `-race` and the fake emits the **same wire
  shapes** production does.
- **Fuzz** (`routing_fuzz_test.go`, stdlib `testing.F`) explores interleavings of
  binds/starts/active-switches/messages.

The bug-reproducing ("cross-talk") cases are **RED** on the current demux and are
gated behind `REACTOR_ROUTING_PINS` so CI stays green until the fix; the
GREEN regression guards (distinct-cwd routing, release cleanup, …) run always.
The fix that binds threads by their initiating request flips the pins GREEN, at
which point the gate is removed and the cases become permanent regression cover.

Any future multiplexed backend adopts this contract rather than rolling its own
per-backend assertions.

## Consequences

- The class of bug ("an agent's output appears in another agent") is caught by a
  named, load-bearing invariant rather than by chance.
- The in-process fake is validated against a real app-server (the e2e uses
  distinct cwds: it confirms the fake routes like a real server, not that the
  cross-talk bug is absent), so a green contract against the fake is trustworthy
  — see [ADR 0002](0002-optin-appserver-e2e-validates-fakes.md).
- The enforcement is **test-pinned** (not statically lint-able); it is catalogued
  in [code-enforcement.md](../technical/code-enforcement.md).
