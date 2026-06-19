# ADR 0005 — Redefine `cmd/server` as the arc daemon's HTTP/WS gateway

Status: Accepted

## Context

`cmd/server` currently owns `termvt.Manager`, `agentlaunch.Dispatcher`, and
`session.Service` directly, running a second runtime in parallel with the arc
daemon. The Master Plan for plan A (the tmux-free split,
[`plans/arc-server-client-split.md`](../../plans/arc-server-client-split.md))
puts session lifecycle and pty I/O exclusively in the daemon's pure core
(`state.Reduce → driver → runtime → termvt`). `server/web` should be a thin
HTTP/WS gateway. This ADR records the A1-α decision that establishes that
boundary.

ADR 0004 already predicted this convergence ("web を daemon の runtime-owned
session に寄せる"). A1-α makes it concrete.

## Decision

Strip `cmd/server` of all runtime ownership and connect it to the arc daemon
over the existing unix socket (`~/.agent-reactor/arc.sock`) via `proto.Client`.
Session lifecycle and pty I/O flow exclusively through the daemon. `cmd/server`
becomes an HTTP/WS gateway: REST `/api/sessions` and `/ws` are translated to
`proto.Command` calls and back to `controlMsg` / asciicast frames.

## Consequences

- The truth of session state lives in one place. Future refactors (tap_manager
  rewrite in A2, persistence change in δ) cannot ripple into the server side.
- REST/WS handlers shrink to thin adapters, reducing review surface.
- Hop count grows: keystrokes now traverse `browser → server → unix socket →
  daemon → termvt`. A 6-hop path replaces the previous 2-hop direct one.
- Daemon restart resilience must be implemented in `cmd/server`
  (reconnect + typed close); see [ADR 0012](./0012-daemon-client-eager-dial-supervisor.md).
- Wire gaps (cols/rows, id systems) must be bridged in the gateway adapter;
  see FR-022 in the [A1-α impl plan](../specs/2026-06-19-a1-alpha-impl-plan.md).

## Alternatives

- **Keep `cmd/server` owning termvt directly** — rejected because it perpetuates
  the parallel runtime that A1 is meant to eliminate.
- **Merge `cmd/server` in-process with the arc daemon** — rejected because the
  TUI and HTTP responsibilities have different lifecycles and testing surfaces;
  collapsing them would tangle process boundaries.

## Related requirements

- FR-001, FR-002, FR-022
