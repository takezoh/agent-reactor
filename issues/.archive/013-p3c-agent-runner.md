# 013: orchestrator/agent — agent runner (prompt + raw codex single turn + events)

- **Phase**: P3c ([plans/04-phases.md#p3-scheduler-core](../plans/04-phases.md))
- **Status**: Done (2026-05-20)
- **Depends on**: 010 (merged; workspace)、P0c (merged; `platform/agent/codexclient`)、008 (merged; `tracker.Issue`)
- **Blocks**: M1 (最小単線通電) — 012 と合わせて end-to-end が通る

## Background

SPEC §10 / §16.5 の Agent Runner を `orchestrator/agent/` に実装する。workspace 準備 → prompt 構築 → codex app-server を `bash -lc` で起動 → **1 turn 実行** → 最小限の §10.4 event を orchestrator に emit。

本フェーズは **single turn**（phase 計画の「1 turn」）。continuation の multi-turn ループ・stall・token 集計は **P6**。sandbox 配線 (agentlaunch 経由) は **P4** — ここでは生の `exec.Command(bash, -lc, codex.command)`。

## Tasks

### A. prompt 構築 (§5.4 / §12)

- [x] `orchestrator/prompt/` 新設: WORKFLOW.md prompt body をテンプレートとして `issue` / `attempt` 変数で render
- [x] **strict 失敗**: 未知変数・未知 filter は render エラー (§5.4) → `template_render_error`
- [x] body 空なら最小デフォルト prompt (`You are working on an issue from Linear.`) を許容 (§5.4)
- [x] **ライブラリ選定 (validate 必須)**: Liquid 互換が要件。候補と strict 失敗対応の可否を検証してから採用:
  - `github.com/osteele/liquid` (Go の代表的 Liquid 実装。**未知変数で空文字 render する既定挙動のため、strict 失敗を満たせるか要検証** — 満たせなければ render 前に変数 set を検証)
  - `github.com/flosch/pongo2` (Jinja2 風、Liquid 構文ではない)
  - stdlib `text/template` + `Option("missingkey=error")` (未知変数で失敗するが Liquid 構文/filter 非対応)
  - → `github.com/osteele/liquid` を採用。`Engine.StrictVariables()` で未知変数エラーを標準機能として得られることを確認済み

### B. agent runner (§10.7 / §16.5 を single-turn に簡約)

- [x] 具象 worker が `scheduler.Worker`（`Kill(reason string) error`）を満たす — codex app-server プロセスと turn ループを保持し、Kill で subprocess 終了 + ループ停止。spawn が `scheduler.Worker` を返す
- [x] `orchestrator/agent/` 新設、`RunAttempt(ctx, issue, attempt, emit func(Event))`:
  1. workspace `Ensure` (010) → 失敗で fail
  2. `before_run` hook (010) → 失敗で fail
  3. codex app-server を `bash -lc <codex.command>` で workspace を cwd に起動 (§10.1)
  4. `codexclient` で session initialize → thread start (cwd = workspace 絶対パス, §10.2)
  5. render した prompt で **1 turn** 実行
  6. turn 完了 / 失敗 / subprocess exit を判定 (§10.3)
  7. `after_run` hook (best-effort) → 正常 exit
- [x] §9.5 Inv1: 起動前に `cwd == workspace_path` を `workspace.VerifyCWD` で検証

### C. session 識別子 (§10.2)

- [x] `thread_id` / `turn_id` を codex 応答から抽出し `session_id = "<thread_id>-<turn_id>"` を emit
- [ ] issue メタ (`<identifier>: <title>`) を turn/session title に載せる (protocol が許せば) — P6 へ送り

### D. 最小 emit event (§10.4)

- [x] `session_started` / `turn_completed` / `turn_failed` を emit (timestamp 付き)
- [ ] 残りの event 種別 (`turn_input_required` 等) と token usage は **P6**

### E. テスト (§17.5)

- [x] prompt render が `issue`/`attempt` を埋め、未知変数で失敗する
- [x] fake codex app-server (stdio で codexclient.Server を喋る test double) で 1 turn を回し、session_started/turn_completed が emit される
- [x] turn 失敗 → turn_failed emit + after_run best-effort 実行
- [x] workspace/ before_run 失敗で fail する

## Acceptance Criteria

- 1 issue で workspace 作成 → codex app-server 起動 → 1 turn → workspace 残存
- `session_id` が `<thread_id>-<turn_id>` 形式
- prompt の strict 失敗が typed error になる
- `go test ./orchestrator/agent/ ./orchestrator/prompt/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §5.4 (Prompt Template), §10 (Agent Runner Protocol), §12 (Prompt Construction), §16.5 (Worker Attempt), §17.5
- [plans/03-agent.md](../plans/03-agent.md)、[plans/04-phases.md#p3](../plans/04-phases.md)
- [010](010-p2c-workspace-manager.md) (workspace)、`platform/agent/codexclient` (stdio protocol)
