# ADR 0077 — pinch fontSize を撤去し horizontal swipe を arrow key 連射 (`\x1b[C` / `\x1b[D`) にリバインドする

Status: Accepted

Supersedes: [ADR 0070](./0070-fontsize-persist-clamp.md), [ADR 0071](./0071-touch-gesture-arbitration-and-long-press-selection.md)
Related: [ADR 0034](./0034-refit-raf-coalesce.md), [ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md), [ADR 0069](./0069-fab-overlay-layout-and-visualviewport-lift.md)
Related code: `src/client/web/src/hooks/useTerminalTouchGestures.ts`, `src/client/web/src/components/TerminalMobileOverlay.tsx`, `src/client/web/src/components/TerminalPane.tsx`
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-SWIPE-ARROW-001..003`, `FR-MOB-FONT-CLAMP-001`, `FR-MOB-PERSIST-001` (改稿)

## Context

web UI のターミナル中身は `src/platform/termvt/session.go` の `exec.Command(spec.Argv[0], ...)` で起動された agent CLI (claude / codex 等) そのもので、agent-reactor は PTY を中継する dumb pipe にすぎない。これら CLI は mouse reporting を有効化していないため、xterm.js の SGR mouse encoding を流しても CLI 側で解釈されず、タップでカーソル直接ジャンプは原理的に不可能。

ADR 0071 で実装した pinch → fontSize は、表面的な可視性は高いが core UX (履歴呼び出し / カーソル左右移動) には寄与しない。一方、Termius モバイル流の『水平 swipe → arrow key 連射』は agent CLI の readline 互換入力欄 (Ink `<TextInput>`, bubbletea textinput 等) に対して CLI 改変ゼロで通る最も leverage の高いジェスチャ。

ユーザー要件:
- pinch 操作は不要 (撤去対象)
- 入力位置のタップ移動相当 = 水平 swipe で `\x1b[C` / `\x1b[D` 連射

## Decision

(1) **`useTerminalTouchGestures` の state machine から `pinch` phase を完全削除**。2-finger touch は `idle` に collapse し、reducer から `{kind:"pinch"}` / `{kind:"scheduleFit"}` effect を消す。

(2) **`swipe` phase に `axis: "undecided" | "horizontal" | "vertical"` と `lastArrowX: number` を追加**。最初の `touchmove` で touch 距離が `MOVE_THRESHOLD` (8px) を超えた時点で `|dx|>|dy|` により axis を一回確定 (lock)。per-frame の dominance test は採らない (45° swipe の flicker を回避)。

(3) **horizontal-locked swipe** のみ `{kind:"arrow", direction, count}` effect を emit する:
  - `n = Math.trunc((currentX - lastArrowX) / cell.width)` (cell.width は `DEFAULT_CELL = 9px` fallback)
  - `n ≠ 0` のとき `direction = n > 0 ? "right" : "left"`, `count = |n|`
  - `lastArrowX += n * cell.width` で残差を保持 (累積エラー / 重複発火なし)
  - vertical-locked swipe は何も emit しない → native `touch-action: pan-y` でスクロール (ADR 0071 (2) を継承)

(4) **`preventDefault` は emit しない**。pinch 撤去で `touches.length=2` の preventDefault 要件が消滅。horizontal swipe は xterm 側に native 動作がないので奪う必要がなく、vertical はそもそも pan-y に譲るため。dwell 成立後の longpress-drag のみ既存通り `preventDefault` (ADR 0071 (5) の dwell 後のルールは継承)。

(5) **apply 層で input mode gate**:
  - hook シグネチャに `onArrowKey?(direction, count)` と `isInputActive?(): boolean` を追加
  - `arrow` effect の apply 時に `isInputActive() === true` の場合のみ `onArrowKey` を呼ぶ
  - `isInputActive` はコールバック (ref-stable closure) で受ける → input mode フリップで listener 再 bind しない (attach-once 規律を維持)
  - view mode (`isInputActive()=false`) では arrow を投げないので既存スクロール挙動と完全同一

(6) **arrow byte の wire 形成は host 側で**。`TerminalMobileOverlay` で `direction === "right" ? "\x1b[C" : "\x1b[D"` を `.repeat(count)` して `sendInput(seq)` に渡す。reducer は幾何のままで、Layer 1 テストが VT100 escape sequence の文字列に依存しなくなる。1 つの touchmove frame → 1 つの `{k:"i"}` wire frame。

(7) **`sendInput` プロップ注入経路**: `TerminalMobileOverlay` に `sendInput: (data: string) => void` prop を新設。`TerminalPane` 側で `(d) => { const sid = sessionRef.current; if (!sid) return; conn.send({k:"i", d, sessionId: sid}); }` の closure を渡す。`Connection` 型を overlay 側に持ち込まないことでテスト面を太らせない。

(8) **long-press selection / FontSizeControl ステッパー / persist 契約は不変**:
  - dwell (500ms 静止) → longpress-drag → `term.select()` の経路は ADR 0071 (3) を継承
  - `useFontSize` の `applyPinch` / `beginPinch` / `pinchBaseRef` は dead code として削除、stepper API (`increase` / `decrease` / `reset`) は維持
  - localStorage key `web.term.fontSize` の persist / clamp 契約は ADR 0070 から継承 (writer 経路がステッパーのみに減るが invariant は同一)

(9) **削除対象**:
  - `src/client/web/src/components/PinchIndicator.tsx` / `PinchIndicator.test.tsx`
  - `src/client/web/src/css/font-size-control.css` の `.pinch-indicator*` ルール
  - `src/client/web/src/css/view.css` の reduced-motion セレクタリストから `.pinch-indicator`
  - `useMobilePinch` / `PINCH_ACTIVE_LINGER_MS` (`TerminalMobileOverlay.tsx` 内)
  - 関連 pinch テスト (`useTerminalTouchGestures.test.ts` / `useFontSize.test.ts` / `TerminalPane.test.tsx` / `TerminalPane.pc-baseline.test.tsx`)

## Alternatives Considered

### (B) mouse wheel emulation (`\x1b[<...M`) で agent CLI の mouse handler に投げる

claude / codex は mouse reporting を opt-in していない (`exec.Command` 起動時に何も渡していない)。仮に有効化されていてもアプリ側で個別ハンドリングが必要で、UX 期待 (cursor 左右移動) と一致しない。**却下**。

### (C) 1 cell 移動ごとに 1 wire frame を送る (`count: 1` × N 回)

高速 swipe で WS を burst してフラッディング、後続の `r` resize frame と reorder するリスク。1 touchmove = 1 wire frame に集約することで `\x1b[C`.repeat(N) で同じ視覚効果を 1/N のフレーム数で達成できる。**却下**。

### (D) pinch を残置して swipe-to-arrow と共存

ADR 0071 の発端である『1→2 finger 遷移で swipe を中断する arbitration』の edge が再発する。pinch 自体の UX 価値が低いという要件発端を無視することになる。**却下**。

### (E) 汎用 `onInput(data: string)` callback で reducer から escape sequence を渡す

VT100 byte 形成が reducer effect 型に漏れ、Layer 1 テストが文字列依存になり brittle。`onArrowKey(direction, count)` で幾何を保つほうが将来 (↑↓ や Home/End 等の拡張) にも開きやすい。**却下**。

### (F) タップ単発 (B 案) で `term.buffer.active.cursorX/Y` との差分を arrow 連射

行折返しを跨ぐと差分計算が嘘になる (表示行 ≠ 内部行)。タップは既に `useInputMode` の入退場トリガに使われており意味の二重化が起きる。失敗時の最悪値が「履歴頭まで飛ぶ」になる。swipe 路線のほうが安全側に倒れる。**却下** (将来 input mode 中の 2 本指タップ等で補完する余地あり)。

## Consequences

- モバイルユーザーは input mode で水平 swipe による Termius 流の cursor / 履歴ナビゲーションを獲得 (claude code の長文プロンプト編集など最大級のユースケースに直撃)
- agent CLI 側に一切の変更を要求しない → claude / codex / 将来の他 agent でも一律で動く
- vertical swipe と long-press 選択は ADR 0071 の契約をそのまま継承 (regression なし)
- DOM / CSS / hook の表面積が縮む: `PinchIndicator` / `useMobilePinch` / pinch CSS / 1 reducer phase + 2 effect 種別が消滅
- reducer purity を維持 (input mode gate は impure hook 層に隔離)
- `useFontSize` の persist / clamp 契約は ADR 0070 から継承し、書込経路がステッパーのみに集約される
- 1 touchmove frame = 1 wire frame の規律で WS フラッディングなし
- 将来 mouse-aware な agent CLI に切り替える場合も hook の `onArrowKey` を別 effect に差し替えるだけで対応可能

## Related Requirements

- `FR-MOB-SWIPE-ARROW-001` — mobile gate true + input mode + horizontal-locked swipe で `\x1b[C` / `\x1b[D` × `trunc(|Δx|/cell.width)` を 1 touchmove あたり 1 `{k:"i"}` wire frame として送信
- `FR-MOB-SWIPE-ARROW-002` — view mode + horizontal swipe → input frame 0 (既存スクロール挙動と byte-identical)
- `FR-MOB-SWIPE-ARROW-003` — 2-finger touchstart / 1→2 finger 遷移 → effect 0 / fontSize 変化なし / PinchIndicator 描画なし (DOM に存在しない)
- `FR-MOB-FONT-CLAMP-001` — `[8,28]` clamp 契約は FontSizeControl ステッパー単独 path で継承 (ADR 0070 旧 `FR-MOB-PINCH-002` の clamp 規律を改名継承)
- `FR-MOB-PERSIST-001` (改稿) — FontSizeControl ステッパー確定時の localStorage 書込 + try/catch degrade (pinch touchend 経路は撤去)
- `FR-MOB-SCROLL-001..003`, `FR-MOB-SELECT-001/002`, `FR-MOB-PERSIST-002` — ADR 0071 / 0070 から不変継承
