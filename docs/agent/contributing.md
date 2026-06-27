# Contributing

This expands the summary in [AGENTS.md](../../AGENTS.md). Read [ARCHITECTURE.md](../../ARCHITECTURE.md) first — every rule below exists to keep the three-layer structure intact.

## Build & test

```sh
make build-server            # Build src/ → ./server (+ reactor-bridge)
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

## Architecture & Code Enforcement

The structural integrity of the project is enforced mechanically. Refer to [Code & Architecture Enforcement](../technical/code-enforcement.md) for:
- Import boundaries (`depguard`)
- Pure-core invariants (no mutex, no I/O, no time)
- Lint rules and length limits
- Feature flag mechanisms
- Wire-format constraints

## Rules

- **Follow the design principles in [ARCHITECTURE.md](../../ARCHITECTURE.md).** The four [core principles](../../ARCHITECTURE.md#core-principles-all-layers) (testability, single-writer event loop, decisions-separated-from-I/O, no fabricated fallbacks) hold in every layer.
- **Actively use libraries.** Do not implement from scratch what an existing dependency covers.
- **Do not overwrite user config files** (`~/.agent-reactor/`). Setup commands must be idempotent.
- **Always write tests** for new features and bug fixes. Work is not complete without tests. Testability is the first core principle: refactor production code (interface extraction, env-var override, dependency injection) when needed to enable a test. Per-package coverage targets and the Tier scheme are in [testing](testing.md).

## Conventions

### Side-effect naming

Distinguish path computation from side effects by function name:

| Pattern | Side effect | Example |
|---|---|---|
| `XxxPath()` | None (pure) | `LogDirPath`, `ConfigDirPath`, `LogPath` |
| `EnsureXxx()` | Directory creation | `EnsureLogDir`, `EnsureConfigDir` |
| `LoadFrom(path)` | File read only | `config.LoadFrom` |
| `Load()` | Directory creation + file read | `config.Load` (convenience wrapper) |

## Library selection

Before adding a third-party dependency:

1. List 2–3 candidates with their trade-offs (size, maintenance, license, API fit).
2. Justify the chosen one against the alternatives in the PR description.
3. Prefer libraries already in `go.mod` when they cover the use case.
4. Wire-format and persistence types must remain **stdlib-only**.

## Conformance

The SPEC §17 ↔ test correspondence table and the documented deviation posture live in [technical/orchestrator/symphony-conformance.md](../technical/orchestrator/symphony-conformance.md). Keep it current when you touch orchestrator behavior.
