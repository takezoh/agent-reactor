# Code & Architecture Enforcement

The mechanisms that keep the **codebase** true to its intended architecture. Unlike review-dependent conventions, most are rejected mechanically at **lint or compile time**. For each: what it prevents, where it is defined, how it is enforced, and how a developer declares an exception.

The design principles themselves are owned by [ARCHITECTURE.md](../../ARCHITECTURE.md); this document covers their enforcement. (Runtime controls over the autonomous *agents* are a separate concern — see [guardrails.md](guardrails.md).)

## 1. Import boundaries (depguard)

The dependency direction across the three layers (platform / client / orchestrator), and intra-layer subsystem isolation, are enforced by `depguard`. Definitions are in `depguard.rules` of `src/.golangci.yml`; violations are rejected by `make lint`.

```mermaid
flowchart TD
    subgraph orchestrator["orchestrator/"]
        ORCH["scheduler / agent / …"]
    end
    subgraph client["client/"]
        STATE["state/<br/>(pure core)"]
        TUI["tui/"]
        PROTO["proto/"]
        RT["runtime/"]
        DRIVER["driver/"]
    end
    subgraph platform["platform/"]
        PLAT["lib / sandbox / …"]
        CODEXC["agent/codexclient/"]
    end

    ORCH -->|OK| PLAT
    CLIENTBOX["client/*"] -->|OK| PLAT
    ORCH -. "✗ converse of client-no-orchestrator<br/>(client ⊄ orchestrator)" .-> CLIENTBOX
    PLAT -. "✗ platform-no-client-or-orchestrator" .-> CLIENTBOX
    PLAT -. "✗" .-> ORCH
    CODEXC -. "✗ codexclient-isolation" .-> CLIENTBOX

    STATE -. "✗ state-pure-core<br/>(no driver/lib/runtime/tui/proto)" .-> DRIVER
    TUI -. "✗ tui-no-driver-lib" .-> DRIVER
    PROTO -. "✗ proto-isolation" .-> DRIVER
    RT -. "✗ runtime-no-driver (root only)" .-> DRIVER
```

| Rule | Scope | Deny (summary) |
|---|---|---|
| `platform-no-client-or-orchestrator` | `platform/**` | `client/`, `orchestrator/` |
| `client-no-orchestrator` | `client/**` | `orchestrator/` |
| `state-pure-core` | `client/state/**` | `driver/`, `platform/lib`, `runtime/`, `tui/`, `proto/` |
| `tui-no-driver-lib` | `client/tui/**` | `driver/`, `platform/lib` |
| `worker-no-driver-lib` | `client/runtime/worker/**` | `driver/`, `platform/lib` |
| `sandbox-tool-agnostic` | `platform/sandbox/**` | `driver/`, `platform/lib`, `runtime/` |
| `proto-isolation` | `client/proto/**` | `driver/`, `platform/lib`, `runtime/`, `tui/` |
| `runtime-no-driver` | `client/runtime/*.go` (root only) | `driver/` |
| `subsystem-isolation` | `client/runtime/subsystem/**` | `tui/` |
| `codexclient-isolation` | `platform/agent/codexclient/**` | `client/`, `orchestrator/` |

Key intents:

- **Layer direction**: platform is the base and knows nothing above it; client does not know orchestrator (the converse is guaranteed by `platform-no-...`).
- **`state/` purity**: the state machine has no I/O and no side effects — a pure functional core. It cannot import driver/runtime/tui at all.
- **`runtime-no-driver`**: only the runtime **root** is forbidden from importing driver. Tool-specific backends move to `runtime/subsystem/<kind>/`. Exception: `client/driver/vt` is explicitly allowed in `exclusions.rules`.
- **`codexclient` reusability**: a shared protocol transport, so it knows nothing of agent-reactor internals.

## 2. Pure-core purity (forbidigo + ruleguard)

The decision-loop functional cores — `client/state` and `orchestrator/scheduler` — must hold no mutex, spawn no goroutine, read no wall clock, and perform no I/O (the only permitted synchronous I/O is bounded read-only `os.Stat`). State is folded as an immutable value; concurrency, timers, and I/O live in the event-loop shell. Observability reads an immutable published snapshot lock-free (`atomic.Pointer[State]`), so neither core needs a mutex.

| Invariant | Enforced by | Notes |
|---|---|---|
| No mutex | `forbidigo` (`sync.Mutex` / `sync.RWMutex`, pkg-scoped) | message: "… is a pure functional core — no mutexes allowed" |
| No goroutine | `gocritic` ruleguard (`gorules/purecore.go`) | `go` is a `GoStmt`, invisible to forbidigo's CallExpr matching |
| No wall clock | `gocritic` ruleguard | `time.Now` / `time.Since` — time enters `Reduce` as a value |
| No direct I/O | `gocritic` ruleguard | `os.Open`/`WriteFile`/…, `net.Dial`/`Listen`, `exec.Command`; `os.Stat` allowed |

`client/state` is wholly pure, so the ruleguard rules apply to every non-test file in it. In `orchestrator/scheduler` the pure reducer and the imperative shell share one package, so the rules skip the shell files (`scheduler.go`, `effects_exec.go`, `clock.go`, `watch.go`) — these legitimately own the loop, timers, and I/O. Test files are exempt.

## 3. Length limits

| Limit | Value | Enforced by |
|---|---|---|
| Function length | 80 lines (`funlen`, `ignore-comments: true`) | lint (`.golangci.yml`) |
| File length | 500 lines (`revive` `file-length-limit`, skipping comments/blanks) | lint (`.golangci.yml`) |

Length exceptions (in `exclusions.rules`):

- `_test.go` — tests relax both function and file length (table-driven tests grow large by nature) as well as errcheck.
- `client/state/reduce_*.go` — state-machine dispatch tables stay cohesive as one unit (function-length exempt).

Exceptions are declared **by path pattern in `.golangci.yml`, not by an in-code annotation** — anything matching `reduce_*.go` is exempt automatically. Generated code (`codexschema/v*/types.gen.go`, etc.) is auto-excluded from length checks too.

## 4. Feature flags

`platform/features/features.go` has **two mechanisms that share no key space** — pick one based on whether the experimental code should physically exist in the binary. The C analogue: runtime flag is `if () {}`, compile-time flag is `#if / #endif`.

| Kind | Mechanism | Toggle | Stays in the binary? | Use when |
|---|---|---|---|---|
| runtime | `Flag` constant + injected `Set` | `~/.agent-reactor/settings.toml` `[features.enabled]` | both branches compiled | the user should opt in without rebuilding |
| compile-time | top-level `const` bool guarded by a build tag | `go build -tags <tag>` (e.g. `make build-experimental`) | off-side removed by dead-code elimination | the code is unfinished / unsafe or must not enter release binaries |

**Runtime — add:** declare a `Flag` constant and list it in `All()`; read it as `st.Features.On(features.Peers)` (`features.go:36`). Gating is allowed in `state/`, `runtime/`, `tui/` — **not** in `driver/`, where driver-specific gating uses `config.Drivers[name]` instead. Users opt in under `[features.enabled]`. `FromConfig` **silently ignores unknown keys** (`features.go:46`), so when a flag stabilises you delete the constant and inline the enabled branch with no config migration.

**Compile-time — add:** create a `//go:build <tag>` / `//go:build !<tag>` file pair exporting the same `const` bool, then gate code with `if features.MyFeat { ... }` — the off-side is removed because `MyFeat` is a `const`. For larger code, put the implementation behind the tag and provide a no-op stub on the `!tag` side so callers need no guarding. Add a Makefile target for first-class variants; CI builds both.

## 5. Wire format is stdlib-only (depguard)

Wire-format / persistence types are written with **stdlib only (`encoding/json`)** (AGENTS.md / ARCHITECTURE.md) — a portability constraint. The `depguard` rule `proto-wire-stdlib-only` (scope `client/proto/**`) denies codec libraries (protobuf, msgpack, cbor) from the wire layer; `client/proto/codec.go` uses only `encoding/json`. The rule is a deny-list of the realistic offenders rather than a stdlib allow-list, matching the intent: do not bring a new codec library into the wire layer.

## 6. Routing isolation (test-pinned)

A multiplexed subsystem backend — one app-server connection fronting many frames — must route every server event to the frame that *initiated* the thread, never to an inferred/active frame. A leak is **cross-talk**: one agent's output surfaces in another agent's session. This is the [No fabricated fallbacks](../../ARCHITECTURE.md#design-principles) principle for `runtime/subsystem/stream`.

Unlike sections 1–5 this cannot be caught at lint/compile time (it is a runtime routing property), so it is **test-pinned**: the [routing-isolation contract](client/stream-backend-testing.md) (`TestStreamRoutingContract`, `TestStreamRoutingWiredIsolation`, `FuzzStreamRouting`) asserts that every emitted `EvSubsystem` carries the owning frame's id. The demux binds threads synchronously at creation/resume (`bindThread`), so an unknown `thread.started` is dropped rather than adopted by the active frame; the contract guards against reintroducing such a fabricated fallback. Rationale: [ADR 0001](../adr/0001-multiplexed-backends-shared-routing-contract.md); fidelity backstop: [ADR 0002](../adr/0002-optin-appserver-e2e-validates-fakes.md).

Exception: none — a multiplexed backend that cannot satisfy the invariant is a defect, not a candidate for opt-out.

## 7. Fan-out isolation (test-pinned)

The tmux-free multiplexer `platform/termvt` is the same shape as §6 — one source (a pty) fanned out to many subscribers — and shares the cross-talk failure mode. Its **fan-out isolation** invariant: every event reaches exactly the live subscribers of its own session (all, in order, control-before-output), and a subscriber that cannot keep up is *severed*, never allowed to block or corrupt the others. Cross-talk here is one session's bytes surfacing in another's terminal, or a slow client wedging a healthy one.

Like §6 this is a runtime property, not lint/compile-catchable, so it is **test-pinned**: the [fan-out isolation contract](platform/termvt-multiplexer-testing.md) (`TestFanoutDeliversToEverySubscriber`, `TestManagerSessionsDoNotCrossTalk`, `TestSlowSubscriberDoesNotStarveFast`, `TestControlPrecedesOutputInChunk`) runs against a real pty under `-race`, and `server/web`'s `FuzzInbound` pins the untrusted client→server frame decode (no panic, no non-positive resize). Rationale: [ADR 0003](../adr/0003-termvt-fanout-isolation.md). Unlike §6 there is no opt-in e2e tier — termvt has no in-process fake to validate (its only backend is a real pty).

Exception: none — a multiplexer that cannot satisfy fan-out isolation is a defect, not a candidate for opt-out.

## Related

- Canonical design principles: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- Per-layer deep dives: [platform](platform/README.md) · [client](client/README.md) · [orchestrator](orchestrator/README.md)
- Agent-control guardrails (admission, concurrency, capability, autonomy, liveness): [guardrails.md](guardrails.md)
