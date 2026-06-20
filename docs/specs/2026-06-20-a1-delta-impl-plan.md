# A1-δ Implementation Plan: persistence(transcript / event-log tail)+ connector(github)+ notification

- **作成日**: 2026-06-20
- **ブランチ**: `feat/tmux-free-web-server`(A1-α/β/γ 完了)
- **親計画**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **前段**: A1-γ で view-update broadcast(`Card` / `StatusLine` / `LogTabs` の selector)が稼働、tab content area は `(coming in δ)` の placeholder

## Goal

3 つの新流路を web に追加:
1. **Transcript / event-log tail**: daemon の `EvtSessionFileLine` を WebSocket 経由で React に流し、`LogTabs` の content area に tail 行を表示。初期 backfill は REST `GET /api/sessions/{id}/{transcript|event-log}?offset=N` で paginate。
2. **Connector view**: `EvtSessionsChanged.Connectors[]`(ADR 0023 で view-update に同伴済みなら活用)を `ConnectorPanel` で render。github connector の Sections を表示、click → `window.open(href)` のみ(深いアクションは別 PR)。
3. **Notification toast**: `EvtAgentNotification`(OSC 9/99/777)を WebSocket frame として受信、`NotificationToast` で render(自動 dismiss 5 秒、ユーザ click で即時 dismiss)。

A1-γ の `LogTabSelector` placeholder を本 δ で実装に置換する。

## Scope

### In scope

#### Go 側
- **`src/server/web/transcript.go`**(新設、~150 行): REST handler
  - `GET /api/sessions/{id}/transcript?offset=N` → `<dataDir>/events/<frameID>.transcript` を `N` から EOF まで返す
  - `GET /api/sessions/{id}/event-log?offset=N` → `<dataDir>/events/<frameID>.jsonl` 同様
  - path traversal 防御: `sessionID` は `[a-zA-Z0-9_-]+` allowlist で正規表現検証
  - ETag(`<frameID>:<file-size>`)対応で 304 Not Modified
  - daemon socket からファイルパスを引く(`CmdGetSessionPaths` 等の proto 追加、必要なら)
- **`src/server/web/gateway.go`**(修正): subscribe lifecycle で以下 event を WebSocket に forward
  - `EvtSessionFileLine{SessionID, FrameID, Kind:'transcript'|'event-log', Line}` → `{k:'tl', sessionId, frameId, kind, line}` frame
  - `EvtAgentNotification{SessionID, Cmd, Title, Body, NowMs}` → `{k:'n', sessionId, cmd, title, body, nowMs}` frame
  - `ConnectorInfo` は γ の view-update に既に同伴(`EvtSessionsChanged.Connectors`)。本 δ では別 broadcast `EvtConnectorsChanged` がある場合のみ追加経路を立てる、無ければ view-update で十分(調査結果に従う)
- **`src/server/web/wire.go`**(修正): `transcript-tail` / `event-log-tail` / `notification` / `connector-update` frame の encode 追加
- **`src/server/web/transcript_test.go`**(新設): path traversal 拒否 / offset paginate / ETag 304 / 404 (frame not found) / 204 (empty range)
- **`src/server/web/gateway_persist_test.go`**(新設): protofake で `EvtSessionFileLine` の broadcast、`EvtAgentNotification` の forwarding

#### TS 側
- **`src/client/web/src/wire/server.ts`**(修正): 新 frame 型追加
  - `TranscriptTailFrame = {k: 'tt'; sessionId: string; frameId: string; line: string}`
  - `EventLogTailFrame = {k: 'et'; sessionId: string; frameId: string; line: string}`
  - `NotificationFrame = {k: 'n'; sessionId: string; cmd: number; title: string; body: string; nowMs: number}`
  - `ConnectorUpdateFrame = {k: 'cu'; connectors: ConnectorInfo[]}`(view-update と重複時は frame を維持し store で merge)
- **`src/client/web/src/wire/codec.ts`**(修正): 新 frame parse 追加
- **`src/client/web/src/store/transcripts.ts`**(新設、~100 行): per-(sessionId, kind) の bounded ring buffer(最新 1000 行保持)
- **`src/client/web/src/store/notifications.ts`**(修正): 既存 LRU 32 件を `NotificationFrame` 受信で増加
- **`src/client/web/src/store/connectors.ts`**(新設、~50 行): connectors array を保持(view-update のサブセットとして)
- **`src/client/web/src/socket/connection.ts`**(修正): 新 frame 受信時に対応する store action を呼ぶ
- **`src/client/web/src/api/transcripts.ts`**(新設、~80 行): REST `GET /api/sessions/{id}/{transcript|event-log}?offset=N` のクライアント、ETag キャッシュ付き
- **`src/client/web/src/hooks/useTranscript.ts`**(新設、~80 行): session 選択時に初期 backfill(REST)→ tail 流入(store buffer)を結合した hook
- **`src/client/web/src/components/LogTabs.tsx`**(新設、~150 行): γ の `LogTabSelector` を 拡張し、選択 tab の内容(transcript / event-log tail)を表示。仮想スクロール風に bottom-pinned で表示
- **`src/client/web/src/components/ConnectorPanel.tsx`**(新設、~80 行): `ConnectorInfo[]` を accordion 風に表示、Sections の `href` クリックで `window.open`
- **`src/client/web/src/components/NotificationToast.tsx`**(新設、~100 行): notifications store を `useStore` で監視、新規 notification を 3 秒間 toast で表示、5 秒で auto-dismiss
- **`src/client/web/src/App.tsx`**(修正): `LogTabSelector` → `LogTabs`(本物)に差し替え、`ConnectorPanel` を右サイドに配置、`NotificationToast` を top-right に固定
- 各 component に testing-library test、store に reducer test、wire に round-trip test、api に msw 経由 REST test

### Deletion
- A1-γ の `LogTabSelector.tsx`(本 δ の `LogTabs` に統合)

### Out of scope
- persist 書き込み経路(本 δ は read-only、書き込みは daemon 側で既存)
- warm restart UI(別 PR)
- transcript の検索 / フィルタ機能
- connector の深いアクション(form submit、destructive actions 等)
- React Suspense / streaming responses(本 δ は素朴な fetch + tail)
- xterm.js の scrollback と LogTabs の重複表示の整理(γ で xterm が出力を持ち、δ で transcript も同じ出力を持つ — 表示を分離するか統合するかは UX 判断、本 δ は分離(別 tab))
- `EvtAgentNotification` の Cmd 値(9/99/777)に応じた挙動分岐(全て同じ toast、Cmd は表示のみ)
- 通知音 / Web Notifications API(別 PR)

## EARS Requirements

| ID | Type | Statement | Rationale |
|---|---|---|---|
| **FR-δ01** | event_driven | daemon が `EvtSessionFileLine{Kind:'transcript'}` を発火したとき、システムは購読中の WebSocket に `transcript-tail` frame を送出しなければならない | tail 流路 |
| **FR-δ02** | event_driven | daemon が `EvtSessionFileLine{Kind:'event-log'}` を発火したとき、システムは購読中の WebSocket に `event-log-tail` frame を送出しなければならない | tail 流路 |
| **FR-δ03** | event_driven | daemon が `EvtAgentNotification` を発火したとき、システムは購読中の WebSocket に `notification` frame を送出し、React 側で `NotificationToast` を 3 秒間表示、5 秒で auto-dismiss しなければならない | OSC 9/99/777 UX |
| **FR-δ04** | ubiquitous | システムは REST `GET /api/sessions/{id}/transcript?offset=N` で session の transcript ファイル(`<dataDir>/events/<frameID>.transcript`)を offset `N` から EOF まで返さなければならない | backfill |
| **FR-δ05** | ubiquitous | システムは REST `GET /api/sessions/{id}/event-log?offset=N` で session の event-log ファイル(`<dataDir>/events/<frameID>.jsonl`)を offset `N` から EOF まで返さなければならない | backfill |
| **FR-δ06** | unwanted | もし `sessionID` が `[a-zA-Z0-9_-]+` 以外の文字を含むなら、システムは HTTP 400 を返さなければならない(path traversal 防御) | security |
| **FR-δ07** | ubiquitous | システムは REST 応答に `ETag: <frameID>:<file-size>` を付与し、`If-None-Match` ヘッダが一致する場合は HTTP 304 を返さなければならない | bandwidth |
| **FR-δ08** | ubiquitous | システムは `LogTabs` の content area に最新 1000 行のローテーション buffer を保持し、bottom-pinned scrollview として表示しなければならない | UI 容量 |
| **FR-δ09** | ubiquitous | `ConnectorPanel` は `ConnectorInfo[]` の各 entry を accordion 風に表示し、Sections の `href` を `window.open(href, '_blank')` で開かなければならない | UX |
| **FR-δ10** | ubiquitous | システムは初期 session 選択時に REST `?offset=0` で transcript / event-log を backfill し、以降は WebSocket tail で追記しなければならない(重複行は最後の REST 行の offset で除外) | seamless tail |
| **FR-δ11** | ubiquitous | システムは wire / store / コンポーネント / REST の各層に round-trip / reducer / render / msw テストを追加し、`go test ./... -race` + `npm run test` が緑でなければならない | 三層テストゲート(γ を継承) |
| **FR-δ12** | unwanted | もし WebSocket が close されたなら、システムは `LogTabs` の tail を停止し、REST 経由の backfill 再試行は reconnect 後に行わなければならない | resource hygiene |

## ADR(本 δ で追加)

| ID | Title | Status |
|---|---|---|
| [ADR 0025](../adr/0025-transcript-rest-backfill-then-ws-tail.md) | Transcript / event-log は初期 REST backfill → WebSocket tail のハイブリッドで配信する | accepted |
| [ADR 0026](../adr/0026-path-traversal-defense-allowlist.md) | REST endpoint の path parameter は正規表現 allowlist で防御する(blacklist せず) | accepted |
| [ADR 0027](../adr/0027-notification-toast-auto-dismiss-policy.md) | NotificationToast は 5 秒で auto-dismiss、複数受信時は LRU で stacked 表示 | accepted |

## Verification

```sh
# Go
cd src && go test ./... -race -count=1
cd src && go vet ./...
cd src && go tool golangci-lint run ./...

# Frontend
cd src/client/web && npm ci
cd src/client/web && npm run typecheck
cd src/client/web && npm run lint
cd src/client/web && npx vitest --run
cd src/client/web && npm run build

# Full
make build-all

# Manual smoke
make run-dev
# arc daemon + codex session で複数行 output、transcript が web の LogTabs に流入
# OSC 9 を発火 → NotificationToast 表示
# github connector の Sections が ConnectorPanel に並ぶ
```

## Open Questions(実装直前に決める)

1. proto に `EvtSessionFileLine` が既に存在するか、`EvtAgentNotification` が gateway で forward されているか、ConnectorInfo が view-update に含まれているか — 実装直前に Go 側を grep して確認、無いものは proto 追加。
2. daemon 側の transcript ファイルパス取得 API: 既存に `<dataDir>` 公開機能があるか、無ければ proto に `CmdGetSessionPaths{SessionID}` を新設して `transcript_path` / `event_log_path` を返す。
3. ETag は `<frameID>:<file-size>` の単純形 vs `<frameID>:<mtime>:<size>` の合成形。本 δ は単純形(append-only 前提)で start。
4. `NotificationToast` の重複(同一 SessionID で連続発火)を merge するか、別 toast を積むか — 別 toast(LRU 32)で start、merge は別 PR。

## Traceability

- **親 plan**: Master Plan(`plans-cheerful-thompson.md`)
- **前段**: A1-γ(view-update broadcast、`LogTabSelector` placeholder)
- **次の作業**: A1-ε(cleanup + `server/session` 完全削除)→ C(tmux 実装削除)
