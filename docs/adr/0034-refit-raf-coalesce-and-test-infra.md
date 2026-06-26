# ADR 0034 — Refit を rAF コアレスへ集約し happy-dom 用テスト基盤を整備 (Web UI 問題4)

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md), [ADR 0029](./0029-terminal-host-flex-height.md)
Related requirements: FR-006, FR-007, FR-008

## Context

Web UI 問題4 (ターミナルがウィンドウ / レイアウトにフィットしない) の従属対策として、`src/client/web/src/components/TerminalPane.tsx` は

- 初回 mount 時の `fit.fit()`
- `window.addEventListener("resize", handleResize)` での `fit.fit()`

を持つが、`terminal-host` への `ResizeObserver` が無く兄弟パネル (`DriverViewPanel` / `LogTabs`) の出現消滅で host 内部サイズが変わっても refit されない。

`ResizeObserver` の素発火で毎回 fit するとドラッグリサイズ中に高頻度 reflow を招き、xterm の renderer が連続再計算でスラッシュする恐れがある (NFR-005)。一方 rAF / debounce を挟むと fit が非同期になりテストで時間依存になる (plan-how 否定役 minor)。

さらに実環境は `happy-dom` で

- `ResizeObserver` 未実装 (`new ResizeObserver` で `ReferenceError`)
- `test-setup.ts` に rAF / `ResizeObserver` の mock が無い

ため、現状のまま `new ResizeObserver(...)` を呼ぶと `TerminalPane.test` の既存ケースすら `ReferenceError` で落ちる (plan-how 否定役 major)。

## Decision

初回 fit と `ResizeObserver` / `window resize` 由来の refit を **単一の `scheduleFit()` (`requestAnimationFrame` で 1 フレーム 1 回に集約)** へ統一する。

```ts
let pending = false;
function scheduleFit() {
  if (pending) return;
  pending = true;
  requestAnimationFrame(() => {
    pending = false;
    fit.fit();
  });
}
```

`src/client/web/src/test-setup.ts` に以下を追加し、refit の観測アサーションを時間非依存にする:

1. `ResizeObserver` の mock (`observe(target, cb)` / `disconnect()` とコールバック手動発火フック `__triggerResize(target, entries)`)
2. `requestAnimationFrame` を同期実行 (コールバック即時 flush) する mock

本 ADR は CSS による高さ確定 ([ADR 0029](./0029-terminal-host-flex-height.md)) の **従属対策** であり、CSS 修正と併せて初めて FR-008 (0 サイズ回避) が成立する。

## Consequences

- fit 呼び出しが初回 / refit の二系統から 1 経路に集約され分岐が減る (FR-006 / FR-007 / FR-008 を同一経路で満たす)
- rAF コアレスでドラッグ中の reflow 過多を抑えつつ追従の即時性を保つ (NFR-005 スラッシュしない)
- test-setup の rAF 同期 flush により「observer コールバック発火 → fit 呼出」を flaky なく検証できる
- happy-dom に欠落する `ResizeObserver` を mock することで `TerminalPane` の既存テストの `ReferenceError` を防ぐ
- `scheduleFit` は flex 高さ確定 ([ADR 0029](./0029-terminal-host-flex-height.md)) が前提で、CSS 修正と併せて初めて 0 サイズ回避が成立する

## Alternatives Considered

### 素の `ResizeObserver` コールバックで毎回 `fit`

却下: ドラッグリサイズ中に高頻度 reflow を招き NFR-005 (スラッシュしない) に反する。rAF コアレスで集約する。

### 時間 debounce (例 50ms) で `fit`

却下: 追従にラグが出て、テストで fake timer の前進が要り検証が複雑。rAF コアレスの方が追従即時かつ同期 flush で検証容易。

### `ResizeObserver` を使わず `window resize` のみに留める

却下: 兄弟パネル出現消滅による host 内部サイズ変化を `window resize` は捕捉できず FR-007 を満たせない。
