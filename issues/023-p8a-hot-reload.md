# 023: orchestrator — WORKFLOW.md hot reload (§6.2 live re-apply)

- **Phase**: P8a ([plans/04-phases.md#p8-hot-reload--linear_graphql-tool](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 006 (merged; wfconfig)、007 (merged; scheduler loop)
- **並行可**: P5 と完全独立。scheduler/wfconfig のみに触れる
- **Blocks**: M3

## Background

SPEC §6.2 は WORKFLOW.md 変更時に **再起動なしで config を re-read・re-apply** することを要請。現状 scheduler は tick ごとに `reloadConfig()` で再読込しており毎 tick で反映はされるが、§6.2 が求める (1) **即時** reload（fsnotify）と (2) **不正 reload 時の last-known-good 保持 + operator-visible warn** が未実装。

## Tasks

### A. fsnotify watch

- [ ] WORKFLOW.md を fsnotify で監視（`go.mod` に `fsnotify` 既存 — `platform/lib` の利用パターン参照）。変更検知で即時 reload signal を loop に送る（single-authority: 実際の re-apply は loop goroutine）
- [ ] poll interval を待たずに反映

### B. live re-apply (§6.2)

- [ ] poll interval / concurrency（global・per-state）/ active-terminal state set を**動的反映**
- [ ] codex settings は次回 dispatch から反映（実行中 turn は触らない）

### B'. 不正 reload 時の挙動（§5.5 — 確定: option 2「新規 dispatch のみブロック」）

SPEC §5.5 は「Workflow file read/YAML errors **block new dispatches until fixed**」と明示。したがって不正 reload では:

- [ ] **last-known-good cfg を保持**（直前に正常 resolve できた cfg をフィールドに持つ。reconcile の依存先）
- [ ] **新規 dispatch のみ gating**: workflow が正常に戻るまで `dispatchOnce`／retry-fire の新規 spawn をスキップし、operator-visible warn を出す
- [ ] **reconcile は last-known-good で継続**: stall 検知・terminal cleanup・tracker refresh は止めない（running を放置しない）
- [ ] **既存 running は無傷**（別 goroutine。cfg 入替の影響を受けない）
- [ ] workflow が正常化したら last-known-good を更新し dispatch を再開
- [ ] 留意: 現状の `tickOnce` は `reloadConfig` 失敗時に **tick 全体を return**（reconcile も止まる）。本 issue で「reconcile は last-known-good で継続 / dispatch のみ gating」に作り替える

### C. テスト (§17.1 系)

- [ ] WORKFLOW.md 変更で interval/concurrency が即時に変わる
- [ ] 不正 reload で **新規 dispatch が gating される**（spawn が呼ばれない）一方、**reconcile は last-known-good で継続**（stall kill / terminal cleanup が動く）
- [ ] 不正 reload 後に正常な WORKFLOW.md へ戻すと dispatch が再開する
- [ ] 不正 reload 中も既存 running は影響を受けない + operator-visible warn が出る
- [ ] fsnotify の発火を fake 化（または一時ファイル書換）して検証

## Acceptance Criteria

- WORKFLOW.md を save すると orchestrator が即時に再読込し設定を live 反映
- **不正な reload では停止せず**、(1) last-known-good cfg を保持、(2) **新規 dispatch のみブロック**（§5.5）、(3) reconcile は last-known-good で継続、(4) 既存 running は無傷、(5) operator-visible warn。正常化で dispatch 再開
- `go test ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §6.2 (Dynamic reload), §5.5 (error surface / dispatch gating)
- [plans/04-phases.md#p8](../plans/04-phases.md)、`orchestrator/scheduler/scheduler.go`（`reloadConfig`）、`orchestrator/wfconfig`
