# 006: wfconfig — typed config view (default / $VAR / ~ / 検証)

- **Phase**: P1b ([plans/04-phases.md#p1-workflow-loader--wfconfig--preflight](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: [005](005-p1a-workflowfile.md) (`workflowfile.Workflow.Config` を入力に取る)
- **Blocks**: 007 (preflight が typed config を検証する)、P2 以降 (tracker/workspace/hooks/agent/codex 設定の読み出し)

## Background

SPEC §5.3 / §6.1 / §6.4 の typed config を `orchestrator/wfconfig/` に実装する。
005 が返す `map[string]any` (front matter root) を入力に、default 適用・`$VAR` 解決・`~` 展開・
型強制と検証を行い、各セクション (tracker/polling/workspace/hooks/agent/codex) を typed struct で公開する。

## Tasks

### A. パッケージ + typed struct (SPEC §5.3, §6.4)

- [ ] `src/orchestrator/wfconfig/` を新設 (`package wfconfig`)
- [ ] `Config` を section 別 struct で定義 (Tracker / Polling / Workspace / Hooks / Agent / Codex)
- [ ] `Resolve(raw map[string]any, workflowDir string) (Config, error)` を公開
  - `workflowDir` は relative path / `workspace.root` の基準

### B. 解決パイプライン (SPEC §6.1)

- [ ] default 適用 (§6.4 cheat sheet の全 default 値):
  - `tracker.endpoint` = `https://api.linear.app/graphql` (kind=linear 時)
  - `tracker.active_states` = `["Todo","In Progress"]`、`terminal_states` = 5 値
  - `polling.interval_ms` = 30000、`hooks.timeout_ms` = 60000
  - `agent.max_concurrent_agents` = 10、`max_turns` = 20、`max_retry_backoff_ms` = 300000
  - `codex.command` = `codex app-server`、`turn_timeout_ms` = 3600000、`read_timeout_ms` = 5000、`stall_timeout_ms` = 300000
  - `workspace.root` = `<system-temp>/symphony_workspaces`
- [ ] `$VAR` 解決は **値が明示的に `$VAR_NAME` を含む場合のみ** (env が global override しない)
  - `tracker.api_key` の `$VAR` が空文字に解決されたら missing 扱い
- [ ] `~` 展開と `$VAR` 展開は **path/command 系の値のみ** に適用 (URI や任意 shell 文字列は書き換えない)
- [ ] `workspace.root` を絶対パスへ正規化 (relative は `workflowDir` 基準)

### C. 型強制と検証 (SPEC §5.3.4–§5.3.6)

- [ ] `max_turns` / `hooks.timeout_ms` の不正値は config validation error
- [ ] `agent.max_concurrent_agents_by_state`: state 名を **lowercase 正規化**、非正/非数値の entry は **無視**
- [ ] 型不一致 (整数期待に文字列等) は coercion を試み、不能ならエラー
- [ ] `codex.command` は **shell 文字列としてそのまま保持** (path 展開しない)

### D. テスト (SPEC §17.1)

- [ ] default 適用、`$VAR` 解決 (api_key + path)、`~` 展開、per-state map 正規化、`codex.command` 保持を網羅

## Acceptance Criteria

- 005 の `Config` map を入力に typed `Config` を返す
- §6.4 の全 default が欠落時に適用される
- `$VAR` / `~` が path/key 値のみに適用され、URI・command 文字列は不変
- per-state concurrency map が正規化され不正 entry を捨てる
- 不正な `max_turns` / `timeout_ms` で validation error
- SPEC §17.1 の config 系項目を test で pass、`go test ./orchestrator/wfconfig/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §5.3, §6.1, §6.4, §17.1
- [plans/04-phases.md#p1](../plans/04-phases.md)
- [005](005-p1a-workflowfile.md) — 入力となる loader
