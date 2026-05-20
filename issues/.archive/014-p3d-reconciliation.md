# 014: scheduler — active-run reconciliation + startup terminal cleanup

- **Phase**: P3d ([plans/04-phases.md#p3-scheduler-core](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: [011](011-p3a-scheduler-state.md) (state)、009 (merged; `RefreshStates`/`TerminalIssues`)、010 (merged; workspace `Remove`)
- **Blocks**: P6 (continuation/stall の本格化が本 reconcile を拡張する)

## Background

SPEC §8.5 / §8.6 / §16.3 の reconciliation を実装する。毎 tick の頭で running issue を点検し (§8.1 step 1)、stall と tracker 状態変化に応じて worker を停止・workspace を整理する。

## Tasks

### A. Part A — stall detection (§8.5)

- [x] running issue ごとに `elapsed_ms` を算出: `last_codex_timestamp` があればそこから、無ければ `started_at`
- [x] `elapsed_ms > codex.stall_timeout_ms` で worker を kill + retry queue (state machine 011 の WorkerExitAbnormal 経路)
- [x] **`stall_timeout_ms <= 0` なら stall 検出を完全に skip** (§5.3.6 / §8.5)

### B. Part B — tracker state refresh (§8.5)

- [x] running 全 issue ID の現在 state を `tracker.RefreshStates` で取得
- [x] 各 running issue:
  - [x] terminal → `run.Worker.Kill("terminal")` + workspace `Remove` (clean)
  - [x] active → in-memory issue snapshot 更新
  - [x] active でも terminal でもない → `run.Worker.Kill("non-active")` (**workspace は残す**)
- [x] **refresh 失敗時は worker を止めず次 tick に再試行** (§8.5)

### C. startup terminal cleanup (§8.6 / §16.3)

- [x] 起動時に `tracker.TerminalIssues` で terminal state の issue を取得
- [x] 各 identifier の workspace ディレクトリを `Remove`
- [x] **terminal-issues fetch 失敗時は warn ログを出して起動継続** (§8.6)

### D. tick への組み込み

- [x] reconcile を tick の **最初** に実行する関数を 012 の tick に注入 (§8.1 step 1: dispatch より前)

### E. テスト (§17.4)

- [x] stall: `last_codex_timestamp`/`started_at` 基準の elapsed 判定、超過で kill+retry、`stall_timeout_ms<=0` で skip
- [x] refresh: terminal→kill+clean / active→snapshot 更新 / 中間→kill のみ (workspace 残存)
- [x] refresh 失敗時に worker 継続
- [x] startup cleanup が terminal workspace を削除、fetch 失敗で起動継続

## Acceptance Criteria

- stall した worker が retry に落ち、terminal 遷移で worker と workspace が消える
- active 中間 state では workspace を残しつつ worker を止める
- 起動時に terminal workspace が掃除される（fetch 失敗でも起動は継続）
- `go test ./orchestrator/scheduler/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §8.5 (Reconciliation), §8.6 (Startup Cleanup), §16.3 (Reconcile Active Runs), §17.4
- [plans/04-phases.md#p3](../plans/04-phases.md)
- [011](011-p3a-scheduler-state.md)、[009](009-p2b-orchestrator-tracker.md)、[010](010-p2c-workspace-manager.md)
