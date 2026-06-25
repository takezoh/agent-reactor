# ADR 0054 — Palette cursor identity の単一情報源を selectedToolId とする

Status: Accepted

Related: [spec](../specs/2026-06-25-web-palette-redesign/spec.md), [plan](../specs/2026-06-25-web-palette-redesign/plan.md), [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: FR-004, FR-006, FR-007, FR-008, FR-011, FR-026, FR-030

## Context

現状 palette store の `paramCursor` は toolSelect / paramSelect 双方の cursor index を兼ねている。ADR-0050 (scope 統合) と FR-026 (active 切替で行位置が動いたら cursor を re-anchor) を満たすには cursor identity を index か id のいずれかに正規化する必要がある。index ベースで継続すると view-update 時に同 id を follow できず silent footgun (cursor が意図しない別 tool に飛ぶ) が再発する。id ベースに正規化すると hover follow (有効行 pointermove) と keyboard 移動 (delta) は index 操作で受け、確定時に id を resolve する設計になる。

ADR-0036 (store 純粋性) と矛盾しない範囲で、cursor identity の単一情報源をどこに置くかが本 ADR の判断対象。本 ADR は再設計 1 PR 範囲で確定し、後段の ADR-0055 (submit freeze) や ADR-0056 (slice 分割) と独立に判断できる粒度に保つ。

## Decision

selectedToolId を cursor identity の単一情報源とし、paramCursor は paramSelect phase 専用のフィールドに責務縮約する。toolSelect phase では sortedList から (1) selectedToolId が enabled として存在すればその index、(2) 存在しなければ previous logical index 起点で前方優先の nearest enabled に re-anchor する。setCursor(index) action は presentation 層の hover / keyboard が呼び、内部で sortedList[index].id を selectedToolId に同期する。moveCursor(delta) も同様に内部で同期する。

cursor 再計算は pure helper `resolveCursorBySelectedToolId(prevSelectedId, prevLogicalIndex, sortedList, enabledOnlyIndexSet)` に閉じる。hover follow / keyboard delta / view-update の 3 経路は同じ resolve 関数を経由する。

## Consequences

- positive: active 切替や fuzzy rank 変化で cursor が同 id を追従でき、silent footgun (cursor が意図しない別 tool に飛ぶ) を回避できる
- positive: hover follow / keyboard delta / view-update 再計算が同じ resolve 関数 (resolveCursorBySelectedToolId) を経由するため、test が 1 pure helper で網羅できる
- positive: FR-030 (Enter target は cursor が指す tool) が selectedToolId 起点で 1 判定点に集約される
- negative: store の API が setCursor + moveCursor の 2 系統に増え、内部実装で selectedToolId 同期を忘れると stale state を起こす (defense in depth として setCursor 内部で id 同期を強制)
- neutral: paramCursor は paramSelect 専用に縮約され、責務境界が明確になる

## Alternatives Considered

### index ベース継続 (現状維持) + view-update 時に prev index を clamp

active 切替で sortedList 順序が変わると別 tool に cursor が飛ぶ。FR-026 を満たせない。

### id + index 二重管理 (両方を store に持つ)

同期コードが presentation / store 両側に必要で stale state リスク。test 表面積が倍増する。

### store action を moveCursor(delta) のみに留め presentation 層で id 解決

presentation 層に cursor identity ロジックが漏れ、複数 component から重複呼び出しされる。pure helper を持ちつつ呼び出し側で同期を取る形になり、二重実装のリスクが残る。
