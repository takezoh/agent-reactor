# Plan: arc を「standalone server + remote client」に分割しきる

> Status: **In progress** (更新 2026-06-19) · Branch: `feat/tmux-free-web-server`
> 設計(仮設計・根拠): [remote-client-design.md](remote-client-design.md) ·
> 決定: [ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md)
> この文書は元設計の phased plan を、**実コードの現状**と突き合わせて
> 残作業に落とした実行計画。設計判断 (B) は確定(= (i) PtyBackend、ADR 0004)。
> **B1b 完了**(2026-06-19): coordinator は PtyBackend に置換、tmux ライフサイクル
> 撤去、配線前提 6 件のうち #1/#2/#3 を解消(#4/#5/#6 は moot)。
> 次は A → C。

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
| 2. pty 対話(核心) | TmuxBackend → **PtyBackend** 差し替え、pure core 無改造 | ✅ **配線完了**(B1b)。coordinator は `runtime.NewPtyBackend()` で起動し、tmux ライフサイクル(`tmux.NewClient`/`setupNewSession`/`ensureHiddenWindow`/`restoreSession`/`SessionExists`/`$TMUX` チェック)は撤去。`Config.Tap=nil` で `tap_manager` を no-op に倒す。`cmd/arc/tmux_layout.go` は削除済み(coordinator が呼ばなくなったため一部 phase C を前倒し)。残り tmux 本体撤去は phase C |
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
  │                 └─ A. web 経路で pure core 再利用                ← 次
  │                       (run-state / driver view / persist / connector /
  │                        termvt.Session.Subscribe を消費する pty_tap)
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

### C. tmux 全削除(phase 3)
- **What**: tmux backend と関連を削除。`cmd/arc/tmux_layout.go`, `client/runtime` の
  tmux 実装(tmux_real / tmux_injector / tmux_pipe_tap / panetap)を撤去し、
  client-side layout に置換。
- **Why**: 設計の最終形(local == remote、transport だけ違う)。
- **触る所**: 56 ファイル(下記 grep)。reducer/driver/state の tmux 参照も含む。
- **完了条件**: `grep -ri tmux src/` = 0 / 全テスト green。
- **前提**: B1 + A 完了(代替 backend 無しに消すと arc が壊れる)。

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
- [ ] **A の最初の課題**: `termvt.Session.Subscribe` を読む `pty_tap` (PaneTap 実装)を新設し、`Config.Tap` に挿す。これが入ると driver run-state 検出が pty backend 上で復活する
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
5. **A: web で pure core 再利用** ← 次。
   - 5a. `pty_tap` 新設(`termvt.Session.Subscribe` を `PaneTap` インターフェースに包む)
   - 5b. `Config.Tap` に pty_tap を挿し、driver run-state 検出を復活
   - 5c. web から pure core を経由した view / persist / connector に
6. C: tmux 実装の残りを削除(56 ファイルから漸減 — `cmd/arc/tmux_layout.go` は B1b で削除済み)
