# A1-α Implementation Plan: `cmd/server` を arc daemon の HTTP/WS gateway 化

- **作成日**: 2026-06-19
- **ブランチ**: `feat/tmux-free-web-server`
- **親計画**: [Master Plan(`plans-cheerful-thompson.md`, 全 PR 骨格)](../../plans/arc-server-client-split.md) (ブランチ実行計画)
- **設計根拠**: [`plans/remote-client-design.md`](../../plans/remote-client-design.md), [ADR 0004: PtyBackend が pure core を再利用する](../adr/0004-ptybackend-reuses-pure-core.md)
- **生成プロセス**: `/plan-how` 4 役 Workflow (planner / critic / optimizer / integrator) → ユーザー判断による open_questions 決着

## Goal

`cmd/server` から自前 runtime(`termvt.Manager` / `agentlaunch.Dispatcher` / `session.Service`)の所有を撤去し、arc daemon の unix socket に `proto.Client` で接続する **HTTP/WS gateway** へ再定義する。pty I/O・session lifecycle は `state.Reduce → driver → runtime → termvt` の pure core 経路に一本化し、ブラウザ向け wire(asciicast v2 配列 + `{k,code,data}` JSON)と vanilla JS UI は **無改造** のまま動作させる。新規 proto コマンド/イベントは Surface 概念に統一して codec / Fuzz / reducer / runtime / gateway の四層で gate する。

## Scope

### In scope
- `client/proto`: Surface 系 4 cmd(Subscribe / Unsubscribe / Resize / WriteRaw)と 2 evt(Output / PromptEvent)を追加、codec の switch と Fuzz テストを更新
- `client/state`: `EvCmdSurface*` と `EffSurface*`(Start/Stop/Resize/WriteRaw)を追加し `reduce_surface.go`(純粋 reducer)を新設
- `client/state`: 購読関係 `(ConnID, SessionID)` を `State.Subscribers.Surface` map として state に正式フィールドで持ち、Reduce で更新する(reducer purity を守りつつ broadcast 判定の真実を state に置く)
- `client/state`: 1 `ConnID` あたりの surface subscribe 上限(8)を Reduce 内で判定し超過時 `RespErr(ResourceExhausted)`
- `client/state`: ActiveFrame 未確定時の `EvCmdSurfaceSubscribe` は `RespErr(Code:'frame-not-ready')` を即時発行し state を変更しない(race 対応の β 持ち越し: [ADR 0018](../adr/0018-defer-subscribe-race-to-beta.md))
- `client/runtime`: `FileRelay` と同形の `terminal_relay.go` を新設し、`EffSurface*` を受けて `PtyBackend.SubscribeSurface(paneID) → per-(ConnID, SessionID)` fan-out goroutine を起動する
- `client/runtime`: SessionID → 該当 frame の `TargetID`(paneID)解決ヘルパを driver/runtime 接合点に追加し、relay は paneID で backend に渡す
- `client/runtime`: `PtyBackend` に `SubscribeSurface(paneID)` / `WriteSurface(paneID, data)` / `ResizeSurface(paneID, cols, rows)` accessor を追加し、`termvt.Manager` への到達を export 経由に限定する
- `client/runtime/proto_bridge`: `EvtSurfaceOutput` / `EvtPromptEvent` を encode し、`State.Subscribers.Surface` に従って該当 `ConnID` outbox にのみ送る(`broadcastSurfaceOutput` / `broadcastPromptEvent`)
- `client/runtime/terminal_relay`: termvt 由来 slow-subscriber close と outbox bounded drop の二段ポリシを明文化し、close を受けたら自動で `EffSurfaceUnsubscribe` 相当の internal event を発火して state を整える
- `server/web`: `daemon_client.go` を新設し eager dial + supervisor goroutine(full jitter exp backoff 250ms→4s, 無制限再試行)+ `Health()` + `/healthz` 露出 + slog 計装
- `server/web`: `gateway.go` を `DaemonAdapter` 化し `AttachWS` を `readInbound` / `writeOutbound` / `subscribeLifecycle` の 3 関数(各 ≤80 行)に分割、typed close ヘルパ `writeTypedClose(reason)` を集約
- `server/web`: daemon disconnect 時に best-effort で `controlMsg {k:'c', code:'daemon-disconnected'}` を送ってから `StatusGoingAway` で typed close する 2 段 close を実装
- `server/web/wire.go`: `EvtSurfaceOutput` → asciicast v2 配列 `['o', t, decoded]` 変換と `EvtAgentNotification` → `{k,code,data}` 変換を 1 ファイルに集約
- `server/web/mux.go`: `server/session.Sessions` interface を撤去し、REST は `proto.Client` 経由で daemon の既存 `CmdCreateSession` 等を呼ぶ。cols/rows は `CreateSessionParams.Options` に詰める adapter を追加
- `cmd/server/main.go`: `session.NewService` / `termvt.Manager` / `agentlaunch.Dispatcher` の生成を撤去、`-arc-sock` フラグ追加(default は `ARC_SOCKET` → `~/.agent-reactor/arc.sock` の順)、socket path 解決は `platform/socketpath` に切り出して daemon と共有
- `server/session/` 一式と既存 session 系テストを build tag `//go:build legacy_session` で本体ビルドから隔離する(ε で `git rm`)
- `client/proto/protofake/` パッケージを新設(`net.Pipe` + ndjson encoder の最小 2 公開 API: `NewPair()` / `Close()`)
- テスト: (1) proto codec + Fuzz、(2) state reducer table(Eff 発火 + `Subscribers.Surface` 更新 + 上限超過 + race RespErr)、(3) runtime relay fan-out + slow-close 自動 unsubscribe、(4) gateway protofake + `net.Pipe` 通し試験、(5) mux daemon `Health=false` で 503、(6) daemon_client dial / disconnect / reconnect / in-flight cancellation
- `src/.golangci.yml` の depguard を更新して「server/* は `client/proto`, `client/state`, `client/runtime` を import 可」のルールを明示
- **PR を 3 分割する**(PR-1: proto + codec + Fuzz / PR-2: state reducer + `Subscribers.Surface` / PR-3: runtime relay + server/web gateway + daemon_client)

### Out of scope
- vanilla JS UI の React+TS 化 (β)
- view-update broadcast の差分化 (γ)
- persistence / connector wire の改変 (δ)
- `server/session` 一式の `git rm`(ε、α では build tag 隔離まで)
- `tap_manager` の 1×1 `vt.Terminal` 撤去 (A2)
- tmux 実装の削除 (phase C)
- `client/state` の reducer/driver core ロジック変更(本 PR は `Ev/Eff/Subscribers.Surface` の追加のみ)
- `platform/termvt` API の wire 改変
- 認証スキーム(bearer / ws ticket / CSP)の変更
- prompt-event の driver 実発火(本 PR は encoder + broadcast 経路のみ用意、β で 1 行で接続)
- ブラウザ側 gap 検知 UI / reconnect UX(β 以降)
- subscribe race 対応(client retry / gateway wait / state pending のいずれも β に倒す: [ADR 0018](../adr/0018-defer-subscribe-race-to-beta.md))
- tracing / metrics 基盤(α は `slog` のみで観測)
- `keystroke → 画面更新` の定量パフォーマンス予算(α は「明らかな regression なし」の定性基準、定量化は β 以降で bench を追加)

## EARS Requirements

| ID | Type | Statement | Rationale |
|---|---|---|---|
| **FR-001** | event_driven | `cmd/server` プロセス起動時、システムは `-arc-sock` フラグ(default: `ARC_SOCKET` 環境変数 → `~/.agent-reactor/arc.sock`)で解決した unix socket に `DaemonClient` を eager dial し、supervisor goroutine による再接続ループを開始しなければならない | boot race の排除と初回 REST 応答の高速化 |
| **FR-002** | ubiquitous | システムは `cmd/server` および `server/*` 配下から `termvt.Manager` および `agentlaunch.Dispatcher` を新規に instantiate してはならない(depguard で強制) | gateway 化の本旨を lint で守る |
| **FR-003** | event_driven | browser が WS `/ws` に attach したとき、システムは `CmdSurfaceSubscribe{SessionID}` を daemon に発行し、`RespOK` を受領してから WS 読み取りループを起動しなければならない。`RespErr` を受領した場合は controlMsg 通知後に typed close する | subscribe 確立前の入力 race を防止。retry は β で UI 側に実装 |
| **FR-004** | event_driven | browser から `{k:'i', d:…}` を受信したとき、システムは `CmdSurfaceWriteRaw{SessionID, Data:[]byte}` に変換して daemon に送らなければならない | 従来 surface 系命名と一貫した raw 入力経路 |
| **FR-005** | event_driven | browser から `{k:'r', cols, rows}` を受信したとき、システムは `CmdSurfaceResize{SessionID, Cols, Rows}` に変換して daemon に送らなければならない | ターミナルリサイズの contract |
| **FR-006** | event_driven | daemon `EvtSurfaceOutput{SessionID, TimeSec, DataB64, Sequence}` を受信したとき、システムは asciicast v2 配列 `[TimeSec, 'o', base64.Decode(DataB64)]` にエンコードして当該 WS に書き出さなければならない | vanilla JS UI 無改造の wire 互換 |
| **FR-007** | event_driven | daemon `EvtAgentNotification` / `EvtExit` などの control 系イベントを受信したとき、システムは既存 `controlMsg {k,code,data}` JSON に変換して当該 WS に書き出さなければならない | 既存 control wire 維持 |
| **FR-008** | event_driven | WS が close または ctx done になったとき、システムは `CmdSurfaceUnsubscribe{SessionID}` を daemon に送らなければならない | リソースリーク防止 |
| **FR-009** | state_driven | daemon socket が unreachable な状態の間、システムは REST 呼び出しに対し HTTP 503 を返し、新規 WS attach 試行にも 503 を返さなければならない | graceful degrade |
| **FR-010** | event_driven | daemon socket が unreachable に遷移したとき、システムは既存の各 WS に対し best-effort で `controlMsg {k:'c', code:'daemon-disconnected'}` を送出した後、`StatusGoingAway + reason='daemon-disconnected'` で typed close しなければならない | UI 無改造制約と将来 UX 改善余地の両立 |
| **FR-011** | event_driven | `state.Reduce` が `EvCmdSurfaceSubscribe` / `EvCmdSurfaceUnsubscribe` / `EvCmdSurfaceResize` / `EvCmdSurfaceWriteRaw` を受けたとき、システムは対応する `Eff*Surface*`(Start/Stop/Resize/WriteRaw)と `EffSendResponse(RespOK)` を発行しなければならない | 純粋 reducer による意思決定の集約 |
| **FR-012** | event_driven | `EvCmdSurfaceSubscribe` を受けたとき、システムは `State.Subscribers.Surface[ConnID][SessionID]` を有効化し、unsubscribe 時に該当エントリを削除しなければならない | broadcast 宛先判定の真実を state に置き reducer purity を保つ |
| **FR-013** | unwanted | もし 1 `ConnID` あたりの surface subscribe 件数が 8 を超えたなら、システムは `EvCmdSurfaceSubscribe` に対し `RespErr(ResourceExhausted)` を返し state を更新してはならない | fan-out 爆発抑止 |
| **FR-014** | event_driven | runtime が `EffSurfaceSubscribeStart` を interpret したとき、システムは `SessionID` から該当 frame の `TargetID`(paneID)を解決し、`PtyBackend.SubscribeSurface(paneID)` を呼び、per-(ConnID, SessionID) で fan-out goroutine を起動して subscription handle を内部 map に保存しなければならない | termvt の id 規約(paneID)と state の SessionID のギャップを runtime で吸収 |
| **FR-015** | event_driven | runtime が `EffSurfaceSubscribeStop` を interpret したとき、システムは保存した subscription handle で `termvt.Session.Unsubscribe` を呼び、対応する fan-out goroutine を終了させなければならない | Unsubscribe の対称契約 |
| **FR-016** | event_driven | runtime が `EffSurfaceResize` / `EffSurfaceWriteRaw` を interpret したとき、システムは `PtyBackend.ResizeSurface` / `PtyBackend.WriteSurface` を該当 paneID に対して呼ばなければならない | resize/write の物理経路 |
| **FR-017** | unwanted | もし termvt が slow-subscriber を理由に subscriber channel を close したなら、システムは当該 `(ConnID, SessionID)` の relay を停止し、internal event 経由で `State.Subscribers.Surface` から該当エントリを削除し、他の購読には影響を与えてはならない | backpressure containment と state 整合 |
| **FR-018** | event_driven | `termvt.Session.Subscribe` が最初のフレームとして reattach snapshot を返したとき、システムはそれを `EvtSurfaceOutput{Sequence: 0, DataB64: base64(snapshot)}` として 1 フレームで該当 `ConnID` にのみ送出しなければならない | termvt の reattach snapshot 仕様を明示的に契約化 |
| **FR-019** | ubiquitous | システムは `EvtSurfaceOutput.Sequence` を **subscribe 単位**で単調増加させ、subscribe 開始時に 0 にリセットする規約に従わなければならない(proto godoc に明文化) | drop 検知契約の明示化、β UI での gap 検知の土台 |
| **FR-020** | optional | driver から prompt phase 変更が発火された場合(β 以降)、システムは `EvtPromptEvent{FrameID, Phase, ExitCode, NowRFC}` を encode し、`State.Frames[FrameID].SessionID` を購読中の `ConnID` にのみ送出しなければならない | α は encoder と経路のみ用意、driver 接続は β で 1 行追加 |
| **FR-021** | ubiquitous | システムは新コマンド(`CmdSurfaceSubscribe` / `Unsubscribe` / `Resize` / `WriteRaw`)と新イベント(`EvtSurfaceOutput` / `EvtPromptEvent`)を `proto.DecodeCommand` / `proto.DecodeEvent` / Fuzz テストで往復可能にしなければならない | wire の closed sum 型を維持 |
| **FR-022** | ubiquitous | システムは REST `POST /api/sessions` のリクエスト `{command, project, cols, rows}` を `CmdCreateSession{Project, Command, Options:{cols, rows}}` に詰め替え、daemon の応答 `SessionID` を既存 wire 形式でブラウザに返さなければならない | wire backward compatibility 維持 |
| **FR-023** | ubiquitous | システムは bearer token / ephemeral WS ticket / strict CSP `script-src 'self'` を α では無改変で維持しなければならない | auth invariant |
| **FR-024** | unwanted | もし `EvCmdSurfaceSubscribe` 受領時に対象 `Sessions[sid].ActiveFrame()` が `nil` なら、システムは `RespErr(Code:'frame-not-ready')` を発行し state を更新してはならず、runtime に `EffSurfaceSubscribeStart` を発行してもならない | race 対応の β 持ち越し([ADR 0018](../adr/0018-defer-subscribe-race-to-beta.md)) |

## Architecture Decision Records

| ID | Title | Status |
|---|---|---|
| [ADR 0005](../adr/0005-cmd-server-as-arc-daemon-gateway.md) | `cmd/server` を arc daemon の `proto.Client` gateway へ再定義する | accepted |
| [ADR 0006](../adr/0006-surface-namespace-for-new-proto-commands.md) | 新 proto 系を Surface 概念に統一して 4 cmd + 2 evt で表現する | accepted |
| [ADR 0007](../adr/0007-subscribers-surface-on-state.md) | subscribe 関係を `State.Subscribers.Surface[ConnID][SessionID]` として state に持つ | accepted |
| [ADR 0008](../adr/0008-session-id-to-pane-id-resolution-in-runtime.md) | SessionID → paneID 解決を runtime で行い `termvt.Manager.Get` は paneID で呼ぶ | accepted |
| [ADR 0009](../adr/0009-ptybackend-subscribe-surface-accessor.md) | `PtyBackend` に `SubscribeSurface(paneID)` accessor を export して termvt 到達を 1 経路にする | accepted |
| [ADR 0010](../adr/0010-surface-output-sequence-per-subscribe.md) | `EvtSurfaceOutput.Sequence` は daemon-side subscribe-scope + subscribe 開始時 0 リセット | accepted |
| [ADR 0011](../adr/0011-two-step-ws-close-on-daemon-disconnect.md) | daemon disconnect 時は control frame 通知 → typed close の 2 段で WS を閉じる | accepted |
| [ADR 0012](../adr/0012-daemon-client-eager-dial-supervisor.md) | `daemon_client` は eager dial + full jitter exp backoff(250ms→4s, 無制限)+ `/healthz` 露出 | accepted |
| [ADR 0013](../adr/0013-attacher-interface-and-protofake.md) | `Attacher` interface を維持し `DaemonAdapter` と `protofake` の両方が満たす | accepted |
| [ADR 0014](../adr/0014-server-session-legacy-build-tag.md) | `server/session` 一式は build tag `legacy_session` で隔離(ε で `git rm`) | accepted |
| [ADR 0015](../adr/0015-a1-alpha-three-pr-split.md) | A1-α PR を 3 段階に分割する(proto → state → runtime+gateway) | accepted |
| [ADR 0016](../adr/0016-depguard-server-layer-rule.md) | depguard の `server/*` ルールを明文化し `client/proto`, `client/state`, `client/runtime` を許可する | accepted |
| [ADR 0017](../adr/0017-platform-socketpath-helper.md) | socket path 解決を `platform/socketpath` に切り出し daemon と server で共有する | accepted |
| [ADR 0018](../adr/0018-defer-subscribe-race-to-beta.md) | ActiveFrame 未確定時の subscribe race 対応を β に倒す(α は `RespErr(frame-not-ready)` 即返) | accepted |

## Components(α 全体)

> 詳細な責務は各 ADR の Decision / Consequences 節を参照。

### `client/proto`
- `surface_command.go` (新設): `CmdSurfaceSubscribe` / `CmdSurfaceUnsubscribe` / `CmdSurfaceResize` / `CmdSurfaceWriteRaw` 型と `CmdName` 定数。`Data` は `[]byte`(wire 上 base64)
- `surface_event.go` (新設): `EvtSurfaceOutput{SessionID, TimeSec, DataB64, Sequence}` / `EvtPromptEvent{FrameID, Phase, ExitCode, NowRFC}`。godoc に Sequence は subscribe 単位 reset を明記
- `codec.go` (修正): `DecodeCommand` / `DecodeEvent` / `DecodeResponseByCommand` の switch に 4 cmd + 2 evt を追加
- `protofake/protofake.go` (新設): `net.Pipe` + ndjson encoder の最小フェイク。公開 API は `NewPair() (*ClientSide, *ServerSide)` と `Close()` のみ

### `client/state`
- `event.go` (修正): `EvCmdSurface{Subscribe,Unsubscribe,Resize,WriteRaw}` 追加(`ConnID`, `ReqID`, `SessionID`, payload)
- `effect.go` (修正): `EffSurface{SubscribeStart,SubscribeStop,Resize,WriteRaw}` / `EffBroadcastSurfaceOutput` / `EffBroadcastPromptEvent` 追加
- `state.go` (修正): `Subscribers` struct に `Surface map[ConnID]map[SessionID]struct{}` 追加(in-memory、永続化対象外)
- `reduce_surface.go` (新設): 4 `EvCmdSurface*` → `Eff*Surface*` + `EffSendResponse` の純粋 reducer。`Subscribers.Surface` を更新。1 `ConnID` 上限 8 判定。`EvConnDisconnect` で当該 `ConnID` の全 Surface entry を削除し `EffSurfaceSubscribeStop` を全件分発火。`ActiveFrame() == nil` なら `RespErr(frame-not-ready)`
- `reduce.go` / `event_dispatch.go` (修正): 新 `Ev*Surface*` を `reduce_surface.go` へルーティング

### `client/runtime`
- `pty_backend.go` (修正): `SubscribeSurface(paneID) (*termvt.Subscription, error)` / `WriteSurface(paneID, data)` / `ResizeSurface(paneID, cols, rows)` を追加。tmux backend は not-implemented を返す
- `terminal_relay.go` (新設): `FileRelay` と同形の reducer-bypass goroutine。`EffSurfaceSubscribeStart` で `SessionID → ActiveFrame.TargetID` 解決 → `PtyBackend.SubscribeSurface(paneID)`、subscription を per-(ConnID, SessionID) で fan-out。snapshot frame を `Sequence=0` で送出。subscriber close 検知で internal event 経由で `Subscribers.Surface` から削除
- `proto_bridge.go` (修正): `internalBroadcastSurface` 受信時に `EvtSurfaceOutput` を encode し、`State.Subscribers.Surface` に従って該当 `ConnID` outbox にのみ送る。`Eff*Surface*` の interpret は `terminal_relay` の public method を呼ぶ
- `runtime.go` (修正): `terminalRelay *TerminalRelay` を Runtime に追加、`NewRuntime` で起動 / `Close` で停止。`internalBroadcastSurface` / `internalSurfaceClosed` を `internalEvent` に追加
- `convert.go` / `interpret.go` (修正): `Eff*Surface*` family の interpret 振り分けを 4 行追加

### `platform/socketpath`
- `socketpath.go` (新設): `ResolveDaemonSocket(flag, envName, fallback)`。daemon と server で共有。stdlib only

### `server/web`
- `daemon_client.go` (新設): `proto.Client` wrapper。eager dial、supervisor goroutine が full jitter exp backoff(250ms→4s, 無制限)。`Health()` / `LastError` / `LastAttemptAt` を atomic 公開。disconnect 中は `ErrDaemonUnavailable`。再接続時は旧 Events chan close → 全 `AttachWS` が typed close → 新 chan で次を受ける
- `gateway.go` (修正): `Attacher` interface 維持、`DaemonAdapter` で実装。`AttachWS` は `readInbound` / `writeOutbound` / `subscribeLifecycle` の 3 関数(各 ≤80 行)に分割、`writeTypedClose(reason)` を集約。disconnect 検知時は `{k:'c', code:'daemon-disconnected'}` → typed close の 2 段
- `wire.go` (修正): `EvtSurfaceOutput` → asciicast v2 配列、`EvtAgentNotification` → `{k,code,data}` 変換を 1 ファイル集約
- `mux.go` (修正): `Sessions` interface 撤去、`DaemonClient` 直接依存。REST `/api/sessions` GET/POST/DELETE は `proto.Client` 経由で既存 cmd を呼ぶ adapter。cols/rows は `CreateSessionParams.Options` に詰める。`Health()=false` で HTTP 503

### `cmd/server`
- `main.go` (修正): `session.NewService` / `termvt.Manager` / `agentlaunch.Dispatcher` 生成を撤去。`-arc-sock` フラグ追加。`platform/socketpath.ResolveDaemonSocket` で解決。`daemon_client` を boot 時に dial して mux に注入。`/healthz` handler 追加

### `server/session/`(隔離)
- 全 `.go` ファイル + 既存テストに `//go:build legacy_session` 付与。本体ビルド・通常テストから隔離(ε で `git rm`)

### Lint / docs
- `src/.golangci.yml` (修正): depguard に「`server/*` は `client/proto`, `client/state`, `client/runtime`, `platform/*` を import 可、`orchestrator/*` 不可」+「`server/*` と `cmd/server` からは `termvt.NewManager` / `agentlaunch.NewDispatcher` の direct call を禁止」
- `docs/ARCHITECTURE.md` (修正): `server/*` を client layer の HTTP gateway として位置付ける節を追加

### テスト
1. `client/proto`: `codec_surface_test.go` + `FuzzDecodeCommand` カバレッジ
2. `client/state/reduce_surface_test.go`: 4 `Ev` → `Eff` + `Subscribers.Surface` 更新 + 上限超過 `RespErr` + `EvConnDisconnect` 一括解除 + ActiveFrame nil → `frame-not-ready` の table test
3. `client/runtime/terminal_relay_test.go`: fake `PtyBackend` で fan-out / snapshot Seq=0 / sequence 単調増加 / slow-close 自動 unsubscribe
4. `server/web/gateway_terminal_test.go`: `protofake` + `net.Pipe` で subscribe → output → asciicast v2 配列出力 → unsubscribe 通し試験
5. `server/web/mux_daemon_test.go`: `Health=false` で 503
6. `server/web/daemon_client_test.go`: dial → disconnect → reconnect / in-flight request cancellation

## PR 分割(ADR 0015)

| PR | スコープ | LOC 推定 | 依存 |
|---|---|---|---|
| **PR-1** | `client/proto`: `surface_command.go`, `surface_event.go`, `codec.go` 修正, `protofake/`, Fuzz テスト | ~300-450 | なし |
| **PR-2** | `client/state`: `event.go`, `effect.go`, `state.go`(`Subscribers.Surface`), `reduce_surface.go`, dispatch 修正, `reduce_surface_test.go` | ~250-400 | PR-1 |
| **PR-3** | `client/runtime`(`terminal_relay`, `pty_backend`, `proto_bridge`, `runtime`, `convert`, `interpret`)+ `server/web`(`daemon_client`, `gateway`, `wire`, `mux`)+ `cmd/server` + `platform/socketpath` + depguard + `server/session` build tag 隔離 + 関連テスト | ~600-900 | PR-2 |

各 PR は単独で `make build-all && cd src && go test ./... -race` が green を維持する。詳細根拠は [ADR 0015](../adr/0015-a1-alpha-three-pr-split.md)。

## Resolved Issues

否定役(critic)が指摘した blocker 4 件 + major 7 件 + minor 5 件と、最適化役(optimizer)が提案した改善 14 件のうち、最終計画で採用 / 解消した記録。

### Blockers(全 4 件 resolved)

1. **`termvt.Manager.Get` は paneID で引く必要があるが initial draft は SessionID で引いていた** → [ADR 0008](../adr/0008-session-id-to-pane-id-resolution-in-runtime.md) で `SessionID → ActiveFrame.TargetID` 解決を runtime に置く設計に変更。FR-014 に明示
2. **`PtyBackend` が `termvt.Manager` を unexported で保持しており relay から到達できない** → [ADR 0009](../adr/0009-ptybackend-subscribe-surface-accessor.md) で `SubscribeSurface`/`WriteSurface`/`ResizeSurface` accessor を追加。termvt API は無改変のまま backend 抽象を対称化
3. **reducer purity と『per-(ConnID, SessionID) 宛先絞り』broadcast の両立矛盾** → [ADR 0007](../adr/0007-subscribers-surface-on-state.md) で『購読関係を `State.Subscribers.Surface` に正式 field として持ち Reduce で更新』へ変更
4. **REST `/api/sessions` の wire(cols/rows)と daemon の `CreateSessionParams`(cols/rows 受けない)の semantic gap** → FR-022 で『cols/rows は `CreateSessionParams.Options` に詰める』adapter を明示

### Majors(全 7 件 resolved)

5. **depguard に `server/*` layer 規約が無く `client/*` を import するルートが未明示** → [ADR 0016](../adr/0016-depguard-server-layer-rule.md)
6. **`termvt.Subscribe` の reattach snapshot 仕様を data flow が無視していた** → FR-018 で「snapshot は `EvtSurfaceOutput{Sequence:0}` で 1 発で送出」と明文化。[ADR 0010](../adr/0010-surface-output-sequence-per-subscribe.md) で subscribe 単位 reset 規約を確定
7. **termvt 側 close と outbox bounded drop の二段ポリシ矛盾** → FR-017 で「termvt slow-close → internal event で `Subscribers.Surface` 削除」と明示。`terminal_relay` 責務に追加
8. **`proto.Client` の close once / reconnect 戦略が空洞** → [ADR 0012](../adr/0012-daemon-client-eager-dial-supervisor.md) で再接続フローと in-flight cancellation を確定。`daemon_client_test.go` で gate
9. **`CmdSurfaceWriteRaw` の ActiveFrame 解決責務が未定義** → [ADR 0008](../adr/0008-session-id-to-pane-id-resolution-in-runtime.md) と同経路で runtime 層解決に統一。FR-016
10. **`server/session` 空殻化が既存テストを破壊する** → [ADR 0014](../adr/0014-server-session-legacy-build-tag.md) で build tag 隔離に変更
11. **PR 単位の分離戦略が未定義** → [ADR 0015](../adr/0015-a1-alpha-three-pr-split.md) で 3 段階分割を確定

### Minors(全 5 件 resolved)

12. **typed close 単体では UI 無改造で UX が告知不能** → [ADR 0011](../adr/0011-two-step-ws-close-on-daemon-disconnect.md) で 2 段 close。FR-010
13. **Sequence の session-scope 規約が再 subscribe で破綻** → [ADR 0010](../adr/0010-surface-output-sequence-per-subscribe.md) で subscribe 単位 reset に変更。FR-019
14. **prompt-event の driver 発火が α に無くカバレッジが Fuzz のみになる** → FR-020 を optional 型として明示。テスト群 (5) で broadcast 経路の単体テストを含める
15. **protofake が proto-isolation depguard を破る可能性** → `protofake.go` responsibility に「state/runtime を import しない、公開 API は `NewPair()` / `Close()` のみ」と制約を明示
16. **performance budget の根拠不在** → open_questions に残し、α 完了基準を「明らかな regression なし」の定性基準に緩める。具体的な計測手段は β/γ 以降で導入

### ユーザー判断による持ち越し
17. **ActiveFrame 未確定時の subscribe race** → [ADR 0018](../adr/0018-defer-subscribe-race-to-beta.md) で β 持ち越しと決定。FR-024 で reducer は `RespErr(frame-not-ready)` 即返、client retry は β の React store と一緒に実装

## Open Questions(PR-3 着手時に決める)

α の核心仕様は確定。以下は実装直前の細部のため、PR-3 着手時に短いミニ ADR か godoc 注記で決める:

1. **daemon_client 再接続時の旧 WS 全切断挙動を README に明記するか**: 現行 vanilla JS UI の `onclose` は 'detached' 表示のみで reload は手動。運用初期に user 向けに「daemon 再起動時はブラウザ reload」を `docs/user/web-server.md` に追記すべきか
2. **`/healthz` の露出範囲**: daemon の reconnect 試行回数 / 最終エラーメッセージをどこまで露出するか(運用観測性 vs 攻撃面の trade-off)
3. **`server/web/internal/` ディレクトリ化**: `daemon_client.go` / `gateway.go` を `server/web/internal/` 配下に隔離するか、現状フラット配置を維持するか(depguard ルールとの整合)

## Verification

```sh
# Build
make build-all

# Lint(depguard + funlen + staticcheck)
make lint

# Test(三層)
cd src && go test ./client/proto/... ./client/state/... ./client/runtime/... -race
cd src && go test ./server/web/... ./cmd/server/... -race
cd src && go vet ./...

# Fuzz(proto wire round-trip)
cd src && go test -fuzz=FuzzDecodeCommand -fuzztime=30s ./client/proto

# Manual smoke
make run-dev
# arc daemon を別ターミナルで起動
# browser: http://127.0.0.1:8080/#token=<printed>
# bash session を作成し、TUI と並走でキーストロークの同期を目視確認(定性パフォーマンス予算)
```

## Traceability

- **親 plan**: [Master Plan(`plans-cheerful-thompson.md`)](../../plans/arc-server-client-split.md)
- **ブランチ実行計画**: [`plans/arc-server-client-split.md`](../../plans/arc-server-client-split.md)
- **前段 PR(A0 = `bcb86a5`)**: `pty_tap` 配線で `EvPaneOsc` / `EvPanePrompt` 経路を復活
- **次の作業(α 完了後)**: A1-β(React+TS 化、wire 互換のまま)→ A1-γ(view-update broadcast)→ A1-δ(persist + connector)→ A1-ε(cleanup, `server/session` 完全削除)→ C(tmux 実装削除)
