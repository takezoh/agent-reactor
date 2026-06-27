# Plan: arc を「standalone server + remote client」に分割しきる

> Status: **A1 完了 / C 全完了** (更新 2026-06-27) · Branch: `main`
> 設計(仮設計・根拠): [remote-client-design.md](remote-client-design.md) ·
> 決定: [ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md)
> この文書は元設計の phased plan を、**実コードの現状**と突き合わせて
> 残作業に落とした実行計画。設計判断 (B) は確定(= (i) PtyBackend、ADR 0004)。
> **B1b 完了**(2026-06-19): coordinator は PtyBackend に置換、tmux ライフサイクル
> 撤去、配線前提 6 件のうち #1/#2/#3 を解消(#4/#5/#6 は moot)。
> **A0 完了**(2026-06-19): `PtyPaneTap` 新設 + `Config.Tap` 配線。driver run-state
> 検出が PtyBackend 上で復活(Claude title prefix と Shell OSC 133)。
> **A1 Master Plan 確定**(2026-06-19): 統合方針 W1(`cmd/server` を arc daemon の
> HTTP/WS gateway 化、IPC client パターン)+ React+TS frontend(Zustand)で、
> 5 PR 段階分割(α/β/γ/δ/ε)。詳細は [A1 Master Plan](/home/ubuntu/.claude/plans/plans-cheerful-thompson.md)。
> **A1-α 詳細計画確定**(2026-06-19): [`docs/specs/2026-06-19-a1-alpha-impl-plan.md`](../docs/specs/2026-06-19-a1-alpha-impl-plan.md) + ADR 0005-0018(14 件)。
> α は更に 3 PR(PR-1: proto+codec / PR-2: state reducer / PR-3: runtime relay + server/web gateway)に細分化。次は A1-α PR-1 → … → C。
> **A1 完了**(2026-06-20): A1-α/β/γ/δ/ε すべてマージ済み。`server/session/` 削除完了、
> wire vocabulary を [`docs/technical/web-gateway.md`](../docs/technical/web-gateway.md) に集約。
> **phase C 再評価**(2026-06-27): 配線は既に PtyBackend driven
> (`cmd/arc/coordinator.go:115` `NewPtyBackend` + `NewPtyPaneTap`)。残る tmux 痕跡は
> (a) dead file 2 本 + tests、(b) 識別子残存(中身は PtyBackend)に分かれる。
> **C-1 完了**(2026-06-27, `b15b05a`): `tmux_real.go` / `tmux_pipe_tap.go` +
> tests を削除、`isMissingPaneErr` の substring fallback を撤去。
> **C-2a 完了**(2026-06-27, `ec17db7`): runtime backend layer の主要識別子を
> rename(`TmuxBackend` → `PaneBackend`、`Config.Tmux` → `Config.Backend`、
> `RuntimeTmuxInjector` → `RuntimePaneInjector`、`tmux_injector.go` →
> `pane_injector.go` ほか)。詳細は §4.C / §7。
> **C-2b 完了**(2026-06-27, `63870ed`): event 名 / 関数名 / test 内 fake 型 /
> driver・state・platform 配下のコメントを完全に rename。残存する "tmux"
> 言及は外部 tmux binary 互換性 integration test、外部 ライブラリ API
> (`rasterm.IsTmuxScreen`)、ユーザー TOML config key (`[tmux]`、後方互換)
> のみで、これらは意図的に保持。

## 1. ゴール(remote-client-design.md より)

`arc` を **standalone server プロセス** と **remote client**(Web first)に分割し、
**tmux を全廃**して自前 pty multiplexer をサーバ側に持つ。pure core
(`client/state.Reduce` / `client/driver.Driver`)は**無改造で再利用**するのが
設計の中核戦略(remote-client-design.md §2)。

## 2. 現状:設計フェーズ × 実コード(2026-06-19 時点)

| フェーズ | 設計の想定 | 実コードの現状 |
|---|---|---|
| 0. transport 抽象 | arc proto を TCP+TLS+token 化(`StartIPCNet` + `Authenticator`) | ❌ 未着手。IPC は unix socket + peercred のみ(`client/runtime/ipc.go`, `peercred_*.go`)。TLS/token は web 用の別スタックにのみ存在 |
| 1. observation 完全化 | `FileRelay`-over-wire | ✅ 実装済み(`client/runtime/filerelay.go`, ipc.go, `cmd/arc/coordinator.go`) |
| 2. pty 対話(核心) | TmuxBackend → **PtyBackend** 差し替え、pure core 無改造 | ✅ **配線完了**(B1b + A0)。coordinator は `runtime.NewPtyBackend()` で起動し、tmux ライフサイクル(`tmux.NewClient`/`setupNewSession`/`ensureHiddenWindow`/`restoreSession`/`SessionExists`/`$TMUX` チェック)は撤去。`Config.Tap` は `runtime.NewPtyPaneTap(ptyBackend)` で配線済み(A0、2026-06-19)— `termvt.Session.Subscribe` の EventOutput を raw passthrough し、`tap_manager` の `vt.Terminal` 経由で OSC 0/9/133 が `EvPaneOsc`/`EvPanePrompt` として復活、driver run-state(Claude title prefix / Shell OSC 133)が PtyBackend 上で動く。`cmd/arc/tmux_layout.go` は削除済み(coordinator が呼ばなくなったため一部 phase C を前倒し)。残り tmux 本体撤去は phase C |
| 3. tmux 削除 | tmux backend 削除 | 🟡 一部前倒し(coordinator 周辺)。残りは `client/runtime/tmux_real.go`/`tmux_pipe_tap.go`/`tmux_injector.go`/`panetap.go` ほか |
| 4. orchestrator 統合 | 任意 | ❌ 未着手(optional) |

### 2.1 核心の発見:設計と実装の分岐

phase 2 で作られた web スタック(`server/web`, `client/web`)は
**`client/state` も `client/driver` も import していない**(grep 0 件)。
remote-client-design.md §2 の「backend だけ差し替えて pure core 再利用」ではなく、
**pure core をバイパスした並行実装**になっている。この分岐の解消が残作業全体の前提。

## 3. 中心の設計判断 (B):termvt をどう core に繋ぐか

**この判断を最初に確定する。** 二択:

- **(i) PtyBackend 案(設計通り・推奨)**
  `platform/termvt` を包む `PtyBackend` を作り、`client/runtime/backends.go` の
  `TmuxBackend` 役割インターフェース(`PaneLifecycle`/`PaneIO`/`WindowLayout`/
  `PaneInspect`/`SessionEnv`/`TmuxControl`)を実装。**既存 runtime/reducer/driver を
  tmux 無しで駆動**。TUI と web を 1 つの core から出せる。
  - 長所: 設計一貫・driver 知能を web でも再利用・tmux 削除が可能になる
  - 短所: 役割 IF のうち tmux 固有概念(window layout 等)の termvt への写像が必要

- **(ii) サーバ再実装案(非推奨)**
  web スタックを別のまま、status 検出 / view / persistence をサーバ側に再実装。
  - 長所: 既存 runtime に触れない
  - 短所: driver ロジックの二重実装・設計乖離が恒久化・tmux 削除に繋がらない

> **決定済み: (i) 採用** — [ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md)。以降の A/C は (i) 前提。

### 3.1 B1 設計で確定した付随決定(ADR 0004 で解決)
- **決定1: session ownership** = runtime の PtyBackend が**自前の `termvt.Manager`** を持つ。
  arc daemon と `cmd/server` は別プロセスなので各自 Manager で衝突しない。B1 では
  `server/session.Service` / `cmd/server` に触らない。web を daemon の runtime-owned
  session へ寄せる収束は **plan A**。
- **決定2: 再起動越しの reattach** = B1 では**しない**。termvt session は daemon の子で
  daemon と運命を共にする(shipped の `cmd/server` と同モデル)。session 定義は
  `SessionSnapshot` で永続し、再起動時は cold 再 spawn。session-env は in-process のみ
  (非永続と doc 明記)。daemon より長生きの supervisor は scope 外。

## 4. 残作業(依存順)

```
phase 2 web スタック = 動くが pure core 非接続の並行実装(完成)
  │
  ├─ B. 設計判断 (i)/(ii) を確定 → (i) 採用 [済 · ADR 0004]
  │     │
  │     └─ B1. PtyBackend(termvt を既存 runtime の backend に)   ← linchpin
  │           ├─ B1a. PtyBackend 型 + 単体テスト                [済 · 8ffd868 +fixup]
  │           └─ B1b. 配線 + 配線前提解消                        [済 · 2026-06-19]
  │                  • coordinator は NewPtyBackend() で起動
  │                  • tmux ライフサイクル(NewClient / setupNewSession /
  │                    ensureHiddenWindow / restoreSession / $TMUX) 撤去
  │                  • Tap=nil(pty_tap は plan A)
  │                  • #1 ErrPaneMissing sentinel 共有 / #2 spawn は sh -c 経由 /
  │                    #3 ResizeWindow target 正規化(windowIndex↔paneID map)
  │                  • #4 session-env / #5 PipePane / #6 main guard は moot
  │                  • cmd/arc/tmux_layout.go 削除
  │                 │
  │                 └─ A. web 経路で pure core 再利用
  │                       (run-state / driver view / persist / connector /
  │                        termvt.Session.Subscribe を消費する pty_tap)
  │                       │
  │                       ├─ A0. pty_tap 新設 + Config.Tap 配線      [済 · 2026-06-19]
  │                       │     • `client/runtime/pty_tap.go` 新設(`PtyPaneTap`)
  │                       │     • coordinator で `Tap: NewPtyPaneTap(ptyBackend)` 配線
  │                       │     • EventOutput を raw passthrough、EventControl は
  │                       │       tap_manager の vt.Terminal で再 parse(structured
  │                       │       経路化は A1 のリファクタ範疇)
  │                       │     • Claude title prefix / Shell OSC 133 経由の
  │                       │       run-state 遷移を test で検証(单体 7 / 配線 2)
  │                       │
  │                       └─ A1. web から pure core を経由した view / persist /  ← 次
  │                             connector
  │                       │
  │                       └─ C. tmux 実装ファイル群の削除
  │                             (`grep -ri tmux src/` = 0 を目標、
  │                              client/runtime/tmux_*.go ほか)
  │
  └─ D.(任意)arc proto の TCP+TLS+token 化 → native client(phase 0/4)
```

### B1. PtyBackend 実装(linchpin) — **済**
- **What**: `platform/termvt` を `client/runtime` の backend として包み、
  `backends.go` の役割インターフェースを満たす `PtyBackend` を新設。
- **Why**: pure core を tmux から切り離す唯一の接続点。A も C もこれ待ち。
- **触った所**: `client/runtime/backends.go`(`ErrPaneMissing` 追加),
  `client/runtime/pty_backend.go`(B1a 実装 + B1b で sh -c 経由 spawn /
  windowIndex↔paneID 解決 / sentinel ラップ),
  `client/runtime/resident.go`(`isMissingPaneErr` を `errors.Is` 対応),
  `cmd/arc/coordinator.go`(NewPtyBackend、tmux ライフサイクル撤去、Tap=nil)。
  `cmd/arc/tmux_layout.go` 削除。
- **完了条件**: ✅ `go test -race ./client/runtime/...` green / ✅ `make build-all` /
  ✅ `go tool golangci-lint run ./...` 0 issues / ✅ runtime カバレッジ 64.5%
  (PtyBackend データ面は 100%、stubbed プレゼン面は意図的に 0%)。
  注意: 本 PR では **arc TUI は動かない**(表示面なし)。表示面の復活は plan A
  の責務(web 経由で pure core を可視化する)。

#### B1-wiring の前提条件(B1b で処理)
PtyBackend を runtime に挿す上で出ていた 6 件:
1. ✅ **missing-pane エラー契約**: `backends.go` に `ErrPaneMissing` sentinel を追加。
   PtyBackend は `fmt.Errorf("runtime: unknown pane %q: %w", target, ErrPaneMissing)`
   でラップ、`isMissingPaneErr` は `errors.Is(err, ErrPaneMissing)` を見るよう拡張。
   legacy `"can't find pane"` substring パスも phase C まで残す。
2. ✅ **command の形式**: PtyBackend.SpawnWindow / RespawnPane が常に `["/bin/sh", "-c", command]`
   経由で起動する形に変更。`agentlaunch.SplitArgs` 撤去。これにより runtime の
   `exec ...` / `bash -c '…'` 等の shell 文字列がそのまま動く。
3. ✅ **ResizeWindow の target 形式**: PtyBackend が `windowIndex→paneID` の map を持ち、
   `resolvePaneTarget` で `"sess:1"` 形式の prefix strip と windowIndex→paneID 変換を行う。
   target を受ける主要メソッド全てに適用。
4. **moot — session-env の非永続**: coordinator が `restoreSession` / warm restart 経路を
   呼ばなくなったため、ADR 0004 / 決定2 の通り「セッションは daemon と運命を共にする」
   モデルに整合。永続化は後続 phase の課題。
5. **moot — PipePane=no-op**: `Config.Tap = nil` で `tap_manager` が即 return するため、
   PipePane が呼ばれないコードパスに収束。`termvt.Session.Subscribe` ベースの pty_tap
   構築は plan A の責務(run-state 検出と一緒に)。
6. **moot — KillPaneWindow main 保護なし**: coordinator が `setupNewSession` /
   `ensureHiddenWindow` を呼ばなくなり、PtyBackend には「main 0.0/0.1/0.2 = window
   index 0」概念が無いため、守るべき "main window" 自体が存在しない。

### A. web 経路で pure core 再利用
- **What**: termvt の Control event(OSC 9/133/title)を driver/state に流し込み、
  run-state(idle/running/waiting)・driver view(claude/codex/gemini の tool log・
  statusline・summarize・tags)・永続化(transcript/warm state)・connector(github)
  を web でも提供。
- **Why**: 現状 web には arc の「エージェント知能」が丸ごと無い(termvt は Control を
  出すが消費側が居ない)。
- **触る所**: `server/web/wire.go`(control の語彙拡張), `server/session/`,
  `client/web/app.js`(view 描画), pure core への bridge 層(新規)。
- **完了条件**: web で run-state 表示・tool log・status が TUI と同等に出る。

#### A0. pty_tap 新設 + Config.Tap 配線 — **済**(2026-06-19)
- **What**: `platform/termvt.Session.Subscribe` を `client/runtime.PaneTap` に
  包む `PtyPaneTap` を新設し、`Config.Tap` に配線。tap_manager → vt.Terminal の
  既存 OSC 経路がそのまま生き、`EvPaneOsc` / `EvPanePrompt` が再び流れる。
- **Why**: B1b 完了直後の状態では `Config.Tap=nil` のため、driver の run-state
  検出(Claude title prefix / Shell OSC 133)が無効化されていた。
- **設計判断**(本サブ計画の根拠):
  - **案 A(raw passthrough)** 採用 — `Event.Kind == EventOutput` のみ抽出して
    `Data` を chunk 配信、`EventControl` は破棄(tap_manager の `vt.Terminal` が
    raw bytes 経由で同じ OSC を再 parse)。`EventExit` / slow disconnect は termvt
    側の `close(ch)` を `readTap` の `!ok` が自然に拾う。
  - **案 T(独立した `PtyPaneTap` 型)** 採用 — `PtyBackend`(474 行)に混ぜると
    500 行制限を超過、責務分離も損なう。`NewPtyPaneTap(b *PtyBackend)` が
    同パッケージから `b.mgr` を共有する。
  - structured 経路化(termvt の `EventControl` を直接消費)は A1 のリファクタ
    範疇として明示的に持ち越し。
- **触った所**: `client/runtime/pty_tap.go`(新規)、`cmd/arc/coordinator.go`
  (`Tap: runtime.NewPtyPaneTap(ptyBackend)`)、`client/runtime/pty_backend.go`
  (`PipePane` TODO を pty_tap 代替済み表現に更新)。
- **完了条件**: ✅ 单体 7 ケース(`pty_tap_test.go`) + 配線 2 ケース
  (`pty_tap_wire_test.go`)で `EvPaneOsc{Cmd:0, Title:"Braille"}` /
  `EvPanePrompt{Phase: Command/Complete, ExitCode: 0}` の enqueue を確認 /
  ✅ `go test -race ./client/runtime/... ./platform/termvt/...` green /
  ✅ `make build-all` / ✅ `golangci-lint run ./...` 0 issues。
- **注意**: 本 A0 PR でも **arc TUI は動かない**(表示面なし)。表示面の復活は
  A1(web 経由)で扱う。

#### A1. web から pure core を経由した view / persist / connector — **進行中**(Master Plan 確定)
- A0 で pure core まで OSC が届くようになったので、A1 は web 側(`server/web` /
  `client/web`)を pure core consumer に書き換える作業。
- **Master Plan**(2026-06-19): [A1 Master Plan](/home/ubuntu/.claude/plans/plans-cheerful-thompson.md)。
  - 統合方針 W1(`cmd/server` を arc daemon の HTTP/WS gateway 化、IPC client
    パターン。`server/session.Service` は削除し、session ownership は daemon 単独)
  - Frontend: React + TypeScript(vite + Zustand + xterm.js)
  - 5 PR 段階分割:
    - A1-α: `cmd/server` を arc daemon の gateway 化(wire 互換維持、~600-900 行)
      - **詳細計画確定**: [`docs/specs/2026-06-19-a1-alpha-impl-plan.md`](../docs/specs/2026-06-19-a1-alpha-impl-plan.md) + [ADR 0005-0018](../docs/adr/)。
      - 更に 3 PR に細分化(ADR 0015): PR-1 proto+codec(~300-450 行)/ PR-2 state reducer + `Subscribers.Surface`(~250-400 行)/ PR-3 runtime relay + server/web gateway + daemon_client(~600-900 行)。
      - subscribe race 対応は β に倒した(ADR 0018)。α は `RespErr(frame-not-ready)` 即返。
    - A1-β: React+TS frontend 置換(wire 互換のまま、~1500-2500 行)
    - A1-γ: view-update broadcast(run-state / driver view / tool log、~600-1000 行)
    - A1-δ: 永続化(transcript / event-log tail)+ connector(github)(~1000-1500 行)
    - A1-ε: cleanup + `server/session` 完全削除(~300-600 行)
- 各サブ PR の詳細 plan は A0 と同じく実装直前に別 plan ファイルで切り出す。
- 次は A1-α。

### C. tmux 全削除(phase 3)— **dead file 削除 + 識別子 rename**
- **2026-06-27 再評価**: 配線は A1 完了時点で既に PtyBackend driven。残作業は
  「dead file 削除 + 識別子の機械的 rename」に縮退。phase は C-1 / C-2a / C-2b に分割。

#### C-1. dead file 削除 — **済**(2026-06-27, `b15b05a`)
- 削除:
  - `client/runtime/tmux_real.go`(`RealTmuxBackend`、`platform/lib/tmux` import)
  - `client/runtime/tmux_real_test.go`
  - `client/runtime/tmux_pipe_tap.go`(`TmuxPipePaneTap`、`pty_tap.go` に置換済み)
- 関連整理:
  - `resident.go` の `isMissingPaneErr` から legacy "can't find pane" substring
    fallback を撤去(`errors.Is(err, ErrPaneMissing)` のみに統一)
  - `backends.go` 冒頭の `RealTmuxBackend` 言及コメントを削除
  - `runtime_test.go` の fake error を substring から `ErrPaneMissing` ラップに変更
    (`tmux: can't find pane: %3` → `runtime: unknown pane %q: %w`)
- 結果: `platform/lib/tmux` への runtime 側 import が消滅。
  package 本体も code-review #2 のフォローアップで削除(orphan を抱え続ける
  意味が無いため)。

#### C-2a. runtime backend layer の識別子 rename — **済**(2026-06-27, `ec17db7`)
- 完了した rename:
  - interface: `TmuxBackend` → `PaneBackend`、`TmuxControl` → `BackendControl`、
    `TmuxInjector` → `PromptInjector`
  - field: `runtime.Config.Tmux` → `runtime.Config.Backend`
  - struct: `noopTmux` → `noopBackend`、`RuntimeTmuxInjector` → `RuntimePaneInjector`
  - constructor: `NewRuntimeTmuxInjector` → `NewRuntimePaneInjector`
  - file rename: `tmux_injector.go` → `pane_injector.go`、`tmux_injector_test.go`
    → `pane_injector_test.go`
  - 内部: `spawnDeps.tmux` → `spawnDeps.backend`
  - コメント追従: `pty_backend.go` / `launcher.go` / `sandbox/manager.go` /
    `backends.go` ほか
- 触らなかったもの(意図的):
  - **ユーザー向け config**(`coordinator.Config.Tmux.SessionName`,
    `.PaneRatioVertical`)は ~/.agent-reactor/config.yaml に直結するので
    後方互換のため **rename しない**。`cmd/arc/coordinator.go` 内では
    `runtime.Config.Tmux` field 初期化のみ `Backend:` に追従。

#### C-2b. 残存 "tmux" 命名の rename — **済**(2026-06-27, `63870ed`)
- 完了した rename:
  - **内部 event 名**: `EvTmuxPaneSpawned` → `EvPaneSpawned`、
    `EvTmuxSpawnFailed` → `EvSpawnFailed`、`EvTmuxWindowVanished` → `EvPaneWindowVanished`
  - **effect 名**: `EffSpawnTmuxWindow` → `EffSpawnPaneWindow`、
    `EffSetTmuxEnv` / `EffUnsetTmuxEnv` → `EffSetPaneEnv` / `EffUnsetPaneEnv`、
    `EffSendTmuxKeys` → `EffSendPaneKeys`
  - **関数名**: `spawnTmuxWindow` → `spawnPaneWindow`、
    `executeTmuxEffect` → `executePaneEffect`、`executeSendTmuxKeys` → `executeSendPaneKeys`、
    `reduceTmux*` → `reducePane*` / `reduceSpawnFailed`
  - **test ローカル fake**: `fakeTmuxBackend` → `fakeBackend`、`newFakeTmux` → `newFakeBackend`、
    `recordingTmux` → `recordingBackend`、変数名 `tmux` / `ftmux` → `backend` / `fbackend`
  - **ユーザー config 識別子**: `runtime.Config.Tmux` → `Config.Pane`、
    `TmuxConfig` → `PaneConfig`、`coordinator` 内 `cfg.Tmux.*` → `cfg.Pane.*`
    (TOML key は `toml:"tmux"` を保持して既存設定ファイル互換)
  - **diagnostic 文字列**: `"tmux-spawned:" → "pane-spawned:"`、`"EvTmux*" → "EvPane*"` 等
  - **driver / state / lib / platform 配下のコメント整理**
- 意図的に残した tmux 言及:
  - `client/driver/vt/{osc_capture,osc_pipe,tmux_passthrough}_test.go` — 実 tmux
    binary との互換性検証 integration test。`exec.Command("tmux", ...)` で実 tmux
    を呼ぶので tmux 識別子そのものが正しい。
  - `client/tui/image/image.go` の `rasterm.IsTmuxScreen()` — 外部ライブラリ API。
    関連コメント・テストも外部 multiplexer 検出ロジックなので保持。
  - `client/config/config.go` の `toml:"tmux"` struct tag および
    `config_test.go` の `[tmux]` TOML 後方互換テスト。
- 結果: `grep -ri "[Tt]mux" src/` の残存は上記 意図的保持カテゴリのみ。
  build / test / lint 全 green、0 issues。

### D.(任意)native client 用 proto remote 化
- **What**: `StartIPCNet`(TCP+TLS+token)+ `Authenticator` seam + proto TLS dialer。
- **Why**: native(Go TUI / Rust)クライアントが typed proto で remote 接続するため。
- **判断**: 設計上 native は「後で任意」。web で完結する間は後回し可。

## 5. リスク(remote-client-design.md §7 Risks + 本調査)

- **VT 忠実度**: `x/vt` が tmux のエッジケース(copy-mode, truecolor, terminfo)に
  耐えるか。緩和 = v1 は raw passthrough(クライアント実端末が emulation)。
- **役割 IF の写像**: データ面は termvt に全写像済み。tmux 固有のプレゼン面
  (`WindowLayout`/`TmuxControl`)は B1 で stub、phase C で client-side layout へ(解決)。
- **reattach atomicity / backpressure**: snapshot + subscribe の原子性、遅いクライアント
  の切断(remote-client-design.md §7)。termvt は fan-out 済みだが境界の再確認が必要。
- **二重実装の恒久化**: (ii) を選ぶと driver ロジックが二系統に分裂する。

## 6. 未確定・要裏取り

- [x] **B の方針 (i)/(ii)** を確定 → (i) 採用([ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md))
- [x] termvt の Control event が pure core に未接続を確認(`server/web`/`server/session` は `client/state`/`client/driver` を import せず)
- [x] `backends.go` 役割 IF の写像 → データ面は termvt に全写像、`WindowLayout`/`TmuxControl` は server 等価物無し→client-side
- [x] **session ownership** → 決定1(自前 termvt.Manager、§3.1 / ADR 0004)
- [x] **再起動越し reattach** → 決定2(B1 ではしない、§3.1 / ADR 0004)
- [x] **B1 配線前提 6 件**(§B1 "B1-wiring の前提条件")— #1/#2/#3 解消、#4/#5/#6 は moot
- [ ] web で複数ペイン同時表示(layout)をどの phase で入れるか(C 後の client-side layout 想定)
- [x] **A の最初の課題**: `termvt.Session.Subscribe` を読む `pty_tap` (PaneTap 実装)を新設し、`Config.Tap` に挿す → A0 として完了(2026-06-19、`client/runtime/pty_tap.go`)。driver run-state 検出が pty backend 上で復活
- [ ] **B1b で coordinator から外した warm-recovery hooks**(documented divergence):
  - `RecoverSandboxFrames` — 持続コンテナを再 Adopt する `AdoptFrame` 経路。
    daemon 再起動でコンテナが孤児化しうる(devcontainer 利用時の実害)。plan A
    で web 経由で recover 同等の機能を提供するか、coldStart にも呼べるよう
    bootstrap.go を整理する。
  - `RecoverWarmStartSessions` — driver の WarmStartRecoverer hook
    (codex 等 durable driver の resume-from-checkpoint)。同様に daemon 再起動で
    呼ばれない。
  - `ReconcileOrphans` / `RecoverActivePaneAtMain` — tmux 特化なので削除でよいが、
    pure-core 駆動の orphan reconcile は plan A で再設計が要る。

## 7. 次アクション

1. ~~B の方針を決める~~ → (i) 採用(裏取り完了)
2. ~~決定を ADR 化~~ → [ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md)(session ownership / reattach も解決済み)
3. ~~B1a: PtyBackend 型 + 単体テスト~~ → 実装・レビュー済み(`8ffd868` + fixup 群)
4. ~~B1b: 配線 + 前提解消~~ → 2026-06-19 完了。coordinator は PtyBackend で起動、
   tmux ライフサイクル撤去、`Config.Tap=nil`、#1/#2/#3 解消、
   `cmd/arc/tmux_layout.go` 削除
5. ~~A0: pty_tap 新設 + Config.Tap 配線~~ → 2026-06-19 完了。
   - `client/runtime/pty_tap.go` で `PtyPaneTap` を新設(案 A:raw passthrough、
     案 T:独立型 + 同パッケージから `PtyBackend.mgr` 共有)
   - coordinator(`cmd/arc/coordinator.go:148`)で
     `Tap: runtime.NewPtyPaneTap(ptyBackend)` を配線
   - `pty_backend.go` の `PipePane` TODO を pty_tap で代替済みの表現に書き換え
   - 単体 7 ケース(`pty_tap_test.go`)+ 配線 2 ケース(`pty_tap_wire_test.go`)で
     `EvPaneOsc{Cmd:0, Title:"Braille"}` / `EvPanePrompt{Phase: Command/Complete}`
     が enqueue されることを確認
6. **A1 Master Plan 確定**(2026-06-19): [A1 Master Plan](/home/ubuntu/.claude/plans/plans-cheerful-thompson.md)。
   - 統合方針 W1(`cmd/server` を arc daemon の HTTP/WS gateway 化)
   - Frontend = React + TypeScript(vite + Zustand)
   - 5 PR 段階分割(A1-α/β/γ/δ/ε)
7. **A1 完了**(2026-06-20):
   - A1-α(`cmd/server` を arc daemon の gateway 化、ADR 0005-0018)
   - A1-β(React + TypeScript frontend、ADR 0019-0022)
   - A1-γ(view-update broadcast、ADR 0023-0024)
   - A1-δ(persist + connector + notification、ADR 0025-0027)
   - A1-ε(`server/session/` 削除 + wire doc 集約)
   - wire vocabulary は [`docs/technical/web-gateway.md`](../docs/technical/web-gateway.md) に集約。
   - `server/session/` ディレクトリと `legacy_session` build tag は ε で完全削除。
8. **C 再評価**(2026-06-27): A1 完了時点で配線は既に PtyBackend driven
   (`cmd/arc/coordinator.go:115` `NewPtyBackend` + `NewPtyPaneTap`)。
   残る tmux 痕跡は (a) dead file 2 本 + tests、(b) 識別子残存。
9. ~~**C-1: dead file 削除**~~ → 2026-06-27 完了(`b15b05a`)。
   `tmux_real.go` / `tmux_pipe_tap.go` + test を削除、
   `isMissingPaneErr` から substring fallback を撤去。
10. ~~**C-2a: runtime backend layer の rename**~~ → 2026-06-27 完了(`ec17db7`)。
    `TmuxBackend` → `PaneBackend`、`Config.Tmux` → `Config.Backend`、
    `RuntimeTmuxInjector` → `RuntimePaneInjector`、ファイル rename、
    `cmd/arc/coordinator.go` の `runtime.Config` field 初期化を追従。
11. ~~**C-2b: 残存 tmux 命名の rename**~~ → 2026-06-27 完了(`63870ed`)。
    event / effect / 関数 / test fake / コメント / ユーザー config の
    Go 識別子から完全に "tmux" を除去。意図的に残したのは外部 tmux binary
    互換性テスト、外部ライブラリ API、ユーザー TOML key のみ。
12. **§6 未消化の積み残し**(C と独立、別 plan で着手):
    - `RecoverSandboxFrames` 再設計(devcontainer 孤児化の実害)
    - `RecoverWarmStartSessions` 再設計(codex durable driver の resume)
    - `ReconcileOrphans` / `RecoverActivePaneAtMain` の pure-core 駆動版
    - web で複数ペイン同時表示(layout)の phase 配置決定
