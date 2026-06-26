# ADR 0074 — PC 専用 `TerminalPane.tsx` を PC + モバイル両対応へ refactor する移行戦略 (gate 内に閉じ込め + PC baseline test で機構的 rollback)

Status: Accepted

Related: [ADR 0029](./0029-terminal-host-flex-height.md), [ADR 0030](./0030-keyed-remount.md), [ADR 0034](./0034-refit-raf-coalesce.md), [ADR 0059](./0059-theme.md), [ADR 0063](./0063-notification-toast-primitive.md), [ADR 0064](./0064-reduced-motion-single-guard.md), [ADR 0065](./0065-terminal-slot-absolute-overlay.md), [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md), [ADR 0067](./0067-mobile-gate-matchmedia.md)
Related code: `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-PC-PRESERVE-001/002/003`, `FR-MOB-GATE-001/002`
Related ux: [Web Terminal Mobile UX ux.md](../specs/web-terminal-mobile-ux/ux.md) (`legacy_context.source_implementation` / `replaced_behaviors` / `inherited_behaviors`)

## Context

`ux.md` `legacy_context.source_implementation` で『`src/client/web/src/components/TerminalPane.tsx` (163 行, xterm.js 5.5.0 + `@xterm/addon-fit` のみ)』が現状 PC 専用 UX として固定された。`replaced_behaviors[]` に **7 件**の置換項目 (focus-block / pan-y swipe / long-press 選択 / FAB only enter / 4 経路 exit / mode 非依存↓最新 / pinch+stepper + clamp + persist)、`inherited_behaviors[]` に **8 件**の維持項目 (click=focus / `onData,onResize` / base64 write / scrollback seed / `scheduleFit` / keyed remount / theme / overlay) が指定された。

PC 経路の『1 bit も変えない』を aspirational から machine-checkable に昇格させる必要があり、rollback 経路も明示する必要がある。

## Decision

(1) **`TerminalPane.tsx` は orchestrator として残し**、新規 9 hook (`useMobileGate` / `useInputMode` / `useHostPointerInterceptor` / `useTerminalTouchGestures` / `useFontSize` / `useJumpToLatest` / `useVisualViewportLift` / `useCoachmarkOnce` / `usePersistedValue`) と 5 overlay 子コンポーネント (`KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` / `Coachmark` / `AriaLiveStatus`) + 共通 `IconButton` primitive を `useMobileGate` true 分岐で mount する。

(2) **全モバイル経路は `useMobileGate` の真偽で early-return する純粋条件分岐**とし、CSS の global `@media` による振る舞い変更は導入しない (CSS `@media` は `touch-action` 等の宣言用補助のみ、要素の存在/不在は JS gate が決定)。

(3) **PC behavior baseline test を最初の chunk (chunk-01) で導入** — 既存 `TerminalPane.tsx` の mount / click→focus→input / wheel scroll / 選択コピー / `data-*` 属性なし / `readonly` なし を snapshot + 関数 assertion で固定し、**以後の全 PR で CI 必須**にして『1 bit も変えない』を machine-checkable に昇格。

(4) `replaced_behaviors[]` の 7 項目は全て gate true scope に閉じ込め、`inherited_behaviors[]` の 8 項目は gate false で 100% 通り抜ける (条件分岐により inherited path が unchanged)。

(5) **rollback 経路**: モバイル経路が壊れたら `useMobileGate` を `() => false` で stub するだけで PC path に degrade (機構的 rollback)、CSS 変更も `.xterm-viewport { touch-action: pan-y }` を mobile gate scope に閉じておけば PC は無影響。

(6) `TerminalPane.tsx` は AGENTS.md の 500 行制約 (関数 80 行) に収まるよう orchestration に責務を絞り、touch / pinch / gate / fontSize / fab / coachmark を独立 hook へ分離。

## Alternatives Considered

### 別 `TerminalPaneMobile.tsx` を新設して mobile 経路を分離

`ux.md` `reference_ux`『専用モバイルビュー: rejected』に違反 / subscribe 所有権 ([ADR 0030](./0030-keyed-remount.md) keyed remount) が壊れる / コードベース二重化。**却下**。

### feature flag (環境変数 / ConfigContext) で mobile 経路を有効化

`useMobileGate` 自体が flag の役割を担う / 二重で過剰。**却下**。

### PC 経路も同 reducer に統合 (mode = pc / mobile-viewing / mobile-input)

PC 1 bit 変更禁止違反のリスク / PC ユーザーが意図せず mobile 経路の bug fix 影響を受ける。**却下**。

### baseline test なしで code review のみで PC 維持を担保

aspirational に留まり気づかない regression を放置 / CI gate でないため後段で気づく。**却下**。

## Consequences

- PC regression が baseline test (chunk-01) で機構的に検出可能、aspirational から CI 必須へ昇格
- モバイル経路の rollback が `useMobileGate` の 1 箇所 stub で完結
- `inherited_behaviors[]` 8 項目が ADR で audit 可能に label 付けられ将来 PC 経路改修時の影響範囲が明示される
- `TerminalPane.tsx` の責務が orchestration に絞られ 500 行 / 関数 80 行制約が守られる
- 別 `TerminalPaneMobile.tsx` を新設しないため subscribe 所有権 ([ADR 0030](./0030-keyed-remount.md) keyed remount) と整合

## Related Requirements

- `FR-PC-PRESERVE-001` — gate false で overlay 全不在 / `data-input-active` 不在 / `readonly` 不在
- `FR-PC-PRESERVE-002` — 700px + pointer:fine narrow-window が PC として動作
- `FR-PC-PRESERVE-003` — gate false で touch-action / wheel scroll の legacy 維持
- `FR-MOB-GATE-001` — matchMedia AND 契約での gate true 判定 + 条件 render
- `FR-MOB-GATE-002` — gate true→false 遷移時の state 破棄順序保証
