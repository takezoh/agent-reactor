# server プロセスが VT emulator の `InsertLineArea` bounds bug で panic して死ぬ

- 作成日: 2026-07-02
- 改訂: 2026-07-02 (修正方針を「三層防御」→「構造起点の最小集合」に revise、codex MCP による多段検証を反映)
- 対象 process: `agent-reactor-server`
- 関係 module: `src/platform/termvt`, `github.com/charmbracelet/ultraviolet`, `github.com/charmbracelet/x/vt`
- 現状: 未修正 (crash パスは残存、trigger が引かれないだけで動いている)
- 関連コメント: `src/client/runtime/tap_manager.go:168` に同種バグの既知記載あり (現状 recover-drop で silent corruption)

## TL;DR

`platform/termvt.Session` の PTY chunk 処理中に、goroutine panic (`runtime error: index out of range [63] with length 63`) が発生し `Restart=on-failure` により server プロセスが exit → 自動再起動する。原因は上流 2 lib の **契約不在** による防御不足:

1. `x/vt` (**producer**) の DECSTBM / DECSLRM handler が explicit `bottom` / `right` を screen bounds に clamp しない (`handlers.go:862, 900`, `screen.go:143`) — invalid margin/scroll region を作る
2. `ultraviolet` (**consumer**) の `Buffer.InsertLineArea` / `DeleteLineArea` が `area.Max.Y > b.Height()` を弾かない (`buffer.go:462`) — invalid area で panic する

両者が「相手が守ってくれる」前提で invariant を保証していない。crash 経路は `ESC M` (reverse index) が cursor 位置と invalid scroll region の組で `ScrollDown → InsertLine → InsertLineArea` を叩いた瞬間に out-of-range。

Session actor 側に `defer recover()` が無いためプロセス全体が exit(2)。frametap 側 (`tap_manager.go:170 feedSafe`) は同じバグに対して既に `recover()` してあるが、**recover は fix ではない**: panic した chunk 内の OSC 133 / prompt phase / notification が silent に消えており、tap 機能は事実上壊れている。

**最終方針は「構造として正しい状態」から逆算した最小集合**:

1. `x/vt` fork で producer 側 invariant を保証 (**A2**)
2. `ultraviolet` fork で consumer 側 defense-in-depth (**A1**)
3. `feedSafe` を撤去して silent corruption 債務を清算
4. `agent-reactor-web.service` の `BindsTo=` を除去して fault isolation を回復
5. 上流 PR を両 lib に提出 (**D**)

`recover` 路線 (前回のレポートで「C」として提案していたもの) は **明示的に採用しない**。silent corruption を延命するだけで機能維持要件と両立しない。

## Symptoms

### Panic 実例 (2 回、両方同一 stack)

```
panic: runtime error: index out of range [63] with length 63

github.com/charmbracelet/ultraviolet.(*Buffer).InsertLineArea
    /pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go:476
github.com/charmbracelet/ultraviolet.(*RenderBuffer).InsertLineArea
    /pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go:731
github.com/charmbracelet/x/vt.(*Screen).InsertLine
    /pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615091924-bb3af1bbe712/screen.go:334
github.com/charmbracelet/x/vt.(*Screen).ScrollDown                     screen.go:313
github.com/charmbracelet/x/vt.(*Emulator).reverseIndex                 cc.go:50   (ESC M = 0x4d)
github.com/charmbracelet/x/vt.(*Emulator).Write                        emulator.go:276
github.com/takezoh/agent-reactor/platform/termvt.(*Session).processChunk  session_actor.go:179
github.com/takezoh/agent-reactor/platform/termvt.(*Session).mainLoop     session_actor.go:166
```

引数レジスタから読み取れる panic 時の状態:
- `Buffer.InsertLineArea(y=0, n=1, cell=nil, area={Min:(0,0), Max:(80,64)})`
- `buffer.Height() = 63`, `area.Max.Y = 64` → `b.Lines[63]` at line 476 で index out of range
- panic 時 chunk_len: 0x356 (854B) と 0x591 (1425B)
- session goroutine 起動時の cols/rows: `mainLoop(0x50, 0x18) = 80×24` (**初期化時の引数**、直近の buffer size を示さない)

### systemd 側の観測

```
Jul 01 15:58:21 agent-reactor-server.service: Main process exited, code=exited, status=2/INVALIDARGUMENT
Jul 01 15:58:23 Scheduled restart job, restart counter is at 1.
Jul 01 16:07:37 Main process exited, code=exited, status=2/INVALIDARGUMENT
Jul 01 16:07:39 Scheduled restart job, restart counter is at 2.
```

`agent-reactor-web.service` は `BindsTo=agent-reactor-server.service` により停止するが、web の exit code は 0 なので `Restart=on-failure` に該当せず、web は **inactive のまま放置** される。手動 `systemctl --user start agent-reactor-web.service` が必要。これは crash とは独立の 2 次被害 (後述の構造的欠陥 §4)。

## 検証済みの証拠

### Phase 1: 上流 bounds bug の直接再現

scratchpad の Go module (`/tmp/claude-.../scratchpad/vt-repro/phase1_bounds_test.go`) で以下を証明:

```go
b := uv.NewBuffer(80, 63)               // Height=63
area := uv.Rect(0, 0, 80, 64)           // Max.Y=64 (height を 1 超える)
b.InsertLineArea(0, 1, nil, area)       // → panic: index out of range [63] with length 63
```

対照実験:

```go
b := uv.NewBuffer(80, 63)
area := uv.Rect(0, 0, 80, 63)           // Max.Y = Height
b.InsertLineArea(0, 1, nil, area)       // → panic せず
```

`Buffer.InsertLineArea` (buffer.go:462) の現状ガード:

```go
if n <= 0 || y < area.Min.Y || y >= area.Max.Y || y >= b.Height() {
    return
}
```

`y >= b.Height()` は check しているが `area.Max.Y > b.Height()` は check していない。以降の copy loop:

```go
for i := area.Max.Y - 1; i >= y+n; i-- {
    for x := area.Min.X; x < area.Max.X; x++ {
        b.Lines[i][x] = b.Lines[i-n][x]     // OOB when i >= len(b.Lines)
    }
}
```

上流 HEAD (2026-06-22 `f39628c8`) の同ファイルも同じコードで未修正。`buffer.go` に対する commit 履歴は `2026-04-28: fix(buffer): ensure RenderBuffer marks lines as touched...` のみ (別修正)。

### Phase 2: CLI 起動 escape の実測

docker exec + `script(1)` で codex を worktree cwd / main repo cwd で起動、初期化 PTY を capture (`/tmp/claude-.../scratchpad/vt-repro/probe-{worktree,main}.log`)。両方に:

```
\x1b[1;24r          DECSTBM: 上端 1、下端 24
\x1b[1;1H           CUP (1,1)
\x1bM × 11〜13      RI (Reverse Index) 連発
\x1b[r              DECSTBM reset
\x1b[1;10r or [1;13r  DECSTBM 再設定
```

この `ESC M` 連発は `Emulator.reverseIndex()` (cc.go:46) を叩き、cursor が scroll region 上端にいると `Screen.ScrollDown(1)` → `Screen.InsertLine` → 上流バグ経路。

差分は window title (`agent-grid` vs `unified-gazelle`) と reverse-index 回数 (13 vs 11)。**両方に同じバグ経路の入力が含まれる**。

### Phase 3: 静的パラメータではない (静的トリガー仮説の反証)

capture bytes を standalone emulator に流した対照:

| 対象 emulator | 入力 | 結果 |
|---|---|---|
| `vt.NewEmulator(1, 1)` | worktree capture | panic (`index out of range [22] with length 1`) |
| `vt.NewEmulator(1, 1)` | main capture | panic (同上) |
| `vt.NewEmulator(80, 24)` + `SetScrollbackSize(10000)` | worktree capture | **panic せず** |
| `vt.NewEmulator(80, 24)` + `SetScrollbackSize(10000)` | main capture | **panic せず** |

1×1 emulator は tap_manager 側 (`vt.New(1, 1)` = `feedSafe` 保護あり) と同型で、コメント通り必ず panic する。しかし production Session と同じ **80×24 で captured bytes を replay しても panic しない** — 静的なバイト列だけでは production の panic を再現できない。

### crash 相関の反証

同じ静的パラメータ (`exec codex --dangerously-bypass-approvals-and-sandbox -C .../worktrees/<name>`、resume 引数なし、adopted CLI-created thread の直後) で:

| 時刻 | worktree | resume | panic |
|---|---|---|---|
| 07-01 15:58:20 | feasible-flounder | なし | ✅ (1s 後) |
| 07-01 16:07:15 | (main repo) | あり | ❌ |
| 07-01 16:07:37 | unified-gazelle | なし | ✅ (0.4s 後) |
| 07-02 01:47:50 | profound-goblin | **なし** | ❌ (40s+ 稼働継続) |

07-02 の profound-goblin は crash 2/2 と全く同じ静的条件だが panic しなかった。**静的パラメータ (cwd / cmd / resume 有無) だけでは crash 発生を予測できない → 動的 race**。

なお 07-01 16:07:39 → 07-02 01:25 の間、server binary は差し替わっているが、`git diff src/client/driver/{codex_event,codex_resume}.go src/client/runtime/bootstrap.go` の中身は observability ログ追加のみ (`logCodexIdentityCaptured` + `bootstrap: deleted unrecoverable snapshot` / `bootstrap: dropping stopped frame on cold start` の Info log)。**crash パスに影響するロジック変更なし**。

## Root cause 分析

### `buffer.Height() = 63` はどこから来るか

`NewSession` (`src/platform/termvt/session.go:71`) は 80×24 で `vt.NewEmulator` を作り、`SetScrollbackSize(10000)` は main screen の scrollback cap だけを変える (visible height は不変)。alt-screen 切替も size を変えない。`Screen.Resize` (`x/vt/screen.go:73`) は `s.scroll = s.buf.Bounds()` で常に scroll region を bounds と同期させる。

したがって Go 側の Session actor 内で `buf.Height()` が 63 に落ち込む純粋な race は見えない。63 の由来として最有力は **browser 側の xterm.js FitAddon** による resize と考察できる:

- `src/client/web/src/components/TerminalPane.tsx:131` mount 直後に `fit.fit()` を実行
- `src/client/web/src/components/TerminalPane.tsx:180` `term.onResize` が daemon に `{k:"r"}` を送る
- `src/server/web/gateway.go:311` `CmdSurfaceResize` が受け取り、最終的に `sess.Resize` (`src/platform/termvt/session.go:175`) を呼ぶ

panic stack の `mainLoop(80, 24)` は **初期化時の goroutine 引数**にすぎない (`s.mainLoop(cols, rows)` の cols/rows は関数ローカル、以降の Resize は emulator 内部を書き換えるが goroutine 引数は不変)。

### race のシナリオ

以下が現状の情報と整合する最も筋の通る sequence:

1. browser attach で session が **80×64** などに resize される
2. child (codex TUI) がその rows を元に `\x1b[1;64r` (DECSTBM with explicit bottom = 64) を送る
3. browser の再 fit (ResizeObserver / window resize) で session が **80×63** に縮む
4. child 側の描画 or `ESC M` (reverse index) が続く
5. `x/vt` 側で `Screen.scroll.Max.Y = 64` を保持したまま `buf.Height() = 63` の状態で `InsertLineArea` が呼ばれて panic

ここで **`x/vt` の DECSTBM handler が explicit `bottom` を clamp していない** ことが重要な下地。no-param reset (`\x1b[r`) だけなら `bottom = e.Height()` で安全だが、`\x1b[1;64r` のような explicit bottom は無防備に scroll region に格納される。その後の Screen.Resize は `s.scroll = s.buf.Bounds()` で修正するが、Resize と DECSTBM の**間**に ESC M が挟まる時間窓で panic が確定する。

Session actor は `em.Write` と `em.Resize` を同一 goroutine で serialize しているので Go race ではない。純粋に **child TUI が送出する DECSTBM の下端が、その時点の Emulator height を超えている** ことが直接原因。

### 「時々」の動的要因

同じ静的条件で結果が反転するのは以下の interleave 差:

- browser がその session に早く subscribe したか
- `fit.fit()` が何回走ったか (初回 fit + ResizeObserver + window resize で 64→63 の揺れは十分あり得る)
- codex startup TUI が DECSTBM / RI を吐いたタイミング
- **resumed session は startup の full-screen redraw が弱い**、または attach 時点で既に初期化を抜けているため crash 経路を通りにくい (= 「resume の方が落ちにくい」相関の合理的説明)

## 構造的欠陥の切り分け

crash 自体の直接原因は上流 lib bug だが、**被害が silent corruption に膨れた背景には repo 側の構造的欠陥がある**。これらを切り分けておかないと、上流 fix だけでは債務が残る。

### 1. `x/vt` と `ultraviolet` の責務分界の壊れ

- `x/vt` (producer) は invalid margin を受け入れる
- `ultraviolet` (consumer) は invalid area で panic する
- **どちらも「相手が守ってくれるだろう」という契約不在の状態**
- **一次責任**: `x/vt` (scroll region の意味論を持つ側)
- **二次責任**: `ultraviolet` (public API が invalid rect で panic するのは境界として弱い)
- どちらも fork して invariant を明示する必要がある = A2 (primary) + A1 (defense-in-depth)

### 2. VT emulator 2 系統並立

- `platform/termvt.Session` は `x/vt.NewEmulator` を直接使う (session display)
- `client/runtime/tap_manager` は `client/driver/vt` wrapper 経由で `x/vt` を別 instance で使う (OSC 抽出)
- **同じ PTY stream を 2 つの emulator が並列に食う設計**
- 片方が panic して片方が生きても、両者の state 整合はもう保証されない
- 本来 tap の要件は full screen model ではなく OSC / prompt phase / title / notification 抽出であって、scrollback や cursor restore ではない
- **tap に full `x/vt` を持たせるのは構造的に過剰**

### 3. recover-drop 債務 (`feedSafe`)

- `tap_manager.go:168` の feedSafe は「vt.New(1,1) panics on ESC M / CSI M / DECRC」を認識した上で `defer recover()` で chunk drop している
- panic した chunk 内の OSC 133 / notification / title は **永久に失われる** = tap の silent 機能欠損
- **「recover は fix ではなく機能の破損を隠している」** — screen bytes / OSC / prompt / notification を欠落させ、emulator の state machine を中途半端に進める
- 主経路 (`session_actor.go:processChunk`) にも同型の recover を入れる案が浮上しがちだが、silent corruption を延命するだけで正しくない

### 4. `Emulator` interface の関心事の混在

- `session_deps.go:Emulator` は Write + Read + Render + Resize + OSC + Scrollback + Cursor をすべて要求
- render 用と event 抽出用で必要な surface はまったく違う
- 責務分離ができていないため、tap にも full emulator を持たせざるを得ない → §2 の並立構造に至る

### 5. `BindsTo=` による fault isolation の放棄

- `agent-reactor-web` と `agent-reactor-server` を別 process にしているのは fault isolation のため
- `BindsTo=` で server の crash が web を巻き込む設計は、その isolation を自分で捨てている
- server は既に `Type=notify` + `sd_notify` で readiness を提供済なので、`After=` + `Requires=` だけで起動順は担保できる
- **`Type=notify` readiness と lifecycle 束縛 (BindsTo=) は思想が違う**。ここに整理漏れがある

## 修正方針 (最終)

### 位置付け

これは「コスト最適な応急処置」ではなく、**壊れた責務分界と不正な invariant を構造的に正す最小集合**。修正コストではなく構造的正しさで順序を切っている。`recover` で凌ぐ路線は最終方針から明示的に外す (**silent corruption を延命するだけで機能維持要件と両立しない**)。

### 必須 (今回スコープ)

| # | 対象 | 内容 | 位置付け |
|---|---|---|---|
| **A2** | `charmbracelet/x/vt` fork + `replace` directive | DECSTBM で `bottom > height` clamp、**DECSLRM で `right > width` clamp** (対称性)、可能なら `setVerticalMargins` / `setHorizontalMargins` 本体で bounds 正規化 | 一次責任 (producer 側 invariant) |
| **A1** | `charmbracelet/ultraviolet` fork + `replace` directive | `Buffer.InsertLineArea` **と `DeleteLineArea`** の area clamp (or reject) | 二次責任 (consumer 側 defense-in-depth) |
| **feedSafe 撤去** | `src/client/runtime/tap_manager.go` | `feedSafe` を削除し raw `term.Feed` に戻す (A2+A1 投入・動作確認後) | 債務清算 (silent corruption の停止) |
| **systemd** | `deploy/systemd/agent-reactor-web.service` | `BindsTo=agent-reactor-server.service` を除去 (`After=` + `Requires=` で起動順は担保) | fault isolation の回復 |
| **D (上流 PR)** | `charmbracelet/x/vt` と `charmbracelet/ultraviolet` の両方 | A2 相当を x/vt に、A1 相当を ultraviolet に PR。merge 後は fork を剥がして pin を戻す | 恒久解 |

**A2 と A1 は両方 mandatory**。片方だけでは:

- A2 だけ: この repo での再発可能性は下がるが、`ultraviolet` の public API は依然として footgun のまま
- A1 だけ: crash は止まるが `x/vt` が invalid margin/state を内部に持つこと自体は放置

### 明示的に採用しない案

以下は検討したが、構造的な理由で不採用:

- **C (recover ベースの chunk drop)**: silent corruption を延命するだけ。screen bytes / OSC / prompt / notification を欠落させ、emulator の state machine を中途半端に進める。機能維持要件と両立しない。前回のレポートで「三層防御の C」として提案していたが撤回
- **emulator を作り直す (session-kill 相当)**: screen state / scrollback / cursor / alt-screen / response pipe を失い、child process は生きているのに server-side VT state だけ飛ぶ。壊れ方が読めない
- **`SafeEmulator` (`x/vt.NewSafeEmulator`)**: `safe_emulator.go` を読む限り `RWMutex` を掛けているだけの concurrency-safe wrapper。`Write()` は素で `se.Emulator.Write(data)` を呼ぶだけで、今回の bug は貫通する。名前に反して panic-safe ではない
- **`Screen.buf` を wrap**: `buf` が unexported なので fork なしでは不可
- **`SetLogger` で hook**: emulator 内部 panic を囲む hook ではなく `e.logf(...)` 地点にしか効かない
- **`Emulator.Write` を wrapper で `defer recover()`**: `processChunk` recover と本質は同じ。silent corruption を延命するだけ
- **library 全替 (`hinshun/vt10x` 系や自前 parser)**: `Emulator` interface が要求する surface (OSC handler / scrollback / render / snapshot / alt-screen / query response pipe) を代替 lib で覆えない。交換コストは fix より圧倒的に高い。設計是正としても筋が悪い
- **`Restart=always` / `sd_notify` 追加**: 対症療法。`BindsTo=` 除去が根本
- **instrumentation で raw chunk を回収して race を実測してから fork**: A2 で producer 側 invariant を保証すれば invalid margin は生成されず、A1 で consumer 側 defense-in-depth があれば万一の invalid input も panic に至らない。race 詳細の実測は「構造として正しい fix」の後には不要。かつ instrumentation 単独先行は daemon crash を継続容認するコストが高い

### 続くべき設計是正 (第 2 段、別 issue)

第 2 段として repo 側の VT layer 再設計を行う。今回の crash 修正の**次の段階**として別立てし、`issues/` に別 report を切る:

- **tap から full emulator を撤去**し、raw ANSI parser で OSC だけ抜く軽量実装に置き換える
- **event extraction を render 用 emulator から分離**
- **`session_deps.go` の `Emulator` interface を「render 用」と「event 抽出用」に分割**

これにより §2 (2 系統並立) と §4 (interface 関心事の混在) の設計汚染を解消する。

同期して、共有すべきなのは emulator 本体ではなく **event stream** である、という原則を repo 全体に敷く。

## 実施順序

1. `A2` (x/vt fork + margin clamp) 先行 — primary fix
2. `A1` (ultraviolet fork + area clamp) — mandatory defense-in-depth
3. systemd fix (`BindsTo=` 除去) — 独立、いつでも可
4. `feedSafe` 撤去 — A2+A1 投入・動作確認後 (安全側順序)
5. 上流 PR (D) — A2/A1 の commit を元に平行して進行
6. 第 2 段 (tap 再設計 + interface 分割) — 別 issue として立てる

### `feedSafe` 撤去の順序について

`feedSafe` を A2/A1 の前に撤去すると **tap 経由の panic が復活する** ため、順序は以下の 2 択:

- **順序 A (安全)**: A2 + A1 で塞ぐ → 動作確認 → feedSafe 撤去
- **順序 B (誠実)**: feedSafe を先に撤去して panic を露出させる → A2 + A1 で塞ぐ

構造的には B のほうが誠実だが、production を意図的に crash させる期間が発生する。**推奨は A**。

## 関連 file 参照

- `src/platform/termvt/session.go` — Session actor + Spec (ScrollbackLines default = 10000 via config)
- `src/platform/termvt/session_actor.go` — mainLoop / processChunk (recover 欠如)
- `src/platform/termvt/session_deps.go` — Emulator interface + `emulatorFor` (vt.NewEmulator 呼び出し)、第 2 段で分割対象
- `src/client/runtime/tap_manager.go` — 同種バグの既知記載 + feedSafe (**撤去対象**)
- `src/client/driver/vt/terminal.go` — tap 用の x/vt 別 instance (第 2 段で撤去対象)
- `src/client/runtime/pty_backend.go` — SpawnFrame → Manager.Create の入り口
- `src/client/web/src/components/TerminalPane.tsx` — xterm.js FitAddon による resize 起点
- `src/server/web/gateway.go` — CmdSurfaceResize から sess.Resize への橋渡し
- `deploy/systemd/agent-reactor-web.service` — `BindsTo=` 除去対象
- `~/.local/share/go/pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260303162955-0b88c25f3fff/buffer.go` — A1 の patch 対象
- `~/.local/share/go/pkg/mod/github.com/charmbracelet/x/vt@v0.0.0-20260615091924-bb3af1bbe712/{cc.go,screen.go,handlers.go,emulator.go,safe_emulator.go}` — A2 の patch 対象、`SafeEmulator` は該当バグに無効
- `/home/dev/.local/state/agent-reactor/server.log.{3,4}` — 07-01 の 2 crash 実ログ
- `~/.config/systemd/user/agent-reactor-{server,web}.service` — systemd unit 定義

## 未解決事項

- alt-screen 切替 (`\x1b[?1049h`) が race に絡むかどうか (A2 で塞げば無害化されるので優先度低)
- `DeleteLineArea` 側で同種 crash が実観測されるか (コード上は同じガード欠落なので A1 で塞ぐ)
- `DECSLRM` (水平 margin) 側で同型 crash が実観測されるか (A2 の対称性で先回りする)
- 第 2 段 (VT layer 設計是正) の詳細スコープと ADR
- `buf.Height() = 63` の**厳密な**由来 (browser resize の中間状態、と考察したが実測は未確認) — A2 で invalid state 自体が発生しなくなるので、race 詳細の実測は fix の後には不要

## 他セッションからの再開情報

### この issue を作成したセッションの成果

作成セッション (2026-07-02) は本 issue の作成と付随する commit のみを実施しており、**実装 patch (A2 / A1) は未投入**。関連 commit:

- `a847a5b` docs(issues): 本レポートを構造起点の最終方針に改訂
- `6b3718e` docs(issues): 本レポート初版
- `0f38bb0` chore(client): cold-start recovery の到達点を Info log 化 (crash 調査中に確認した未 commit 差分を確定した副次成果、本 crash とは無関係)
- `0a6bb21` refactor(client/web): dist を全 ignore + `.gitkeep` 化 (Vite build と placeholder の衝突整理、本 crash とは無関係)

### codex MCP thread

多段検証は codex MCP に依頼して行った。同じ context で反復対話を継続したいなら thread ID: `019f2086-4879-77d1-9d82-9cb050c525ea` を渡して `mcp__codex__codex-reply` で resume する。thread は codex app-server (agent-reactor daemon 内) が生きている間有効。

### verified negatives (再検証不要)

以下は作成セッションで確認済で、次のセッションが再度試す必要はない:

- **`SafeEmulator` (`x/vt.NewSafeEmulator`)**: `RWMutex` を掛けているだけの concurrency-safe wrapper、bounds-safe ではない。`Write()` は素で `se.Emulator.Write(data)` を呼ぶだけで今回のバグは貫通する
- **80×24 emulator + 実 codex 起動 capture の replay**: standalone では panic しない (Phase 3)。production 特有の追加要因 (browser resize 系) が絡む
- **「worktree cwd で必ず落ちる」静的仮説**: 07-02 01:47:50 の `profound-goblin` spawn (crash 2/2 と同一静的条件 = `exec codex --dangerously-bypass-approvals-and-sandbox -C .../worktrees/... `、resume 引数なし) が panic せず反証
- **上流 HEAD (2026-06-22 `f39628c8`)**: `buffer.go` 未修正で同じコード。`go get -u` では直らない
- **`SetLogger` / `Screen.buf` wrap / library 全替**: いずれも該当バグに無効か過剰コスト (§修正方針 の「明示的に採用しない案」参照)

### 現状の live server 状態 (作成セッション時点)

- 07-02 01:26:42 に手動再起動 (binary は observability log 追加分の rebuild)、以降 crash なし
- 07-02 01:47:50 の新規 worktree codex spawn (`profound-goblin`, thread `019f2082-cbd4-...`) は panic なしで稼働継続
- 依然として race 経路は残存、trigger を引かないだけの状態
- ライブラリ pin: `ultraviolet@v0.0.0-20260303162955-0b88c25f3fff`, `x/vt@v0.0.0-20260615091924-bb3af1bbe712`, `x/ansi@v0.11.7` (`src/go.mod`)
- server.log path: `/home/dev/.local/state/agent-reactor/server.log{,.1..5}`

### TBD (次セッションで判断)

- **fork 先 GitHub organization / repo 名**: 現時点で未決定 (`takezoh/x-vt`, `takezoh/ultraviolet` あたりの命名になりそうだが git remote は未追加)
- **上流 PR の tone と scope**: producer 側 (`x/vt`) と consumer 側 (`ultraviolet`) を 1 PR ずつに分けるか、それぞれ ADR / 補足 test も含めるかの判断
- **`feedSafe` 撤去のタイミング**: 順序 A (A2/A1 投入・動作確認後) を推奨しているが、tap の silent OSC loss をどこまで許容するかで順序 B (先に撤去して panic を露出) も選択肢
- **第 2 段の開始時期**: 本 issue closed → 別 issue を切るタイミング。tap 再設計と `Emulator` interface 分割は独立して進められるが、ADR 必要

### 検証環境の再構築 (scratchpad `/tmp/claude-.../scratchpad/vt-repro/` が失われた場合)

- **Phase 1** (bounds bug 直接再現): 空 module + `github.com/charmbracelet/ultraviolet` を依存に追加 (作成セッション時 `v0.0.0-20260303162955-0b88c25f3fff`) して以下:

  ```go
  b := uv.NewBuffer(80, 63)
  area := uv.Rect(0, 0, 80, 64)
  b.InsertLineArea(0, 1, nil, area) // → runtime error: index out of range [63] with length 63
  ```

- **Phase 2** (codex 起動 escape capture): container `d73e8870030b` (`reactor-shared`) 上で:

  ```
  docker exec -u ubuntu -w /home/dev/dev/agent-grid/.agent-reactor/worktrees/<any> \
      d73e8870030b bash -c 'export TERM=xterm-256color; \
      timeout 1.5 script -q -c "stty cols 80 rows 24; \
      codex --dangerously-bypass-approvals-and-sandbox 2>&1" /tmp/probe.log \
      >/dev/null 2>&1; base64 -w0 /tmp/probe.log'
  ```

- **Phase 3** (replay): `vt.NewEmulator(80, 24)` + `em.SetScrollbackSize(10000)` + `go io.Copy(io.Discard, em)` で drainer を立ててから `em.Write(bytes)`。CSI reply pipe を drain しないと block する。1×1 emulator (`vt.NewEmulator(1, 1)`) では両 capture とも panic する

### memory への相互参照

作成セッションで残した persistent memory (auto memory `~/.claude/projects/-home-dev-dev-agent-grid/memory/`):

- `feedback_no_recover_for_reproducible_bugs.md` — 再現性のある panic に defer recover を提案するなという教訓 (本 issue の C 案却下と同根)

## 変更履歴

- **2026-07-02 (初版)**: 三層防御 (A1 + A2 + C + D) を推奨
- **2026-07-02 (改訂)**: codex MCP による多段検証を経て、以下の判定を反映:
  - `SafeEmulator` は panic-safe ではないと判明 (`RWMutex` のみ)
  - `A2` の scope を DECSTBM に加え DECSLRM (対称) に拡大
  - `A1` の scope を `InsertLineArea` に加え `DeleteLineArea` にも拡大
  - **`C` (recover ベースの chunk drop) を明示的に不採用に変更** — silent corruption を延命するだけで機能維持要件と両立しない
  - `feedSafe` を撤去対象に格上げ (silent corruption 債務)
  - `BindsTo=` 除去を明示、`Restart=always` / `sd_notify` 追加は対症療法として不採用
  - 「構造起点の最小集合」という位置付けを明示 (コスト最適化ではない)
  - 第 2 段 (VT layer 再設計) を続く設計是正として明示
