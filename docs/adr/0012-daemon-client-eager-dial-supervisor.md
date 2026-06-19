# ADR 0012 — `daemon_client` uses eager dial + full-jitter exp backoff supervisor

Status: Accepted

## Context

`cmd/server` needs a `proto.Client` connection to the arc daemon. Two
properties are non-negotiable:

- The server process survives daemon restarts (the daemon may go down for
  maintenance without taking the web server with it).
- Disconnects propagate cleanly to all attached WebSockets (see
  [ADR 0011](./0011-two-step-ws-close-on-daemon-disconnect.md)).

`proto.Client` itself is single-shot: it owns a single `net.Conn` plus a
`closeOnce`. After disconnect, reconnection means creating a new client
instance and rewiring every consumer of the old `Events()` channel.

## Decision

Implement `daemon_client.go` as a wrapper with the following behaviour:

- **Eager dial** at boot. If the daemon is unreachable, the supervisor starts
  immediately rather than failing startup.
- **Supervisor goroutine** runs the reconnect loop with **full-jitter
  exponential backoff**: delay = `rand(0, min(cur, 4s))`, starting at 250ms,
  cap 4s. Retries are unbounded.
- On (re)connect, swap the internal client pointer atomically. Close the
  previous `Events()` channel so all `AttachWS` consumers observe end-of-
  stream and execute their two-step typed close (ADR 0011).
- Expose `Health() bool`, `LastError() error`, and `LastAttemptAt() time.Time`
  via atomic reads. `/healthz` reports these.
- Disconnected calls return `ErrDaemonUnavailable` synchronously; `mux.go`
  maps this to HTTP 503.
- In-flight requests on the old client are cancelled (their `ctx.Done()`
  fires when the supervisor closes the connection) — they must not block
  reconnect.
- `slog.Debug` / `slog.Warn` log with stable keys: `conn_id`, `session_id`,
  `sub_id`, `reason`.

No new dependencies — `math/rand` + `time` only.

## Consequences

- The connection state is observable from outside (`/healthz`), making 503
  triage straightforward.
- Reconnect is robust against daemon restarts.
- Implementation fits in ~30-50 lines for the backoff loop itself.
- Disconnected WSes are explicitly closed rather than left dangling; aligns
  with ADR 0011.
- `req_id` accounting must handle "in-flight cancelled by reconnect" without
  panicking. `daemon_client_test.go` exercises this case.

## Alternatives

- **Lazy dial** — rejected; the first REST call would absorb the dial
  latency, and boot races become user-visible.
- **Maintain WS with transparent re-subscribe via new client** — rejected;
  re-introduces the complexity that ADR 0011 explicitly avoided.

## Related requirements

- FR-001, FR-009, FR-010
