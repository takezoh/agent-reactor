# ADR 0056 — palette store を zustand StateCreator slice パターンで分割する

Status: Accepted

Related: [spec](../specs/2026-06-25-web-palette-redesign/spec.md), [plan](../specs/2026-06-25-web-palette-redesign/plan.md), [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: FR-009, FR-010, FR-012, FR-031, FR-032

## Context

現状 palette.ts は 489 行で 500 行制約に余裕 11 行。本 PR で active_context / inline_status / freeze 関連 state と action を追加すると確実に超過する。zustand の StateCreator slice パターンを採用すれば、複数ファイルに分割しつつ単一 store を維持できる。

一方で分割数を増やすと slice 間 (active_context が submitting flag を参照する等) で cross-cutting な dependency が生まれ、過剰分割は循環依存を招く。500 行制約 (project default) と高凝集 (slice 単位で test できる) の両立を考えた slice 数と境界を確定する必要がある。

## Decision

palette store を以下の 4 ファイル構成にする:

1. `palette.ts` (composition root): phase / query / paramCursor / submit lifecycle / scope 削除後の base state を持つ。reducer / derive ロジックは置かず、純粋に set/get の合成のみとする invariant を維持する。
2. `palette_active_context.ts` (slice): ActiveContextSnapshot 型 + flashSeq + announceSeq + deriveActiveContext pure 関数 + projBase pure helper を持つ。
3. `palette_inline_status.ts` (slice): message + seq + kind + 4s timer state + emitDisabledFeedback action を持つ。
4. `palette_freeze.ts` (slice): submitStartedAt 等 freeze 補助 state のみ。frozenSnapshot 本体は CommandPalette useRef に置く (ADR-0055)。

各 slice は `StateCreator<RootState, [], [], SliceState>` 型で書き、palette.ts の create() が slice closure を合成する。slice 間相互参照は `(...a, ...b)` 順序で composition し、後段 slice が先段 slice の get() を参照する形に限定する (循環依存禁止)。

## Consequences

- positive: palette.ts が 250 行以下に縮減し、500 行制約に余裕ができる
- positive: slice 単位で unit test が書け、active_context flash / inline_status seq / freeze の transient state を独立検証できる
- positive: 将来 slice 追加 (例: param values の history) で 4 つ目以降の slice を足すパターンが確立される
- negative: StateCreator パターンの import boilerplate が増え、初見の reader には slice 合成順序の理解が必要になる
- neutral: zustand devtools / persist middleware は composition root 1 箇所で噛ませるため、middleware 適用は不変

## Alternatives Considered

### palette.ts を分割せず行数制約を回避するため lint exclusion を申請

500 行制約は code-enforcement の中核ルール。例外申請は本 PR の趣旨と無関係なノイズを発生させる。

### 2 slice (active_context + inline_status) のみで freeze は palette.ts 内に inline

freeze 関連 state が将来 grow する想定 (submitStartedAt / retry count 等) があり、最初から独立 slice として用意する方が境界が安定。

### 4 つ以上 (例えば param input slice も独立) に分割

本 PR の diff が増え、レビュー対象が散る。phase 関連は composition root に残し、cross-cutting でない state のみ slice 化する境界が明確。
