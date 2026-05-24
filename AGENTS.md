## Build & Test

```sh
make build                   # Build Go sources under src/ → ./roost (+ roost-bridge)
make build-orchestrator      # Build → ./orchestrator
make build-claude-app-server # Build → ./claude-app-server
make build-all               # Build all 3 main binaries
make vet                     # go vet ./...
make lint                    # golangci-lint (depguard, funlen, staticcheck, etc.)
cd src && go test ./...          # Run all tests
cd src && go test ./path/to/pkg  # Run tests for a specific package
cd src && go test -run TestName ./...  # Run a specific test
```

## Three-layer architecture

One Go module, three top-level trees under `src/` and three binaries:

| Binary | Source | Layer | Role |
|---|---|---|---|
| `roost` | `src/cmd/roost/` | `client/` | TUI session lifecycle manager (original roost) |
| `orchestrator` | `src/cmd/orchestrator/` | `orchestrator/` | Symphony SPEC implementation — autonomous poll/dispatch/reconcile + observability HTTP |
| `claude-app-server` | `src/cmd/claude-app-server/` | `platform/` + `orchestrator/` | Codex app-server stdio shim for Claude; enables agent-switch via `codex.command` in WORKFLOW.md |

`platform/` is shared infrastructure; `client/` is roost; `orchestrator/` is the Symphony pipeline. Import direction (enforced by `depguard`, `src/.golangci.yml`): `platform/*` imports neither `client/*` nor `orchestrator/*`; `client/*` does not import `orchestrator/*`; `orchestrator/*` does not import `client/*`. Full layer definition: [ARCHITECTURE.md](ARCHITECTURE.md).

Orchestrator-scoped test run: `cd src && go test ./orchestrator/... ./platform/tracker/... ./cmd/orchestrator/... ./cmd/claude-app-server/...`
Conformance: `docs/technical/orchestrator/symphony-conformance.md` — SPEC §17 ↔ test 対応表と逸脱 posture の正本。

## Rules

- Follow the design principles in [ARCHITECTURE.md](ARCHITECTURE.md)
- Keep files under 500 lines and functions under 80 lines. State-machine reducers in `client/state/reduce_*.go` are exempt from the function-length limit — dispatch tables stay cohesive (see ARCHITECTURE.md "Layer Structure")
- Actively use libraries. Do not implement from scratch
- Do not overwrite user config files (~/.roost/)
- Always write tests for new features and bug fixes. Do not consider work complete without tests
- Testability is a primary design constraint. Refactor production code (interface extraction, env-var override, dependency injection) when it's needed to enable a test. Per-package coverage targets and the Tier scheme are in `docs/agent/testing.md`

## Library Selection

Before adding a third-party dependency:
1. List 2-3 candidates with their trade-offs (size, maintenance, license, API fit)
2. Justify the chosen one against the alternatives in the PR description
3. Prefer libraries already in `go.mod` when they cover the use case
4. Wire-format and persistence types must remain stdlib-only

## Documentation

Structured by audience × layer under [`docs/`](docs/README.md): [contributing](docs/agent/contributing.md) (this file expanded), [WORKFLOW.md authoring](docs/agent/workflow-authoring.md), [testing](docs/agent/testing.md), and per-layer internals ([platform](docs/technical/platform/README.md) · [client](docs/technical/client/README.md) · [orchestrator](docs/technical/orchestrator/README.md)).
