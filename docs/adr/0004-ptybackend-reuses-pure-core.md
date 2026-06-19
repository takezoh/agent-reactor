# ADR 0004 — Reuse the pure core via a PtyBackend, not a parallel server stack

Status: Accepted

## Context

The tmux-free split (plan: [arc-server-client-split.md](../../plans/arc-server-client-split.md),
design: [remote-client-design.md](../../plans/remote-client-design.md)) shipped a
phase-2 web stack — `platform/termvt`, `server/session`, `server/web`,
`client/web` — that operates pty-backed sessions and streams them to the browser.

That stack works, but it **bypasses arc's pure core**: `server/web` and
`server/session` import neither `client/state` (the reducer) nor `client/driver`.
The design's strategy (remote-client-design.md §2) was the opposite — *replace the
`TmuxBackend` implementation with a `PtyBackend`, keep the pure core untouched* —
so the runtime/reducer/driver run unchanged on a tmux-free backend.

We must decide how the two reconcile:

- **(i) PtyBackend** — wrap `platform/termvt` behind the existing `TmuxBackend`
  role interfaces (`client/runtime/backends.go`) so the unchanged
  runtime/reducer/driver drive pty sessions; the web gateway renders
  driver-derived state.
- **(ii) Server reimplementation** — leave the web stack separate and
  re-implement status detection / driver views / persistence on the server side.

### Due-diligence findings

The `TmuxBackend` seam splits cleanly into a **data plane** and a
**presentation plane**, and termvt already supplies the data plane:

| `TmuxBackend` method | `platform/termvt` primitive |
|---|---|
| `SpawnWindow` | `NewSession(Spec)` |
| `SendKeys` / `SendKey` / `SendEnter` / `PasteBuffer` | `WriteInput([]byte)` |
| `PipePane` (output tap) | `Subscribe()` (snapshot-first fan-out) |
| `CapturePane` | `Snapshot()` / `em.Render()` |
| `PaneSize` / `ResizeWindow` | `Resize()` / `Size()` |
| `KillPaneWindow` | `Close()` |
| `PaneAlive` / `PaneExitStatus` | `EventExit` (exit **code** not yet retained) |
| OSC 9/133 + title/bell tee | `registerOSC()` → `Control` event |

Two facts make (i) low-risk and incremental:

1. A complete no-op `TmuxBackend` (`noopTmux`) already exists "until production
   wiring lands" — proof the runtime can be wired to a non-tmux backend.
2. The data-plane methods map 1:1 onto existing termvt primitives; only small
   additions are needed (below).

The **presentation plane** — `WindowLayout` (`SwapPane`/`BreakPane`/`JoinPane`/
`SelectPane`/`RunChain`) and `TmuxControl` (`SetStatusLine`/`DetachClient`/
`KillSession`/`DisplayPopup`) — has no server-side equivalent in a pty
multiplexer. The design already anticipates this: layout composition moves
client-side (remote-client-design.md, "client-side layout composition replaces
the tmux 3-pane control screen").

## Decision

Adopt **(i)**. Introduce a `PtyBackend` that implements the `TmuxBackend` role
interfaces over `platform/termvt`, and keep the pure core (`state.Reduce`,
`Driver`) unchanged.

- **Data plane** (`PaneLifecycle`, `PaneIO`, `PaneInspect`, `SessionEnv`,
  liveness): implemented for real against termvt + an in-process pane-id map.
- **Presentation plane** (`WindowLayout`, `TmuxControl`): stubbed (like
  `noopTmux`) initially; relocated client-side in the tmux-removal phase.

This is the linchpin (plan §4, B1). It unblocks reuse of driver intelligence on
the web surface (plan A) and the eventual tmux removal (plan C).

## Consequences

**Positive**

- One source of agent intelligence; the web surface inherits driver views,
  run-state detection, tags, and persistence instead of re-deriving them.
- Unblocks tmux removal (plan C): once the runtime runs on PtyBackend, the tmux
  backend can be deleted.
- Incremental and testable: the seam + `noopTmux` let PtyBackend land method by
  method behind existing reducer/driver tests.

**Required termvt additions (small)**

- Retain `cmd.Wait()`'s exit code and expose it on `EventExit` (for
  `PaneExitStatus`).
- OSC parity: add 777 (notify), 7 (cwd), 99 to the existing 9/133/title/bell.
- `SendKey` named-key → byte translation (`Escape` → `0x1b`, etc.).
- `CapturePane` adapter: trailing N lines, SGR-stripped, from the emulator grid.

**Rejected — (ii) server reimplementation**

- Permanent duplication of driver logic across two code paths.
- Cements the design divergence rather than closing it.
- Does not enable tmux removal.

## Open questions (settle in B1 design)

- **Session ownership.** `server/session.Service` currently owns lifecycle over
  `termvt.Manager`. With PtyBackend the runtime also spawns/controls sessions.
  Either the runtime's PtyBackend drives `termvt.Manager` directly (and
  `server/session` delegates to / is replaced by it), or the web gateway talks
  *through* the runtime via `proto`. To be decided before B1 implementation.
- **Reattach after daemon restart.** tmux sessions survived the arc daemon;
  host-owned termvt sessions live in the server process. The persistence story
  for pane recovery (today keyed on tmux session env `ROOST_SESSION_<sid>`)
  needs a pty-world replacement.
