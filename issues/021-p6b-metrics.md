# 021: platform/metrics — token/runtime aggregation + codex activity (stall) tracking

- **Phase**: P6b ([plans/04-phases.md#p6-continuation--reconciliation--metrics](../plans/04-phases.md))
- **Status**: Done (2026-05-20)
- **Depends on**: 013 (merged; runner emits events)、014 (merged; stall 検知の枠)
- **並行可**: P5 と別領域。codex の既存 event から集計でき claude shim 不要（agent 非依存の集計）
- **Blocks**: M3、022 (HTTP server が集計値を出す)

## Background

SPEC §13.5 が要請する token/runtime 集計と、§8.5 Part A の stall 検知に必要な **`last_codex_timestamp` 更新**を実装する（後者は M1 レビュー #7 の積み残し）。現状 `RunAttempt.LastCodexTimestamp`/`Total*Tokens` は誰も更新せず、stall は常に `StartedAt` 基準・token は 0。

## Tasks

### A. codex activity tracking（#7 解消）

- [x] agent runner が codex event 受信ごとに `RunAttempt.LastCodexTimestamp`/`LastCodexEvent`/`LastCodexMessage` を更新する seam を入れる（state 更新は single-authority を守る経路で）
- [x] これにより 014 の stall 検知が「最終活動からの経過」で正しく効く（現状 dispatch からの経過）

### B. token / runtime 集計 (§13.5)

§13.5 の「delta」混在を以下に分離して扱う（[plans/05-conformance.md](../plans/05-conformance.md) と一致）:

- [x] `platform/metrics/` 新設: **absolute thread totals のみ**を集計に使う。`last_token_usage` 等の **delta 形式 payload は無視**（SPEC「Ignore delta-style payloads」= conformance「delta フォールバック禁止」）。絶対値を出さない agent への delta 合算フォールバックは**持たない**
- [x] **二重計上回避の bookkeeping は実装必須**: 絶対値は累積報告されるため、`last_reported_total` との **差分を取って aggregate に積み**、`last_reported` を更新する（SPEC「track deltas relative to last reported totals to avoid double-counting」）。これは禁止される delta-fallback とは別物
- [x] runtime seconds 集計（turn/session の経過）
- [x] rate-limit snapshot（codex/claude が返す場合）の保持
- [x] codex の `turn/completed` usage を取り込み `RunAttempt.Total*Tokens` に反映。**claude は per-turn usage を shim(019)が absolute に積み上げて emit する責務**（orchestrator は常に absolute を受ける前提）

### C. テスト (§17.5)

- [x] event 受信で LastCodexTimestamp が進む → stall 検知が活動基準になる
- [x] 同じ累積 absolute total を複数回報告しても **二重計上されない**（last-reported 差分追跡が効く）
- [x] `last_token_usage` 等の delta 形式 payload は集計に**混入しない**（無視される）
- [x] usage event から input/output/total が集計される

## Acceptance Criteria

- stall 検知が「最終 codex 活動からの経過」で動く（dispatch 基準でない）
- token は **absolute thread totals のみ**から集計し、last-reported 差分追跡で**二重計上しない**。delta 形式 payload は無視（conformance「delta フォールバック禁止」と一致）
- runtime が正確に集計され RunAttempt/observability に載る
- orchestrator は常に absolute を受ける前提（claude の per-turn → absolute 積み上げは shim 019 の責務）。集計コード自体は agent 非依存
- `go test ./platform/metrics/ ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §13.5 (Token/runtime accounting), §8.5 Part A (stall via last activity), §10.4 (usage events)
- [plans/04-phases.md#p6](../plans/04-phases.md)、`orchestrator/scheduler/state.go`（`RunAttempt.LastCodex*`/`Total*Tokens`）、`orchestrator/scheduler/reconcile.go`（stall）
