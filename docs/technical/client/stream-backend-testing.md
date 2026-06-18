# Stream backend: routing-isolation test harness

The stream subsystem backend multiplexes many frames (agents) over a single
codex app-server connection. Its one safety-critical property is **routing
isolation**: an event from a thread must reach only the frame that owns that
thread. A leak is *cross-talk* — one agent's output (including tool results)
surfacing in another agent's session. This page documents the harness that pins
that property. Rationale lives in
[ADR 0001](../../adr/0001-multiplexed-backends-shared-routing-contract.md) and
[ADR 0002](../../adr/0002-optin-appserver-e2e-validates-fakes.md). Setup for the
real-server backstop: [stream-backend-e2e.md](stream-backend-e2e.md).

## The invariant

> Every `state.EvSubsystem` emitted from a thread T carries `FrameID == owner(T)`,
> where `owner(T)` is the frame whose `BindFrame` started/resumed T.

Corollary: thread→frame binding derives from the **initiating request**, never
from ambient state such as the active frame (a "fabricated fallback").

## How the fix makes cross-talk impossible

`bindThread` creates each thread **synchronously** — cold start issues a
`thread/start` request and binds the returned id; resume binds the resumed id —
so the frame→thread mapping is established before any event arrives. Two frames
sharing a cwd therefore get **distinct thread ids**, and every server event
routes by exact thread id (`frameForThread`). A `thread.started` for an unknown
thread is dropped, never adopted by a cwd or active-frame heuristic. (Before the
fix, cold threads were bound asynchronously by matching the start cwd, falling
back to the active frame — the cross-talk bug; see ADR 0001.)

## Files

| File | Role |
|---|---|
| `routing_contract_test.go` | `recordingRuntime`, `assertMarkerFrames`, the direct-drive `inProc` harness, and `TestStreamRoutingContract` (the case table). |
| `routing_wired_test.go` | the `wired` harness driving the real `codexclient.Conn` against a fake app-server (reuses `bindServer` + `codexclient.Server`); async, run under `-race`. |
| `routing_fuzz_test.go` | `FuzzStreamRouting` (stdlib `testing.F`) over random message/release interleavings. |
| `routing_e2e_test.go` | `//go:build e2e` real **app-server** fidelity backstop (any conforming server, not just codex); skips when no backend env is set. See [stream-backend-e2e.md](stream-backend-e2e.md). |

`recordingRuntime` is the shared observation point: it records each emitted
`EvSubsystem`'s `FrameID`, and markers travel in `Payload.LastAssistantMessage`,
so `framesWithMarker` answers "which frames received this thread's output".

## How a case is built

The direct-drive contract binds frames the way `bindThread` leaves them (each to
a distinct thread id), then feeds server events into the handlers:

```go
h := newInProc(t)
h.bind("A", "tA", "/work") // distinct thread id, even with a shared cwd
h.bind("B", "tB", "/work")
h.message("tA", "MARK_A")
h.message("tB", "MARK_B")
h.wantMarkerFrames("MARK_A", "A") // isolation: only A
h.wantMarkerFrames("MARK_B", "B")
```

The wired harness exercises the real path end-to-end: a cold `BindFrame` issues
`thread/start`, binds the returned id, and `TestStreamRoutingWiredIsolation`
asserts two same-cwd frames get distinct ids and never cross-talk — under
`-race`.

## Running

```sh
# regression guards + structural fuzz seeds (the ci job's test step;
# a separate `fuzz` CI job also actively fuzzes — see .github/workflows/ci.yml)
cd src && TMPDIR=/tmp go test ./client/runtime/subsystem/stream/

# concurrency check
cd src && go test -race ./client/runtime/subsystem/stream/

# active fuzzing
cd src && go test -run x -fuzz 'FuzzStreamRouting$' -fuzztime=30s \
  ./client/runtime/subsystem/stream/

# fidelity backstop against a real app-server (opt-in; see stream-backend-e2e.md)
REACTOR_E2E_CODEX_BIN=$(which codex) \
  go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/
```

## Invariant ↔ pinning tests

| Behaviour | Pinned by |
|---|---|
| Same-cwd frames (distinct ids) never cross-talk | `TestStreamRoutingContract/two_frames_same_cwd_distinct_threads`, `TestStreamRoutingWiredIsolation` |
| Completion routes by exact thread id | `.../completion_reverse_order` |
| `thread.started` confirms an already-bound thread | `.../thread_started_confirms_bound` |
| Unknown `thread.started` is dropped (no cwd/active adoption) | `.../thread_started_for_unknown_thread_drops`, `TestHandleThreadStartedUnknownThreadDrops` |
| Released frame drops stray events | `.../release_drops_stray_events` |
| Random interleavings preserve by-id isolation | `FuzzStreamRouting` |
| No duplication / garbage-frame / panic | `FuzzStreamRouting` (structural checks) |
| Fake matches real app-server wire behaviour | `TestStreamRoutingE2EIsolation` (opt-in, per backend) |
