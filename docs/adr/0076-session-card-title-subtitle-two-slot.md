# ADR 0076 — セッションカードを Title + Subtitle の 2 slot 構造に変更し、Subtitle を user-prompt-only LLM 要約で埋め、表示幅は CSS でクランプする

Status: Accepted

Supersedes: [ADR-0033](0033-display-label-empty-policy.md)
Related code: `src/client/web/src/components/SessionList.tsx`, `src/client/web/src/components/DriverViewPanel.tsx`, `src/client/web/src/css/session-list.css`, `src/client/web/src/css/view.css`, `src/client/driver/{codex_view.go,gemini_view.go,summary_prompt.go,summary_job.go,claude_event.go,codex_event.go,gemini_event.go}`

## Context

ADR-0033 はセッションカードのラベルを `title → subtitle → sessionID` の 1 slot chain で決め、両方空のときは `sessionID` を出していた。運用してわかった問題:

1. **sessionID は人間が読むものではない** — 一覧で UUID 断片を見せられても何のセッションか分からない
2. **Title と Subtitle は alternatives ではなく complementary** — Title (固定/明示的なラベル) と Subtitle (動的な作業内容要約) は別の情報。1 slot に押し込むのは情報損失
3. **Subtitle に assistant 出力が混入していた** — codex/gemini driver の `Subtitle = firstNonEmpty(Summary, LastPrompt, LastAssistantMessage)` で assistant 文が紛れ込み、LLM 要約も `recentUserTurns` の名前と裏腹に user/assistant 両 turn を入力していた。結果、Subtitle に「アシスタントが何を言ったか」が出ることがあり、ユーザの意図と乖離していた
4. **要約が長文化すると card 幅が崩れた** — `~30 characters` という指示は LLM が守らず、過剰に長い Subtitle がレイアウトを横に押し広げていた

## Decision

### 1. Card UI を 2 slot 構造へ

`SessionRow` を以下に変更 (`SessionList.tsx`):

```
┃ ● <title or TITLE_PLACEHOLDER>      [driver]
┃   <subtitle (CSS-clamped, ellipsis)>
┃   [tag] [tag]  <border_badge>
```

- `TITLE_PLACEHOLDER = "New Session"` — Title slot は常に何かを描画する
- Subtitle slot は値があるときだけ DOM に出す (空なら row 自体を省略)
- `displayLabel(card, id): string` は `titleText(card)` と `subtitleText(card)` の 2 関数に分割。`displayLabel` は後方互換のため残すが titleText 相当に縮退
- sessionID は **UI 上には一切出さない**。devtools / e2e 用に `data-session-id` 属性のみ残す

### 2. Subtitle の入力ソースを user-prompt only に揃える

- `codex_view.go` / `gemini_view.go` の `firstNonEmpty(Summary, LastPrompt, LastAssistantMessage)` から **`LastAssistantMessage` を削除**
- `summary_prompt.go` に `userOnlyTurns(turns, n)` を新設。`Role == "user"` の turn だけを直近 n 個まで取り、assistant / tool / system turn は完全に弾く。旧 `recentUserTurns` (実体は user 限定でなかった) は廃止
- `formatSummaryPrompt` の指示文を:
  - `~30 characters` → `about 25 characters`
  - 「Use ONLY the user inputs; never summarize assistant outputs, tool results, or any non-user content」を明示
  - 入力ブロックを `<recent_turns>` → `<user_inputs>` に rename
- `claude_event.go:287` / `codex_event.go:109` / `gemini_event.go:123` の summary job 起動箇所を全て `userOnlyTurns(appendHookPromptTurn(...), 2)` 経由に統一
- `applySummaryJobResult` で結果 summary を `clampGraphemes(s, 30)` でクランプ (LLM が長さ指示を破った場合の defense in depth)

### 3. 表示幅は CSS でクランプ

JS で文字数を数えると CJK / emoji / surrogate pair の扱いが煩雑になり、a11y (screen reader / find) も犠牲になる。代わりに CSS で:

- `.session-list__subtitle`: `max-width: 25ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`
- `.driver-view-title` / `.driver-view-subtitle`: 同じく `max-width: 100ch` 付き ellipsis

full 文字列は DOM に残るので、コピー / find / screen reader / e2e はすべて全文を見られる。

### 4. Summary トリガーは現状維持

`claude_event.go` の UserPromptSubmit hook、`codex_event.go` の SubsystemPromptSubmitted、`gemini_event.go` の BeforeAgent — いずれもユーザ入力イベントで発火しており、要件「ユーザ入力イベントをトリガに」は既に満たしている。Assistant 由来の TurnCompleted / AfterAgent / MessageUpdated は LastAssistantMessage を seed するだけで summary job を呼んでいない。

## Consequences

- Card は「Title はそのまま、Subtitle はユーザがやろうとしている作業の要約」という構造になり、一目で識別可能 (sessionID を見る必要がなくなる)
- Subtitle に assistant 出力が出る事象が消える。要約モデルへの入力にも assistant が混じらない
- 表示幅は CSS で常に予測可能なまま。LLM がたまに 60 文字返しても 25ch で切れる
- 既存の `displayLabel(card, id): string` を呼んでいるテストや外部参照は title fallback 相当 ("New Session") を返すようになる。`SessionList.test.tsx` の FR-012 系 (id を期待していた) は本 ADR と合わせて書き換え済み
- ADR-0033 は本 ADR で Superseded

## Alternatives Considered

### JS で文字数をクランプ (`clampText(s, n)` + `title=` 属性)

却下: code-point 数を Array.from で数える helper を一度実装したが、(1) full 文字列が DOM から消えて a11y / find / コピー時に取れない、(2) CJK ch との混在で文字数感が一貫しない、(3) CSS 1 行で済む処理を React コンポーネント・ヘルパ・テストへ分散させるのは過剰、という 3 つの理由で取り下げ、CSS のみに集約した。Go 側の `clampGraphemes(s, 30)` は network/store に流れる値の長さ防御として残してある。

### Subtitle が空のときに「Title slot に Subtitle を上げる」 (= ADR-0033 chain の継続)

却下: 要件「Title があれば Title、Subtitle が Subtitle」は文字どおり 2 slot を意図している。1 slot に押し込む chain は元のバグの源泉だった。

### 「Blank Session」「Untitled」など複数 placeholder の使い分け

却下: i18n / a11y / wording 一貫性のメンテコストに対して UX 上の利得が薄い。単一定数 `"New Session"` のほうが単純で testable。
