# ADR 0017 — Share daemon socket path resolution via `platform/socketpath`

Status: Accepted

## Context

The arc daemon listens on `~/.agent-reactor/arc.sock`. A1-α adds `cmd/server`
as a dialer of that same socket, parameterised by a `-arc-sock` flag. If
both sides hard-code the path independently, operational drift becomes a
silent failure mode (the daemon listens on path A while the server dials path
B and silently 503s).

## Decision

Add `platform/socketpath/socketpath.go` with:

```go
func ResolveDaemonSocket(flag string, envName string, fallback string) string
```

Resolution order: explicit flag → `ARC_SOCKET` environment variable →
fallback (which is the hard-coded default `~/.agent-reactor/arc.sock`).

`cmd/arc` (daemon listener) and `cmd/server` (gateway dialer) both call
the same helper. Tests can swap a tempdir socket via the env variable.

Implementation: stdlib only (`os.UserHomeDir`, `filepath.Join`). Less than
40 lines.

## Consequences

- One source of truth for the socket path; operational drift impossible by
  construction.
- Tests get a clean override without touching production paths.
- One new package in `platform/` — the layer chosen because both daemon
  (client layer) and server (client layer / gateway) need it, and
  `platform/*` does not import either, satisfying the existing import
  direction.

## Alternatives

- **Hard-code on both sides** — rejected; introduces silent drift.
- **Put the helper in `client/`** — rejected; daemon would import client
  for what is a process-environment concern.

## Related requirements

- FR-001
