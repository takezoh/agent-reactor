# Contributing

This expands the summary in [AGENTS.md](../../AGENTS.md). Read [ARCHITECTURE.md](../../ARCHITECTURE.md) first — every rule below exists to keep the three-layer structure intact.

## Build & test

```sh
make build                   # Build src/ → ./roost (+ roost-bridge)
make build-orchestrator      # → ./orchestrator
make build-claude-app-server # → ./claude-app-server
make build-all               # All 3 main binaries
make vet                     # go vet ./...
make lint                    # golangci-lint (depguard, funlen, staticcheck, etc.)

cd src && go test ./...                 # All tests
cd src && go test ./path/to/pkg         # One package
cd src && go test -run TestName ./...   # One test
```

Layer-scoped test run for orchestrator changes:

```sh
cd src && go test ./orchestrator/... ./platform/tracker/... ./cmd/orchestrator/... ./cmd/claude-app-server/...
```

## The three layers

| Binary | Source | Layer | Role |
|---|---|---|---|
| `roost` | `src/cmd/roost/` | client | TUI session lifecycle manager |
| `orchestrator` | `src/cmd/orchestrator/` | orchestrator | Autonomous poll/dispatch/reconcile + observability HTTP |
| `claude-app-server` | `src/cmd/claude-app-server/` | platform/orchestrator | Codex app-server stdio shim for Claude |

Import direction (enforced by `depguard`, see `src/.golangci.yml`):

```
cmd/* → client/* + orchestrator/* + platform/*   (no reverse)
platform/* imports neither client/* nor orchestrator/*
client/*  does not import orchestrator/*
orchestrator/* does not import client/*
```

## Rules

- **Follow the design principles in [ARCHITECTURE.md](../../ARCHITECTURE.md).** The four [core principles](../../ARCHITECTURE.md#core-principles-all-layers) (testability, single-writer event loop, decisions-separated-from-I/O, no fabricated fallbacks) hold in every layer; *how* you satisfy them follows from the layer's role. The two **decision-loop** layers, `client/` and `orchestrator/`, both use the strict Functional Core / Imperative Shell split: a pure `Reduce(state, event) → (state', []Effect)` over an immutable, **mutex-free** `State`, interpreted by a single event-loop shell that owns I/O and live handles. When editing `orchestrator/scheduler`, put decisions in `Reduce`/`reduce_*.go` and I/O in the shell (`effects_exec.go`) — do **not** add a mutex or reach for I/O inside the reducer. `platform/` is a library layer (not a decision loop): use dependency-injection seams for testability, and concentrate tool-specific knowledge there so the layers above stay generic.
- **Keep files under 500 lines and functions under 80 lines.** State-machine reducers in `client/state/reduce_*.go` are exempt from the function-length limit — dispatch tables stay cohesive (see [ARCHITECTURE.md → Layer Structure](../../ARCHITECTURE.md) and the [client internals](../technical/client/README.md)). File-length and naming rules still apply to reducers.
- **Actively use libraries.** Do not implement from scratch what an existing dependency covers.
- **Do not overwrite user config files** (`~/.roost/`). Setup commands must be idempotent.
- **Always write tests** for new features and bug fixes. Work is not complete without tests. Testability is the first core principle: refactor production code (interface extraction, env-var override, dependency injection) when needed to enable a test. Per-package coverage targets and the Tier scheme are in [testing](testing.md).
- **Respect layer/tool isolation.** This is one rule with two sides: `platform/` (`lib/<tool>/`, `hostexec/`, the external `credproxy` library) is where tool-specific host paths (`~/.claude*`) and env var names (`AWS_*`, `ANTHROPIC_*`, `GOOGLE_*`, `OPENAI_*`, …) live; the generic `client/` layers (`state/`, `runtime/`, `tui/`, `proto/`, `sandbox/`) must stay free of them. Violations are caught by `depguard` and `runtime/isolation_test.go`.

## Conventions

### Side-effect naming

Distinguish path computation from side effects by function name (see [ARCHITECTURE.md → Side-Effect Naming](../../ARCHITECTURE.md)):

| Pattern | Side effect | Example |
|---|---|---|
| `XxxPath()` | None (pure) | `LogDirPath`, `ConfigDirPath` |
| `EnsureXxx()` | Directory creation | `EnsureLogDir` |
| `LoadFrom(path)` | File read only | `config.LoadFrom` |
| `Load()` | Directory creation + file read | `config.Load` |

## Library selection

Before adding a third-party dependency:

1. List 2–3 candidates with their trade-offs (size, maintenance, license, API fit).
2. Justify the chosen one against the alternatives in the PR description.
3. Prefer libraries already in `go.mod` when they cover the use case.
4. Wire-format and persistence types must remain **stdlib-only**.

## Conformance

The SPEC §17 ↔ test correspondence table and the documented deviation posture live in [technical/orchestrator/symphony-conformance.md](../technical/orchestrator/symphony-conformance.md). Keep it current when you touch orchestrator behavior.
