# ADR 0075 — tmux mode 分離 + Termius/a-Shell キーボード toggle + Slack jump-to-latest + Material FAB + iOS sticky toolbar + WAI-ARIA live region パターンを採用し `IconButton` primitive で a11y 仕様を集約する

Status: Accepted

Related: [ADR 0057](./0057-palette-single-aria-live.md), [ADR 0059](./0059-theme.md), [ADR 0067](./0067-mobile-gate-matchmedia.md), [ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md), [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md), [ADR 0073](./0073-arialive-debounce-and-jump-fab-seed-stability.md)
Related code: `src/client/web/src/components/IconButton.tsx` (new primitive), `src/client/web/src/components/{KeyboardFAB,JumpToLatestFAB,FontSizeControl}.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-FAB-001..004`, `FR-MOB-STEPPER-001`, `FR-MOB-VVP-001`, `FR-MOB-MODE-006`, `FR-MOB-JUMP-004`
Related ux: [Web Terminal Mobile UX ux.md](../specs/web-terminal-mobile-ux/ux.md) (`reference_ux[]`)

## Context

`ux.md` `reference_ux[].stance:modeled_on` で **6 つの業界パターン** (tmux copy/insert mode separation / Termius・a-Shell keyboard toggle / Slack・Telegram jump-to-latest FAB / Material Design FAB primitive / iOS `inputAccessoryView` via `visualViewport` / WAI-ARIA live region polite status) が採用候補として指定された。`reference_ux[].stance:rejected` で **3 つの代替** (desktop xterm `tap=focus` 全環境継承 / 専用モバイルビュー / OS browser zoom 委譲) が却下理由付きで指定された。

各 FAB component を独立実装すると `aria-pressed` / 44×44 / focus-block / theme token が重複実装される。本 ADR でパターン採用の根拠と取り込む aspect を 1 箇所に集約する。

## Decision

(1) **tmux copy/insert mode separation** を採用 — 閲覧モード既定 + 入力モードは明示遷移 + 閲覧中の scroll が入力に吸われない (F-001 〜 F-003 の中核 metaphor)。

(2) **Termius / a-Shell keyboard toggle button** を採用 — `aria-pressed` トグル + 専用ボタンでの明示制御 + 表示領域半減をユーザーが選ぶ (F-001 step 4 〜 7)。

(3) **Slack / Telegram jump-to-latest FAB** を採用 — 末尾離脱時のみ表示 + 到達後消滅 + overlay 配置 (F-004 全体)。

(4) **Material Design FAB primitive** を採用 — absolute overlay + 44px + 固定スタック順 (F-007)。

(5) **iOS `inputAccessoryView` via `visualViewport`** を採用 — CSS 変数経由で sticky toolbar 化 (F-001 step 9, `FR-MOB-VVP-001`)。

(6) **WAI-ARIA live region polite status** を採用 — 非ジェスチャ起点 mode 変化 (blur → 閲覧復帰 / `JumpToLatestFAB` 出現) を polite に通知 (UAC-006, UAC-013)。

(7) **`IconButton` (もしくは `FabButton`) primitive を 1 個抽出**し、`KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` trigger / `FontSizeControl` 内 +/-/Reset の 4 用途が薄く wrap。`aria-pressed` / `aria-label` / 44×44 size / `pointerdown.preventDefault()` による focus 奪取抑止 / theme token (`--accent` / `--surface-*`) を primitive 内に閉じ込める (既存 `SessionDrawer` close / `CommandSearchTrigger` のスタイル言語に揃える)。

(8) `FontSizeControl` disclosure popover は iOS Safari Reader Controls (Aa popover) / Kindle iOS (Aa popover) / VS Code mobile (font menu) の業界慣習に倣う。

## Alternatives Considered

### desktop xterm `tap=focus` を全環境で継承

`ux.md` `reference_ux: rejected` 明記 / モバイル主ユース『scrollback 確認』を破壊 / 表示領域半減 (PC では従来どおり採用、不採用はモバイル gate 内に限定)。**却下**。

### 専用モバイルビュー (要件方針 c, 別レイアウト全置換)

`ux.md` `reference_ux: rejected` 明記 / 非ゴール / 既存 `TerminalPane` / `terminal-slot` 構造を保ったまま overlay + `touch-action` で実現。**却下**。

### OS browser zoom (viewport scale) に委譲

`ux.md` `reference_ux: rejected` 明記 / xterm グリッド refit されず文字ぼやけ / scrollback 行折り返しが狂う / `term.options.fontSize` + refit でグリッドを保つ。**却下**。

### `KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` trigger を 3 独立 component で実装

`aria-pressed` / 44px / focus-block / theme token の重複実装 / a11y 仕様が分散して regression 検出困難 / `IconButton` primitive で集約すれば 20-30 行ずつに縮小。**却下**。

### `FontSizeControl` 常時表示 (3 ボタン縦 stack)

業界慣習 (iOS Reader / Kindle / VS Code mobile) は disclosure popover / 視覚クラッタ大 / 浮遊要素 5 個。**却下**。

## Consequences

- 既存業界慣習に整合し学習コスト最小 (tmux user / Termius user / Slack user が体験を transfer 可能)
- `IconButton` primitive で a11y 仕様 (44px / `aria-pressed` / `aria-label` / `pointerdown.preventDefault`) が 1 箇所に集約され `FR-A11Y-001` / `FR-MOB-FAB-001` の regression が primitive のテストで担保
- 各 FAB component が 20-30 行に縮小し AGENTS.md の関数 80 行制約に余裕
- 浮遊要素が disclosure (`FontSizeControl`) により 3 個に抑制 (`KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` disclosure trigger) + Coachmark 1 個 (一時)
- WAI-ARIA live region pattern を採用したことで [ADR 0057](./0057-palette-single-aria-live.md) (palette single `aria-live`) と role 分離で共存

## Related Requirements

- `FR-MOB-FAB-001` — KeyboardFAB IconButton primitive wrap + a11y
- `FR-MOB-FAB-002` — terminal-host box 不変
- `FR-MOB-FAB-003` — safe-area 二重計上禁止
- `FR-MOB-FAB-004` — 固定スタック順 + Toast 別 portal
- `FR-MOB-STEPPER-001` — FontSizeControl disclosure popover + +/-/Reset a11y
- `FR-MOB-VVP-001` — visualViewport 連動 sticky toolbar (CSS 変数)
- `FR-MOB-MODE-006` — AriaLive polite emit
- `FR-MOB-JUMP-004` — JumpToLatestFAB 出現 polite emit
