# ADR 0073 — `AriaLiveStatus` は TerminalPane local single slot + 同一テキスト 1.5s デバウンスで連続抑止し、`JumpToLatestFAB` は ADR 0066 seed flush 完了まで suppress する

Status: Accepted

Related: [ADR 0057](./0057-palette-single-aria-live.md), [ADR 0064](./0064-reduced-motion-single-guard.md), [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md), [ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md), [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md)
Related code: `src/client/web/src/hooks/useAnnouncer.ts` (new + Context), `src/client/web/src/hooks/useJumpToLatest.ts` (new), `src/client/web/src/components/{AriaLiveStatus,JumpToLatestFAB}.tsx` (new), `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-MODE-006`, `FR-MOB-JUMP-001/002/004/005`

## Context

`FR-MOB-MODE-006` と `FR-MOB-JUMP-004` は『1 回 setText』を契約化したが、慣性 scroll / kinetic swipe で `JumpToLatestFAB` が mount / unmount を繰り返し `aria-live polite` 連続 emit が SR ユーザーで ear-fatigue を起こす。

[ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md) (tmux-style scrollback seed) で server-side VT buffer の 2 段 seed が完了する前後で `scrollHeight` が動的に変化するため、seed 完了前の `scrollTop=0` を末尾不在と誤判定して FAB が即出現 → seed 完了で末尾追従して即 unmount / remount する『**late-join 初期 FAB ちらつき**』が起きる。

`AriaLiveStatus` の所有 (TerminalPane local vs App-level `useAnnouncer`) と [ADR 0057](./0057-palette-single-aria-live.md) (palette single `aria-live`) の関係も未確定だった。

## Decision

(1) **`AriaLiveStatus` は TerminalPane 内の visually-hidden `<div aria-live='polite'>` 1 個** (terminal-slot 直下) を提供する。子コンポーネントは Context もしくは ref forwarding 経由で `setText` API にアクセスする。App-level `useAnnouncer` への昇格は将来 fast-follow 余地としてのみ note し、本タスクでは TerminalPane local で完結 (scope 拡大しない)。

- [ADR 0057](./0057-palette-single-aria-live.md) (palette single aria-live) とは **role 分離** — palette は palette 開閉、terminal は mode 変化通知。両者の同時 emit は現状アーキテクチャ上発生しない。

(2) emit は **同一テキスト連続 1.5s デバウンスで抑止** — hook 内に `last-text` + `last-emit-ts` を保持し、同一テキストの重複 emit は 1.5s 経過まで no-op。異なるテキストは即時 emit。

(3) **`JumpToLatestFAB` の seed-gating**: TerminalPane は [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md) の 2 段 seed frame 完了タイミング (server-side VT buffer 適用後の write callback) を受け取り、seed 完了 signal を `useJumpToLatest` に伝播する。
- seed 完了 signal が立つまでは `shouldShowFab=false` 強制で suppress
- seed 完了後の初回 scroll イベントが届くまでも `shouldShowFab=false` (追従中 flag)

(4) **末尾判定 ±2px** は assumption: Retina (DPR=2 / 3) で sub-pixel `scrollTop` が出ても整数 px に丸めた誤差を吸収できる margin。Browser zoom 110-150% 等での fluctuation が出たら margin 増を再評価する条件で採用 (実機チェックリストで検証)。

## Alternatives Considered

### `AriaLive` emit にデバウンスなし

慣性 scroll で同一テキストが連続発火し SR で『最新へ移動できます…最新へ移動できます…』が止まらず ear-fatigue。**却下**。

### App-level `useAnnouncer` ([ADR 0057](./0057-palette-single-aria-live.md) 拡張) を本タスクで採用

scope 拡大 / 本タスクは TerminalPane local で完結可能 / palette と terminal の同時 emit は現状アーキテクチャでは発生しない / fast-follow で昇格可能。**現時点で却下**。

### `JumpToLatestFAB` は scroll イベントで即時 mount / unmount (seed 完了無視)

late-join 初期に FAB がちらつき (visible regression) / SR ユーザーで `aria-live` が連続発火。**却下**。

### 末尾判定 ±0px (厳密一致)

sub-pixel `scrollTop` で FAB がチラつく / DPR=2 環境で再現。**却下**。

### 末尾判定 ±10px などの広い margin

scrollback 中 (10px 未満) でも FAB が出ず UAC-013 (300px 戻し) と整合性が崩れる可能性 / 2px が最小実用 margin。**却下**。

## Consequences

- SR ユーザーの ear-fatigue (同一テキスト連発) が機構的に防止される
- late-join 初期の `JumpToLatestFAB` ちらつき (seed flush race) が seed 完了 gate で構造的に排除
- [ADR 0057](./0057-palette-single-aria-live.md) (palette single `aria-live`) と TerminalPane `aria-live` が role 分離で共存
- 末尾判定 ±2px の検証根拠 (DPR / zoom levels) が ADR で明示され実機検証時の比較対象が明確
- App-level `useAnnouncer` 移行余地が ADR に note されており fast-follow を将来検討可能
- [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md) (scrollback seed) との境界面が明示され、seed flush signal の伝播経路が contract 化される

## Related Requirements

- `FR-MOB-MODE-006` — blur / Escape で『閲覧モードに戻りました』を debounce 経由 1 回 emit
- `FR-MOB-JUMP-001` — 末尾 ±2px で FAB DOM 不在 (条件 render)
- `FR-MOB-JUMP-002` — 末尾離脱で FAB 出現 + 44×44 / aria-label
- `FR-MOB-JUMP-004` — FAB 初出現時に 1 回 emit (debounce 重複抑止)
- `FR-MOB-JUMP-005` — ADR 0066 seed flush 完了まで FAB 強制 DOM 不在
