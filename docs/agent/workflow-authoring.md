# WORKFLOW.md Authoring

`WORKFLOW.md` has two parts: a YAML front-matter block (configuration) and a Markdown body (the **prompt template** handed to each dispatched agent). This page covers the body — how to write the prompt that drives the autonomous agent. The configuration fields are documented in the [orchestrator user guide](../user/orchestrator.md).

This is the `orchestrator` layer seen from the perspective of someone *operating* the autonomous loop: you are programming agent behavior through a prompt, not through code.

## The prompt template

The body is rendered per dispatch with a Liquid-compatible template renderer ([SPEC §12](../technical/orchestrator/symphony-conformance.md)). Each dispatch re-reads the latest body, so edits take effect on the next dispatch without a restart.

Available variables include:

| Variable | Meaning |
|---|---|
| `{{ issue.identifier }}` | Human identifier (e.g. `DEV-183`) |
| `{{ issue.id }}` | Linear internal ID (used as `$id` in `linear_graphql`) |
| `{{ issue.title }}`, `{{ issue.description }}` | Issue content |
| `{{ issue.state }}` | Current tracker state — route on this |
| `{{ issue.priority }}`, `{{ issue.url }}` | Metadata |
| `{{ attempt }}` | Retry attempt count — the same issue can be re-dispatched |
| `{{ project.name }}` | Linear project name of the issue |
| `{{ project.branch }}` | Base branch from the project's content front matter (empty = repository default) |
| `{{ project.prompt }}` | Additional prompt body from the project's content (substituted verbatim) |

`{{ project.* }}` is derived from the issue's Linear project content (front matter + body); see [orchestrator configuration](../user/orchestrator.md#per-project-configuration). `branch` is also passed to hooks as `ROOST_PROJECT_BRANCH`.

## Invariants the prompt must enforce

These are the rules that keep the autonomous loop from spinning. A well-authored prompt states them explicitly to the agent:

1. **Always transition out of an active state before ending a turn.** If a turn ends with the issue still in an active state (`Todo` / `In Progress` / `Merging` / `Rework`), the orchestrator re-dispatches it forever. Every terminating turn must move the issue to a non-active state (a handoff state like `Human Review`, or a terminal state like `Done` / `Failed`).
2. **Be idempotent.** A worker can be re-dispatched mid-flight (timeout, `max_turns`, abnormal exit). The clone may be recreated and local commits lost, but pushed branches and PRs survive on GitHub. Each flow must first check *how far it already got* and resume — never redo from scratch or create a duplicate PR.
3. **On a real blocker, fail loudly.** If the agent hits something it cannot resolve (missing auth/permission/secret), it records the cause via a comment and transitions to `Failed`. Leaving it active causes a re-dispatch loop.

## State routing

The prompt inspects `{{ issue.state }}` and runs the matching flow. The reference `WORKFLOW.md` routes like this:

| State | Flow |
|---|---|
| `Backlog` | Out of scope — change nothing and stop (wait for a human to move it to `Todo`). |
| `Todo` | Transition to `In Progress`, then start work. |
| `In Progress` | Resume-check existing branch/PR → implement + test → commit → push → create PR (only if none exists) → comment the PR URL → transition to `Human Review`. |
| `Rework` | Read review comments (tracker + PR) → fix → push the same branch → comment → transition to `Human Review`. |
| `Merging` | Check PR state → merge if not already merged → transition to `Done`. |

The non-active `Human Review` state is the handoff point: the orchestrator parks the issue there and waits for a human, then re-dispatches when a human moves it to `Rework` or `Merging`.

## The `linear_graphql` tool

The orchestrator provides a `linear_graphql` tool so the agent can read and advance tracker state without holding any credentials (the orchestrator keeps the token; the agent never sees it). The prompt should document the queries the agent may use:

```graphql
# Resolve the id of a target state
query States($id: String!) { issue(id: $id) { team { states { nodes { id name type } } } } }
# Transition state
mutation Move($id: String!, $stateId: String!) { issueUpdate(id: $id, input: { stateId: $stateId }) { success } }
# Progress / review comment
mutation Note($id: String!, $body: String!) { commentCreate(input: { issueId: $id, body: $body }) { success } }
# Fetch review feedback (Rework)
query Comments($id: String!) { issue(id: $id) { comments { nodes { body createdAt user { name } } } } }
```

`$id` is `{{ issue.id }}`; `$stateId` is the id of the destination state (resolve it with the `States` query first). Unknown mutations or input types can be discovered via introspection (`__type`).

> Note on the tool's wire path: `linear_graphql` is advertised via the `thread/start` `dynamicTools` entry and invoked over `item/tool/call`. It is advertised only when the Linear client (tracker `api_key` + `endpoint`) is configured. See [symphony-conformance.md](../technical/orchestrator/symphony-conformance.md) and the orchestrator internals for details.

## See also

- [orchestrator user guide](../user/orchestrator.md) — running the pipeline and the front-matter config
- [orchestrator internals](../technical/orchestrator/README.md) — how dispatch and reconcile turn the prompt into a running agent
