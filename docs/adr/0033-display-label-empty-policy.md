# ADR 0033 — セッションラベルのフォールバック空判定を undefined / 空文字の両方とし規約統一 (Web UI 問題6)

Status: Superseded by [ADR-0076](0076-session-card-title-subtitle-two-slot.md)

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md)
Related requirements: FR-011, FR-012

## Context

Web UI のセッション一覧でセッションを ID ではなく、driver が付与したタイトル / 要約 / 最終入力プロンプトで表示したい (Web UI 問題6)。arc TUI (`client/driver/claude_view.go` 等) は

```go
Card: state.Card{
  Title:    cs.Title,
  Subtitle: firstNonEmpty(cs.Summary, cs.LastPrompt),
  ...
}
```

として `title` と `subtitle` をすでに載せている。`subtitle` は driver 側 `firstNonEmpty(Summary, LastPrompt)` で生成済み。

ところが `src/client/web/src/components/SessionList.tsx` は

```tsx
<span className="title">{s.view.card.title ?? s.id}</span>
```

と書かれており、`??` 演算子は **空文字を非空扱いする** ため、`title === ""` のカードで空ラベルになるバグがある (plan-how 否定役 / 最適化役)。

加えて `src/client/web/src/components/DriverViewPanel.tsx` は `card.title && / card.subtitle &&` の truthy 判定で空文字を弾いており、**空判定の規約がコンポーネント間で不一致**。

## Decision

`displayLabel(card, id): string` という小さな純関数を定義し:

```ts
function displayLabel(card: Card, id: string): string {
  const t = card.title?.trim();
  if (t) return t;
  const s = card.subtitle?.trim();
  if (s) return s;
  return id;
}
```

`title → subtitle → id` を trim 後の非空 (undefined と空文字の両方を空とみなす) で最初に選ぶ。`SessionList` のラベルをこの関数に置換し、`DriverViewPanel` の truthy 空判定と規約を揃える。

専用ライブラリ (lodash 等) は導入せず、小さな純関数で済ませる。

## Consequences

- 空タイトルカードで空ラベルになる現行バグを解消し、`title` / `subtitle` 双方空のときのみ `id` にフォールバック (FR-011 / FR-012)
- 空判定規約が `SessionList` と `DriverViewPanel` で統一され、将来のカード表示の一貫性が保たれる
- `displayLabel` が純関数のため `title` / `subtitle` / `id` の各分岐を単体テストで網羅でき観測しやすい
- 起動直後に `title` / `subtitle` 未確定で生 `id` が一瞬見える点は arc と挙動一致だが UX 上の是非は follow-up (本 ADR の Open follow-up、Spec の Open Question Q5 で「arc 挙動維持」と決定済み)

## Alternatives Considered

### `title ?? subtitle ?? id` (`??` チェーン)

却下: `??` は空文字を素通りさせ空タイトルで subtitle / id にフォールバックしない。trim 後非空判定が必要。

### `title` / `subtitle` 不在時にプレースホルダ (`"New session…"`) を表示

却下: arc TUI との差異と追加状態管理が生じる。`id` フォールバックが arc と揃い単純。プレースホルダ案は follow-up で再考可能。
