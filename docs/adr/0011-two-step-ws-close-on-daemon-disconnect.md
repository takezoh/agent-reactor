# ADR 0011 — Close WebSockets in two steps on daemon disconnect

Status: Accepted

## Context

When the daemon unix socket becomes unreachable, every active WebSocket on
`cmd/server` must terminate cleanly. We considered three options:

1. **Plain typed close** — `StatusGoingAway + reason='daemon-disconnected'`
   alone. Simple, but the current `client/web/app.js` only displays
   "detached" on close; the user has no signal *why* the connection ended.
2. **Maintain WS and reconnect transparently** — keep the WebSocket open and
   re-subscribe once a new daemon connection is established. Avoids visible
   disruption but pulls in sequence-gap handling, scrollback restore, and
   state-reconciliation logic far beyond α's scope.
3. **Two-step: control frame then typed close** — best-effort send of
   `controlMsg {k:'c', code:'daemon-disconnected'}` immediately followed by
   `StatusGoingAway` typed close.

Option 3 stays wire-compatible (the vanilla UI silently ignores unknown control
codes), while leaving a hook for β's React UI to render a meaningful banner
with zero further proto work.

## Decision

On daemon disconnect detection in `daemon_client`, the gateway:

1. Walks every active `AttachWS` session,
2. Sends `controlMsg {k:'c', code:'daemon-disconnected'}` on a best-effort
   basis (ignores write errors),
3. Immediately writes `StatusGoingAway + reason='daemon-disconnected'` typed
   close and tears down the connection.

`writeTypedClose(reason)` is a single helper used for every typed close path
to keep the protocol consistent.

## Consequences

- Wire compatibility is preserved: existing vanilla JS ignores the unknown
  control code.
- β's React UI can read the control frame and render an informed banner with
  zero new wire work.
- Implementation stays small — no sequence-gap handling, no scrollback
  restore.
- Current UX requires the user to reload after a daemon restart (acceptable
  in α; the daemon is assumed to be a stable long-running process).
- Frequent daemon restarts during development require user-side reload —
  document this in `docs/user/web-server.md` (deferred to PR-3 open question).

## Alternatives

- **Maintain WS with transparent re-subscribe** — rejected; pulls in
  out-of-scope complexity (gap, restore, state reconciliation).
- **Typed close only without control frame** — rejected; removes β's hook for
  UX improvement at zero cost.

## Related requirements

- FR-009, FR-010
