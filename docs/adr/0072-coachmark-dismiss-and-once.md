# ADR 0072 — Coachmark dismiss は tap or 5s の早い方とし `hintSeen` 書込は初回 render 時に冪等化する

Status: Accepted

Related: [ADR 0064](./0064-reduced-motion-single-guard.md), [ADR 0070](./0070-fontsize-persist-clamp.md)
Related code: `src/client/web/src/hooks/useCoachmarkOnce.ts` (new), `src/client/web/src/components/Coachmark.tsx` (new or extension of existing primitive), `src/client/web/src/css/view.css` (reduced-motion guard 末尾追記)
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-COACH-001/002`

## Context

`ux.md` UAC (F-001 step 3) は『dismissible coachmark で tap または数秒で消える』とし、Open Question 4 で dismiss タイミングが 3 つ (5s 自動 / 初回 tap / 両方) の選択肢として残った。再表示しない冪等性 (`hintSeen` 書込タイミング) と『起動直後の離脱で coachmark を見ない可能性』のトレードオフが ADR で明示されていなかった。

`aria-live` を使うか通常 DOM popup として SR に読ませるかも未確定。

## Decision

(1) **dismiss 経路は (a) tap (b) 5 秒経過 のどちらか早い方** で fade-out + unmount (Option C 採用)。

(2) localStorage `web.term.hintSeen='1'` の書込は **初回 render 時に 1 回限り** (tap / auto を待たない)、冪等性確保のため。これにより『起動直後にユーザーが離脱してもセッション 2 回目以降は再表示しない』を機構的に保証する (再表示する経路を残すと『何度も出る coachmark バグ』の温床)。

(3) Coachmark の DOM は通常 `<div role='status'>` (`aria-live` は使わず popup として SR にも自然に読まれる) — `aria-live` は `AriaLiveStatus` single slot 専用 ([ADR 0073](./0073-arialive-debounce-and-jump-fab-seed-stability.md))。

(4) 既存 Tooltip / Popover / Hint 系 primitive が `src/client/web/src/components` に存在するかを **実装前 1 chunk で grep 調査**し、存在すれば extend (dismissible variant 追加)、無ければ最小 `<div role='status'>` で実装 (新規 primitive を増やさない方針)。

(5) fade-out 250ms は [ADR 0064](./0064-reduced-motion-single-guard.md) の view.css 末尾 `@media (prefers-reduced-motion: reduce)` single guard block に追記し reduce では即時化。

(6) `hintSeen` の永続化は [ADR 0070](./0070-fontsize-persist-clamp.md) の `createPersistedValue<boolean>(key, ...)` adapter を共有する。

## Alternatives Considered

### 5 秒経過のみで自動 dismiss

自分で消したいユーザーが取り残され UX 摩擦。**却下**。

### 初回 tap のみで dismiss

放置で消えず注意散漫 / 入力モード突入時にも残って邪魔。**却下**。

### 永続表示 (ユーザー明示 dismiss のみ)

認知ノイズ大 / 2 回目以降も出続けると邪魔。**却下**。

### `hintSeen` を tap / auto 後に書く

tap / auto 前にクラッシュ / 離脱した場合に次回も coachmark が出る / 『出続けるバグ』再現性が高い / 冪等性が壊れる。**却下**。

### `aria-live` で coachmark テキストをアナウンス

`AriaLiveStatus` single slot と競合 / SR ユーザーは Coachmark が DOM 上の popup として自然に読まれるため二重通知になる。**却下**。

### 新規 `Coachmark.tsx` を独立実装

既存 Tooltip / Popover primitive と重複の可能性 / 実装前 1 chunk の調査で再利用可能なら新規追加を避ける。**条件付き却下** (調査結果次第)。

## Consequences

- 認知ノイズなし (2 回目以降は表示しない / 5s で自動消滅 / tap で即消滅)
- 起動直後離脱で coachmark を未閲覧のまま `hintSeen` が立つ可能性ありを承認 (UX 損失と冪等性のトレードオフを ADR で明示)
- fade-out が ADR 0064 reduced-motion single guard で即時化
- Coachmark 実装が既存 primitive 再利用 (調査結果次第) で重複 component を生まない
- `aria-live` が Coachmark で発火しないため `AriaLiveStatus` の polite 連続 emit と直交

## Related Requirements

- `FR-MOB-COACH-001` — 初回閲覧モード突入時の 1 回 render + `hintSeen` 同時冪等書込
- `FR-MOB-COACH-002` — tap or 5s 早い方で fade-out + reduced-motion 即時化
