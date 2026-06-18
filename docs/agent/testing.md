# Testing

## Design Principle

Testability is a primary design constraint, not an afterthought. When a function reaches for `os/exec`, the filesystem, a socket, or any other external dependency, the path that hits the dependency lives behind an interface or env-var override so tests can substitute a fake. Refactoring production code to enable a test is in scope; "we can't test it" is a design defect, not a justification.

Concrete patterns in use:

- **Subprocess wrappers** expose a `Runner` interface (e.g. `lib/github.Runner`) with a `DefaultRunner` for production and a fake for tests.
- **External config paths** accept an env-var override (`GEMINI_SETTINGS_PATH`, `CODEX_CONFIG_DIR`).
- **Runtime-injected dependencies** are interfaces, not concrete types (e.g. `runtime/subsystem/stream.RuntimeHook`).
- **`net.Pipe` + fake server** stands in for Unix sockets when verifying the proto client.

## Test patterns by layer

Both decision-loop layers (`client/` and `orchestrator/scheduler`) share the Functional Core / Imperative Shell test style: the pure `Reduce` is verified by its return value with no mocks, and the shell is exercised by injecting fakes for its dependencies. `platform/`, a library layer, injects fakes through interface seams. Test files live beside the target as `*_test.go`.

- **`state.Reduce` / `scheduler.Reduce` tests** — no mocks. Pure function tests that verify the return value `(state', []Effect)` of `Reduce(state, event, …)`. No goroutine / channel / timing dependencies; time enters as a value.
- **`Driver.Step` tests** — no mocks. Directly verify the return value `(next, effects, view)` of `Step(prev, driverEvent)`.
- **shell tests** (`client/runtime`, `orchestrator/scheduler` loop) — inject fakes for backend interfaces (`runtime.Config` `noopTmux`/`noopPersist`; scheduler `Deps{ Tracker, Spawn, Clock, … }` with a fake clock). Drive events through the loop and assert the published state.
- **TUI tests** — pass messages directly to Bubbletea's `Model.Update` and verify the returned model. No real terminal required.

## Multiplexed-subsystem routing harness

The stream subsystem multiplexes many frames over one codex app-server
connection; its safety-critical property is **routing isolation** (an event
reaches only the frame that owns its thread). The demux binds each thread
synchronously at creation/resume, so same-cwd frames get distinct ids and cannot
cross-talk by construction. It is pinned by a dedicated harness — direct-drive
contract, a wired fake app-server exercised under `-race`, a stdlib
`FuzzStreamRouting`, and an opt-in real app-server fidelity backstop
([setup](../technical/client/stream-backend-e2e.md)). Full guide:
[stream backend testing](../technical/client/stream-backend-testing.md). This is
the test-pinned enforcement catalogued in
[code-enforcement.md §6](../technical/code-enforcement.md).

## Coverage Tiers

Coverage targets are tiered by architectural blast radius. A regression in `state` corrupts every session; a regression in `lib/tmux` typically surfaces as one broken pane.

| Tier | Target | Layer | Members |
|------|--------|-------|---------|
| **S** | ≥85% | Pure domain layer & wire types | `state`, `state/view`, `proto`, `features`, `orchestrator/scheduler` (pure `Reduce` + transitions) |
| **A** | ≥75% | Core execution layer | `runtime`, `runtime/worker`, `runtime/subsystem/*`, `driver`, `driver/vt`, `connector`, `config`, `sandbox/devcontainer` |
| **B** | ≥60% | Infrastructure integrations | `lib/*` (except thin CLI wrappers), `proto/sessions`, `hostexec`, `mcpproxy`, `tui`, `tools` |
| **C** | ≥40% | Thin CLI & wiring | `main`, `cli`, `lib/tmux`, `lib/gemini`, `lib/notify` |
| **D** | smoke tests minimum | Trivial packages | `event`, `internal/globutil`, `lib/wsl`, `runtime/subsystem` (shared utilities), `sandbox`, `cmd/bridge` |

Tier S and A packages must not lose coverage in a PR. Tier B packages should improve over time; new B-tier code arrives with tests. Tier C packages aim for the goldenpath; full coverage isn't expected. Tier D packages need at least one test that exercises the package surface.

## Running Coverage

```sh
cd src && TMPDIR=/tmp go test -short -cover ./...
```

`TMPDIR=/tmp` is required because the sandbox blocks Unix socket creation under the default `TMPDIR`. Packages that exercise sockets (`proto`, `proto/sessions`, `mcpproxy`, etc.) will fail without it.

Per-package detail:

```sh
cd src && TMPDIR=/tmp go test -coverprofile=/tmp/c.out ./path/to/pkg
go tool cover -func=/tmp/c.out
```

## Enforcement

CI runs `scripts/check-coverage.sh` (the `coverage` step in `.github/workflows/ci.yml`), which executes the full test suite with coverage and compares each package against the floor declared in `scripts/coverage-floors.txt`. Any package below its floor — or any covered package missing from that file — fails the build.

Floors sit a few points below current measurement so legitimate variance does not break the build; the *target* in the Tier table above is the aspiration. When coverage gains stick, raise the floor in the same PR — never lower one without a written justification.

The `Simplify` workflow (`.github/workflows/simplify.yml`) runs on every pull request and applies the `/simplify` skill (parallel reuse / quality / efficiency review agents) to the diff, fixing defects, leaky abstractions, narration-only comments, no-assert tests, and concrete duplication. Treat its results like any other reviewer.

## When Coverage Can't Be Reached

Some packages can't hit their Tier target in CI because the dependency is the OS itself — `lib/tmux` wraps the tmux binary, `cmd/bridge` is a process entry point. For these:

1. Cover everything that doesn't require the external process (pure parsing, command-string assembly, etc.).
2. Document the structural ceiling in the package's test file.
3. Don't lower the Tier target — the gap is a real risk, just not one a unit test can close. Integration tests, not coverage adjustments, are the answer.
