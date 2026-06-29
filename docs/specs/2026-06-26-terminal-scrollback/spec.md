# Spec — Web UI terminal の履歴を別デバイス join 時に復元する

- **作成日**: 2026-06-26
- **ブランチ**: `main`
- **ADRs**: [0066](../../adr/0066-terminal-scrollback-via-vt-buffer.md)
- **関連**: [ADR 0010](../../adr/0010-surface-output-sequence-per-subscribe.md), [ADR 0025](../../adr/0025-transcript-rest-backfill-then-ws-tail.md)

## Goal

Web UI で terminal タブを開いた後、別デバイス (別ブラウザ / 別タブ) で同じセッションへ後から接続したとき、claude / codex / bash の **画面外に流れた行を遡って読める** ようにする。逆に vim / less 等の全画面プログラムについては「現在画面だけ届く」既存挙動を維持する (xterm 系の端末多重化器と同じ意味論)。サーバ側 VT エミュレータの scrollback バッファを使い、`termvt.Session.Subscribe()` の seed が `scrollback + 現在グリッド` の 2 frame で届くようにする。永続化はしない (in-memory)。

## Functional Requirements (EARS)

- **FR-001 — late-join scrollback delivery**
  ある session に対して subscriber が既に存在し、その session の primary screen に visible grid (rows × cols) を超える行数が出力済みであるとき、新規 subscriber が `Subscribe()` を呼ぶと、最初に受信する `EventOutput` frame には **画面外に流れた行 (scrollback)** が含まれ、続く `EventOutput` frame に現在の visible grid が届く。

- **FR-002 — scrollback frame elision when empty**
  scrollback バッファが空 (fresh session 直後 / 全画面プログラムが alt-screen を専有している間) のとき、`Subscribe()` の seed は visible grid 1 frame のみで構成され、scrollback frame は emit されない。

- **FR-003 — scrollback bound by configuration**
  ある session が `Spec{ScrollbackLines: N}` (N > 0) で構築されたとき、scrollback バッファに保持される行数は最大 N 行で、それを超える古い行は drop される (FIFO)。`ScrollbackLines = 0` のときは underlying emulator のデフォルト (xvt の `DefaultScrollbackSize = 10000`) が有効になる。

- **FR-004 — frame separation by newline**
  scrollback frame と続く screen frame の境界に newline (`'\n'`) を 1 文字付与し、xterm.js 側で scrollback 最終行と screen 1 行目が同じ row に collapse しないことを保証する。

- **FR-005 — configuration plumbing**
  `~/.agent-reactor/settings.toml` の `[terminal] scrollback_lines = N` で daemon が起動する全 session の scrollback 上限を上書きできる。default は 10,000。`[terminal]` セクション省略時は default が適用される。

- **FR-006 — wire shape preservation**
  本 feature は `EvtSurfaceOutput` の wire shape を変更しない。browser 側 (`TerminalPane.tsx`) の `term.write` 経路 (base64 decode → xterm.js) は無改修で 2 frame を順に書く。既存の WebSocket 接続 / 切断 / 再接続契約に影響しない。

- **FR-007 — alt-screen does not falsify history**
  alt-screen プログラム (vim / less / htop) の使用中、その描画は primary screen の scrollback に積まれない。alt-screen 終了 (DECSET 1049 reset) 後の subscribe は alt-screen 開始前の primary screen 履歴を引き継ぐ。これは VT spec 通りであり、本 feature 由来の new behaviour ではないが、テストで明示的に pinning する。

## Counterexamples (誤実装論証)

- ❌ scrollback frame の trailing newline を省くと、xterm.js は scrollback 最終行の cursor 位置のままで screen render を上書きし、最終行と screen 1 行目が同じ row に重なる (FR-004 反例)。
- ❌ alt-screen 中も scrollback frame を強引に作って送ると、空のフレームが 1 本余計に流れて wire ノイズになる (FR-002 反例)。`SerializeScrollback()` は空時 nil を返し subscribeCmd 側で省略する。
- ❌ raw PTY バイトを replay する設計に戻すと、resize / alt-screen 遷移後の絶対座標 ANSI が新 cols×rows に当たらず全画面プログラムが崩壊する (ADR-0066 で却下した raw / asciicast 方式の根拠)。
- ❌ scrollback を ANSI 化せず cells 構造体で送ると、wire shape 変更が必要になり FR-006 違反。`uv.Lines.Render()` で ANSI text に統一する。

## Out of Scope

- daemon プロセス再起動を跨いだ scrollback の永続化 (将来 raw byte append-only ファイル等の補助を別 ADR で追加する余地は残す)
- scrollback 中の検索 / フィルタ UI
- xterm.js 側 scrollback の動的な server cap 連携 (現状は静的に server default と同じ 10,000 に揃えている。settings.toml の `[terminal] scrollback_lines` を変更した場合の client 側追従は別 PR)
- ADR 0010 (subscribe-scoped Sequence) の意味論変更 — 2 frame seed でも frame 単位 incremental ID は live chunk から数え始まる、契約は無変更
- 既存 transcript / event-log の REST backfill (ADR 0025) との統合 — terminal は別経路、文書化のみ
- 本 PR は Web UI に閉じる
