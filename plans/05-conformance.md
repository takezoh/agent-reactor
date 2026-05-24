# SPEC Conformance / Deviations

Symphony SPEC.md v1 Draft への conformance 方針と、明示的に逸脱する項目を整理する。

`docs/technical/orchestrator/symphony-conformance.md` として SPEC リリース時に同等の表を維持する想定。

## 厳守する項目

| SPEC §  | 項目 | 我々の対応 |
|---|---|---|
| §1 §2.2 §11.5 | Symphony は scheduler/runner であり、チケット状態遷移 (Planning → Done) を orchestrator に持たない | Linear status の state machine 化はせず、phase は agent prompt + agent tool で実装（orchestrator に workflow phase を持たない） |
| §3.1 | 8 component の責務分離 | `orchestrator/workflowfile/wfconfig/scheduler/tracker/workspace/agent/httpserver/` で 1:1 対応。logging は `platform/logger/` |
| §4 | 8 ドメインエンティティ (Issue/Workflow/Config/Workspace/RunAttempt/LiveSession/RetryEntry/OrchestratorRuntimeState) | 同名の Go 型として実装 |
| §4.2 | sanitized identifiers / session_id 形式 | `<thread_id>-<turn_id>`、workspace key sanitize 正規表現 `[A-Za-z0-9._-]` |
| §5 | WORKFLOW.md YAML front matter + Markdown body | `orchestrator/workflowfile/` で parser 実装 |
| §5.3 | front matter schema (tracker/polling/workspace/hooks/agent/codex) | typed Go struct で受ける。unknown keys は無視 (forward compat) |
| §5.4 | strict template (Liquid 互換、unknown var/filter は error) | Liquid 互換ライブラリを採用 (候補: `gopkg.in/osteele/liquid.v1`) |
| §6.1 | config 解決順序 (file → defaults → `$VAR` → coerce → validate) | `wfconfig` で順序明示 |
| §6.2 | dynamic reload (`WORKFLOW.md` change で再適用、不正 reload で last known good 保持) | fsnotify + reload-on-tick の二重保険 |
| §6.3 | preflight validation (startup + 毎 tick) | scheduler 内で実装 |
| §7.1 | orchestration states (Unclaimed/Claimed/Running/RetryQueued/Released) | `orchestrator/scheduler/` 内 enum |
| §7.1 | continuation turn — 同一 thread で `max_turns` まで、turn 後に tracker 再確認、worker 終了後 1s 連続 retry | scheduler が thread を保持し agent runner と協調 |
| §8.1 | poll loop 順序 (reconcile → validate → fetch → sort → dispatch → notify) | 厳守 |
| §8.2 | dispatch eligibility + blocker(Todo) rule | 厳守 |
| §8.3 | global + per-state concurrency | 厳守 |
| §8.4 | retry/backoff (continuation 1s 固定、失敗 `min(10000*2^(n-1), max)`) | 厳守 |
| §8.5 | reconciliation Part A (stall) + Part B (tracker state) | 厳守 |
| §8.6 | startup terminal workspace cleanup | 厳守 |
| §9.5 | safety invariants (cwd == workspace, root prefix check, sanitized key) | invariant チェックを agent 起動直前に実装 |
| §10.1 | `bash -lc <codex.command>` invocation, workspace cwd | 厳守 |
| §10.2 | session 起動責任 (per-issue workspace, thread/turn 識別、issue metadata 付与) | codexclient で実装 |
| §10.4 | event 名 (`session_started`, `turn_completed`, `turn_failed`, etc.) | claude-app-server shim も同一 event 名で emit |
| §10.6 | timeout (`read_timeout_ms`, `turn_timeout_ms`, `stall_timeout_ms`) | scheduler と codexclient で分担実装 |
| §11.1 | tracker adapter 3 操作 (`fetch_candidate_issues` / `fetch_issues_by_states` / `fetch_issue_states_by_ids`) | `platform/tracker/linear/` で実装 |
| §11.2 | Linear specifics (slugId フィルタ、ID 型、pagination 50/page、timeout 30s) | 厳守 |
| §11.3 | 正規化ルール (labels lowercase, blockers from `blocks` 反転, priority int-only, ISO-8601) | 厳守 |
| §11.5 | tracker writes は agent 側 | orchestrator は read-only |
| §12 | prompt rendering (issue + attempt 注入、strict mode、空 prompt fallback) | 厳守 |
| §13.1 | structured log (`issue_id` / `issue_identifier` / `session_id`、key=value) | `platform/logger/` に key=value helper を追加 |
| §13.5 | token accounting (absolute thread totals 優先、delta フォールバック禁止、runtime aggregate) | `platform/metrics/` で実装 |
| §14 | failure model (カテゴリ別 recovery、orchestrator は crash しない) | 各 module で error category を返す |
| §14.3 | restart は tracker re-poll + workspace 残存 で復旧 | 永続 DB なし |
| §15 | filesystem safety / secret 非ログ / hook timeout | 厳守 |
| §16 | reference algorithms (pseudo code) | scheduler 実装の出発点として使用 |
| §17.1-§17.7 | Core Conformance test | P9 で test 群を整備 |

## 逸脱 / 拡張する項目

逸脱は `docs/technical/orchestrator/symphony-conformance.md` の "Documented Posture" セクションで宣言する想定。

| SPEC §  | SPEC | 我々の選択 | 理由 |
|---|---|---|---|
| §10 | Codex app-server 専用 | **`codex.command` 経由の stdio shim 方式で複数 agent 対応** | SPEC §10.1 が定義する extension point を素直に活用。`codex.command: claude-app-server` で agent 切替 |
| §3.3 | sandbox は impl-defined | **devcontainer mode をデフォルト推奨**。direct mode も per-project 選択可 | roost の既存資産を活用、隔離強度のデフォルト引き上げ |
| §9.3 | workspace population は impl-defined (VCS 非前提) | **`after_create` hook で `git worktree add` を強く推奨** (必須ではない) | 実用上ほぼ全てが Git ベース。ただし SPEC の VCS 非前提性は守る |
| §13.7 | HTTP server は OPTIONAL | **必須として実装** | orchestrator は TUI を持たないため、唯一の view 手段 |
| §15.5 | harness hardening は文書化のみ | **devcontainer + credproxy + mcpproxy がデフォルト** | 隔離強度をデフォルトで引き上げ。SPEC が許容する範囲内の選択 |
| §10.5 (`linear_graphql`) | OPTIONAL extension | **codex native `item/tool/call` で実装**（MCP 不採用）。handler は実装済だが advertise は pinned codex 0.133.0 で blocked | SPEC は「orchestrator 自身が tool を実行」と規定し MCP は SPEC 非語彙。`DynamicToolSpec` が schema 上 orphan のため tool 宣言の wire 経路が無く、実機到達は schema bump 待ち（[issues/024](../issues/024-p8b-linear-graphql-tool.md) §B） |
| §18.2 (Persistence TODO) | future work | **現時点では実装しない** (SPEC §14.3 in-memory 設計を維持) | 永続化は SPEC roadmap 次第 |
| §18.2 (tracker write API TODO) | future work | **実装しない**。agent tool 経由のまま | SPEC §11.5 に従う |
| §18.2 (pluggable tracker TODO) | future work | **Linear 専用で開始**。adapter pattern を保つ | 多 tracker 化は需要が立証されてから |
| Appendix A (SSH worker) | OPTIONAL extension | **対象外** | container 隔離で代替可能 |

## documented posture (SPEC §10.5 §15.5 要請)

SPEC が「実装は documented posture を宣言せよ」と要請する項目への我々の posture:

### approval / sandbox policy (§10.5)

**Posture**: orchestrator は **sandbox enforcement に依存** する。Codex の `approval_policy` / `thread_sandbox` / `turn_sandbox_policy` を auto-approve 相当で起動し、実際の境界は `platform/sandbox/devcontainer/` が提供する container で保証する。

- `codex.command: codex app-server` の場合: Codex 側の auto-approve policy を WORKFLOW.md で指定
- `codex.command: claude-app-server` の場合: claude には approval 概念がないため shim が**即実行**（強制しない）。`turn/start` で受け取った `approvalPolicy`/`sandboxPolicy` 値は `slog.Warn` に記録するのみ（オペレータへの逸脱通知）。実際の安全境界は container が担う

### user-input-required (§10.5)

**Posture**: user input を要求する turn は **fail 扱い** する (SPEC §10.5 が許可する選択肢のひとつ)。自動運転前提のため、人間の介入が必要な状況は run を失敗させて next attempt の判断を orchestrator に委ねる。

### harness hardening (§15.5)

**Posture**: 以下を default で適用:

- 全 agent 実行を devcontainer 内で行う (direct mode は per-project opt-in)
- credproxy で AWS SSO / gcloud / ssh-agent を境界化
- mcpproxy で agent から見える MCP tool を whitelist 化
- hostexec で container → host への capability 委譲を allow-list で制限
- `tracker.api_key` 等の secret はログ出力禁止
- hook script 出力はログで truncate

### secret 取扱い (§15.3)

**Posture**:

- `$VAR` indirection は SPEC 通り実装
- WORKFLOW.md の `tracker.api_key` 等が `$VAR` の場合のみ環境変数解決
- 解決後の値はログ出力禁止
- 存在チェックのみログ可

**devcontainer-default (§15)**: SPEC §15 lists container/VM isolation as RECOMMENDED hardening. The orchestrator supports it via `DevcontainerLauncher` wired through `SandboxDispatcher`. When `~/.roost/settings.toml` sets `mode = "devcontainer"` for a project, the codex app-server runs inside a devcontainer. This is a SPEC-compliant extension (not a violation).

## SPEC 用語と実装名の対応

| SPEC 用語 | 我々の実装名 |
|---|---|
| `Workflow Loader` (§3.1.1) | package `orchestrator/workflowfile/` |
| `Config Layer` (§3.1.2) | package `orchestrator/wfconfig/` |
| `Issue Tracker Client` (§3.1.3) | package `orchestrator/tracker/` (`platform/tracker/linear/` を使う wrapper) |
| `Orchestrator` (§3.1.4 component) | package `orchestrator/scheduler/` (※サービス全体名と区別のため `scheduler` に rename) |
| `Workspace Manager` (§3.1.5) | package `orchestrator/workspace/` |
| `Agent Runner` (§3.1.6) | package `orchestrator/agent/` (`platform/agent/codexclient/` を使う wrapper) |
| `Status Surface` (§3.1.7) | package `orchestrator/httpserver/` |
| `Logging` (§3.1.8) | package `platform/logger/` |
| `Workflow Definition` (§4.1.2) | type `orchestrator/workflowfile.Workflow` |
| `Service Config (Typed View)` (§4.1.3) | type `orchestrator/wfconfig.Config` |
| `Run Attempt` (§4.1.5) | type `orchestrator/scheduler.RunAttempt` |
| `Live Session` (§4.1.6) | type `orchestrator/scheduler.LiveSession` |
| `Retry Entry` (§4.1.7) | type `orchestrator/scheduler.RetryEntry` |
| `Orchestrator Runtime State` (§4.1.8) | type `orchestrator/scheduler.State` |

## conformance test の整備 (P9)

SPEC §17 の各項目を Go test で埋める。命名規約:

```
TestSPEC_<section>_<short_name>
  例: TestSPEC_17_1_WorkflowFilePathPrecedence
      TestSPEC_17_3_LinearProjectFilterUsesSlugId
      TestSPEC_17_4_RetryBackoffCapHonored
```

`docs/technical/orchestrator/symphony-conformance.md` に SPEC § と test name の対応表を維持する。

## 今後 SPEC が更新された際の対応指針

- SPEC v1 → v2 等 major bump 時は `plans/06-spec-v2-migration.md` のような新規 plan を作成
- minor 改訂は本ドキュメントに inline 反映
- Codex app-server schema は別 lifecycle (`platform/agent/codexschema/` の pin を上げる PR で更新)
- 我々の逸脱項目は SPEC が公式に extension として受け入れた場合に削除候補となる
