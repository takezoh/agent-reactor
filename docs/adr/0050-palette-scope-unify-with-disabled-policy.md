# ADR 0050 — コマンドパレットを 1 listbox に統合し disabled tool を visible + skip-navigation + group-sort で扱う

Status: Accepted

Related: [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: F-002, F-007 (UAC-004〜UAC-005, UAC-015〜UAC-016)

## Context

commit 9287c7f は CommandPalette UI に `ScopeSegment.tsx` を導入し、standard / push の 2 tab で tool を分離した。これは arc TUI 版の操作モデルを直訳したもので、TUI では画面幅と縦スクロールの制約から tab 分割が合理的だった。しかし Web の command palette UX 標準 (Raycast / Linear / VSCode Quick Pick) は 1 list 混在 + fuzzy filter による横断検索で、tab 分割は無い。

本リポジトリの現状では active session が未選択のとき push tab 全体が空になり、ユーザーには『push が押せない』という事実は分かっても『なぜ押せないか』の理由 (No active session) が見えない。disabled の理由 (`disabledReason`) は ADR-0047 で `scopeDisabledReason()` という single source として確立済みだが、その値が UI に露出していない。さらに tab 内が空であることが loading / context 失敗 / 単なる empty の区別がつかず、silent failure に近い。

新規 UX 再設計 (本 spec) では『scope 統合』『disabled visible』『skip navigation』『group sort』『inline 不可フィードバック』が一つの整合した方針として現れる。これらを別 ADR に分割すると相互参照が増え方針反転コストが膨らむため、本 ADR で統合的に決定する。なお spec.md / plan.md は本 ADR と並行する後続 plan-how フェーズで生成される。

## Decision

(1) `ScopeSegment.tsx` を撤去し、ToolSelectPhase の listbox に全 tool (standard + push) を 1 list として統合描画する。`paletteScope` 状態 / `setScope` action / tab 用 keybinding (Shift+Tab) はすべて削除する。

(2) 並び順は『有効グループ → separator (視覚的横線, role='separator', aria-orientation='horizontal') → disabled グループ』の固定 2 段構成とする。グループ内は registry 順 (現行 `listTools` の declarative 順) を保持する。recently used / 使用頻度に基づく動的 sort は本 ADR スコープ外 (open_questions の将来 ADR 候補)。

(3) disabled 行は warning icon (`!` triangular) + 行末 secondary text として `disabledReason` (ADR-0047 single source、加工せず文字列をそのまま埋め込む) を表示する。色だけでなく icon + secondary text で disabled 状態を伝える (WCAG 1.4.1)。

(4) keyboard ↑↓ は disabled 行を skip し、有効グループ内のみで cursor 移動する。pointer hover は cursor state を更新しない (詳細は ADR 0051)。Ctrl+N/P / Home / End も同じ skip ルールに従う。

(5) disabled 行を Enter / pointer click で選択しようとした場合は palette を閉じず、該当行を 1 回 shake + flash し、入力欄直下の inline status 領域に `"<label>" is unavailable: <reason>` を表示する。同文言を role='status' aria-live='polite' で 1 回読み上げる。toast は出さない (ADR-0047 single source を維持し他経路で重複させない)。

(6) movable な行 (有効グループ件数) が 0 件のとき、palette 上部の status badge slot に状況別文言を明示する: sessionConfig 未 hydrate なら `Loading commands…`、hydrate 済みで本当に 0 件なら `No commands available`、ctx 構築失敗 (httpFactory invalid) なら `Unavailable`。Enter は no-op だがメッセージで silent failure を回避する。

(7) view-update で `sessionConfig.pushCommands` が増減した場合は次 render で listbox に反映する。cursor 位置の tool が削除された場合は selectedToolId ベースで cursor を再計算し、同 index に別 tool が来る silent footgun を回避する。

## Consequences

- **positive**: 1 list で fuzzy 検索が standard / push 横断に効くため、ユーザーは tab 切替の認知コスト無しに全 tool を発見できる (Web command palette 標準と整合)。
- **positive**: disabled の理由が常時 visible になり、push の構造的制約 (active session 未選択 / push-capable driver 不在) が UI 上で説明される。サポート問い合わせや「なぜ押せないか分からない」というユーザー困惑を構造的に削減する。
- **positive**: ADR-0047 (disabledReason single source) を維持しつつ、その値の唯一の表示先が secondary text と inline status に限定され重複が生まれない。
- **positive**: ADR-0036 (store 純粋性) と矛盾しない。本 ADR で追加される group sort と inline status は表示層 (React 層) に閉じ、store の責務は変えない。
- **negative**: pushCommands 件数が将来増えた場合 (例: 100 件超) に listbox 縦方向が膨張する。本 ADR では sort 機構を導入しないため、その際は open_questions の dynamic sort 検討に繋ぐ。
- **negative**: 既存 `ScopeSegment.tsx` と関連 store action / keybinding を削除する破壊的変更が発生する。TUI ↔ Web の操作モデル差が広がるため release note と key hint で説明責任を負う。
- neutral: registry 順を維持するため既存 fuzzy ランクの相対順序は変わらず、ユーザーの筋肉記憶への影響は最小化される。

## Alternatives Considered

### tab (ScopeSegment) を残したまま push tab を常時 enabled にして空 state を明示する

tab という UI 要素を残す理由 (modal 切替 / context 分離) の説明責任が残る。Raycast / Linear / VSCode と乖離し、新規 Web ユーザーへの learning cost が高い。Q1 (本タスク resolved_issues) で 1 list 統合に確定済み。

### disabled 行を keyboard skip ではなく stop して理由を読ませる

cursor が disabled 行で停止し screen reader で reason を読み上げるアプローチ。技術的には可能だが、有効行に到達する操作回数が増え、典型ユースケース (有効 tool を選んで送信) の手数が膨らむ。secondary text + inline status で reason が伝わるため stop は冗長。Q1 結論通り skip を採択。

### sort 機構 (recently used / 使用頻度) を本 ADR に含める

scope 統合と動的 sort はどちらも 1 list を扱う設計判断で関連が深い。しかし sort の判定ロジック (recent window / decay / persistence) は単独で 1 ADR 相当のスコープがあり、本 ADR (案 A 最小) と混ぜると判断が肥大化する。open_questions の将来 ADR 候補として分離。

### disabled 行を hidden にして必要時だけ展開する `Show disabled` toggle を出す

clutter は減るが、disabled の存在自体が見えなくなり「なぜこの tool が出ないのか」というユーザー疑問が解決しない。本 ADR の目的 (理由を見せる) に反する。
