# ADR 0069 — FAB overlay を `terminal-slot` 直下の `.terminal-fab-layer` に集約し、visualViewport-lift は CSS custom property で配信する

Status: Accepted

Related: [ADR 0029](./0029-terminal-host-flex-height.md), [ADR 0063](./0063-notification-toast-primitive.md), [ADR 0064](./0064-reduced-motion-single-guard.md), [ADR 0065](./0065-terminal-slot-absolute-overlay.md), [ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md), [ADR 0073](./0073-arialive-debounce-and-jump-fab-seed-stability.md), [ADR 0075](./0075-pattern-adoption-mode-affordances.md)
Related code: `src/client/web/src/hooks/useVisualViewportLift.ts` (new), `src/client/web/src/components/{KeyboardFAB,JumpToLatestFAB,FontSizeControl,PinchIndicator}.tsx` (new), `src/client/web/src/css/view.css` (`.terminal-fab-layer` rule), `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-FAB-001..004`, `FR-MOB-VVP-001..003`, `FR-MOB-STEPPER-001`, `FR-MOB-PINCH-004`

## Context

[ADR 0029](./0029-terminal-host-flex-height.md) (terminal-host `flex:1 1 0 / dvh`)、[ADR 0065](./0065-terminal-slot-absolute-overlay.md) (terminal-slot absolute overlay)、`FR-LAYOUT-004` (safe-area single-source via `.app-shell`) を壊さず、`KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` / `Coachmark` / `PinchIndicator` の 5 種 overlay を配置する必要がある。

iOS soft keyboard は `dvh` / layout viewport を縮めず overlay するため `visualViewport` API でしか入力行可視を保てない。各 FAB が `visualViewport.resize/scroll` の更新を React state 経由で受けると FAB 数だけ再 render が走り性能と coupling が悪化する。`FontSizeControl` の視覚配置 (常時表示 vs disclosure) も Open Question として残っていた。

## Decision

(1) **`terminal-slot` 直下に新規 `<div class='terminal-fab-layer' data-overlay='true'>` 兄弟**を追加し、全 mobile FAB を absolute overlay の子として配置 (terminal-host の flex sizing は absolute 兄弟により変化しない = ADR 0029 / 0065 維持)。

(2) `env(safe-area-inset-*)` は `.app-shell` が四辺で適用済みのため FAB 側で再加算しない (`FR-LAYOUT-004` 単一情報源を堅持)。terminal-slot 内側の 16px offset で配置。

(3) **固定スタック順**:
- 下端 `KeyboardFAB` (bottom: 16px) → 8px gap → `JumpToLatestFAB` (bottom: 68px)
- `FontSizeControl` は top-right inset 内 16px (別位置で重ならない)

(4) **visualViewport-lift**: `useVisualViewportLift` hook が `.terminal-fab-layer` の inline CSS custom property `--terminal-fab-offset` を `(window.innerHeight - visualViewport.height - visualViewport.offsetTop + 16)px` で更新し、各 FAB は CSS で `bottom: var(--terminal-fab-offset, 16px)` を参照する (**React 再 render 0 / 1→多 fan-out**)。

(5) **listener lifecycle 順序保証**: `visualViewport` listener は入力モード突入時に subscribe し、入力モード退出 + rotation (gate `true→false`) で listener を unsubscribe してから input mode state を破棄する順序を守る。

(6) `visualViewport` 不在環境は CSS custom property の default `16px` が自動 fallback。

(7) **`FontSizeControl` は disclosure popover** (Aa アイコン 1 個 → tap で popover が +/-/Reset 3 ボタンを露出) で配置 — iOS Safari Reader Controls / Kindle iOS の Aa popover / VS Code mobile の font menu の業界慣習に倣う。SR ユーザーが popover を開く 1 タップが余分になるが UAC-020 (role=button / 44×44 / aria-label) は disclosure でも満たせる。

(8) `.notification-toast` layer は別 portal で z-index と DOM 位置を分離し、`PinchIndicator` は Toast primitive を `ariaHidden=true` prop 拡張で再利用 (新規 primitive を作らない)。

## Alternatives Considered

### `terminal-host` の flex 兄弟として FAB を挿入

`flex:1 1 0` の terminal-host が残余を奪われ box が縮む / ADR 0029 / 0065 違反 / UAC-025 fail。**却下**。

### 各 FAB が `inline style.bottom` を React state 経由で書く

`visualViewport` 変化のたびに FAB 数だけ再 render が走る / coupling 大 / 性能劣化。**却下**。

### FAB 側で `env(safe-area-inset-*)` を加算

`FR-LAYOUT-004` (`.app-shell` 単一情報源) 違反 / notched 端末で過剰 inset。**却下**。

### CSS `env(keyboard-inset-height)` のみで対応

Chromium 限定 / Safari 未対応のため iOS で機能しない。**却下**。

### `dvh` / `svh` CSS のみで対応

iOS Safari は soft keyboard で layout viewport を縮めないため機能しない。**却下**。

### `FontSizeControl` 常時表示 (+/-/Reset 3 ボタン縦 stack)

浮遊要素が 5 個になり視覚クラッタ大 / `KeyboardFAB` と並ぶと誤タップ。**却下**。

### `FontSizeControl` をハイブリッド (tap で +1 / long-press で popover)

覚えづらい / iOS Safari の text selection と long-press 衝突。**却下**。

### `PinchIndicator` を新規 primitive として独立実装

Toast primitive と fade / position / reduced-motion が重複実装、保守コスト増。`ariaHidden` prop 拡張で十分。**却下**。

## Consequences

- ADR 0029 (flex height) と ADR 0065 (terminal-slot overlay) が機構的に維持され UAC-025 (terminal-host box 不変) が満たされる
- `visualViewport` 変化時の React 再 render が 0 になり (DOM 1 箇所への CSS custom property 書込のみ)、FAB 追加時の coupling が消える
- iOS soft keyboard 上に FAB が常に到達可能 (CSS 変数経由で sticky toolbar 化)
- `FontSizeControl` disclosure により浮遊要素が 3 個 (`KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` disclosure trigger) に抑制され視覚クラッタを回避
- `PinchIndicator` が Toast primitive 再利用で fade timing / position / reduced-motion 対応を継承
- listener unsubscribe 順序が契約化されて回転時の listener leak / race が構造的に防止される

## Related Requirements

- `FR-MOB-FAB-001` — KeyboardFAB IconButton primitive wrap + a11y 仕様
- `FR-MOB-FAB-002` — FAB 出現/状態変化で terminal-host box 不変
- `FR-MOB-FAB-003` — safe-area 二重計上禁止 + 16px offset 配置
- `FR-MOB-FAB-004` — 固定スタック順 + Toast layer 別 portal
- `FR-MOB-VVP-001` — `visualViewport` 連動 CSS 変数更新
- `FR-MOB-VVP-002` — `visualViewport` 不在で CSS default 自動 fallback
- `FR-MOB-VVP-003` — gate `true→false` / 入力モード退出時の unsubscribe 順序保証
- `FR-MOB-STEPPER-001` — FontSizeControl disclosure popover + +/-/Reset a11y
- `FR-MOB-PINCH-004` — PinchIndicator が Toast primitive 再利用
