## Build & Test

```sh
make build                   # Build Go sources under src/ → ./roost (+ roost-bridge, sockbridge)
make build-orchestrator      # Build → ./orchestrator
make build-claude-app-server # Build → ./claude-app-server
make build-all               # Build all 3 main binaries (requires go.work for sockbridge)
make vet                     # go vet ./...
make lint                    # golangci-lint (depguard, funlen, staticcheck, etc.)
cd src && go test ./...          # Run all tests
cd src && go test ./path/to/pkg  # Run tests for a specific package
cd src && go test -run TestName ./...  # Run a specific test
```

## Rules

- Follow the design principles in ARCHITECTURE.md
- Keep files under 500 lines and functions under 80 lines. State-machine reducers in `state/reduce_*.go` are exempt from the function-length limit — dispatch tables stay cohesive (see ARCHITECTURE.md "Reducer files")
- Actively use libraries. Do not implement from scratch
- Do not overwrite user config files (~/.roost/)
- Always write tests for new features and bug fixes. Do not consider work complete without tests
- Testability is a primary design constraint. Refactor production code (interface extraction, env-var override, dependency injection) when it's needed to enable a test. Per-package coverage targets and the Tier scheme are in `docs/testing.md`

## Library Selection

Before adding a third-party dependency:
1. List 2-3 candidates with their trade-offs (size, maintenance, license, API fit)
2. Justify the chosen one against the alternatives in the PR description
3. Prefer libraries already in `go.mod` when they cover the use case
4. Wire-format and persistence types must remain stdlib-only
