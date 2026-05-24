# platform/ — Shared Infrastructure

`platform/` is the base layer. Both `roost` (`client/`) and the `orchestrator/` depend on it; it depends on **neither** of them (enforced by the `depguard` rule `platform-no-client-or-orchestrator`). Wire-format and persistence types here stay stdlib-only.

Because it sits below both services, `platform/` is where tool-specific knowledge (paths, env var names, CLI invocations) is allowed to live — keeping it out of the generic layers above.

## Packages

| Package | Responsibility |
|---|---|
| `platform/sandbox/` | Project-level sandbox backends (generic `Manager[I]`). `devcontainer/` implements per-project container lifecycle via docker. See [sandbox.md](sandbox.md). |
| `platform/hostexec/` | Host-exec broker — a `container.Provider` that runs allowlisted host binaries on behalf of container processes via SCM_RIGHTS stdio forwarding. |
| `platform/mcpproxy/` | MCP proxy broker — runs MCP servers on the host with JSON-RPC stdio relayed into the container, with tool-level policy enforcement. Generates a `.mcp.json` overlay so Claude Code routes configured aliases through the broker. |
| `platform/pathmap/` | Container↔host path translation using `WrappedLaunch.Mounts`. |
| `platform/logger/` | `slog` initialization + log file management. |
| `platform/features/` | Feature flags — `Flag`/`Set` types (runtime) and build-tag `const` (compile-time). No external deps, so the pure `state/` core can import it. |
| `platform/lib/` | External tool integration — `git`, `github`, `gemini`, `wsl`, `openurl`, `notify`, …  Per-agent argv builders: `lib/codex/argv.go` (`AppServerListenArgs`, `RemoteAttachArgs`, `ParseCommand([]string)`, `ShellJoinArgv`, driver constants) and `lib/claude/cli/argv.go` (`SandboxFlags`, `AppServerArgs`). |
| `platform/tracker/` | Issue tracker adapters (e.g. `linear/`). Shared by the orchestrator. |
| `platform/metrics/` | Token / runtime metrics accumulation. |
| `platform/agentlaunch/` | Agent launch primitives: `LaunchPlan`/`WrappedLaunch` (dual `Command` string for tmux pane + `Argv []string` for `Spawn`), `Dispatcher.Wrap` interface, `Spawn` (argv-direct exec via `procgroup`, no host shell), `SplitArgs` POSIX tokenizer. Shared by client and orchestrator. |
| `platform/procgroup/` | Process-group spawn wrapper (`procgroup.Command`) with `WaitDelay`-bounded SIGKILL. `Tracker` records pgids so a future boot's `PruneOrphans` can reap orphaned processes. |
| `platform/agent/` | Shared agent launch interfaces (e.g. `codexclient/`). |
| `platform/credproxy/` | Credential providers (AWS SSO, gcloud CLI, ssh-agent) — the external `credproxy` library; the only place tool-specific credential env var names live. |

## Per-subsystem deep dives

- **[spawn-and-launch.md](spawn-and-launch.md)** — `agentlaunch` + `procgroup` + `pathmap`. The layer that turns a command string into a running process: `LaunchPlan`/`WrappedLaunch`/`Dispatcher`/`Spawn`, process-group lifecycle and orphan reaping, container↔host path translation.
- **[brokers.md](brokers.md)** — implementation of `hostexec` + `mcpproxy` + `credproxy`: SCM_RIGHTS proxied execution, JSON-RPC tool gating, per-project tokens. The security model is in [sandbox.md](sandbox.md).
- **[agent-protocol.md](agent-protocol.md)** — `agent/codexclient` + `codexschema` (v1/v2) + `lib/codex` + `lib/claude`. The Codex app-server stdio protocol, the turn sequence, and the claude-app-server shim's translation.
- **[sandbox.md](sandbox.md)** — `sandbox/` backends: per-project devcontainer isolation, image resolution, credential proxy.
- Cross-cutting enforcement (import boundaries, length limits, runtime gates, feature flags) is in [guardrails.md](../guardrails.md).

## Feature flags

The `features/` mechanism lives here because it must be importable by the pure `state/` core without pulling in third-party packages. There are two independent flag mechanisms (runtime vs compile-time) — see [ARCHITECTURE.md → Feature Flags](../../../ARCHITECTURE.md) for how to add each.

## Sandbox isolation and credential brokering

The sandbox, host-exec, and MCP-proxy packages together provide the security model: long-lived secrets stay on the host, and the container only ever sees short-lived tokens or brokered stdio. The architecture and lifecycle are documented in [sandbox.md](sandbox.md); user-facing configuration is in the [sandbox setup guide](../../user/sandbox.md).
