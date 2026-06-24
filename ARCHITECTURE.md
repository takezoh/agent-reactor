# Architecture

This is the canonical overview of the system: its scope, design principles, and three-layer structure. **Per-layer deep dives** — terminology, package responsibilities, design decisions, and dependency graphs — live under [`docs/technical/`](docs/technical/README.md); coding conventions are in [`docs/agent/contributing.md`](docs/agent/contributing.md).

## Scope

agent-reactor's client (run via the `arc` command) is a **session lifecycle manager — not an agent orchestrator**. It does not control what agents do; it gives you visibility and fast access to every agent session from a single tmux-based TUI. The separate `orchestrator` binary *does* drive agents autonomously against an issue tracker — a different concern in a different layer. This split is the top-level boundary the layer structure below enforces.

## Design Principles

The **core principles** below are normative for every layer that owns a decision loop; what differs is each layer's role, not whether the principles apply. The unifying goal is **testability** — and specifically the kind of testability that lets the code be written, tested, and corrected without a live environment: decision logic must be reachable by feeding inputs and asserting outputs, with no real I/O, concurrency, or wall-clock reads inside the code under test.

### Core principles (all layers)

- **Testability is the primary design constraint**: decision logic is a pure function of its inputs, so it can be exercised by feeding inputs and asserting outputs/state. "We can't test it" is a design defect, not a justification. This is the *why* behind the next two principles. Per-layer test patterns and the Coverage Tier scheme: [docs/agent/testing.md](docs/agent/testing.md)
- **Single-writer event loop**: state mutation is owned by one loop. Long-lived I/O sources (worker pool, stream readers, retry timers, file watchers) may only *emit events* to that loop — they never mutate state themselves. The client `runtime` loop and the orchestrator's `scheduler.Run` (`src/orchestrator/scheduler/scheduler.go`, one `for { select {} }`) are both instances of this.
- **Decisions separated from I/O**: the code that decides *what should happen* is a pure function; I/O, concurrency, and live handles live in a thin imperative shell. The shell performs the I/O and feeds the result back to the core as the next event — it never lets I/O leak into the decision.
- **No fabricated fallbacks**: do not synthesize "if source A is unavailable, use B" in a way that invents truth. In the client the status does not change until `Driver.Step` updates it; in the orchestrator issue truth comes from the tracker via reconcile and is never faked (a failed workflow reload keeps last-known-good config but *gates* dispatch rather than fabricating issue state).

### Per-layer realizations

A layer's *role* decides how it realizes the core. The canonical detail lives in the per-layer deep dive linked under [Layer Structure](#layer-structure).

- **Decision-loop layers — `client/` and `orchestrator/` — realize the core as strict Functional Core / Imperative Shell.** Each is a pure `Reduce(state, event) → (state', []Effect)` over an **immutable `State` (no mutex)**, interpreted by a single event-loop shell that owns all I/O and live handles (workers, timers) in id→handle maps. Both enforce no-mutex on the functional core via `forbidigo` (`client/state`, `orchestrator/scheduler`). Time enters `Reduce` as a value, never read from the wall clock inside it. Observability reads an immutable published snapshot **lock-free** (`atomic.Pointer[State]`), so there is no lock to contend or time out.
  - **`client/`** adds: value-type Driver plugins (per-frame `DriverState` round-trips through `Driver.Step`); Driver/Subsystem **isolation** keeping tool-specific concepts out of `state/`, `runtime/`, `tui/`, `proto/`, `sandbox/`. The only synchronous I/O permitted inside `Reduce` is bounded read-only filesystem stat. **Routing isolation** is the no-fabricated-fallbacks principle applied to multiplexed subsystems: a backend that fronts one app-server connection for many frames must bind each server thread to the frame that *initiated* it, never to an inferred/active frame, so one agent's output cannot surface in another agent's session (pinned by the `runtime/subsystem/stream` routing contract + fuzz — see [stream backend testing](docs/technical/client/stream-backend-testing.md)). Full detail: [client deep dive](docs/technical/client/README.md).
  - **`orchestrator/`** adds: **single-authority** (`ErrDuplicateDispatch` enforces SPEC §7.4); **agent-agnostic** dispatch (codex and `claude-app-server` emit one uniform event sequence); **reconcile = truth reconciliation** (agents transition issue state autonomously; reconcile re-reads the tracker and detects it). `scheduler.Reduce` returns `[]Effect`; the shell in `scheduler.go` interprets them and feeds I/O results back as events. Full detail: [orchestrator deep dive](docs/technical/orchestrator/README.md).
- **`platform/` is a library layer, not a decision loop, so FC/IS does not apply** — its testability comes from **dependency-injection seams** instead: external dependencies (`exec`, docker, network) sit behind injectable interfaces or env-var overrides (e.g. `lib/github.Runner`) so callers substitute fakes in tests. It is the base layer (imports neither `client/` nor `orchestrator/`); tool-specific knowledge (paths, env-var names, CLI invocations) is concentrated here so it stays out of the generic layers above; the agent-launch primitive (`agentlaunch`) is agent-agnostic; wire-format and persistence types are stdlib-only. Enforcement (import boundaries, name-literal leaks, no-mutex) is catalogued in [code & architecture enforcement](docs/technical/code-enforcement.md).

## Documentation

All documentation is organized by **audience × architecture layer** under [`docs/`](docs/README.md) — user guide, agent/contributor guide, per-layer technical deep dives, and cross-cutting topics (guardrails, code & architecture enforcement).

## Layer Structure

Three top-level trees under `src/`:

```
platform/      Shared infrastructure — the client, server, and orchestrator all depend on this
client/        client-specific code — TUI, state machine, runtime, drivers
orchestrator/  Symphony SPEC implementation — poll/dispatch/reconcile + observability HTTP
server/        HTTP/WS gateway — stateless proxy fronting the arc daemon over its Unix socket
cmd/           Binary entry points — cmd/arc/, cmd/server/, cmd/reactor-bridge/, cmd/orchestrator/, cmd/claude-app-server/
```

**Import direction**: `cmd/*` → `client/*` + `orchestrator/*` + `server/*` + `platform/*` → (no reverse). The layer boundaries are enforced by `depguard` (see `src/.golangci.yml`, rules `platform-no-client-or-orchestrator`, `client-no-orchestrator`, and `server-layer`):

| from \ to      | platform | client/proto | client/state | client/runtime | orchestrator | server |
|----------------|----------|--------------|--------------|----------------|--------------|--------|
| `platform/*`   | ✅        | ❌            | ❌            | ❌              | ❌            | ❌      |
| `client/*`     | ✅        | ✅            | ✅            | ✅              | ❌            | ❌      |
| `orchestrator/*` | ✅      | ❌            | ❌            | ❌              | ✅            | ❌      |
| `server/*`     | ✅        | ✅            | ✅            | ✅              | ❌            | ✅      |

Key invariants:
- `platform/*` imports neither `client/*` nor `orchestrator/*` nor `server/*`
- `client/*` does not import `orchestrator/*` or `server/*`
- `orchestrator/*` does not import `client/*` or `server/*`
- `server/*` does not import `orchestrator/*`

The full set of `depguard` rules (including the intra-`client/` isolation rules) and every other code-level enforcement mechanism are catalogued in [code & architecture enforcement](docs/technical/code-enforcement.md).

### The layers at a glance

- **[`platform/`](docs/technical/platform/README.md)** — shared base: the agent-launch primitive (`agentlaunch`: argv-based `Spawn` + `SplitArgs`, host/container `Dispatcher`, on `procgroup`), sandbox backends, host-exec and MCP-proxy brokers, path translation, logger, tool wrappers (`lib/<tool>`), trackers, metrics, credential providers. Tool-specific knowledge is allowed here so it stays out of the generic layers above. Agent-agnostic launch lives here; per-agent command construction stays in `lib/<tool>`, while transport, `codexclient.Conn`, and `Handler` remain per-layer.
- **[`client/`](docs/technical/client/README.md)** — all of the client: the pure `state/` domain core, `runtime/` imperative shell, value-type `driver/` plugins, `runtime/subsystem/` (`cli` and `stream`), the `proto/` IPC wire layer, and the Bubbletea `tui/`. Terminology, the design-decision log, and the full dependency graph are documented there.
- **[`orchestrator/`](docs/technical/orchestrator/README.md)** — a TUI-less, single-authority service implementing the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md): `workflowfile/`, `wfconfig/`, `scheduler/` (poll/dispatch/reconcile), `workspace/`, `agent/`, `prompt/`, `httpserver/`, `lineargql/`. It shares `platform/` with the client but does not import `client/`. Per-issue workspaces are local git clones of the source repo (GitHub); issue state lives in the tracker (Linear or GitHub). SPEC ↔ package correspondence and deviation posture: [`docs/technical/orchestrator/symphony-conformance.md`](docs/technical/orchestrator/symphony-conformance.md).
- **`server/`** — HTTP/WS gateway that fronts the `arc daemon` over its Unix socket; stateless proxy bridging the browser front-end to `client/runtime`. Does not import `orchestrator/*`. See [Server gateway (server/*)](#server-gateway-server) below for full detail.

### Server gateway (server/*)

`server/*` is the HTTP/WS façade that fronts a long-lived `arc daemon`
(the `client/runtime` event loop) over its Unix socket. It is a **stateless
proxy** — sessions and side effects live in the daemon — so the same daemon
can be reached by the TUI (`cmd/arc`), the browser front-end (`cmd/server` +
xterm.js), and future native clients with consistent behaviour.

- `server/web/daemon_client.go` wraps `proto.Client` with an eager dial +
  supervisor goroutine. `Health()` / `LastError()` / `LastAttemptAt()` give
  the HTTP layer enough signal to return `503` while the daemon is down
  ([ADR 0012](docs/adr/0012-daemon-client-eager-dial-supervisor.md)).
- `server/web/gateway.go` bridges one WebSocket to one daemon-side surface
  subscription (`proto.CmdSurfaceSubscribe`). On daemon disconnect it sends
  a `controlMsg{k:"c"}` payload and immediately follows with a typed close
  (`StatusGoingAway`) — the two-step shutdown defined in
  [ADR 0011](docs/adr/0011-two-step-ws-close-on-daemon-disconnect.md).
- `server/web/mux.go` maps REST `/api/sessions` GET/POST/DELETE to
  `proto.CmdEvent{Event: state.Event{Create,List,Stop}Session}` via the
  daemon client; cols/rows are packed into `state.LaunchOptions`
  ([ADR 0005](docs/adr/0005-cmd-server-as-arc-daemon-gateway.md), FR-022).
- `cmd/server/main.go` is the binary entry point: it resolves the daemon
  socket via `platform/socketpath.ResolveDaemonSocket(-arc-sock, ARC_SOCKET,
  ~/.agent-reactor/arc.sock)`, boots `DaemonClient`, and serves
  `server/web.NewMux(daemon, token)` behind a bearer-token + ws-ticket gate
  (auth invariant unchanged from the previous in-process design;
  [ADR 0017](docs/adr/0017-platform-socketpath-helper.md)).
- `server/session` was removed in A1-ε
  ([ADR 0014](docs/adr/0014-server-session-legacy-build-tag.md), superseded);
  the directory and its `legacy_session` build tag no longer exist.

**Design invariant**: `server/*` never calls `platform/termvt`, `platform/agentlaunch`,
or any other platform primitive that controls agent I/O directly. All session
state and side effects remain in the daemon. The server layer is purely a
protocol translator: JSON-over-HTTP/WS on the external face, typed `proto`
IPC on the daemon face.

**Import direction** (enforced by `depguard` rule `server-layer`,
[ADR 0016](docs/adr/0016-depguard-server-layer-rule.md)): `server/*` may
import `platform/*`, `client/proto`, `client/state`, and `client/runtime`
(the subset needed to speak IPC). It must not import `orchestrator/*`.

Related ADRs: [0005](docs/adr/0005-cmd-server-as-arc-daemon-gateway.md) (cmd/server as gateway) ·
[0011](docs/adr/0011-two-step-ws-close-on-daemon-disconnect.md) (two-step WS close) ·
[0012](docs/adr/0012-daemon-client-eager-dial-supervisor.md) (daemon client supervisor) ·
[0014](docs/adr/0014-server-session-legacy-build-tag.md) (legacy_session build tag) ·
[0016](docs/adr/0016-depguard-server-layer-rule.md) (depguard server layer rule).

Files matching `client/state/reduce_*.go` host state-machine dispatch tables. They are exempt from the 80-line function limit (see [AGENTS.md](AGENTS.md)) because forced extraction of dispatch arms fragments the state machine without adding clarity. File-length (500 lines) and naming rules still apply.

The daemon and TUI are separate processes communicating via typed IPC (`proto`) over a Unix socket, with two physical endpoints (host + container). Details, the per-package breakdown, terminology, and the design-decision log are in the [client deep dive](docs/technical/client/README.md).
