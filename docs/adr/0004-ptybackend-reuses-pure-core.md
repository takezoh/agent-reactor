# ADR 0004 — Reuse the pure core via a PtyBackend, not a parallel server stack

Status: Accepted

## Context

A pty-backed web stack — `platform/termvt`, `server/session`, `server/web`,
`client/web` — operates pty sessions and streams them to the browser.

Bypassing the pure core would mean reimplementing status detection / driver
views / persistence on the server side. The alternative is to drive that
same web stack from the runtime/reducer/driver, so the pure core stays the
single source of agent intelligence.

We must decide how the two reconcile:

- **(i) PtyBackend** — wrap `platform/termvt` behind the existing backend
  role interfaces (`client/runtime/backends.go`) so the unchanged
  runtime/reducer/driver drive pty sessions; the web gateway renders
  driver-derived state.
- **(ii) Server reimplementation** — leave the web stack separate and
  re-implement status detection / driver views / persistence on the server
  side.

### Due-diligence findings

The backend role interfaces split cleanly into a **data plane** and a
**presentation plane**. The data plane maps onto existing
`platform/termvt` primitives:

| Backend role surface                      | `platform/termvt` primitive                 |
|---|---|
| Spawn a frame                             | `NewSession(Spec)`                          |
| Send keys / paste                         | `WriteInput([]byte)`                        |
| Output tap                                | `Subscribe()` (snapshot-first fan-out)      |
| Capture grid                              | `Snapshot()` / `em.Render()`                |
| Size / resize                             | `Resize()` / `Size()`                       |
| Kill frame                                | `Close()`                                   |
| Frame exit                                | `EventExit`                                 |
| OSC 9/133 + title/bell tee                | `registerOSC()` → `Control` event           |

The data-plane methods map 1:1 onto existing termvt primitives; only small
additions are needed (below).

The **presentation plane** — window layout and session/client control — has
no server-side equivalent in a pty multiplexer. Layout composition lives
client-side; the backend exposes only the data plane.

## Decision

Adopt **(i)**. Introduce a `PtyBackend` that implements the backend role
interfaces over `platform/termvt`, and keep the pure core (`state.Reduce`,
`Driver`) unchanged.

- **Data plane** (`FrameLifecycle`, `FrameIO`, `FrameInspect`, `SessionEnv`,
  liveness): implemented for real against termvt. `FrameID` is the single
  key — there is no separate physical-handle namespace.
- **Presentation plane** (window layout, session/client control): removed
  from the role interfaces entirely — layout composition lives client-side.

## Consequences

**Positive**

- One source of agent intelligence; the web surface inherits driver views,
  run-state detection, tags, and persistence instead of re-deriving them.
- Incremental and testable: the seam lets PtyBackend land method by method
  behind existing reducer/driver tests.

**termvt additions (small)**

- Retain `cmd.Wait()`'s exit code and expose it on `EventExit` (for
  `FrameExitStatus`).
- OSC parity: add 777 (notify), 7 (cwd), 99 to the existing 9/133/title/bell.
- `SendKey` named-key → byte translation (`Escape` → `0x1b`, etc.).
- `CaptureFrame` adapter: trailing N lines, SGR-stripped, from the emulator grid.

**Rejected — (ii) server reimplementation**

- Permanent duplication of driver logic across two code paths.
- Cements the design divergence rather than closing it.

## Resolved decisions

- **Session ownership → the runtime's PtyBackend owns its own `termvt.Manager`.**
  `NewPtyBackend()` constructs a private `termvt.Manager`; processes that
  embed PtyBackend each hold their own Manager so they cannot collide.
- **Reattach after daemon restart → not preserved across restart.** termvt
  sessions are children of the daemon and die with it. Session *definitions*
  persist via `SessionSnapshot`; on restart the daemon cold re-spawns rather
  than re-attaching live processes. PtyBackend's `SetEnv`/`ShowEnvironment`
  are in-process only and documented as non-persistent; a detached
  supervisor that outlives the daemon is out of scope here.
