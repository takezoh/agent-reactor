# platform/ — Shared Infrastructure

`platform/` is the base layer. Both the `server` backend (`client/`) and the `orchestrator/` depend on it; it depends on **neither** of them (enforced by the `depguard` rule `platform-no-client-or-orchestrator`). Wire-format and persistence types here stay stdlib-only.

Because it sits below both services, `platform/` is where tool-specific knowledge (paths, env var names, CLI invocations) is allowed to live — keeping it out of the generic layers above.

## Design principles (platform realization)

`platform/` is a **library layer, not a decision loop**, so the Functional Core / Imperative Shell form of the [core principles](../../../ARCHITECTURE.md#core-principles-all-layers) does not apply here. It still serves the same overriding goal — **testability** — but through **dependency-injection seams** rather than a pure reducer:

- **Testability via DI seams** — code that wraps an external dependency (`exec`, docker, the network, a filesystem path) puts that dependency behind an injectable interface or an env-var override, so tests substitute a fake. Examples: subprocess wrappers expose a `Runner` interface (`lib/github.Runner`) with a `DefaultRunner` for production and a fake for tests; external config paths accept overrides (`CODEX_CONFIG_DIR`). "We can't test it without the real binary" is a design defect — cover the parsing/command-assembly logic behind the seam.
- **Base layer, no upward imports** — `platform/` imports neither `client/` nor `orchestrator/` (enforced by `depguard` rule `platform-no-client-or-orchestrator`). It knows nothing about the services above it.
- **Tool-specific knowledge concentrated here** — paths, env-var names, and CLI invocations live in `lib/<tool>/` and `credproxy/` so the generic layers above stay tool-agnostic. This is the receiving side of client's Driver isolation.
- **Agent-agnostic launch primitive** — `agentlaunch` (`Spawn`/`SplitArgs`/`Dispatcher`) turns a command string into a running process without knowing which agent it launches; per-agent argv construction stays in `lib/<tool>`.
- **Wire-format / persistence is stdlib-only** — types that cross the wire or hit disk depend only on the standard library, for portability.

## Packages

| Package | Responsibility |
|---|---|
| `platform/sandbox/` | Project-level sandbox backends (generic `Manager[I]`). `devcontainer/` implements per-project container lifecycle via docker. See [sandbox.md](sandbox.md). |
| `platform/hostexec/` | Host-exec broker — a `container.Provider` that runs allowlisted host binaries on behalf of container processes via SCM_RIGHTS stdio forwarding. |
| `platform/mcpproxy/` | MCP proxy broker — runs MCP servers on the host with JSON-RPC stdio relayed into the container, with tool-level policy enforcement. Generates a `.mcp.json` overlay so Claude Code routes configured aliases through the broker. |
| `platform/pathmap/` | Container↔host path translation using `WrappedLaunch.Mounts`. |
| `platform/logger/` | `slog` initialization + log file management. |
| `platform/lib/` | External tool integration — `git`, `github`, `gemini`, `wsl`, `openurl`, `notify`, …  Per-agent argv builders: `lib/codex/argv.go` (`AppServerListenArgs`, `RemoteAttachArgs`, `ParseCommand([]string)`, `ShellJoinArgv`, driver constants) and `lib/claude/cli/argv.go` (`SandboxFlags`, `AppServerArgs`). |
| `platform/tracker/` | Issue tracker adapters (e.g. `linear/`). Shared by the orchestrator. |
| `platform/metrics/` | Token / runtime metrics accumulation. |
| `platform/agentlaunch/` | Agent launch primitives: `LaunchPlan`/`WrappedLaunch` (dual `Command` string for pty pane + `Argv []string` for `Spawn`), `Dispatcher.Wrap` interface, `Spawn` (argv-direct exec via `procgroup`, no host shell), `SplitArgs` POSIX tokenizer. Shared by client and orchestrator. |
| `platform/procgroup/` | Process-group spawn wrapper (`procgroup.Command`) with `WaitDelay`-bounded SIGKILL. `Tracker` records pgids so a future boot's `PruneOrphans` can reap orphaned processes. |
| `platform/agent/` | Shared agent launch interfaces (e.g. `codexclient/`). |
| `platform/credproxy/` | Credential providers (AWS SSO, gcloud CLI, ssh-agent) — the external `credproxy` library; the only place tool-specific credential env var names live. |

## Per-subsystem deep dives

- **[spawn-and-launch.md](spawn-and-launch.md)** — `agentlaunch` + `procgroup` + `pathmap`. The layer that turns a command string into a running process: `LaunchPlan`/`WrappedLaunch`/`Dispatcher`/`Spawn`, process-group lifecycle and orphan reaping, container↔host path translation.
- **[brokers.md](brokers.md)** — implementation of `hostexec` + `mcpproxy` + `credproxy`: SCM_RIGHTS proxied execution, JSON-RPC tool gating, per-project tokens. The security model is in [sandbox.md](sandbox.md).
- **[agent-protocol.md](agent-protocol.md)** — `agent/codexclient` + `codexschema` (v1/v2) + `lib/codex` + `lib/claude`. The Codex app-server stdio protocol, the turn sequence, and the claude-app-server shim's translation.
- **[sandbox.md](sandbox.md)** — `sandbox/` backends: per-project devcontainer isolation, image resolution, credential proxy.
- Agent-control guardrails (capability sandboxing, autonomy policy, concurrency, liveness) are in [guardrails.md](../guardrails.md); code-level enforcement (import boundaries, length limits) is in [code-enforcement.md](../code-enforcement.md).

## Sandbox isolation and credential brokering

The sandbox, host-exec, and MCP-proxy packages together provide the security model: long-lived secrets stay on the host, and the container only ever sees short-lived tokens or brokered stdio. The architecture and lifecycle are documented in [sandbox.md](sandbox.md); user-facing configuration is in the [sandbox setup guide](../../user/sandbox.md).
