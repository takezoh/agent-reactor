# Symphony SPEC Gap Analysis

[Symphony SPEC.md](https://github.com/openai/symphony/blob/main/SPEC.md) v1 Draft と既存資産 (agent-roost) との gap を §単位で整理する。

## SPEC の前提

実装方針を決める前提条件:

| SPEC §  | 重要な前提 |
|---|---|
| §1, §2.2, §11.5 | **Symphony はスケジューラ / ランナーであって workflow engine ではない**。チケット状態遷移 (Planning → Done 等) は agent prompt + agent tool 経由で行う |
| §10 | **Codex app-server stdio protocol が agent runner の唯一の規定**。Claude 等は SPEC 外だが、`codex.command` 経由で交換可能 |
| §14.3 | **永続 DB 不要**。restart は tracker re-poll + workspace 残存で復旧 |
| §5 | `WORKFLOW.md` が repo-owned な policy carrier (YAML front matter + prompt body) |
| §7.1, §16.5 | **continuation turn** — 同一 thread で `max_turns` まで turn を回し、turn 完了ごとに tracker 状態を再確認 |
| §9.3 | workspace 作成は VCS 非前提。git は実装の hook 内で行う扱い |

## §単位の gap 表

| SPEC §  | 項目 | roost | Gap 充足策 |
|---|---|---|---|
| §3.1.1 | Workflow Loader (`WORKFLOW.md` parser) | 無 | **新規** |
| §3.1.2 | Config Layer (typed getter / `$VAR` / preflight) | TOML + DataDir | **新規** |
| §3.1.3 | Issue Tracker Client (Linear GraphQL) | 部分的 (connector に類似) | **`platform/tracker/linear/` に新規実装** |
| §3.1.4 | Orchestrator (poll/dispatch/reconcile/retry の単一 authority) | 無 | **新規** |
| §3.1.5 | Workspace Manager (sanitize / 再利用 / hooks) | 部分 (worktree 管理) | **新規** (VCS 非前提化 + hooks) |
| §3.1.6 | Agent Runner (Codex app-server stdio) | **強い** (`runtime/subsystem/stream`) | **`platform/agent/codexclient/` に抽出** |
| §3.1.7 | Status Surface (OPTIONAL) | TUI 完備 | orchestrator では **HTTP server** で実装 (D6) |
| §3.1.8 | Logging (structured, key=value) | slog 完備 | **`platform/logger/` に key=value helper 追加** |
| §5.3 | front matter schema (tracker/polling/workspace/hooks/agent/codex) | 該当無し | **新規** (orchestrator/workflowfile/) |
| §5.4 | strict prompt template (Liquid 互換) | 無 | **新規** (orchestrator/prompt/) |
| §6.2 | WORKFLOW.md dynamic reload (fsnotify) | fsnotify 利用済 | **新規** (パターンは roost 流用) |
| §6.3 | preflight validation (startup + 毎 tick) | 無 | **新規** |
| §7.1 | orch state machine (Unclaimed/Claimed/Running/RetryQueued/Released) | 無 | **新規** (orchestrator/scheduler/) |
| §7.1, §16.5 | continuation turn (max_turns ループ) | 土台あり (stream subsystem) | **新規** |
| §8.1 | poll loop (reconcile → validate → fetch → dispatch) | 無 | **新規** |
| §8.2 | dispatch eligibility (active state, blocker(Todo) rule) | 無 | **新規** |
| §8.3 | global + per-state concurrency 制限 | 無 | **新規** |
| §8.4 | retry/backoff (`min(10000*2^(n-1), max)` + 連続 retry 1s) | 無 | **新規** |
| §8.5 | reconciliation (stall detection + tracker state refresh) | 無 | **新規** |
| §8.6 | startup terminal cleanup | 無 | **新規** |
| §9.1-9.3 | workspace (sanitized key, root containment, VCS 非前提) | 部分 | **新規** (sanitize 強化 + 再設計) |
| §9.4 | 4 種 hooks (after_create / before_run / after_run / before_remove) | 無 | **新規** (orchestrator/workspace/hooks.go) |
| §9.5 | safety invariants (cwd == workspace, root prefix, regex sanitize) | 部分 | invariant チェック明示 |
| §10.1-10.6 | agent runner (codex app-server, `bash -lc`, stdio framing, thread/turn 抽出, `session_id`) | **強い** (`runtime/subsystem/stream/`) | **`platform/agent/codexclient/` に抽出 + roost と orchestrator が共有** |
| §10.5 | approval policy (実装依存、文書化必須) | sandbox/mcpproxy/credproxy で土台 | **roost 既存資産を活用** |
| §10.5 | `linear_graphql` client-side tool | 無 | **新規** (codex native `item/tool/call`、advertise は schema 制約で blocked) |
| §11.1-11.4 | Linear adapter (GraphQL, pagination, normalize, error map) | 無 | **`platform/tracker/linear/` に新規実装** |
| §12 | prompt construction (strict, issue+attempt 注入) | 無 | **新規** |
| §13.1-13.5 | observability (structured log + token accounting + rate limits) | slog のみ | **新規 + `platform/metrics/` に共通化** |
| §13.3 | snapshot API (synchronous) | proto に view 型あり | **新規** (orchestrator/httpserver/) |
| §13.7 | HTTP server (dashboard + `/api/v1/*`) | 無 | **新規** (orchestrator/httpserver/、ベース部は `platform/httpsurface/` で共有検討) |
| §14 | failure model (カテゴリ別 recovery) | 部分 | 体系化 |
| §15 | security (workspace 境界 + secret 非ログ + hook timeout) | sandbox/devcontainer で過充足 | **roost 資産活用** |
| §16 | reference algorithms (擬似コード) | — | そのまま実装テンプレ |
| §17 | conformance test matrix | テスト多数 | **orchestrator/ 専用テスト群を新規** |
| Appendix A | SSH worker extension | container 隔離はあるが用途別 | **対象外** (将来) |

## roost で**強くカバー**できる領域

1. **Codex app-server 統合 (§10)**
   - `runtime/subsystem/stream/` がまさに codex app-server の stdio 接続実装
   - `codex.command` の `bash -lc` invocation, stdio framing, thread/turn 抽出, `session_id` の組み立ては既存実装を `platform/agent/codexclient/` に抽出して再利用

2. **隔離環境 (§15)**
   - devcontainer + hostexec + mcpproxy + credproxy が SPEC 要求を大きく超える
   - SPEC §15.5 が要請する harness hardening の選択肢を既に持っている

3. **`linear_graphql` tool (§10.5)**
   - `mcpproxy/` の MCP relay が host MCP server を container に橋渡しできる
   - Linear MCP server を host に置けば SPEC §10.5 の tool として agent から呼べる

4. **continuation turn の土台**
   - `Subsystem.BindFrame/ReleaseFrame` + `TargetID` (thread 識別) が continuation 設計の前提を充足

## 既存資産がほぼ無い領域 (新規実装)

1. **`WORKFLOW.md` parser** (§5)
2. **Symphony 形の orchestrator state machine** (§7-8)
3. **4 種の workspace hooks** (§9.4)
4. **continuation turn の制御** (§7.1, §16.5)
5. **strict prompt template renderer** (§12)
6. **per-state concurrency / blocker ルール / dispatch sort** (§8.2-8.3)
7. **token accounting** (absolute vs delta の判別、§13.5)
8. **OPTIONAL HTTP server** (§13.7) — orchestrator では必須化

## 取り込めない / 修正必要な領域

| 項目 | 問題 | 対応 |
|---|---|---|
| Linear status の state machine 化 (Planning/PendingApproval/...) | SPEC §11.5 と衝突。Symphony は orchestrator に workflow phase を持たない | **採用しない**。phase は prompt + agent tool に押し込む |
| sqlite 等での永続化 | SPEC §14.3 と衝突。in-memory + tracker 再 poll で復旧 | **採用しない**。tracker と filesystem を source of truth に |
| git worktree 前提の workspace | SPEC §9.3 と衝突。VCS 非前提 | workspace は mkdir、git は `after_create` hook で実施 |
| 単一 agent (Claude/Codex のみ) 専用設計 | SPEC §10 は Codex app-server stdio を規定 | **`claude-app-server` shim** で stdio protocol を喋らせ、SPEC の `codex.command` 経由で切替 |
| roost の `LaunchPlan` ベース起動 | SPEC §10.1 の `bash -lc` invocation と stdio framing 要件を subsystem の起動経路に乗せる必要 | `platform/agentlaunch/` に抽出する際に Codex-compatible 起動経路を整える |

## 新規実装の概算 LOC

| モジュール | LOC |
|---|---|
| workflow loader + wfconfig | ~400 |
| scheduler (state machine + dispatch + retry + reconcile) | ~800 |
| reconciliation / stall / retry detail | ~400 |
| workspace manager + hooks | ~300 |
| prompt template renderer | ~150 (ライブラリ採用前提) |
| Linear adapter (新規実装) | ~500 |
| Codex stdio client (抽出 + 整理) | ~200 |
| claude-app-server shim | ~400 |
| HTTP server | ~300 |
| metrics (token / runtime / rate-limit) | ~200 |
| **合計** | **~3700 LOC** |

## 着手順序

[04-phases.md](04-phases.md) の Phase 計画に従う。要点:

- **P0** で物理分離 (`platform/` 抽出 + `client/` リネーム) を済ませる
- **P1-P2** で loader と Linear adapter を整備
- **P3-P4** で最小の poll → dispatch → 1 turn 単線を通す
- **P5** で `claude-app-server` shim を加えて agent 選択可能化
- **P6** で continuation turn と reconciliation を完成
- **P7** で HTTP server (観測手段確立)
- **P8-P9** で SPEC §10.5 extension と conformance test
