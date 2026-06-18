# ADR 0002 — Opt-in real app-server e2e validates the in-process fake

Status: Accepted

## Context

The routing-isolation contract ([ADR 0001](0001-multiplexed-backends-shared-routing-contract.md))
runs against an **in-process fake app-server** (the wired harness in
`routing_wired_test.go`: the existing `bindServer` handler plus a
`codexclient.Server` emitter). A fake is only as good as its fidelity: if it drifts from
how a real server frames `thread/started`, `turn/completed`, or
`item/agentMessage/delta`, then a test that is green against the fake stops
meaning "green against a real server" — exactly the false confidence that let
the original cross-talk bug ship.

The stream backend fronts the **codex app-server protocol** (WebSocket-over-UDS
JSON-RPC), not a specific binary. codex is the reference implementation today,
but the backend is agent-agnostic, so the fidelity backstop must not be wired to
codex alone — any conforming app-server is a valid validation target.

## Decision

Run the **same isolation invariant** against any configured **real** app-server,
gated so it never runs in normal CI:

- `routing_e2e_test.go` carries the build tag `//go:build e2e` (excluded from
  default builds) and additionally **skips** unless at least one app-server is
  configured via the environment:
  - `REACTOR_E2E_CODEX_BIN` — the codex app-server (convenience alias).
  - `REACTOR_E2E_APPSERVER_BIN` (+ `…_NAME`, `…_ARGS`) — any other conforming
    server.
  Each configured backend runs as its own subtest; absent all of them the test
  warns-and-skips, never fails.
- Each subtest launches the server via the production `Start` path (spawn → dial
  → initialize), binds two frames in distinct working dirs with prompts that echo
  unique markers, and asserts each marker reaches only its own frame using the
  shared `recordingRuntime` + `assertMarkerFrames`.

Fidelity is pinned two further ways, both already in the suite:

- The fake emits via `codexclient.Server.Emit*`, i.e. the **same helpers and wire
  shapes** a production server uses.
- Protocol method-name parity is asserted by the existing
  `TestSPEC_17_5_AgentSwitchEventParity` pattern (method names come from
  `codexschema`, not ad-hoc strings).

Setup and the full env contract live in
[stream-backend-e2e.md](../technical/client/stream-backend-e2e.md).

## Consequences

- A green contract against the fake can be trusted to mean the same against a
  real server; when a server changes its event wire shape, the e2e catches the
  fake going stale.
- It is **protocol-validation, not codex-validation**: codex is the default
  backend, but the same harness validates the fake against any app-server that
  speaks the protocol the backend fronts.
- The e2e is a **manual/local backstop**, not a CI gate — it needs a real binary
  (and, for full turns, model access) and is too slow/flaky to gate merges.
- Mirrors the broader project posture: fast fakes in CI, opt-in real integration
  for fidelity (cf. the env-gated `TestSPEC_17_8` real-tracker tests).
