# ADR 0052 — palette header に client-local active session を常時表示し、変化時は flash + aria-live で告知、submit 中は表示凍結

Status: Accepted

Related: [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: F-001, F-005, F-006, F-008 (UAC-001〜UAC-002, UAC-013〜UAC-018)

## Context

commit 9287c7f の CommandPalette は active session の情報を palette 内に一切表示しない。push 系 tool は active session を送信宛先とするため、ユーザーは「いま push が誰宛に飛ぶか」を palette を閉じてから session list を見て確認する必要がある。さらに複数 client (別ブラウザタブ / 別 window) が同じ daemon に接続している場合、別 client 側の操作で view-update が届きこの client の active session が silent に切り替わる。palette 開いている間にこの切替が起きると、ユーザーが意図した宛先と異なる session に push が飛ぶ。

ADR-0046 (web-active-session-ownership) で active session の論理 source は **client-local activeSessionID** に確定している。daemon-global active を表示すると複数 client 運用で意味が崩れる (どの client の active かが不明確になる)。本 ADR ではこの単一 source を palette UI 上にどう露出し、変化をどう告知し、submit 中にどう凍結するかを統合的に決める。spec.md / plan.md は後続 plan-how フェーズで生成される。

## Decision

(1) palette header (input 欄の上、または上端領域) に Active context 行を**常時表示**する。表示文言は `Active: <projBase> / <sid8>` (例: `Active: bar / sess_abcd1234`)。active session が null のときは icon (`—` prefix) + `— No active session` を secondary 表示で出す。色だけに依存せず prefix 文字 + secondary 色で無効状態を示す (WCAG 1.4.1)。

(2) source は **client-local activeSessionID (ADR-0046) のみ**。daemon-global active は参照しない。Zustand selector で `(state) => state.activeSessionID` を購読し、view-update 後の client-local 反映タイミングと同期する。

(3) palette open 中に active が変わったら、Active context 行に約 600ms の subtle background flash (例: 半透明 accent color の overlay を CSS transition で fade-in/out) を適用し、role='status' aria-live='polite' で `Active session changed to <projBase> / <sid8>` を 1 回読み上げる。null → 値 / 値 → null / 値 → 別値 の 3 ケースすべてで同じ告知パターンを使う。

(4) その active 変化で disabled → 有効に変わった push 行 (例: 直前まで `No active session` で disabled だった push:save) は warning icon + secondary text が消え、separator 上段 (有効グループ) に group 移動して再描画される。該当行も 1 回 flash する (ADR 0050 の group sort と整合)。

(5) submit in-flight (status badge slot が `Sending…` 中) は palette UI 全体を凍結する。具体的には Active context 行 / listbox / status badge slot のいずれも変更しない。送信が解決 (成功 / 失敗いずれも) した瞬間に palette は閉じる (paramless push) または次状態に進む。**次回 palette open で新 active が反映**される。これにより送信先と表示のズレが構造的に防止される。

(6) ctx 構築失敗 (httpFactory invalid) では Active context 行を**描画しない**。代わりに status badge slot に `Unavailable` を表示する。palette 全体が不通であることを矛盾なく示すため、active 表示と不通表示が同時に存在する状態を作らない。

(7) `projBase` は `projects[].path` の **basename**。実装ルール: (a) path が `/` 終端や empty のときは path そのものを fallback、(b) 同名 basename が複数 projects に存在するときは disambiguator として親 dir 名を併記 (例: `work (under foo)`)、(c) Windows path にも対応するため `/` と `\` の両方を separator として扱う。

(8) `sid8` は sessionID の先頭 8 char を `monospace` font で表示する。8 char は uniqueness を保証しないため、Active context 行に `title` attribute (tooltip) で full sessionID を提供する。pointer hover で full sessionID を確認できる。

## Consequences

- **positive**: push の送信宛先が palette 内で常時可視になり、誤送信の事前検知が可能になる。「いま push が誰宛か」を palette を閉じずに確認できる。
- **positive**: silent context shift (palette open 中の別 client 起因 active 切替) が flash + aria-live で構造的に告知される。視覚 + 聴覚の両方で気づける。
- **positive**: submit in-flight 中の表示凍結 (F-008) により、表示と送信先のズレが構造的に防止される。Race condition で「画面上は新 active、送信先は旧 active」となる footgun が消える。
- **positive**: ADR-0046 (client-local active 単一 source) と矛盾せず、daemon-global active を参照しない設計を palette UI 層でも貫徹する。
- **positive**: ctx 構築失敗時に Active 行と Unavailable 表示が共存しないため、不通状態の表現が一貫する。
- **negative**: palette の縦方向が 1 行 (header) 増える。modal の overall height が伸びるため、小さな画面で visible 行数が 1 行減る。
- **negative**: 同名 basename の disambiguator 表示 (`work (under foo)`) が長くなると 1 行に収まらない可能性がある。truncate ルールは表示層の責務として CSS `text-overflow: ellipsis` + `title` attribute で full 表示する。
- neutral: TUI 側の palette には Active context 行が無いため、Web ↔ TUI で palette の見た目に差が生じる。TUI は単一 session 運用が主のため Active 表示の必要性が薄い設計判断と整合する。
- neutral: flash の duration (600ms) と aria-live の `polite` レベルは UX 監視対象 (頻繁すぎる切替で読み上げが累積する場合の調整余地あり)。

## Alternatives Considered

### header ではなく toast で active 変化を告知する

palette open 中に毎回 toast が出ると、複数 client 運用でノイズが累積する。header の flash は palette を見ているときだけ目に入り、palette を閉じれば消えるため、ノイズが palette 内に閉じる。

### daemon-global active を表示する

ADR-0046 で確定した client-local source と矛盾する。複数 client 運用では「どの client の active を表示しているか」が不明確になり、`Active:` の意味自体が崩れる。

### submit 中も active を更新する (凍結しない)

submit リクエストは『送信開始時の active』宛に出ているのに、表示は新 active を出すことになる。ユーザーが「いま送られたのは新 active 宛」と誤認するため、表示と送信先のズレ footgun が残る。

### Active context 行を null のとき hidden にする

null 状態がそもそも「session が無い」という情報なので、hidden にするとユーザーに状況が伝わらない。`— No active session` を明示することで「session が必要な操作 (push) が disabled な理由」とも整合する (ADR 0050 の disabledReason `No active session` と表示文言が一致)。

### sid8 (先頭 8 char) ではなく full sessionID を表示する

full sessionID は 32+ char で 1 行に収まらない。先頭 8 char + monospace + tooltip full sessionID で「視認性」と「事後検証可能性」を両立する。
