# ADR 0009 — Expose `SubscribeSurface(target)` on `PtyBackend`

Status: Accepted

## Context

`PtyBackend` holds `mgr *termvt.Manager` as an unexported field. The web
gateway's `terminal_relay` needs to reach `termvt.Session.Subscribe` to fan
output out to subscribed connections, but it sits behind the `cfg.Backend`
abstraction. Reaching into `mgr` directly would bypass the abstraction and
block future backend swaps (e.g. a remote backend).

## Decision

Add three methods to `PtyBackend`:

```go
SubscribeSurface(target string) (*termvt.Subscription, error)
WriteSurface(target string, data []byte) error
ResizeSurface(target string, cols, rows int) error
```

`target` (the `string(FrameID)` key into the `termvt.Manager`) is the only
argument. `ConnID` and `SessionID` are not exposed — they are state-level
concerns that the runtime owns.

`platform/termvt` is **not modified**. The accessors are pure forwarders to
`mgr.Get(target)` plus existing termvt methods.

## Consequences

- termvt's API stays frozen; only the backend abstraction gains symmetry.
- The backend interface stays free of state-level types (`state.ConnID`,
  `state.SessionID`) — no reverse import direction.
- A future remote backend implements the same three methods, and the wire
  carries only the backend `target` (`string(FrameID)`).
- `terminal_relay` holds the per-`(ConnID, SessionID)` map; the backend holds
  none of it.

## Alternatives

- **Access `termvt.Manager` directly from runtime** — rejected because it
  breaks the backend abstraction and blocks future backend swaps.
- **Add `SubscribeRaw` to `platform/termvt` itself** — rejected because
  modifying `platform/termvt` is out of scope (the layer below the backend
  abstraction stays frozen).

## Related requirements

- FR-014, FR-015, FR-016
