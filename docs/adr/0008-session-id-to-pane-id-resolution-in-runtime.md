# ADR 0008 — Resolve `SessionID` to `paneID` in runtime, not in proto

Status: Accepted

## Context

The `termvt.Manager.Get` key space is paneID (e.g. `"pty:1"` for PtyBackend,
`"%1"` for tmux backend) — physical resource IDs allocated by
`PtyBackend.SpawnWindow`. The state-level `SessionID` (e.g. `"s1"`) is a
distinct, logical ID minted by `Reduce(EvCreateSession)`. A single session can
own multiple frames (multiple paneIDs); the currently visible one is
`Sessions[sid].ActiveFrame().TargetID`.

Naively calling `mgr.Get(SessionID)` always returns not-found. The browser must
not see paneIDs (they are internal physical handles and would couple wire to
backend choice). So someone has to translate.

## Decision

Translate `SessionID → ActiveFrame.TargetID` inside `client/runtime` when
interpreting `EffSurfaceSubscribeStart` / `EffSurfaceWriteRaw` /
`EffSurfaceResize`. The wire and `Reduce` deal exclusively in `SessionID`;
`PtyBackend` sees only `paneID`.

If `Sessions[sid].ActiveFrame()` is `nil` at the moment of `Reduce`, the
reducer returns `RespErr(frame-not-ready)` immediately — see
[ADR 0018](./0018-defer-subscribe-race-to-beta.md) for the race deferral.

## Consequences

- The existing state / driver / wire ID conventions stay intact. Surface adds
  a single new field (`Subscribers.Surface`) and no new ID types.
- ActiveFrame swaps inside the daemon are transparent to the web client —
  `Reduce` keeps the same SessionID-keyed map.
- Future "multi-frame attach" (one browser viewing several frames of one
  session) extends the wire to `(SessionID, FrameID)` without breaking the
  α contract.
- `terminal_relay` needs a tiny helper (`Sessions[sid].ActiveFrame()`) to look
  up the paneID at subscribe time and store it alongside the subscription
  handle.

## Alternatives

- **Expose paneID on the wire** — rejected because it leaks an internal
  handle to the browser and couples the public wire to the backend choice.
- **Make `SessionID == paneID`** — rejected because it collapses the 1-session
  / N-frame model the driver relies on.

## Related requirements

- FR-014, FR-015, FR-016
