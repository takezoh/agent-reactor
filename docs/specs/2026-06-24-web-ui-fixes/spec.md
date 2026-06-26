# Spec — Web UI Fixes (EVENTS タブ / stale terminal / fit / spinner / session label)

- **作成日**: 2026-06-24
- **ブランチ**: `feat/tmux-free-web-server`
- **plan**: [plan.md](./plan.md)
- **ADRs**: [0029](../../adr/0029-terminal-host-flex-height.md), [0030](../../adr/0030-terminal-keyed-remount.md), [0031](../../adr/0031-kindoftab-server-symmetry.md), [0032](../../adr/0032-runstate-spinner-additive.md), [0033](../../adr/0033-display-label-empty-policy.md), [0034](../../adr/0034-refit-raf-coalesce-and-test-infra.md)

## Goal

agent-reactor の Web UI (`src/client/web` — React + zustand + xterm.js) における 6 件の不具合 / UX 改善を **`client/web` 内で完結する変更** として修正する。EVENTS タブ空パネル検出バグ、セッション切替後のターミナル stale render、ターミナルのウィンドウ/レイアウト非追従(真因は flex 内 `height:100%`)、セッション一覧のステータス spinner 化、ラベル表示(`title→subtitle→id` チェーン、空文字も空とみなす)を **観測可能な振る舞い** として直し、各変更に vitest を付ける。

wire shape (`src/client/web/src/wire/server.ts`) とサーバ/driver 側は変更しない。

## Background — 報告された 6 件

1. EVENTS タブがターミナルタブのように見え、中身が表示されない
2. 本当の EVENTS (イベントログ) を表示するタブが機能していない
3. ターミナルにセッション切替後も前セッションの表示結果が残ったまま描画される (stale render)
4. ターミナルの幅が Window にフィットしていない
5. セッション一覧のステータスを spinner で表現する (現状はテキストバッジ)
6. セッション一覧に session ID ではなく、エージェントが付与したタイトル / 要約文章 / 最終入力プロンプトを表示する (arc TUI と同じ)

## 根本原因 (調査済み)

| # | 真因 | 修正点 |
|---|------|--------|
| 1·2 | `LogTabs.tsx` の `kindOfTab()` が label `"EVENTS"` (実 driver の label) と path 末尾 `.log` (実 driver の path: `<eventLogDir>/<sid>.log`) を認識せず `null` を返す → `ContentArea` が描画されない。**サーバ側 REST `/api/sessions/:id/event-log` と WS `"et"` フレームは正常に動作している**。壊れているのはクライアントの kind 検出のみ。 | `kindOfTab` を server `matchLogTab` と対称化 ([ADR 0031](../../adr/0031-kindoftab-server-symmetry.md)) |
| 3 | `TerminalPane.tsx` が単一 xterm インスタンスを全セッションで共有する (コメント L84-92 に明記)。`App.tsx` は `TerminalPane` を sessionId で key していないため、切替時に前セッションの buffer が xterm に残る。`OutputFrame` は `frame[3]===sessionId` で filter 済みなので **別セッション出力の混入は防げているが、xterm の既存描画はクリアされない**。 | `<TerminalPane key={activeSessionID}>` で keyed remount + 購読所有者を TerminalPane に一本化 ([ADR 0030](../../adr/0030-terminal-keyed-remount.md)) |
| 4 | **真因は CSS**: `.terminal-host` は `height: 100%` のみで、flex コンテナ内の `%` 高さが安定して解決されない。さらに `FitAddon.fit()` が mount 時と window resize のみで、host への `ResizeObserver` が無い。兄弟パネル (`DriverViewPanel` / `LogTabs`) の出現消滅で host サイズが変わっても refit されない。 | `.terminal-host` を `flex: 1 1 0 + min-height: 0` に ([ADR 0029](../../adr/0029-terminal-host-flex-height.md)) + `scheduleFit()` を rAF コアレスへ集約 + `ResizeObserver` 追加 ([ADR 0034](../../adr/0034-refit-raf-coalesce-and-test-infra.md)) |
| 5 | `RunStateBadge.tsx` はテキストバッジ。spinner ライブラリ未導入。既存テストが `el.textContent === status` を契約として固定しているため、テキスト置換は破壊的。 | 可視テキスト + `aria-label` を温存し、`aria-hidden` な CSS spinner を **加法的** に追加 ([ADR 0032](../../adr/0032-runstate-spinner-additive.md)) |
| 6 | `SessionList.tsx` が `s.view.card.title ?? s.id` で表示。`??` は空文字を非空扱いするため、**空タイトルのカードで空ラベルになるバグ** がある。`DriverViewPanel` は `card.title && / card.subtitle &&` の truthy 判定で空文字を弾いており、規約がコンポーネント間で不一致。 | `displayLabel(card, id) = title → subtitle → id` の trim 後非空チェーンへ ([ADR 0033](../../adr/0033-display-label-empty-policy.md)) |

## Scope

### In Scope

- `src/client/web/src/components/LogTabs.tsx` の `kindOfTab` に event-log 検出を追加 (`.log` / `.jsonl` path 末尾 + label `includes("events"|"event-log")`, server `matchLogTab` と対称化)
- `src/client/web/src/css/app.css` の `.terminal-host` を flex 内で確定高さを得る形 (`flex: 1 1 0` + `min-height: 0`) に修正
- `src/client/web/src/components/TerminalPane.tsx` の fit を `scheduleFit()` (rAF コアレス) に集約。terminal-host に `ResizeObserver` を張る
- `src/client/web/src/App.tsx` で `<TerminalPane key={activeSessionID ?? "none"} ...>` として keyed remount
- TerminalPane の subscribe/unsubscribe を購読の単一所有者に一本化。`SessionList.onClick` からの subscribe/unsubscribe を撤去
- `src/client/web/src/components/RunStateBadge.tsx` に active 状態 (running / waiting) 用の aria-hidden CSS spinner を加法的に追加
- `src/client/web/src/css/view.css` に `@keyframes` spinner と run-state スタイルを追加
- `src/client/web/src/components/SessionList.tsx` のラベルを `displayLabel(card, id)` 純関数経由に変更
- `src/client/web/src/test-setup.ts` に `ResizeObserver` mock (observe / disconnect + 手動コールバック発火) と `requestAnimationFrame` の同期 flush mock を追加
- 上記各変更に対応する vitest (`*.test.tsx`) の追加・更新

### Out of Scope

- `src/client/web/src/wire/server.ts` の型変更 (Go ミラーのため不変)
- driver 側 `client/driver/view_builder.go`・サーバ側 `server/web/transcript.go` の変更 (REST/WS tail は既に動作)
- spinner 等のための新規 npm 依存追加 (CSS `@keyframes` で実現)
- タブ ⇔ ターミナルの UX を arc TUI のタブ切替式へ作り替える大規模リデザイン (Open Question として記録、別 issue 起票)
- EVENTS の `<pre>` を生 ndjson 並べから人間可読表現へ整形する表示品質改善 (Open Question として記録、今回は「空でない=活性化」まで)
- Go embed (`client/web/embed.go`) や dist ビルドパイプラインの変更
- 認証・WS プロトコル・`connection.ts` の transport 層の挙動変更 (`onOutput` 単一スロット契約は維持)
- ターミナルスクロールバック履歴をセッション切替後も保持する仕組み (connection / サーバ拡張が要りスコープ外)

## Requirements (EARS)

> EARS = Easy Approach to Requirements Syntax。`ubiquitous` (常に成立) / `event_driven` (X したとき) / `state_driven` (X の間) / `unwanted` (X したら) / `optional` / `complex` の型を持つ。

### 機能要件 (FR)

#### EVENTS タブ (問題1・2)

- **FR-001** *(event_driven)* — ユーザが path 末尾が `.log` または `.jsonl` の LogTab を選択したとき、システムは当該タブを `event-log` kind として解決し `ContentArea` を描画しなければならない。
  - *Rationale*: 実 driver は EVENTS タブを `Path=<sid>.log` で載せるが、client の `kindOfTab` が `.log` を未対応で空パネルになる。server `matchLogTab` の `pathSuffixes=[.log, .jsonl]` と対称化する。
- **FR-002** *(event_driven)* — ユーザが label が小文字化後に `"events"` または `"event-log"` を含む LogTab を選択したとき、システムは当該タブを `event-log` kind として解決しなければならない。
  - *Rationale*: server `matchLogTab` は `labelTokens=[events, event-log]` を `strings.Contains` で見るため、client も exact-match ではなく `includes` で対称化し判定規則の非対称を解消する。
- **FR-003** *(event_driven)* — ユーザが EVENTS タブを選択したとき、システムは event-log バッファの行を `ContentArea` の `<pre>` に描画し、空パネルを表示してはならない。
  - *Rationale*: 問題1・2 の観測可能な受け入れ。表示内容 (生 ndjson か可読整形か) の品質改善は別 issue (Open Question を参照)。
- **FR-004** *(ubiquitous)* — システムは `kindOfTab` の既存の transcript / event-log 解決 (`.transcript`・`/transcript` パスおよび `"transcript"` ラベル) を変更後も同一の結果で返さなければならない。
  - *Rationale*: 回帰防止。追加分岐は既存の `.transcript` / `.event-log` 判定の後方に置き検出順を保つ。

#### ターミナル切替 (問題3)

- **FR-005** *(event_driven)* — ユーザがセッションを別のセッションへ切り替えたとき、システムはターミナル表示領域に前セッションの出力を残さず、新セッションの内容のみ (または到着前は空) を表示しなければならない。
  - *Rationale*: `TerminalPane` を `activeSessionID` で keyed remount し、新セッションは必ず空 term から始まることを React の key 機構で保証する。

#### ターミナル refit (問題4)

- **FR-006** *(event_driven)* — `terminal-host` のサイズ (幅・高さ) が変化したとき、システムは `FitAddon.fit()` を再実行し xterm の cols/rows をホスト実サイズへ追従させなければならない。
  - *Rationale*: window resize に加え、兄弟パネルの出現消滅による host 内部サイズ変化を `ResizeObserver` で捕捉する。
- **FR-007** *(event_driven)* — `DriverViewPanel` または `LogTabs` の出現・消滅で `terminal-host` の利用可能領域が変わったとき、システムはターミナルを新サイズへ refit しなければならない。
  - *Rationale*: window resize では捕捉できないレイアウト起因のサイズ変化を `ResizeObserver` で扱う。前提として `.terminal-host` が flex 上で確定高さを得ている必要がある ([ADR 0029](../../adr/0029-terminal-host-flex-height.md))。
- **FR-008** *(event_driven)* — `TerminalPane` が初回マウントされ flex レイアウトが確定したとき、システムは実サイズで `fit()` を実行し、0 サイズで初期化してはならない。
  - *Rationale*: 初回 fit を `scheduleFit` (rAF) 経由に通し、flex 確定後のサイズで fit する。`.terminal-host` の flex 高さ確定 ([ADR 0029](../../adr/0029-terminal-host-flex-height.md)) が前提。

#### ステータス spinner (問題5)

- **FR-009** *(state_driven)* — `status` が active 状態 (`running` または `waiting`) の間、システムは `RunStateBadge` に回転する spinner を表示しなければならない。
  - *Rationale*: active = 処理中 / 応答待ちの能動状態。`idle` / `stopped` / `pending` は静的。`waiting` を含める根拠は [ADR 0032](../../adr/0032-runstate-spinner-additive.md) の Decision を参照。
- **FR-010** *(ubiquitous)* — システムは `RunStateBadge` の可視テキストを status 文字列のまま維持し、spinner 要素は `aria-hidden` とし、status は `aria-label` でも提供しなければならない。
  - *Rationale*: 既存 `RunStateBadge.test.tsx` の `textContent===status` / `aria-label` 契約を壊さず加法的に spinner を追加するため。

#### セッション一覧ラベル (問題6)

- **FR-011** *(complex)* — セッションのラベルを表示するとき、システムは `card.title` を、trim 後に空 (undefined または空文字) であれば `card.subtitle` を、それも空であれば `id` を、この順で最初に非空の値を表示しなければならない。
  - *Rationale*: 現状の `title ?? id` は空文字を非空扱いし空タイトルで `id` にフォールバックしない。空判定を undefined と空文字の両方を空とみなす規約に統一し `DriverViewPanel` と揃える。
- **FR-012** *(state_driven)* — `card.title` と `card.subtitle` がともに空であるセッションについて、システムはラベルに `id` を表示しなければならない。
  - *Rationale*: 問題6 のフォールバック終端。arc TUI と挙動を揃える。

### 非機能要件 (NFR)

| ID | 種別 | Criteria | Measurement |
|----|------|----------|-------------|
| NFR-001 | maintainability | 変更した全コンポーネント・純関数 (`kindOfTab` / `displayLabel` / `RunStateBadge` / `TerminalPane` / `SessionList`) に vitest を追加・更新し、新規 FR を観測可能なアサーションで検証する | `cd src/client/web && npx vitest run` が green、各 FR に対応する `it` ケースが存在 |
| NFR-002 | maintainability | 各ソースファイルは 500 行未満、各関数は 80 行未満を維持する (AGENTS.md) | biome / 目視 |
| NFR-003 | compatibility | wire shape (`src/client/web/src/wire/server.ts`) を変更しない。`connection.ts` の `onOutput` 単一スロット契約・transport 挙動を変更しない | `git diff` に `server.ts` / `connection.ts` の wire 変更が無い |
| NFR-004 | maintainability | 新規 npm 依存を追加しない (spinner は CSS `@keyframes`、refit は標準 `ResizeObserver` / `requestAnimationFrame`) | `package.json` の dependencies 差分なし |
| NFR-005 | performance | `ResizeObserver` による refit は連続発火時にレンダリングをスラッシュせず、rAF で 1 フレーム 1 回に集約する。spinner は CSS `transform` ベースで JS タイマーに依存しない | `scheduleFit` が rAF 1 回に間引かれることを test-setup の rAF mock で検証。spinner は `@keyframes` のみ |
| NFR-006 | maintainability | biome lint/format を通過し、変更は `client/web` 内で完結しレイヤ import 方向に違反しない | `biome check` が pass、`import` が `platform → client` 逆流等を起こさない |

## Open Questions (decisioned)

下記は plan-how の Open Questions に対し、本 spec 時点で確定させた決定。詳細は対応 ADR / 別 issue を参照。

| # | 論点 | 決定 | 取り扱い |
|---|------|------|---------|
| Q1 | EVENTS タブの表示内容 (生 ndjson か可読整形か) | **今回は「空でない=活性化」までで止める** | 可読整形は別 issue を `docs/issues/` に起票 (本 spec の Out of Scope) |
| Q2 | `RunStateBadge` の active 定義 (`running` のみ vs `running + waiting`) | **`running + waiting` を active とする** | [ADR 0032](../../adr/0032-runstate-spinner-additive.md) Decision。arc TUI の能動待機表現と一貫 |
| Q3 | web のタブ ⇔ ターミナル設計差 (常時表示 vs arc タブ切替式) | **今回は最小修正に留める** | リデザインは別 issue を `docs/issues/` に起票 (本 spec の Out of Scope) |
| Q4 | `.log` 末尾判定の将来リスク (INFO 等が `.log` を持つと誤検出) | **client は server `matchLogTab` と対称化のみ。driver の path 規約を docs 化** | driver 側 path 規約 docs 化は別 issue (本 spec の Out of Scope) |
| Q5 | 起動直後に title/subtitle 未確定で生 id が一瞬見える | **arc 挙動を維持** | プレースホルダ化はせず ([ADR 0033](../../adr/0033-display-label-empty-policy.md) Consequences) |

## Assumptions / Constraints

- driver 側の `EventLogTab` (`client/driver/view_builder.go`) は `Label="EVENTS"` / `Path=<eventLogDir>/<sid>.log` / `Kind=TabKindText` を載せ続ける (本修正の前提)
- サーバ側 `server/web/transcript.go` の `matchLogTab` は `labelTokens=[events, event-log]` + `pathSuffixes=[.log, .jsonl]` を維持する (本修正の対称化対象)
- `happy-dom` は `ResizeObserver` / `requestAnimationFrame` を未実装または不安定 → test-setup で mock する必要がある
- xterm.js 5.5.0 + `@xterm/addon-fit` 0.10.0 を継続利用 (依存追加なし)
- React の key prop で remount する挙動 (子コンポーネントの全 state 破棄) に依存する
