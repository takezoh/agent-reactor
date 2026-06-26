# ADR 0071 — touch gesture (swipe / long-press 選択 / pinch) を 1 hook の state machine で arbitration し long-press 選択は xterm 標準 `term.select()` API で実装する

Status: Accepted

Related: [ADR 0034](./0034-refit-raf-coalesce.md), [ADR 0064](./0064-reduced-motion-single-guard.md), [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md), [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md), [ADR 0070](./0070-fontsize-persist-clamp.md)
Related code: `src/client/web/src/hooks/useTerminalTouchGestures.ts` (new), `src/client/web/src/css/view.css` (mobile gate scope `.xterm-viewport { touch-action: pan-y }`), `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-SCROLL-001..003`, `FR-MOB-SELECT-001/002`, `FR-MOB-PINCH-001..004`

## Context

UX Open Question 1 を plan-how で決着させる (**POC 先送り禁止**)。`.xterm-viewport` 上で swipe / long-press / pinch が同じ touch source を共有するため、別々の listener を attach すると順序依存と double-`preventDefault` のバグ温床になる。xterm.js 5.x は viewport 上の `touchstart` / `touchmove` を内部 selection/scroll handler で扱う可能性があり、`preventDefault` 戦略を確定しないと両機能が排他的に動かなくなる。

要件は『標準 API 優先 / addon 追加 0 維持』を `ux.md` assumption §10 で固定。

## Decision

(1) **`useTerminalTouchGestures` hook 1 個に gesture state machine (`idle / swipe / dwell / longpress-drag / pinch`) を実装**し、`.xterm-viewport` への touch listener を 1 系統だけ attach する。listener 数 = 1。

(2) **swipe scroll**: mobile gate 内 CSS で `.xterm-viewport { touch-action: pan-y }` を適用し browser ネイティブ scroll に委ねる (xterm 5.5 default DOM renderer + addon-fit のみで成立、依存追加 0)。

(3) **long-press 選択 (DECISION not POC)**:
- `touchstart` で 500ms dwell timer を起動
- 500ms 内に `touchmove` が threshold 8px を超えなければ dwell 成立
- dwell 成立後の `touchmove` で xterm 5.5.0 標準 `term.select(startCol, startRow, length)` API を programmatic に呼ぶ
- `term.getSelection()` で UAC-010 観察契約を満たす
- 依存追加 0 で完結する

(4) **pinch fontSize**: `touches.length=2` で state machine が pinch に遷移し swipe handler を中断、`touchmove` ごとに `d_now/d_start` 比率を fontSize に反映 (詳細は `FR-MOB-PINCH-001/002/003/004`)。

(5) **`preventDefault` 戦略** — pinch 中の `touchmove` (`touches.length=2`) と dwell 成立後の `touchmove` のみ `preventDefault()` で browser pan-y を奪う。それ以外 (`idle` / `swipe` / dwell 中) は `preventDefault` しないことで xterm 内部 handler と競合しない。

(6) dwell 中の haptic feedback は出さない (iOS native は出るが Android Chrome は出ない、OS 差を許容)。

(7) 複数行選択は xterm 標準挙動に従う。

## Alternatives Considered

### (B) xterm の `registerCharacterJoiner` / Selection API を呼ぶ custom selection-handler addon を自作

`term.select()` 標準 API が xterm 5.5 で公開されており同等の効果が得られる / addon 自作は車輪の再発明 / 保守責任が増える。**却下**。

### (C) `@xterm/addon-canvas` + canvas renderer の selection を使う

依存追加 + renderer 切替の副作用 (新規 ADR 1 件追加) / 最終手段。標準 API で成立するなら不要。**却下**。

### (D) `window.getSelection` (DOM-native text selection) で実装

UAC-010 観察契約は `term.getSelection() 非空` を要求 / `window.getSelection` は xterm 内部 state と独立で assertion が変質する / UAC 修正が必要になり ux 観察契約を変える越権。**却下**。

### `useLongPressSelection` + `usePinchFontSize` の 2 hook で listener を別 attach

同一 touch source の race / double-`preventDefault` バグ温床 / `touches.length` 1→2 遷移の排他制御が分散して保守困難。**却下**。

### 全 `touchmove` で `preventDefault()`

xterm 内部 scroll / selection handler を破壊 / browser pan-y も死んで swipe scroll が成立しない。**却下**。

## Consequences

- 依存追加 0 のまま long-press 選択が成立 (UAC-010 観察契約 `term.getSelection()` 非空 / `.xterm-selection-layer` 描画を satisfy)
- 1 listener / 1 reducer で arbitration されるため double-`preventDefault` race と順序依存 bug が構造的に排除
- swipe と long-press と pinch が同じ touch source で排他動作 (`FR-MOB-PINCH-003` の `touches.length` 遷移で swipe 中断)
- iOS と Android Chrome の OS feedback 差を許容する明示判断を ADR に記録 (将来 haptic 統一要求が来た時に再評価できる)
- 将来 xterm が selection API を変えても hook 1 箇所の修正で済む
- happy-dom 上の `TouchEvent` shim で gesture state machine が pure reducer として unit test 可能

## Related Requirements

- `FR-MOB-SCROLL-001` — `.xterm-viewport` `touch-action:pan-y` の invariant
- `FR-MOB-SCROLL-002` — 縦 swipe で `.xterm-viewport.scrollTop` 追従
- `FR-MOB-SCROLL-003` — 連続 swipe 中の focus 0 / data-input-active='false' 維持
- `FR-MOB-SELECT-001` — 500ms dwell + term.select() programmatic 起動
- `FR-MOB-SELECT-002` — dwell 不在の 200px swipe は scroll のみ
- `FR-MOB-PINCH-001` — touches.length=2 比率追従 + clamp + scheduleFit
- `FR-MOB-PINCH-002` — clamp 下限 8px 張り付け
- `FR-MOB-PINCH-003` — 1→2 finger 遷移で swipe 中断 + 入力モード非遷移
- `FR-MOB-PINCH-004` — PinchIndicator が Toast primitive 再利用 (詳細は [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md))
