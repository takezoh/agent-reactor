# 011: orchestrator/scheduler — orchestration state machine + runtime state

- **Phase**: P3a ([plans/04-phases.md#p3-scheduler-core](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: [007](007-p1c-preflight-stub-scheduler.md) (scheduler package + loop scaffold)、008 (merged; `tracker.Issue`)
- **Blocks**: 012 (dispatch tick)、014 (reconciliation)

## Background

SPEC §7 の orchestration state machine と in-memory runtime state を `orchestrator/scheduler/` に実装する。
ここは **scheduling brain のデータ層と遷移関数** のみ。poll/dispatch ロジック (§8) は 012、agent 起動は 013、reconciliation は 014。

orchestrator は scheduling state を変更する唯一の権威 (§7.4 single-authority)。永続 DB は持たず (§14.3)、復旧は tracker 再 poll + workspace 永続に委ねる。

## Tasks

### A. claim state と runtime state (§7.1)

- [x] claim state を定義: `Unclaimed` / `Claimed` / `Running` / `RetryQueued` / `Released`
- [x] in-memory state (§16.4 の構造に対応):
  - [x] `running` map (issue_id → run state: worker, identifier, issue snapshot, session_id, codex_app_server_pid, last_codex_{message,event,timestamp}, started_at, retry_attempt)
  - [x] `claimed` set
  - [x] `retry_attempts` map (issue_id → {attempt, identifier, error, due_at_ms, timer handle})
- [x] token/runtime 集計フィールドは **構造だけ用意** (実集計は P6)
- [x] worker は `any` でなく **consumer-side の `Worker` interface** で保持 (driver encapsulation):

```go
// scheduler パッケージに定義。具象は 013 の agent runner が実装。
type Worker interface {
    Kill(reason string) error // §7.2 CanceledByReconciliation / stall のログ・event 用
}
```
  - scheduler は agent/codex の具体を知らず `Kill` だけ呼ぶ。メソッドは現状 Kill のみ (投機的に増やさない)

### B. 遷移関数 (§7.3) — I/O を持たない純粋関数中心

- [x] `Dispatch(issue, attempt)` — running 追加 + claimed 追加 + retry_attempts 削除 (§16.4)
- [x] `WorkerExitNormal(issue_id)` — running 削除 + continuation retry (attempt 1) スケジュール (§7.3)
- [x] `WorkerExitAbnormal(issue_id, err, attempt)` — running 削除 + exponential backoff retry スケジュール
- [x] `ReleaseClaim(issue_id)` — claimed/running/retry から除去 → Unclaimed
- [x] `Snapshot()` — observability/HTTP (P7) 向けの読み取り専用 view

### C. single-authority 直列化 (§7.4)

- [x] state mutation を単一 goroutine / mutex で直列化し duplicate dispatch を防ぐ
- [x] worker からの結果 (codex update / exit) は channel 経由で authority に集約

### D. run attempt lifecycle (§7.2)

- [x] 11 phase (`PreparingWorkspace`..`CanceledByReconciliation`) を enum で定義 (実遷移の駆動は 013/014、ここでは型と記録のみ)

### E. テスト (§17.4 の state 部分)

- [x] Dispatch → running/claimed に入る、retry_attempts から消える
- [x] WorkerExitNormal → running から消え continuation retry(attempt 1) が立つ
- [x] WorkerExitAbnormal → backoff retry が立つ
- [x] ReleaseClaim → 全 map から消える
- [x] 同一 issue の重複 Dispatch を防ぐ (claimed/running チェック)

## Acceptance Criteria

- claim state の全遷移が単一権威下で直列化され、duplicate dispatch が起きない
- backoff/continuation の retry entry が §8.4 の区別通りに作られる (実 delay 計算は 012)
- `go test ./orchestrator/scheduler/` 緑、lint 緑

## Notes

- backoff の**数値計算式** (§8.4 `min(10000*2^(n-1), max)`) は 012 で実装。011 は「どの種類の retry を立てるか」の構造まで
- continuation の **multi-turn ループ自体は P6**。011 は worker 正常終了後に 1s retry を立てる経路だけ用意

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §7 (State Machine), §16.4 (Dispatch One Issue), §17.4
- [plans/04-phases.md#p3](../plans/04-phases.md)
- [007](007-p1c-preflight-stub-scheduler.md) (loop scaffold)
