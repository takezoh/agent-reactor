# ADR 0066 — Web UI terminal の履歴は server-side VT scrollback バッファで供給する

Status: Accepted

Related: [ADR 0010](./0010-surface-output-sequence-per-subscribe.md), [ADR 0011](./0011-websocket-typed-close-frame.md), [ADR 0013](./0013-attacher-interface-and-protofake.md), [ADR 0025](./0025-transcript-rest-backfill-then-ws-tail.md)
Related code: `src/platform/termvt/session.go`, `src/platform/termvt/session_actor.go`, `src/platform/termvt/session_deps.go`, `src/client/runtime/pty_backend.go`, `src/client/config/config.go`, `src/client/web/src/components/TerminalPane.tsx` (xterm.js scrollback cap raised to 10000 to match server default)

## Context

Web UI から `arc` の terminal タブを開いた後に別デバイス (別ブラウザ / 別タブ) で同じセッションへ接続すると、初期表示は **現在の可視グリッドのみ** になっていた。`termvt.Session.Subscribe()` の seed event が `EventOutput{Data: em.Render()}` の 1 本だけで、画面外へ流れた行 (scrollback) は保持されていなかったため、claude / codex / bash のチャット出力や shell 履歴を後から接続したデバイスでは追えなかった。

設計検討段階で 3 案を比較した:

1. **raw PTY バイトの append-only ダンプ → REST backfill** (ADR-0025 の transcript 経路に倣う):  bit-perfect だが、絶対座標 ANSI (`\x1b[H` 等) と resize / alt-screen 遷移をまたいだ replay は vim・less・claude-code 等の全画面 TUI 描画を破壊する (cols が違うと座標が当たらない、alt-screen 中の DECSET 1049 を含むストリームを後から再生しても primary screen 状態を復元できない)。
2. **asciicast v2 ファイル化**: 1 と同じ structural problem に加え、`data` field の UTF-8 必須仕様が本プロジェクトの wire 層 (`TerminalPane.tsx:113` のコメント参照) のバイト忠実性方針と衝突する。サイズも raw 比 +30〜60%。
3. **tmux モデル — server-side VT エミュレータに scrollback バッファを保持し、Subscribe 時に `scrollback + 現在グリッド` をスナップショットとして送る**: バイトを replay するのではなく rendered state を送るため、TUI の絶対座標問題と resize 跨ぎ問題が構造的に存在しない。alt-screen 中は scrollback が空 (DECSET 1049 仕様) なので、「vim 中に join した別デバイスは vim の現在画面だけ見える、終了後に再 join すれば vim 前後の bash 履歴も scrollback で読める」という自然な挙動になる。

依存中の `github.com/charmbracelet/x/vt v0.0.0-20260615091924-bb3af1bbe712` が `Emulator.SetScrollbackSize` / `Emulator.Scrollback() *vt.Scrollback` / `vt.Scrollback.Lines() []uv.Line` / `IsAltScreen()` を完備しており、scrollback の取得・上限設定・alt-screen 判定はすべて library 内で済む。`uv.Lines.Render()` (ultraviolet `buffer.go:230`) は SGR 付き ANSI テキストを `\n` 区切りでシリアライズし、cursor 移動 / clear screen escape を一切含まないため、`em.Render()` (中身は `Lines(buf.Lines).Render()` — `ultraviolet/buffer.go:271`) と連結しても xterm.js 側で scrollback バッファに自然に積まれる。

## Decision

(1) `termvt` の `Emulator` interface に 2 メソッドを追加する: `SetScrollbackSize(maxLines int)` (上限設定) と `SerializeScrollback() []byte` (バッファを ANSI テキストにシリアライズ、空時は nil)。`realEmulator` は前者を `*vt.Emulator` から委譲し、後者は `uv.Lines(sb.Lines()).Render()` で実装する。`vt.Scrollback` 型を interface に漏らさないことで、テスト fake の負担を最小に保つ。

(2) `termvt.Spec` に `ScrollbackLines int` を追加する。`NewSession` 内で emulator 構築直後に `spec.ScrollbackLines > 0` のときだけ `SetScrollbackSize` を呼ぶ。ゼロは「underlying emulator の default を使う」を意味し、xvt の `DefaultScrollbackSize = 10000` がそのまま効く。

(3) `subscribeCmd.run` の seed フレーム生成を **2 段** にする。`SerializeScrollback()` が non-empty なら `Data: append(sb, '\n')` を 1 本目の `EventOutput` として送り、続けて `Data: Render()` を 2 本目として送る。空のとき (= fresh session / alt-screen 中) は 1 本目を省略する。trailing newline は xterm.js が 2 frame を連続して `term.write` する際に scrollback 最終行と screen 1 行目が衝突しないようにする分離記号。

(4) 設定は `~/.agent-reactor/settings.toml` の `[terminal] scrollback_lines = N` として `MonitorConfig` パターンに揃える。default は `10000` (xvt のデフォルトと一致)。`cmd/arc/coordinator.go` の `buildRuntime()` で `runtime.NewPtyBackend(cfg.Terminal.ScrollbackLines)` として渡し、`PtyBackend` が `SpawnWindow` / `RespawnPane` の構築する `termvt.Spec` に伝搬する。

(5) Wire shape (`EvtSurfaceOutput`) は変更しない。Browser 側 (`TerminalPane.tsx`) も変更不要 — 既存の `term.write(b64ToBytes(frame[2]))` が 2 frame 連続でも順に効く。

(6) **永続化は行わない**。scrollback は in-memory のみで、daemon プロセス再起動を跨ぐと失われる。要件は「別デバイスから join したとき履歴が読める」であって「daemon 跨ぎ」ではないため。将来 daemon 跨ぎが必要になった場合は本 ADR を別 ADR で superede し、append-only 補助ファイルを追加する余地を残す。

## Consequences

- positive: 別デバイス join 時に、claude / codex / bash の過去出力が xterm.js の scrollback で遡れるようになる (本 PR の主目的)。
- positive: vim 等の alt-screen TUI は「現在画面だけ届く」挙動になり、tmux / mosh と一致。仕様外の history を捏造しないため UX 矛盾が無い。
- positive: ADR-0010 の subscribe-scoped Sequence 契約は無変更で済む (frame 単位の incremental ID は live chunk から数え始めるため、2 frame seed でも意味論が壊れない)。
- positive: scrollback の上限は行数で表現されており、`[terminal] scrollback_lines = N` という意味論的に明確な knob で運用可能。10,000 行 ≈ 数 MB / session に収まる。
- negative: subscribe 時の seed frame サイズが最大で「scrollback 行数 × cols × SGR overhead」まで膨らむ。10,000 × 120 × 数 byte で数 MB の単一 WebSocket message。`coder/websocket` の default buffer はサーバ側送信に明示的な上限を設けないので機能上は問題ないが、低帯域環境での初回 join が「ハロー → 数 MB → 描画」とラグり得る。要監視。
- negative: daemon プロセス再起動で scrollback が失われる。session 自体が落ちる前提なら受容できるが、warm restart 機能 (現状なし) を将来入れる場合は scrollback の永続化を再検討する必要がある。
- negative: `Emulator` interface に 2 メソッド追加 → fake (`session_actor_test.go::fakeEmulator`) のメンテ表面が広がる。これは tmux 由来モデルに移った副作用として受容。

## Alternatives Considered

### raw PTY バイトのファイル化 + REST backfill (ADR-0025 と同形)

ADR-0025 が transcript / event-log でやっている経路を terminal にもコピーする最も自然な拡張。実装テンプレートも揃っている (`src/server/web/transcript.go`)。だが上記 Context で述べたように TUI 描画が resize / alt-screen 遷移で構造的に破壊される。bit-perfect だが render が壊れる「正しいが使えない」状態。却下。

### asciicast v2 ファイル化

タイミング情報を保持して `asciinema play` でも再生可能になる利点はある。しかし spec の UTF-8 必須要件が本プロジェクトの wire 層方針と相性が悪く (`TerminalPane.tsx:113` 参照: 非 UTF-8 バイトを silently U+FFFD に置換すると 256-color sequence や非 ASCII 出力が壊れる)、加えて raw 比 +30〜60% のサイズ膨張も scrollback cap 下では履歴が短くなる方向に効く。要件 (= 別デバイスで遡って読める) にはオーバースペックで、TUI 描画問題も raw と同じく解決しない。却下。

### 自前の line scroll tap (xvt の scrollback を使わず LF で押し出された行を独自リングに積む)

xvt が scrollback API を持たない世界線での実装案だったが、実際には `xvt.Emulator.SetScrollbackSize` 等が完備されていることが確認できた (`scrollback.go:1` 〜)。自前実装は library が提供しているものの再発明で、`uv.Line` のシリアライズ・alt-screen 連携・cap roll-over など考慮事項が多すぎる。却下。

### `IsAltScreen()` を見て seed の frame 数を明示的に切り替える

「alt-screen 中は scrollback frame を完全に dropping する」明示ロジックも検討した。だが `SerializeScrollback()` が空時 nil を返す方針で十分かつ、`subscribeCmd.run` 側は `len(sb) > 0` の単一条件で済む。`IsAltScreen` を直接見るのは依存を増やすだけで挙動は同じ。後者を採用。
