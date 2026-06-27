# orchestrator

`orchestrator` is the unattended, headless scheduler — the user-facing side of the [`orchestrator` layer](../technical/orchestrator/README.md). It reads a `WORKFLOW.md`, polls an issue tracker, dispatches a coding agent into a per-issue workspace, reconciles running/stalled sessions, and exposes a read-only observability HTTP server. It implements the [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md).

> Unlike `server`, which is interactively supervised through the browser UI, the orchestrator runs autonomously: agents advance issues by transitioning tracker states, and the scheduler detects progress by polling. There is no human in the loop during a run — see [WORKFLOW.md authoring](../agent/workflow-authoring.md) for how the driving prompt is written.

## Run

```bash
orchestrator -workflow ./WORKFLOW.md [-port 8080]
```

| Flag | Default | Meaning |
|---|---|---|
| `-workflow` | `./WORKFLOW.md` | Path to the workflow file |
| `-port` | `0` | HTTP observability port. `0` disables it **unless** `server.port` is set in `WORKFLOW.md`. An explicit `-port` on the command line overrides the file. |

On startup the orchestrator loads and validates the workflow, runs a **preflight** check on the resolved config, warms the per-project container, and then enters the poll loop. A fatal config error aborts before any agent is dispatched.

## WORKFLOW.md

`WORKFLOW.md` is a Markdown file with a YAML front-matter block (configuration) followed by a body (the prompt template handed to the agent). This page covers the **configuration**; the prompt body and issue state flow are covered in [WORKFLOW.md authoring](../agent/workflow-authoring.md).

```yaml
---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY      # $VAR is expanded from the environment
  project_slugs:                # one or more Linear project slugIds
    - c01cdba6fe92
  active_states:                # states the agent works on
    - Todo
    - In Progress
    - Merging
    - Rework
  terminal_states:              # completion states (agent stops)
    - Done
    - Failed
    - Canceled
    - Duplicate
polling:
  interval_ms: 30000            # poll cadence
workspace:
  root: /path/to/.agent-reactor/worktrees   # per-issue workspace root
hooks:
  timeout_ms: 120000
  after_create: |               # initialization run after a workspace is created
    git clone --depth 1 ...
    git checkout -b "symphony/$(basename "$PWD")"
agent:
  max_concurrent_agents: 2      # dispatch slots
  max_turns: 30                 # max agent turns per issue
codex:
  command: claude-app-server    # agent binary (see "Agent selection")
  turn_timeout_ms: 3600000
  read_timeout_ms: 60000
server:
  port: 8080                    # observability HTTP server
  bind: 127.0.0.1
---
# … prompt template body …
```

### State semantics

- **active_states** — the agent is dispatched to work on issues in these states.
- **terminal_states** — completion states; the agent stops and the issue is released.
- States that are **neither** active nor terminal (e.g. `Human Review`) are a **handoff**: the orchestrator parks the issue and waits for a human to move it back into an active state. This is how the autonomous loop hands control back to people.

### Per-project configuration

`project_slugs` may list several projects. Each Linear project can carry its own configuration in its **content** (the project overview), reusing the WORKFLOW.md grammar: an optional YAML front matter followed by an optional additional prompt body. Both are optional.

```
---
branch: develop
---
Extra instructions specific to this project.
```

- **branch** — the base branch for this project's work. It is exposed to the prompt template as `{{ project.branch }}` and to hooks as the `ROOST_PROJECT_BRANCH` environment variable. When `branch` (or the whole front matter) is absent, the value is empty and the hook/prompt should fall back to the repository default branch (e.g. `base=${ROOST_PROJECT_BRANCH:-$(git ... rev-parse --abbrev-ref HEAD)}`).
- The body after the front matter is exposed as `{{ project.prompt }}` (substituted verbatim — its own Liquid tags are not re-rendered). Place it wherever the prompt body should include the per-project instructions.
- `{{ project.name }}` is the Linear project name.

## Agent selection

The `codex.command` field chooses which agent binary the orchestrator drives. Both speak the same Codex app-server stdio protocol, so switching never requires orchestrator-side changes.

```yaml
---
codex:
  # Use the native Codex agent (default; requires Codex CLI)
  command: codex app-server

  # — OR — use the Claude shim (no Codex CLI required)
  command: claude-app-server

  # Optional policy hints forwarded to the agent.
  # claude-app-server logs these but does not enforce them; actual
  # isolation is provided by the devcontainer (see sandbox.md).
  approval_policy: localSandboxed
  thread_sandbox: projectDirectory
  turn_sandbox_policy: ""

  # Timeouts in milliseconds (0 = use defaults)
  turn_timeout_ms: 0
  read_timeout_ms: 0
  stall_timeout_ms: 0
---
```

Both agents emit the same `thread/started → turn/started → item/* → thread/tokenUsage/updated → turn/completed` event sequence.

## Observability HTTP

When enabled, the server exposes a read-only dashboard and a small REST API (Symphony SPEC §13.7):

| Method + path | Purpose |
|---|---|
| `GET /` | HTML dashboard (fetches the JSON API client-side) |
| `GET /api/v1/state` | Full scheduler state as JSON |
| `GET /api/v1/{issue_identifier}` | Per-issue detail |
| `POST /api/v1/refresh` | Trigger a manual tracker refresh |

Bind to `127.0.0.1` (the default) unless you intentionally expose it.

## See also

- [WORKFLOW.md authoring](../agent/workflow-authoring.md) — the prompt body, state-routing rules, and the `linear_graphql` tool
- [sandbox setup](sandbox.md) — isolating each dispatched agent in a container
- [orchestrator internals](../technical/orchestrator/README.md) — the poll/dispatch/reconcile pipeline
