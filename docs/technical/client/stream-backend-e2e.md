# Stream backend: real app-server e2e (fidelity backstop)

The routing-isolation harness ([stream-backend-testing.md](stream-backend-testing.md))
runs against an **in-process fake app-server**. This e2e runs the *same* routing
isolation invariant against a **real** app-server, so the fake is proven faithful
to the wire behaviour it imitates. Rationale:
[ADR 0002](../../adr/0002-optin-appserver-e2e-validates-fakes.md).

It is **not codex-specific**. The stream backend fronts the codex app-server
*protocol* (WebSocket-over-UDS JSON-RPC); any binary that serves that protocol —
launched as `<bin> [args] app-server --listen unix://<sock>` (the same interface
`Backend.Start` dials) — is a valid target. codex is the reference
implementation, but the harness validates the fake against the *protocol*, not a
single product.

## What it checks

Two frames are launched in distinct working dirs, each prompted to echo a unique
marker. The test asserts each marker is delivered **only** to its own frame —
the same `assertMarkerFrames` invariant the in-process contract uses. If a real
server routes per-frame the way the fake does, the fake is faithful.

## Configuration

The test discovers backends from the environment and runs one subtest per
configured server. With none set, it skips.

| Env var | Meaning |
|---|---|
| `REACTOR_E2E_CODEX_BIN` | Path to a codex binary (convenience alias; subtest `codex`). |
| `REACTOR_E2E_APPSERVER_BIN` | Path to any other conforming app-server. |
| `REACTOR_E2E_APPSERVER_NAME` | Subtest label for the generic server (default `appserver`). |
| `REACTOR_E2E_APPSERVER_ARGS` | Extra argv for the generic server, space-split. |

Build tag: `e2e` (this file is excluded from default builds).

Prerequisites: the binary must serve the codex app-server protocol over
WebSocket-over-UDS via `--listen unix://…`, and — because the test drives real
turns — have whatever model credentials it needs to answer a prompt.

## Running

```sh
# codex
REACTOR_E2E_CODEX_BIN=$(which codex) make test-e2e

# any other conforming app-server
REACTOR_E2E_APPSERVER_BIN=/path/to/server \
REACTOR_E2E_APPSERVER_NAME=myserver \
REACTOR_E2E_APPSERVER_ARGS="--flag value" \
  go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/ -v

# both at once → one subtest each
REACTOR_E2E_CODEX_BIN=$(which codex) REACTOR_E2E_APPSERVER_BIN=/path/to/server \
  go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/ -v
```

## CI posture

Never gated. The e2e needs a real binary and model access and is too slow/flaky
to block merges; CI relies on the fast in-process fake, and this backstop is run
manually/locally to keep the fake honest. This mirrors the env-gated
`TestSPEC_17_8` real-tracker integration tests.
