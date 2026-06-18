# Stream backend: routing-isolation test harness

The stream subsystem backend multiplexes many frames (agents) over a single
codex app-server connection. Its one safety-critical property is **routing
isolation**: an event from a thread must reach only the frame that started that
thread. A leak is *cross-talk* — one agent's output (including tool results)
surfacing in another agent's session. This page documents the harness that pins
that property. Rationale lives in
[ADR 0001](../../adr/0001-multiplexed-backends-shared-routing-contract.md) and
[ADR 0002](../../adr/0002-optin-appserver-e2e-validates-fakes.md). Setup for the
real-server backstop: [stream-backend-e2e.md](stream-backend-e2e.md).

## The invariant

> Every `state.EvSubsystem` emitted from a thread T carries `FrameID == owner(T)`,
> where `owner(T)` is the frame whose `BindFrame` started/resumed T.

Corollary: thread→frame binding must derive from the **initiating request**,
never from ambient state such as the active frame (a "fabricated fallback").

## Files

| File | Role |
|---|---|
| `routing_contract_test.go` | `recordingRuntime`, `assertMarkerFrames`, the direct-drive `inProc` harness, and `TestStreamRoutingContract` (the case table). |
| `routing_wired_test.go` | the `wired` harness driving the real `codexclient.Conn` against a fake app-server (reuses `bindServer` + `codexclient.Server`); async, run under `-race`. |
| `routing_fuzz_test.go` | `FuzzStreamRouting` (stdlib `testing.F`) over random interleavings. |
| `routing_e2e_test.go` | `//go:build e2e` real **app-server** fidelity backstop (any conforming server, not just codex); skips when no backend env is set. See [stream-backend-e2e.md](stream-backend-e2e.md). |

`recordingRuntime` is the shared observation point: it records each emitted
`EvSubsystem`'s `FrameID`, and markers travel in `Payload.LastAssistantMessage`,
so `framesWithMarker` answers "which frames received this thread's output".

## How a case is built

The contract drives the backend the way `BindFrame` would, then feeds server
events:

```go
h := newInProc(t)
h.bindCold("A", "/work")     // unbound frame awaiting its thread
h.bindCold("B", "/work")     // same cwd — the shared-container case
h.setActive("B")             // B is foregrounded
h.started("tA", "/work")     // ground truth: tA is A's thread
h.message("tA", "MARK_A")
h.wantMarkerFrames("MARK_A", "A")   // isolation: only A may receive it
```

The GREEN cases (distinct-cwd routing, reverse-order completion, resume + cold
mix, release cleanup) run in CI as regression guards. The cross-talk pins
(`crosstalk_ambiguous_cwd`, `crosstalk_zero_candidates_binds_active`, the wired
sibling, and the fuzz isolation check) are **RED on the current demux** and gated
behind `REACTOR_ROUTING_PINS` so CI stays green until the fix. When the demux is
fixed to bind by initiating request, the pins flip GREEN and the gate is removed.

## Running

```sh
# GREEN regression guards + structural fuzz seeds (the ci job's test step;
# a separate `fuzz` CI job also actively fuzzes — see .github/workflows/ci.yml)
cd src && TMPDIR=/tmp go test ./client/runtime/subsystem/stream/

# concurrency check
cd src && go test -race ./client/runtime/subsystem/stream/

# demonstrate the cross-talk pins (RED until the fix lands)
cd src && REACTOR_ROUTING_PINS=1 go test ./client/runtime/subsystem/stream/ \
  -run 'TestStreamRoutingContract/crosstalk|TestStreamRoutingWiredCrosstalk'

# active fuzzing
cd src && go test -run x -fuzz 'FuzzStreamRouting$' -fuzztime=30s \
  ./client/runtime/subsystem/stream/

# fidelity backstop against a real app-server (opt-in; see stream-backend-e2e.md)
REACTOR_E2E_CODEX_BIN=$(which codex) \
  go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/
```

## Invariant ↔ pinning tests

| Behaviour | Pinned by | Status on current demux |
|---|---|---|
| Distinct-cwd threads route per frame | `TestStreamRoutingContract/two_frames_distinct_cwd`, `interleaved_starts_distinct_cwd` | GREEN |
| Completion routes by exact thread id | `.../completion_reverse_order` | GREEN |
| Resume + cold-start coexist | `.../resume_then_coldstart_same_backend` | GREEN |
| Released frame drops stray events | `.../release_drops_stray_events` | GREEN |
| Wired path routes & is race-clean | `TestStreamRoutingWiredHappyPath` | GREEN |
| Ambiguous-cwd start ≠ active-frame steal | `.../crosstalk_ambiguous_cwd`, `TestStreamRoutingWiredCrosstalk` | RED (gated) |
| Foreign thread not adopted by active frame | `.../crosstalk_zero_candidates_binds_active` | RED (gated) |
| Random interleavings preserve isolation | `FuzzStreamRouting` (isolation tier) | RED (gated) |
| No duplication / garbage-frame / panic | `FuzzStreamRouting` (structural tier) | GREEN |
| Fake matches real app-server wire behaviour | `TestStreamRoutingE2EIsolation` (opt-in, per backend) | n/a |
