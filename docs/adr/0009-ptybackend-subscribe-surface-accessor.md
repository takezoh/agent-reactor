# ADR 0009 — Expose `SubscribeSurface(paneID)` on `PtyBackend`

Status: Accepted

## Context

`PtyBackend` currently holds `mgr *termvt.Manager` as an unexported field.
A1-α's `terminal_relay` needs to reach `termvt.Session.Subscribe` to fan output
out to subscribed connections, but it sits behind the `cfg.Tmux` backend
abstraction (tmux backend still exists pending phase C deletion). Reaching
into `mgr` directly would bypass the abstraction and prevent future backend
swaps (e.g. phase D's remote backend).

## Decision

Add three methods to `PtyBackend`:

```go
SubscribeSurface(paneID string) (*termvt.Subscription, error)
WriteSurface(paneID string, data []byte) error
ResizeSurface(paneID string, cols, rows int) error
```

`paneID` is the only argument. `ConnID` and `SessionID` are not exposed —
they are state-level concerns that the runtime owns. The tmux backend
implements these as `ErrNotImplemented` until phase C removes it.

`platform/termvt` is **not modified**. The accessors are pure forwarders to
`mgr.Get(paneID)` plus existing termvt methods.

## Consequences

- termvt's API stays frozen; only the backend abstraction gains symmetry.
- The backend interface stays free of state-level types (`state.ConnID`,
  `state.SessionID`) — no reverse import direction.
- Phase D's remote backend implements the same three methods, and the wire
  carries only `paneID`.
- Three more methods become visible on the backend surface. tmux backend
  paths return `ErrNotImplemented`, which is acceptable since tmux backend is
  scheduled for removal in phase C.
- `terminal_relay` holds the per-`(ConnID, SessionID)` map; the backend holds
  none of it.

## Alternatives

- **Access `termvt.Manager` directly from runtime** — rejected because it
  breaks the backend abstraction and blocks future backend swaps.
- **Add `SubscribeRaw` to `platform/termvt` itself** — rejected because
  modifying `platform/termvt` is out of scope for α (the layer below the
  backend abstraction stays frozen).

## Related requirements

- FR-014, FR-015, FR-016
