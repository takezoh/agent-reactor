# Plan: arc を「standalone server + remote client」に分割しきる

> Status: **Planning** (2026-06-18) · Branch: `feat/tmux-free-web-server`
> 設計(仮設計・根拠): [remote-client-design.md](remote-client-design.md)
> この文書は元設計の phased plan を、**実コードの現状**と突き合わせて
> 残作業に落とした実行計画。設計判断 (B) が未確定なので、まずそこを決める。

## 1. ゴール(remote-client-design.md より)

`arc` を **standalone server プロセス** と **remote client**(Web first)に分割し、
**tmux を全廃**して自前 pty multiplexer をサーバ側に持つ。pure core
(`client/state.Reduce` / `client/driver.Driver`)は**無改造で再利用**するのが
設計の中核戦略(remote-client-design.md §2)。

## 2. 現状:設計フェーズ × 実コード(2026-06-18 時点)

| フェーズ | 設計の想定 | 実コードの現状 |
|---|---|---|
| 0. transport 抽象 | arc proto を TCP+TLS+token 化(`StartIPCNet` + `Authenticator`) | ❌ 未着手。IPC は unix socket + peercred のみ(`client/runtime/ipc.go`, `peercred_*.go`)。TLS/token は web 用の別スタックにのみ存在 |
| 1. observation 完全化 | `FileRelay`-over-wire | ✅ 実装済み(`client/runtime/filerelay.go`, ipc.go, `cmd/arc/coordinator.go`) |
| 2. pty 対話(核心) | TmuxBackend → **PtyBackend** 差し替え、pure core 無改造 | ⚠️ **別物として実装**。`platform/termvt` + `server/session` + `server/web` + `client/web` の並行スタックを新設。既存 runtime に挿す `PtyBackend` は存在しない |
| 3. tmux 削除 | tmux backend 削除 | ❌ 未着手。非テストで tmux 参照 **56 ファイル** |
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

## 4. 残作業(依存順)

```
[今] phase 2 web スタック = 動くが pure core 非接続の並行実装
  │
  ├─ B. 設計判断 (i)/(ii) を確定 → ADR
  │     │
  │     └─ B1. PtyBackend 実装(termvt を既存 runtime の backend に)   ← linchpin
  │           │
  │           └─ A. web 経路で pure core 再利用
  │                 (run-state / driver view / persist / connector)
  │                 │
  │                 └─ C. tmux 全削除(56 ファイル、grep -ri tmux src/ = 0)
  │
  └─ D.(任意)arc proto の TCP+TLS+token 化 → native client(phase 0/4)
```

### B1. PtyBackend 実装(linchpin)
- **What**: `platform/termvt` を `client/runtime` の backend として包み、
  `backends.go` の役割インターフェースを満たす `PtyBackend` を新設。
- **Why**: pure core を tmux から切り離す唯一の接続点。A も C もこれ待ち。
- **触る所**: `client/runtime/backends.go`, 新規 `client/runtime/pty_backend.go`(仮),
  `platform/termvt/`(必要なら snapshot/resize/fan-out の API 追加),
  `client/runtime/launcher.go`(backend 選択の DI seam)。
- **完了条件**: 既存 reducer/driver テストが PtyBackend 上で green / `go test -race` green /
  tmux 未起動の環境で arc TUI が pty backend で動く。

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
- **役割 IF の写像**: tmux 固有概念(window layout 等)を termvt/PtyBackend に
  どう写すか。B1 の設計で要詰め。
- **reattach atomicity / backpressure**: snapshot + subscribe の原子性、遅いクライアント
  の切断(remote-client-design.md §7)。termvt は fan-out 済みだが境界の再確認が必要。
- **二重実装の恒久化**: (ii) を選ぶと driver ロジックが二系統に分裂する。

## 6. 未確定・要裏取り

- [x] **B の方針 (i)/(ii)** を確定 → (i) 採用([ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md))
- [x] termvt の Control event が pure core に未接続を確認(`server/web`/`server/session` は `client/state`/`client/driver` を import せず)
- [x] `backends.go` 役割 IF の写像 → データ面は termvt に全写像、`WindowLayout`/`TmuxControl` は server 等価物無し→client-side
- [ ] **session ownership**(`server/session.Service` vs runtime PtyBackend)を B1 着手前に決定 ← 新たな未決(ADR 0004 Open questions)
- [ ] web で複数ペイン同時表示(layout)をどの phase で入れるか(C 後の client-side layout 想定)

## 7. 次アクション

1. ~~B の方針を決める~~ → (i) 採用(本調査の裏取り完了)
2. ~~決定を ADR 化~~ → [ADR 0004](../docs/adr/0004-ptybackend-reuses-pure-core.md)
3. **session ownership の未決を詰める**(B1 着手前)→ `plan-how` で B1 技術設計
4. B1(PtyBackend)→ A → C を順に着手(各々テスト必須 = AGENTS.md)
