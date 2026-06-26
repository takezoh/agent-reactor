# ADR 0068 — 閲覧モードの keyboard 抑止を helper-textarea focus-block と font-size 16px CSS `!important` で機構化する

Status: Accepted

Related: [ADR 0067](./0067-mobile-gate-matchmedia.md), [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md), [ADR 0073](./0073-arialive-debounce-and-jump-fab-seed-stability.md), [ADR 0075](./0075-pattern-adoption-mode-affordances.md)
Related code: `src/client/web/src/hooks/useInputMode.ts` (new), `src/client/web/src/hooks/useHostPointerInterceptor.ts` (new), `src/client/web/src/css/app.css` (mobile gate scope), `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-MODE-001..007`, `FR-MOB-FAB-PD-001`

## Context

`ux.md` assumption §3 が『閲覧モードで tap がキーボードを出さない load-bearing 機構は helper textarea への focus 移動をブロックすること (readonly は defense-in-depth)』と固定したが、

- **(a)** 実装手段 (pointerdown capture phase `preventDefault` / `inert` / `pointer-events:none` / blur on focus) の確定
- **(b)** focus を奪う全経路 (mousedown synthesized / touchend→`term.focus()` / helper textarea 直接 click) の網羅
- **(c)** iOS focus-zoom 抑止のための helper textarea font-size 16px 強制の機構 (CSS `!important` / JS inline style override / MutationObserver)
- **(d)** outside-tap (UAC-005) の購読と focus-block の listener 統合

— を plan-how で決定する必要があった。UAC-002 / UAC-009 / UAC-010 の load-bearing assertion (focus 発火数 0 / `activeElement` 不変) はこの機構が間違っていれば false positive (通る counterexample 実装) を許す。

## Decision

(1) **`useHostPointerInterceptor` hook 1 個に focus-block と outside-tap を統合**する。host (`terminal-host`) の capture-phase `pointerdown` listener を 1 系統だけ attach する。
- 閲覧モードでは `preventDefault()` で focus 移動を奪う
- 入力モードでは `target.closest('[data-overlay]')` / helper textarea を除外して outside-tap を判定し `useInputMode.exit('outside-tap')` を呼ぶ

(2) focus を奪う全経路 (mousedown synthesized / touchend 後の `term.focus()` / helper textarea 直接 click) は host capture pointerdown が最も上流のため、capture phase の `preventDefault` で一斉に抑止される。helper textarea 直接 click は target が helper textarea のときに限り入力モードへ遷移する経路として許可 (FAB 経由でない exit→re-enter 経路は許容)。

(3) `readonly` 属性は **defense-in-depth** として閲覧モード中のみ helper textarea に付与 (focus 機構が破綻しても無キーボード)。

(4) **iOS focus-zoom 抑止**: mobile gate scope の CSS で `.xterm-helper-textarea { font-size: 16px !important; }` を適用。inline style 上書き / MutationObserver 再注入は xterm 内部との race を生むため不採用。grid 描画は `.xterm-rows` 側で 8-28px clamp、helper textarea は表示に影響しないため 16px 固定で問題なし。

(5) `AriaLiveStatus` への『閲覧モードに戻りました』emit は同一テキスト連続 1.5s デバウンスで抑止 (詳細は [ADR 0073](./0073-arialive-debounce-and-jump-fab-seed-stability.md))。

## Alternatives Considered

### HTML `inert` attribute を helper textarea container に付与

iOS Safari 15.5+ / Chrome 102+ で対応するが (a) 既存 button / FAB の互換性検証コストが高い (b) input modality 切替で `inert` を on/off するパフォーマンス検証が未済 (c) jest-dom / happy-dom の `inert` 対応が弱く ATDD harness で再現困難。将来 fast-follow 候補としては有望。**現時点で却下**。

### `readonly` のみで focus-block しない

iOS は readonly textarea を focus してもキーボードを出さないが、focus イベントは dispatch されるため UAC-002 / UAC-009 の『focus 発火数 0』assertion で fail。チラ見せ counterexample (focus→blur) も通り抜ける。**却下**。

### blur on focus (focus されたら即 blur)

focus イベントが実際に dispatch されるため UAC-002 counterexample がそのまま通る (発火数 0 を満たせない)。**却下**。

### `pointer-events:none` を host に付与

FAB tap / 選択操作も死ぬ (overlay は子なので別、しかし xterm 内部 click も死ぬ)。**却下**。

### JS で helper textarea の `inline style.fontSize` を 16px に直接書き換える

xterm 内部が `term.options.fontSize` 適用時に inline style を再注入するため MutationObserver で追従が必要、race と double-render を生む。**却下**。

### MutationObserver で helper textarea の inline style を監視して 16px に再注入

xterm の内部書き換えと無限 loop の余地 / cost が高い / CSS `!important` で同等以上の効果が単純に得られる。**却下**。

## Consequences

- host `pointerdown` listener が 1 系統に統合され、focus-block と outside-tap の競合バグが構造的に発生しなくなる
- 閲覧モードで tap → focus 発火数 0 が機構的に保証され、UAC-002 / UAC-009 の load-bearing assertion が判別力を持つ
- iOS 17+ で helper textarea focus 時の viewport auto-zoom が CSS `!important` により browser native に抑止される
- `readonly` はあくまで defense-in-depth で、focus-block が破綻しても無キーボードで degrade のみ
- mobile gate scope の CSS で 16px `!important` を当てるため PC 環境では一切影響なし
- long-press 選択 ([ADR 0071](./0071-touch-gesture-arbitration-and-long-press-selection.md)) は capture-phase `preventDefault` の前段で dwell timer を起動するため、focus-block と排他的に設計可能

## Related Requirements

- `FR-MOB-MODE-001` — 閲覧モード初期 invariant (`data-input-active='false'` / `readonly` / `activeElement≠helper`)
- `FR-MOB-MODE-002` — 閲覧モード中の focus 発火数 0 維持 (load-bearing 機構)
- `FR-MOB-MODE-005` — outside-tap 経路 (host capture pointerdown 集約)
- `FR-MOB-MODE-006` — blur / Escape での exit + AriaLive 1 回 emit
- `FR-MOB-MODE-007` — helper textarea font-size 16px 維持 (iOS zoom 抑止)
- `FR-MOB-FAB-PD-001` — FAB pointerdown.preventDefault で `activeElement` 不変
