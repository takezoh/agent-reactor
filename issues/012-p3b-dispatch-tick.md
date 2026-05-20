# 012: scheduler — poll-and-dispatch tick (eligibility / sort / concurrency / retry)

- **Phase**: P3b ([plans/04-phases.md#p3-scheduler-core](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: [011](011-p3a-scheduler-state.md) (state machine)、009 (merged; tracker `Candidates`)
- **Blocks**: M1 (最小単線通電) — 013 と合わせて end-to-end が通る

## Background

SPEC §8.1–§8.4 / §16.2 の poll-and-dispatch を実装し、007 の stub loop の「dispatch しない」を実際の dispatch に置き換える。worker 起動 (agent runner) は 013 が提供するため、**dispatch は worker-spawn 関数を注入**して 013 と疎結合にする。

## Tasks

### A. tick sequence (§8.1)

- [x] tick: ① reconcile (014 の関数を呼ぶ。未実装段階は no-op 注入) → ② preflight 再検証 (007) → ③ `tracker.Candidates` → ④ sort → ⑤ slots がある限り dispatch → ⑥ observability 通知 (P7 まではログ)
- [x] per-tick validation 失敗時は **reconcile は実行しつつ dispatch を skip** (§8.1)
- [x] startup: config validate → startup cleanup (014) → 即時 tick → 以後 `polling.interval_ms` 間隔

### B. candidate eligibility (§8.2)

- [x] `id`/`identifier`/`title`/`state` が揃う
- [x] state が `active_states` かつ not `terminal_states`
- [x] running/claimed に未登録
- [x] global / per-state スロットに空き
- [x] **blocker rule**: state == `Todo` のとき、非 terminal な blocker が1つでもあれば dispatch しない

### C. sorting (§8.2)

- [x] ① priority 昇順 (null は最後) ② created_at 古い順 ③ identifier 辞書順 tie-break (stable)

### D. concurrency (§8.3)

- [x] global: `available = max(max_concurrent_agents - running_count, 0)`
- [x] per-state: `max_concurrent_agents_by_state[state]` (正規化 key) があればそれ、無ければ global
- [x] running map の現在 state 別カウントで判定

### E. retry / backoff (§8.4)

- [x] continuation retry: 固定 `1000ms`
- [x] failure retry: `delay = min(10000 * 2^(attempt-1), agent.max_retry_backoff_ms)`
- [x] retry timer 発火時 (§8.4 手順): active candidate を再取得 → 当該 issue を探す → 不在なら claim release → eligible なら slots 次第で dispatch、無ければ `no available orchestrator slots` で requeue → active でないなら release

### F. worker-spawn 注入

- [x] dispatch は `spawn func(issue, attempt) (scheduler.Worker, error)` を注入で受ける (013 の agent runner を後で配線)。spawn 失敗時は §16.4 通り retry スケジュール
- [x] `cmd/orchestrator` の loop を本 tick に配線 (007 stub から差し替え)

### G. テスト (§17.4)

- [x] eligibility 全条件 (active/terminal、running/claimed 重複、blocker-Todo rule)
- [x] sort 順 (priority/created_at/identifier、null priority 最後)
- [x] global/per-state concurrency の slot 計算
- [x] backoff 計算 (attempt ごと、max cap)、continuation 1s
- [x] retry 発火時の dispatch / requeue / release 分岐 (fake tracker + fake spawn)

## Acceptance Criteria

- 1 tick で eligible issue を優先度順に slots ぶん dispatch し、blocker-Todo・concurrency を尊重
- backoff/continuation の delay が §8.4 通り
- per-tick validation 失敗で dispatch skip しても reconcile 経路は止まらない
- `go test ./orchestrator/scheduler/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §8.1–§8.4 (Polling/Scheduling/Retry), §16.2 (Poll-and-Dispatch Tick), §17.4
- [plans/04-phases.md#p3](../plans/04-phases.md)
- [011](011-p3a-scheduler-state.md) (state)、[009](009-p2b-orchestrator-tracker.md) (candidates)
