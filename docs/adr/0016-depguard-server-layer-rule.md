# ADR 0016 — Codify `server/*` as a client-layer HTTP gateway in depguard

Status: Accepted

## Context

`AGENTS.md` enumerates three layers — `platform` / `client` / `orchestrator`
— but `server/*` is not assigned to any of them in either the docs or
`src/.golangci.yml`. Historically `server/web` and `server/session` touched
only `platform/termvt` and each other, so the omission was harmless.

A1-α changes that: `server/web` will import `client/proto`, `client/state`,
and `client/runtime` (via the `daemon_client` and gateway adapter). Without a
depguard rule, this import pattern is technically allowed today but has no
documented justification.

## Decision

Update `src/.golangci.yml` depguard to add:

- `server/*` may import `client/proto`, `client/state`, `client/runtime`,
  and `platform/*`.
- `server/*` may **not** import `orchestrator/*`.
- `server/*` and `cmd/server` may **not** directly call `termvt.NewManager`
  or `agentlaunch.NewDispatcher` (enforces FR-002's "no parallel runtime"
  rule).

Add a section to `docs/ARCHITECTURE.md` positioning `server/*` as the HTTP
gateway sub-tree within the client layer.

## Consequences

- Documentation and lint agree on the layering.
- Future drift (e.g. someone importing orchestrator into the web stack) gets
  caught by CI.
- `ARCHITECTURE.md` needs a short update — ~one paragraph.

## Alternatives

- **Skip the rule, rely on convention** — rejected; future violations would
  not surface until review.
- **Promote `server/*` to its own layer** — rejected; the actual responsibility
  is "HTTP face of the client", so a separate layer overstates the boundary.

## Related requirements

- FR-002
