# ADR 0065 — terminal-slot を log panel と分離した absolute オーバーレイ層にする

Status: Proposed

Related: [ADR 0029](./0029-terminal-host-flex-height.md), [ADR 0030](./0030-terminal-keyed-remount.md), [ADR 0034](./0034-refit-raf-coalesce-and-test-infra.md), [ADR 0061](./0061-apg-tabs-manual-activation.md)
Related code: `src/client/web/src/components/MainTabs.tsx`, `src/client/web/src/components/LogTabs.tsx`, `src/client/web/src/css/view.css`

## Context

ADR-0029 で `.terminal-host` を `flex: 1 1 0; min-height: 0` の flex child にして残余高さを取らせ、ADR-0030 で TerminalPane は `<TerminalPane key={activeSessionID}>` の keyed remount で subscribe 所有権を保持し、ADR-0034 で ResizeObserver + rAF coalesce で `fit()` を発火する、という積み重ねで terminal レイアウトを成立させてきた。

その後 `feat: add TERMINAL tab to make Terminal/Transcript/Events exclusive` (`6a60a79`) で MainTabs に synthetic TERMINAL タブが入り、`.main-tabs-body` (flex column) の中に terminal-slot と log panel (TRANSCRIPT / EVENTS) を **flex 兄弟** として並べた。当初 inactive 側の terminal-slot は `display: none` で完全に潰してあったが、UI 全面刷新 (`5ca0c41`) で `visibility: hidden + height: 0 + overflow: hidden` 方式に変えた際、**`flex: 1 1 0` の指定が inactive 側にも残った**。

`flex-direction: column` の flex item では `flex-basis: 0` が `height: 0` を上書きするため、inactive な terminal-slot が `flex-grow: 1` で残余の半分を確保し、active な log panel (TRANSCRIPT / EVENTS) は残り半分しか表示できない。結果として log content が下半分に押し込まれ、画面では「コンテンツが縦中央に寄っている」regression に見えた。

加えて、MainTabs の panel wrapper (`<div role="tabpanel">`) の内側で `LogTabs.tsx::ContentArea` も `<div className="log-tab-content" role="tabpanel">` を返しており、1 つの tab に対して tabpanel role が 2 階層ネストする ARIA 違反が同居していた。

否定役の指摘: 「inactive 側の `flex` を `0 0 0` に書き換える CSS 修正は対症療法。terminal-slot と log panel を flex 兄弟に置く構造自体が "誰が flex 残余を取るか" の矛盾を生んでいる」。

## Decision

(1) `.terminal-slot` を `.main-tabs-body` の **absolute オーバーレイ層** にする。`.main-tabs-body` を `position: relative` のステージにし、`.terminal-slot` は `position: absolute; inset: 0;` で親 box 全体を常時占有する。log panel は通常フローの flex child のまま active 1 個だけ `display: flex` で表示する。

(2) active 切替は class 名 (`tab-panel--active`) ではなく **`data-active="true"/"false"` 属性** で表現する。`view.css` で `.terminal-slot[data-active="false"] { visibility: hidden; pointer-events: none; }` を適用する。`aria-hidden` も同値で同期する。`.tab-panel--terminal` modifier は廃止する。

(3) terminal-slot は親 `.main-tabs-body` と常に同じ box サイズを持つので、log panel の表示状態に依存せず `.terminal-host` の高さが確定する。ResizeObserver は親リサイズだけを観測すれば `fit()` が常に正しい寸法で走るため、tab 切替に連動した能動 fit() 発火は不要 (ADR-0034 の rAF coalesce 経路をそのまま使う)。

(4) ARIA 二重 tabpanel を解消する。`ContentArea` から `role="tabpanel"` を外して純粋なスクロールコンテナにし、tabpanel role は親 wrapper (MainTabs の panel wrapper、もしくは LogTabs 単独使用時の `<div className="log-tab-content" role="tabpanel">`) が単独で所有する。

(5) React 18.3.1 のため `inert` 属性は使わず、`visibility: hidden` + `pointer-events: none` + `aria-hidden="true"` の 3 点で inert 相当の動作を再現する。

## Consequences

- positive: log content が flex 残余の半分に押し込められる構造的可能性が消える (「縦中央寄せに見える」regression の再発防止)
- positive: terminal-host の box サイズが log panel 状態と独立 → `fit()` 経路の前提条件が単純化し、ADR-0029/0030/0034 の関係が clearer になる
- positive: ARIA tabpanel が tab : tabpanel = 1 : 1 になり、AT (screen reader / axe-core) の解釈が安定する
- positive: `tab-panel--active` modifier の二重管理 (`hidden` 属性と `display:flex` 切替) のうち、terminal-slot 側を `data-active` 属性に一本化したため、状態フローを 1 箇所で読める
- negative: 既存 MainTabs テスト (`tab-panel--active` クラスを検査していたもの) を `data-active` 属性検査に書き換える必要がある (済)
- negative: terminal-slot は通常フローから外れる → `.main-tabs-body` のサイズが log panel の min-content で決まるパターンでは「terminal-host が log panel 高さに追従する」挙動になる。`.main-tabs-body` 自身が `flex: 1 1 0` で親 (`.main-tabs`) の残余を取るので実害はないが、将来 `.main-tabs-body` を別 layout に置き換える際は注意が必要

## Alternatives Considered

### CSS hotfix: `.tab-panel--terminal` の inactive 時を `flex: 0 0 0` にする

最小修正 (3 行 + 回帰テスト 1 件) で症状は止まる。しかし terminal-slot と log panel が flex 残余を取り合う構造自体は残る。`flex: 0 0 0` で「片方は領域を取らないことにする」のは矛盾を CSS で塗り潰すだけで、将来 `.main-tabs-body` の layout を触る人が `flex` を戻すと regression が再発する。否定役の指摘どおり対症療法。

### terminal-slot を `display: none` に戻す (`6a60a79` の初期実装)

flex 残余を確実に潰せる。ただし `display: none` ↔ `display: flex` 遷移で xterm host の box が 0 から復活するタイミングで ResizeObserver が新サイズを捉えるまで 1 フレームの "0-size 測定" 経路が走る (ADR-0034 が rAF で coalesce するので実害は限定的だが、構造的には少し神経質)。本決定 (absolute オーバーレイ) は box サイズが常に親と等しいまま `visibility` だけ切れるので、この経路自体が消える。

### tab ごとに別 `<Subscription>` を持たせて TerminalPane を完全に lazy mount にする

ADR-0030 の subscribe 所有権モデルを根本から書き換える必要があり、scope が広すぎる。本 ADR は「既存の subscribe 所有権を維持したまま、レイアウトの矛盾だけを解消する」最小不変式の変更に留める。
