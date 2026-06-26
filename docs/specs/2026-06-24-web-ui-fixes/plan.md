# Plan — Web UI Fixes

- **spec**: [spec.md](./spec.md)
- **ADRs**: [0029](../../adr/0029-terminal-host-flex-height.md), [0030](../../adr/0030-terminal-keyed-remount.md), [0031](../../adr/0031-kindoftab-server-symmetry.md), [0032](../../adr/0032-runstate-spinner-additive.md), [0033](../../adr/0033-display-label-empty-policy.md), [0034](../../adr/0034-refit-raf-coalesce-and-test-infra.md)

## Components

| Component | Responsibility | Depends on |
|-----------|----------------|------------|
| `kindOfTab` (`LogTabs.tsx`) | `LogTab` を `TranscriptKindParam | null` に解決する純関数。`.log` / `.jsonl` path 末尾と `includes("events"|"event-log")` label を `event-log` に解決し、server `matchLogTab` と対称化。既存 `.transcript` / `.event-log` 判定の後方に追加し検出順と既存マッピングの不変を保つ | `TranscriptKindParam` (`api/transcripts.ts`), `LogTab` (`wire/server.ts`) |
| `ContentArea` (`LogTabs.tsx`) | 既存・無改修。kind 解決後 `useTranscript` で REST backfill + WS tail をバッファに集め `<pre>` 描画。`kindOfTab` 修正で EVENTS が自動的に到達し活性化する | `useTranscript`, `useTranscriptStore`, `kindOfTab` |
| `TerminalPane.tsx` | keyed remount 前提で **1 マウント = 1 conn ライフサイクル** を保つ。`scheduleFit()` (rAF コアレス) へ初回 fit と `ResizeObserver` / window resize refit を集約。terminal-host に `ResizeObserver` を張る。購読 (subscribe/unsubscribe) の単一所有者 | `Connection` (`socket/connection.ts`), `xterm.Terminal`, `FitAddon`, `.terminal-host` (`app.css`) |
| `App.tsx` | `<TerminalPane key={activeSessionID ?? "none"} conn={conn} sessionId={activeSessionID} />` として keyed remount し、切替時の full reset を React に委ねる | `TerminalPane`, `useDaemonStore` (`activeSessionID`) |
| `RunStateBadge.tsx` | 可視テキスト (`status`) と `aria-label` を温存しつつ、active 状態 (`running` / `waiting`) で `aria-hidden` な CSS spinner を加法的に付加。`SessionList` と `DriverViewPanel` の共有部品で両呼び出し元互換 | `css/view.css` (`.run-state-*` + `@keyframes spinner`) |
| `SessionList.tsx` | ラベルを `displayLabel(card, id) = title → subtitle → id` の trim 後非空チェーンで表示。subscribe/unsubscribe を撤去し `selectSession` のみ (購読は `TerminalPane` が所有) | `useDaemonStore`, `RunStateBadge`, `displayLabel` |
| `displayLabel` (`SessionList.tsx` 内) | `card.title → subtitle → id` を trim 後非空で選ぶ純関数。undefined と空文字の両方を空とみなし `DriverViewPanel` の truthy 規約と統一 | `Card` (`wire/server.ts`) |
| `css/app.css` (`.terminal-host`) | `.terminal-host` を `flex: 1 1 0` + `min-height: 0` とし flex 上で確定した残余高さを得る (問題4 真因)。必要なら親 `.terminal` にも `min-height: 0` | — |
| `css/view.css` | `@keyframes` による回転 spinner と `run-state` スタイルを提供 (CSS `transform` ベース、JS タイマー非依存) | — |
| `test-setup.ts` | `ResizeObserver` の mock (observe/disconnect + コールバック手動発火フック) と `requestAnimationFrame` の同期 flush mock を追加。`FakeTerminal` は dispose 中心で keyed remount を支える | — |

## Data Flow

### EVENTS タブ活性化 (FR-001/002/003)

```
driver (EventLogTab) → SessionInfo.view.log_tabs[]: {label:"EVENTS", path:"<sid>.log", kind:"text"}
  ↓ ViewUpdateFrame "v" (wire)
LogTabs.tsx tabs[] (props)
  ↓ ユーザクリック → setActive(i)
kindOfTab(tab) → "event-log"  ← [ADR 0031] 修正後はここで解決される
  ↓
<ContentArea kind="event-log" sessionId=... bearerToken=... />
  ├── useTranscript: REST GET /api/sessions/:id/event-log → useTranscriptStore.appendBackfill
  └── WS "et" フレーム → connection.ts → useTranscriptStore.appendLine
  ↓
<pre>{ buffer.lines.join("\n") }</pre>
```

### ターミナル切替 (FR-005)

```
SessionList.onClick(s.id)
  → useDaemonStore.selectSession(s.id)        ← subscribe/unsubscribe はここでは呼ばない (ADR 0030)
  ↓
App.tsx: activeSessionID 変更 → <TerminalPane key={s.id}> ← 旧 instance アンマウント / 新 instance マウント
  ├── 旧 TerminalPane cleanup: term.dispose(), conn.onOutput = undefined, unsubscribe(旧 sid)
  └── 新 TerminalPane mount: new Terminal(), fit.fit(), conn.onOutput = (...) => term.write(...), subscribe(新 sid)
  ↓
新セッションは必ず空 term から出力到着待ち
```

### ターミナル refit (FR-006/007/008)

```
TerminalPane mount
  → scheduleFit()  ─┐
window resize       ├─ rAF コアレスで 1 フレーム 1 回 → fit.fit() → conn.send({k:"r", cols, rows, sessionId})
ResizeObserver(host)─┘
```

## Build Sequence (chunks 依存順)

依存方向: `test-infra-and-pure-logic` → (`display-layer` ∥ `terminal-lifecycle`)

### Chunk 1: `test-infra-and-pure-logic` (依存なし)

| ファイル | 変更内容 |
|---------|---------|
| `src/client/web/src/test-setup.ts` | `ResizeObserver` mock を `globalThis.ResizeObserver` に注入。`observe(target, cb)` / `disconnect()` と「手動発火フック」(`__triggerResize(target, entries)`) を提供。`requestAnimationFrame` を同期 flush (即時 callback 呼び出し) に差し替え |
| `src/client/web/src/components/LogTabs.tsx` | `kindOfTab(tab)` に分岐追加: path 末尾 `.log` / `.jsonl` → `"event-log"`、label 小文字化後 `includes("events")` または `includes("event-log")` → `"event-log"`。既存判定の後方に置く ([ADR 0031](../../adr/0031-kindoftab-server-symmetry.md)) |
| `src/client/web/src/components/LogTabs.test.tsx` | FR-001/002/003/004 の `kindOfTab` 単体テスト追加 (label=EVENTS / path=`<sid>.log` / label="event-log" / `.jsonl` / 既存 `.transcript` 回帰) |
| `src/client/web/src/components/SessionList.tsx` | `displayLabel(card, id): string` 純関数を追加 (内部関数で可)。`title.trim()` → `subtitle.trim()` → `id` の順で最初に非空を返す ([ADR 0033](../../adr/0033-display-label-empty-policy.md)) |
| `src/client/web/src/components/SessionList.test.tsx` | FR-011/012 の `displayLabel` 単体テスト追加 (title 有 / title 空 + subtitle 有 / 両方空 → id / undefined / 空白のみ) |

### Chunk 2: `display-layer` (← Chunk 1)

| ファイル | 変更内容 |
|---------|---------|
| `src/client/web/src/components/RunStateBadge.tsx` | active 状態 (`running` / `waiting`) のとき `aria-hidden` な `<span className="run-state-spinner" />` を可視テキストの直前または直後に追加。可視テキスト・`aria-label` は変更しない ([ADR 0032](../../adr/0032-runstate-spinner-additive.md)) |
| `src/client/web/src/css/view.css` | `.run-state-spinner { display:inline-block; width:8px; height:8px; border:2px solid currentColor; border-top-color: transparent; border-radius: 50%; animation: run-state-spin 0.8s linear infinite; margin-right: 4px; }` + `@keyframes run-state-spin { to { transform: rotate(360deg); } }` |
| `src/client/web/src/components/RunStateBadge.test.tsx` | FR-009/010 のアサーション追加: `running` / `waiting` のとき `[aria-hidden="true"].run-state-spinner` が 1 件存在、`idle` / `stopped` / `pending` / `unknown` のとき 0 件。既存の `textContent===status` / `aria-label` 契約は温存 |
| `src/client/web/src/components/SessionList.tsx` | (Chunk 1 で追加した) `displayLabel` を `<span className="title">` に適用。`onClick` から `await conn.unsubscribe(activeId)` / `await conn.subscribe(s.id)` を削除し `selectSession(s.id)` のみに ([ADR 0030](../../adr/0030-terminal-keyed-remount.md)) |
| `src/client/web/src/components/SessionList.test.tsx` | FR-011/012 のレンダリングテスト追加 (display 表示の検証)。onClick が subscribe を呼ばないことを `Connection` mock で検証 |
| `src/client/web/src/components/LogTabs.tsx` | ContentArea は無改修 (kind 解決が直れば自動で活性化する) |

### Chunk 3: `terminal-lifecycle` (← Chunk 1)

| ファイル | 変更内容 |
|---------|---------|
| `src/client/web/src/css/app.css` | `.terminal { ... min-height: 0; }` (必要なら) + `.terminal-host { flex: 1 1 0; min-height: 0; width: 100%; }` に変更 ([ADR 0029](../../adr/0029-terminal-host-flex-height.md)) |
| `src/client/web/src/components/TerminalPane.tsx` | `scheduleFit()` 関数を導入 (rAF コアレスで pending フラグを使い 1 フレーム 1 回)。初回 fit / `window resize` / `ResizeObserver` を `scheduleFit()` 経由に統一。`useEffect` で `new ResizeObserver(scheduleFit)` を作り `observe(hostRef.current)` / cleanup で `disconnect()` ([ADR 0034](../../adr/0034-refit-raf-coalesce-and-test-infra.md)) |
| `src/client/web/src/App.tsx` | `<TerminalPane key={activeSessionID ?? "none"} conn={conn} sessionId={activeSessionID} />` ([ADR 0030](../../adr/0030-terminal-keyed-remount.md)) |
| `src/client/web/src/components/TerminalPane.test.tsx` | FR-005: 旧 sessionId でマウント → `conn.onOutput` で stale 出力 write → React の `key` 変更で remount → 新 instance で `term.write` が呼ばれていない (= 旧出力が残らない) ことを検証 (※ keyed remount は親側、当ファイルでは sessionId-prop 変化と key 変更を組み合わせて検証)。FR-006/007/008: `__triggerResize` で host サイズ変化を発火 → `fit.fit` が rAF flush 後に 1 回呼ばれることを検証 |
| `src/client/web/src/App.test.tsx` (存在すれば) | `activeSessionID` 切替で `TerminalPane` が remount される (key prop) ことの間接検証 |

## Test Approach

| Test Tier | 対象 | 検証内容 |
|-----------|------|---------|
| **単体 (pure function)** | `kindOfTab`, `displayLabel` | 入力空間を網羅 (label/path 既知パターン × 4-6、empty/whitespace/undefined) |
| **コンポーネント (RTL + happy-dom)** | `RunStateBadge`, `SessionList`, `TerminalPane`, `LogTabs` | rendering、ARIA 契約、event handler、effect (mount/unmount/key remount) |
| **統合 (App レベル)** | `App.test.tsx` | `activeSessionID` 切替が `TerminalPane` の key 変更を引き起こすことを間接検証 (React の key remount に依存) |

**回帰防止アサーション**:
- `kindOfTab`: 既存ケース (`.transcript` 末尾 / label="transcript" / label="event-log") の戻り値不変
- `RunStateBadge`: 全 status × `textContent === status` / `aria-label` 既存契約
- `SessionList`: title 有時 `displayLabel === title` (現状互換)
- `TerminalPane`: `conn.onOutput` の sessionId filter (`frame[3]!==sessionRef.current` で drop) は変更しない

## Verification

ローカル受け入れ手順 (PR 提出前):

1. `cd src/client/web && npm run build` が green
2. `cd src/client/web && npx vitest run` が green、全 12 FR に対応する `it` ケースが存在
3. `cd src/client/web && npx biome check .` が pass
4. `cd src && go test ./...` が green (server/web 側に影響しない)
5. **手動確認** (`make build-all` 後、`./arc daemon` + `./arc` web gateway 起動 + ブラウザ):
   - EVENTS タブをクリック → event-log の内容 (1 行以上) が `<pre>` に表示される
   - セッションを切替 → ターミナルが空になり、新セッションの出力のみ表示される
   - ブラウザウィンドウをリサイズ → ターミナルが追従して fit する
   - New Session ダイアログを開閉 → terminal-host サイズ変化に追従して fit する
   - セッション一覧の `running` / `waiting` ステータスに spinner が回る
   - title 空のセッションが subtitle (summary or lastPrompt) を表示する。両方空なら id を表示する

## Risks

| Risk | Mitigation |
|------|-----------|
| `flex: 1 1 0 + min-height: 0` の CSS 変更で `.terminal` 内の他要素 (DriverViewPanel / LogTabs) の高さが意図せず潰れる | 手動確認で host が画面残余を占めること、兄弟パネルが内容で表示されることを目視確認 |
| `ResizeObserver` mock の発火順序が実環境と乖離し、テストで passしても実環境で fit 取り損ねる | rAF 同期 flush mock で「observer cb → scheduleFit → fit 1 回」の不変は検証できる。実環境追従は手動受け入れに残す |
| keyed remount で xterm 生成破棄コストが上がる | 人間操作のセッション切替頻度では無視できる。スクロールバック履歴消失は Open Question として記録 (本 spec の Out of Scope) |
| `kindOfTab` の `.log` 末尾判定で将来 INFO 等の `.log` ファイルが event-log に誤検出される | driver 側 path 規約 docs 化を別 issue 起票 (本 spec の Out of Scope) |

## Out of Scope (再掲、別 issue 化推奨)

- EVENTS の人間可読整形 (Q1)
- web のタブ ⇔ ターミナル設計差リデザイン (Q3)
- driver 側 LogTab `path` 命名規約の docs 化 (Q4)
- ターミナルスクロールバック履歴の切替後保持
