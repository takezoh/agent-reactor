# Agent Guide

Documentation for **agents** — AI agents and human contributors doing work in this repository. If you are changing code, read [contributing](contributing.md). If you are authoring the workflow that drives the autonomous orchestrator, read [WORKFLOW.md authoring](workflow-authoring.md).

The repo's canonical build/test/rules summary lives in [AGENTS.md](../../AGENTS.md) at the root (it is loaded automatically by Claude/Gemini/Codex via `@AGENTS.md`). These pages expand on it.

## Pages

- [Contributing](contributing.md) — build/test/vet/lint commands, coding rules (file/function limits, the reducer exemption, mandatory tests), and the library-selection process
- [WORKFLOW.md authoring](workflow-authoring.md) — how the orchestrator's driving prompt is structured: the issue state flow, idempotency invariants, and the `linear_graphql` tool
- [Testing](testing.md) — testability as a design constraint and the Tier-based coverage targets

## Architecture context

All three layers and their import boundaries are defined in [ARCHITECTURE.md](../../ARCHITECTURE.md). Before adding code, know which layer you are in:

- `platform/` — shared infrastructure; must not import `client/` or `orchestrator/`
- `client/` — roost; must not import `orchestrator/`
- `orchestrator/` — Symphony pipeline; must not import `client/`

Layer internals: [platform/](../technical/platform/README.md) · [client/](../technical/client/README.md) · [orchestrator/](../technical/orchestrator/README.md).
