# ADR 0015 — Ship A1-α as three sequential PRs (proto → state → runtime+gateway)

Status: Accepted

## Context

A1-α touches three concern layers: wire format (`client/proto`), pure logic
(`client/state`), and I/O (`client/runtime` + `server/web`). Bundling them
into a single PR creates a review where wire shape, reducer semantics, and
networking adapters all compete for attention simultaneously. Rollback
becomes coarse-grained, and merge-conflict resolution against `main` gets
expensive.

The Master Plan adopted phased delivery for the same reasons at the A1 level
(α/β/γ/δ/ε). The same logic applies within α.

## Decision

Split A1-α into three PRs:

- **PR-1 — `client/proto` + codec + Fuzz**: Add `surface_command.go`,
  `surface_event.go`, switch updates in `codec.go`, the `protofake` package,
  and the fuzz corpus. Merges as dead code (no consumer yet). ~300-450 LOC.
- **PR-2 — `client/state` reducer + `Subscribers.Surface`**: Add the new
  `Ev`/`Eff` types, `state.go` field, `reduce_surface.go`, dispatch wiring,
  and the reducer table tests. Still dead code from a runtime perspective.
  ~250-400 LOC.
- **PR-3 — runtime relay + `server/web` gateway + `daemon_client`**: Bring
  the I/O layer online, swap `cmd/server` to gateway mode, ship the
  `platform/socketpath` helper, depguard rule, and `server/session` build
  tag quarantine. ~600-900 LOC.

Each PR keeps `make build-all && cd src && go test ./... -race` green.

## Consequences

- Reviews focus on one layer at a time.
- Rollback granularity matches concern boundaries.
- PR-1 and PR-2 carry dead code temporarily; this is acceptable since the
  proto and state additions form a closed contract that PR-3 simply
  consumes.
- PR-2's `Subscribers.Surface` map shape is fixed at merge — PR-3 must not
  drift from it. Reducer table tests in PR-2 act as the golden reference.

## Alternatives

- **Single PR** — rejected; review confusion and unmergeable wedge if any
  one layer needs revision.
- **Two PRs (wire+state / runtime+gateway)** — rejected; wire and state
  discussions still tangle in PR-1.

## Related requirements

(none directly)
