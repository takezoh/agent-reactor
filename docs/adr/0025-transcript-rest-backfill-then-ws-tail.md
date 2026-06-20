# ADR 0025 — Transcript / event-log uses REST backfill + WebSocket tail

Status: Accepted

## Context

A1-δ exposes per-session transcripts and event-logs to the browser. The
files live on the daemon side under `<dataDir>/events/<frameID>.transcript`
and `.jsonl`. They grow append-only as the session runs.

Two options for delivery:

1. **WebSocket-only tail**: the daemon broadcasts every line as
   `EvtSessionFileLine`; the browser starts subscribed and accumulates.
   For a long-running session the initial backlog is huge and would
   either be re-sent on every browser attach or lost entirely.
2. **REST backfill + WS tail**: on session select, the browser issues
   `GET /api/sessions/{id}/{transcript|event-log}?offset=0`, then opens
   a WS subscription for live tail. Subsequent reconnects re-issue REST
   from the last seen offset.

Option 1 forces the daemon into a "replay history" responsibility that
doesn't belong on a pub/sub channel. Option 2 cleanly separates
"historical bulk" (REST) from "live tail" (WS) — each protocol does what
it's best at.

## Decision

Use REST for the initial backfill and WebSocket for the live tail.

- `GET /api/sessions/{id}/transcript?offset=N` returns bytes from `N` to
  EOF, with `ETag: <frameID>:<size>` and 304 support.
- `GET /api/sessions/{id}/event-log?offset=N` is the parallel for the
  structured event-log.
- The WebSocket forwards `EvtSessionFileLine` as `transcript-tail` /
  `event-log-tail` frames once a session is selected.
- The client tracks `lastSeenOffset` per session/kind so that on
  reconnect it can REST-backfill from that point, then resume tail.

## Consequences

- Backfill bandwidth is bounded by file size; the daemon does not
  pre-load all sessions' histories into memory.
- The WS channel stays "incremental events"; no replay semantics needed.
- The browser handles the seam between REST tail and WS tail (a small
  overlap zone may exist — see FR-δ10's "last REST offset deduplication"
  policy).
- ETag support lets repeated session switches reuse cached responses.

## Alternatives

- **WS-only with replay** — bloats the WS layer with replay
  responsibility; rejected.
- **REST-only with polling** — wastes bandwidth and adds latency to live
  updates; rejected.
- **Server-Sent Events for tail** — viable, but we already have WS
  established and SSE adds a second long-poll connection per browser;
  rejected.

## Related requirements

- FR-δ01, FR-δ02, FR-δ04, FR-δ05, FR-δ07, FR-δ10, FR-δ12
