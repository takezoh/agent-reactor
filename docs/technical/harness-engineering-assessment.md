# Harness Engineering Assessment

A point-in-time assessment (2026-05-31) of agent-roost evaluated as a piece of
**harness engineering** — judged not on the agent's internal reasoning, but on
how well the system *uses* the coding agents it drives.

> **Snapshot, not reference.** Unlike the rest of `docs/technical/`, this page is a
> dated evaluation, not an evergreen description of how the code works. Re-run it
> as the recommendations below land; the [scorecard](#scorecard) is the thing to
> update.

## Framing: where the harness lives

The term *harness* is overloaded. In the agent-harness-engineering literature the
harness is the scaffolding around an LLM that turns it into an agent — the loop,
tool calls, context management, memory, guardrails, and tracing
([O'Reilly](https://www.oreilly.com/radar/agent-harness-engineering/),
[awesome-harness-engineering](https://github.com/ai-boost/awesome-harness-engineering)).

For agent-roost that scaffolding is split across a boundary worth naming explicitly:

- **The inner harness is Claude Code / Codex.** They own the agent loop, their own
  tools, context compaction, and the model's reasoning. agent-roost does **not**
  reimplement any of this — and deliberately so.
- **The outer harness is agent-roost.** The orchestrator + `claude-app-server` shim
  + roost are the orchestration layer that *launches, configures, drives, and
  observes* those inner harnesses.

This split makes the assessment question precise: not "how good is the agent's
internal harness" (that is delegated), but **"how well does agent-roost engineer
its use of Claude/Codex?"** — across design, implementation, test, documentation,
and CI. The deliberate non-reimplementation of context management/memory inside
the orchestrator is, under this framing, a *correct* design choice rather than a
gap. The real gaps are all in the outer, orchestration-level concerns.

## Assessment lens

Each axis is judged against the consensus harness-engineering principles:

- **"Agent = Model + Harness."** A decent model with a great harness beats a great
  model with a bad one — so the orchestration layer is the high-leverage surface.
- **Silent Success, Verbose Failure.** Successes are quiet; failures inject
  detailed, actionable error context so the system (or a human) can self-correct.
- **The Ratchet Principle.** Every observed failure becomes a permanent,
  externalized rule — the harness *owns* an operating manual that encodes them.
- **Eval-driven iteration.** You cannot improve what you cannot measure; behavioral
  evals (does the agent actually complete the task?) gate changes
  ([Anthropic — writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents),
  [METR elicitation](https://evaluations.metr.org/elicitation-protocol/)).
- **Intent-level tools, not API passthrough.** Tools should match how a human
  subdivides the task, with actionable errors — not wrap every raw endpoint.
- **Harness logic as a portable natural-language artifact**
  ([Natural-Language Agent Harnesses](https://arxiv.org/abs/2603.25723)):
  contracts, roles, stages, state semantics, and a *failure taxonomy* that drives
  recovery, kept out of controller code.

---

## Axis 1 — Design — **A−**

**Strengths**

- **Agent-agnostic dispatch.** Both `codex app-server` and `claude-app-server` are
  normalized to one Codex app-server protocol (`platform/agent/codexclient`). The
  driven agent is swapped with a single `codex.command` line in `WORKFLOW.md` — a
  clean realization of "the model is replaceable, the harness is the product."
- **`WORKFLOW.md` ≈ a natural-language harness artifact.** Harness logic lives
  outside controller code as YAML front-matter (config) + a Markdown prompt
  template (behavior). `docs/agent/workflow-authoring.md` further externalizes the
  *invariants* learned from failure ("always leave the active state before ending a
  turn", "be idempotent", "fail loudly on a real blocker") — the Ratchet Principle
  in document form.
- **Credential isolation by design.** `linear_graphql` keeps the tracker token in
  the orchestrator; the agent never holds it. `hostexec`/`mcpproxy`/`credproxy`
  mediate host capability from inside the sandbox. See
  [guardrails.md](guardrails.md).
- **Functional Core / Imperative Shell + single authority.** `scheduler.Reduce` is
  pure; `ErrDuplicateDispatch` enforces single-writer dispatch (SPEC §7.4).

**Gaps**

1. **Tool altitude is low.** `linear_graphql` is a raw GraphQL passthrough — the
   anti-pattern Anthropic warns against (wrap intent, not endpoints). The
   credential-isolation and flexibility benefits are real, so the fix is to *add*
   intent-level tools (e.g. `transition_issue`, `comment`, `fetch_review`) and keep
   raw GraphQL as an escape hatch, not to remove it.
2. **The outer harness owns no operating manual.** Beyond the rendered prompt, the
   orchestrator injects no operating instructions / skills / `AGENTS.md` into the
   driven agent — skill access depends entirely on the agent's own config. There is
   no seam through which the harness can encode "what went wrong last time" as a
   durable rule.
3. **Thin continuation.** Turn continuation is the fixed string
   `continuationPrompt = "Continue working on the issue."` (`orchestrator/agent/runner.go`),
   and across retries only the `{{ attempt }}` counter is available to the template —
   there is no structured carry-over of *what was tried and why it failed* (the NLAH
   failure taxonomy). This is legitimately the outer harness's job and does **not**
   require reimplementing the agent's own memory.

---

## Axis 2 — Implementation — **B+**

**Strengths**

- **Clear multi-turn state machine.** `runLoop` / `awaitTurn` / `shouldContinue` /
  `nextTurn` (`orchestrator/agent/runner.go`) read as a small, legible loop;
  `awaitTurn` distinguishes a graceful orchestrator kill (→ `turn_cancelled`) from a
  stall/timeout/unexpected exit (→ `turn_failed`) via `Worker.WasKilledGracefully()`.
- **Correct token accounting.** `completeTurn` / `addUsage`
  (`cmd/claude-app-server/turn.go`) implements per-turn vs. cumulative semantics
  correctly (SPEC §13.5).
- **Resume continuity.** The shim stores the Claude `session_id` from `SystemInit`
  in `threads[threadID]` and passes it back via `--resume` on the next turn.
- **Liveness floor.** A Claude process that exits without a result line still emits
  `turn/failed` (`runTurnLoop`), so the orchestrator never waits out the full turn
  timeout on a crash.

**Gaps**

1. **Silent JSON decode violates fail-fast (highest-value fix).** `parseTurnStart`
   (`cmd/claude-app-server/turn.go`) does `_ = json.Unmarshal(params, &p)` — a
   malformed `turn/start` proceeds with zero-value `threadID`/`cwd` rather than
   failing loudly. This is the clearest violation of *Silent Success, Verbose
   Failure*.
2. **Tool-reply handling masks bad replies.** In `toolBridge.handleConn`
   (`cmd/claude-app-server/toolbridge.go`) the reply is decoded with
   `_ = json.Unmarshal(result, &r)`, falls back to `string(result)` on an empty
   output, and treats `Success == nil` as success (`r.Success == nil || *r.Success`).
   (The *request* decode and the `conn.Request` error path are handled correctly —
   only the reply path is lenient.)
3. **Tool errors are not client/server differentiated.** A `linear_graphql` failure
   surfaces as one undifferentiated error, so the agent cannot tell "malformed query
   (fix it)" from "transient API error (retry it)" and choose a strategy.
4. **Hook output is not captured.** `after_create` / `before_run` / `after_run` /
   `before_remove` stdout/stderr is discarded, leaving failed hooks hard to debug
   and unaudited.

---

## Axis 3 — Test — **B−**

**Strengths**

- **Reusable fake agent.** `fakeServer` (`orchestrator/agent/runner_test.go`) pipes
  real stdio via `io.Pipe` and covers launch → session-init → turn → exit, including
  timeout-kill and before-run-failure paths.
- **Loop-branch coverage.** `runner_loop_test.go` exercises two-turn continuation,
  `MaxTurns`, turn failure, graceful-kill (no `turn_failed`) vs. stall-kill (emits
  `turn_failed`).
- **Shim stream parsing.** `shim_test.go` covers system-init → message → result,
  result-less exit → `turn/failed`, `--resume` continuation, tool_use/tool_result,
  and cumulative token usage.
- **Tool round-trip.** `handler_test.go` / `toolbridge_test.go` cover
  `linear_graphql` success, GraphQL errors, unknown/disabled tools, and concurrent
  socket access.

**Gaps**

1. **No behavioral / outcome eval (biggest gap).** No fixtures, golden transcripts,
   recorded-session replays, or SWE-bench-style scenarios were found. The suite
   verifies *the loop runs correctly*, never *whether a driven agent actually
   resolves an issue*. Without this, harness changes cannot be scored and
   behavioral regressions cannot be caught — the exact iteration loop the
   literature treats as foundational.
2. **Malformed-input robustness untested.** The silent-decode paths in Axis 2 have
   no negative tests.
3. **No real-process integration.** Every test uses a fake stream; nothing exercises
   a real Claude/Codex launch or confirms `--resume` continuity end-to-end.
4. **Template error path untested.** `prompt.Render`'s `StrictVariables` failure
   mode is not exercised.

Coverage discipline itself is strong (tiered floors enforced by
`scripts/check-coverage.sh`; see [testing.md](../agent/testing.md)) — the gap is in
*kind* of test, not in coverage rigor.

---

## Axis 4 — Documentation — **A**

**Strengths**

- **Audience × layer map** (`docs/README.md`) gives clear navigation.
- **`workflow-authoring.md`** documents driving an agent *through a prompt* — the
  prompt invariants, state-routing table, and `linear_graphql` query catalog. It is,
  in effect, the operator's guide to the outer harness.
- **`symphony-conformance.md`** (SPEC §17 ↔ test table + deviation posture),
  **`testing.md`** (coverage tiers), and **ARCHITECTURE.md** (FC/IS + layer
  boundaries) are thorough.

**Gaps**

1. No chapter frames the system *as harness engineering* — the delegation boundary
   (what is Claude/Codex's job vs. the orchestrator's) is implicit rather than
   stated. (This page is a first step.)
2. No guide on **how to evaluate the driven agent** — the counterpart to the Axis 3
   eval gap.

---

## Axis 5 — CI — **A**

**Strengths**

- `ci.yml`: vet, test, **coverage-floor gate**, three-binary build, golangci-lint,
  plus a separate **`-race` job** guarding the single-writer loop.
- **Codex schema-drift job.** CI installs a pinned codex and diffs
  `generate-json-schema` against the committed bundles in
  `platform/agent/codexschema` — a machine-checked protocol contract with the inner
  harness.
- **Dogfooding the harness in the dev loop.** `auto-fix-ci.yml` has Claude Code fix a
  failing main build; `simplify.yml` runs `/simplify` on every PR. Using the agent as
  a harness *for its own development* is itself a harness-engineering practice.
- Layer/`no-mutex`/pure-core invariants enforced via depguard + forbidigo + ruleguard
  (see [code-enforcement.md](code-enforcement.md)).

**Gaps**

1. **No eval gate** — the CI counterpart to the Axis 3 behavioral-eval gap; nothing
   stops a behavioral regression.
2. **Schema guard is one-directional.** It validates codex → orchestrator, but the
   shim's *emitted* protocol (`claude-app-server` → orchestrator) is never validated
   against the schema.
3. **Pinned-version drift.** `ci.yml` installs codex `0.133.0` while the `Makefile`
   comments (and the codexschema README) still say `0.128.0` — a stale reference to
   reconcile.

---

## Scorecard

| Axis | Grade | One-line |
|---|---|---|
| Design | **A−** | Correct delegation + agent-agnostic; tool altitude and a harness-owned operating manual are the headroom |
| Implementation | **B+** | Loop / accounting / resume are solid; silent JSON decode is the main fail-fast debt |
| Test | **B−** | Protocol/loop coverage is thick; **behavioral eval is absent** |
| Documentation | **A** | Structure and operator guidance are excellent; a harness lens and an eval guide are missing |
| CI | **A** | Coverage / race / schema-drift / dogfooding are mature; no eval gate, one-directional schema guard |

**Bottom line:** a production-grade harness *for protocol conformance and loop
correctness*. The one structural gap — running through design, test, and CI — is the
absence of any layer that measures and protects **whether the driven agent actually
completes its tasks.**

## Recommendations (by leverage)

| # | Action | Axes | Effort / payoff |
|---|---|---|---|
| **P1** | **Behavioral eval harness** — golden-transcript replay + state-routing scenario fixtures against the fake agent, optionally a small real-agent smoke; add an eval gate to CI | Test / CI / Design | Highest payoff; medium effort. Establishes eval-driven iteration |
| **P2** | **Fail-fast + verbose failure** — promote the silent `json.Unmarshal` paths (`parseTurnStart`, tool reply) to surfaced errors; differentiate client vs. server tool errors | Implementation | Low effort, high payoff (quick win) |
| **P3** | **Tool-design pass** — add intent-level tools (`transition_issue`/`comment`/`fetch_review`) with actionable errors; keep `linear_graphql` as an escape hatch | Design / Implementation | Medium / medium |
| **P4** | **Observability** — capture per-attempt transcripts and hook stdout/stderr; aggregate retry/failure counts; correlate via session/thread/turn IDs | Implementation / CI | Medium; large debuggability gain |
| **P5** | **Structured retry context** — inject a "what was tried / why it failed" summary of the prior attempt into the template (outer-harness scope; not agent memory) | Design | Medium / medium |
| **P6** | **Two-directional schema guard** — validate the shim's emitted protocol against the codex schema in a test/CI step; reconcile the pinned-version drift | Test / CI | Low / medium |
| **P7** | **Harness-engineering docs** — a delegation-boundary chapter and a "how to eval the driven agent" guide | Documentation | Low effort |

## References

- [O'Reilly — Agent Harness Engineering](https://www.oreilly.com/radar/agent-harness-engineering/)
- [awesome-harness-engineering](https://github.com/ai-boost/awesome-harness-engineering)
- [SWE-agent — Agent-Computer Interfaces (ACI)](https://arxiv.org/abs/2405.15793) ·
  [aci.md](https://github.com/SWE-agent/SWE-agent/blob/main/docs/background/aci.md)
- [Anthropic — Writing effective tools for AI agents](https://www.anthropic.com/engineering/writing-tools-for-agents)
- [Anthropic — Context engineering for agents](https://howaiworks.ai/blog/anthropic-context-engineering-for-agents)
- [Natural-Language Agent Harnesses](https://arxiv.org/abs/2603.25723)
- [METR — Guidelines for capability elicitation](https://evaluations.metr.org/elicitation-protocol/)

## See also

- [guardrails.md](guardrails.md) — the existing control surface (admission, concurrency, sandboxing, autonomy policy, liveness)
- [orchestrator/symphony-conformance.md](orchestrator/symphony-conformance.md) — SPEC ↔ test table and deviation posture
- [../agent/workflow-authoring.md](../agent/workflow-authoring.md) — programming the agent through the prompt
- [../agent/testing.md](../agent/testing.md) — testability as a design constraint and coverage tiers
