# ADR 0032 — `RunStateBadge` の spinner を既存テキスト契約を温存して加法的に追加 (Web UI 問題5)

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md)
Related requirements: FR-009, FR-010

## Context

Web UI のセッション一覧でステータスを spinner で表現したい (Web UI 問題5)。現状の `src/client/web/src/components/RunStateBadge.tsx` は status 文字列 (`running` / `waiting` / `idle` / `stopped` / `pending` / `unknown`) をテキストバッジとして表示する。

既存 `src/client/web/src/components/RunStateBadge.test.tsx` は **全 status について `el.textContent === status` と `aria-label=/status:/` を契約として固定** している。spinner で可視テキストを置換する初稿案は、`SessionList` / `DriverViewPanel` 両 panel の視覚回帰と既存テストの全書き換えを招く (plan-how 否定役 major)。

また active の定義が初稿の FR と Decision で揺れていた (否定役 minor)。

spinner は AGENTS.md の "Library Selection" に従い、新規依存追加は避ける。CPU 負荷の低い CSS `transform` + `@keyframes` で実現可能。

## Decision

可視テキストは `status` のまま維持し、active 状態 (`running` / `waiting`) のときだけ `aria-hidden` な spinner 要素を **加法的に付加** する。spinner は CSS `@keyframes` (`transform: rotate`) で JS タイマー非依存。

```tsx
return (
  <span className={`run-state-badge run-state-${normalized}`} aria-label={`status: ${normalized}`}>
    {isActive && <span className="run-state-spinner" aria-hidden="true" />}
    {normalized}
  </span>
);
```

active の定義は **`running` (進行中) と `waiting` (応答待ち = 能動的に待機)** とし、`idle` / `stopped` / `pending` は静的。

## Consequences

- 既存の `textContent` / `aria-label` 契約を壊さず spinner FR (FR-009 / FR-010) を加法的に満たし、テスト全書き換えを回避
- 観測アサーションが「各 status × spinner 要素の有無」で書け、`SessionList` / `DriverViewPanel` 両呼び出し元の互換が保たれる
- spinner は `aria-hidden` で読み上げず status は `aria-label` で提供しアクセシビリティを担保
- `waiting` を回す「待ちなのに動いて見える」懸念は残るが、arc TUI の能動待機表現との一貫性を優先。`running` のみに絞る案は本 ADR の follow-up として記録 (UX レビューで再考可能)
- CSS `@keyframes` のみで新規 npm 依存ゼロ (AGENTS.md library selection に適合)

## Alternatives Considered

### active 状態でテキストを spinner に置換し status は `aria-label` のみ (初稿方向)

却下: 既存 `textContent === status` 契約を破壊し全 each テスト書き換え + 両 panel の視覚回帰を招く。加法的追加なら回避できる。

### `running` のみ spinner、`waiting` は別表現

却下: 表現が分岐し実装 / テストが増える。arc TUI の能動待機表現に合わせ `running + waiting` を active とする方が一貫。`running` 限定は follow-up で再考可能。

### spinner ライブラリ導入

却下: CSS `@keyframes` で十分。AGENTS.md は追加時の候補比較を要求し本件には過剰。
