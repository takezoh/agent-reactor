# Web UI Terminal Mobile UX — Specification

Status: Draft
Upstream UX: [./ux.md](./ux.md)
Implementation Plan: [./plan.md](./plan.md)

Related ADRs:
- [ADR 0067 — モバイル UX gate (matchMedia AND 契約)](../../adr/0067-mobile-gate-matchmedia.md)
- [ADR 0068 — mode 分離 / focus-block / iOS zoom-guard](../../adr/0068-mode-separation-focus-block-and-zoom-guard.md)
- [ADR 0069 — FAB overlay layout + visualViewport-lift (CSS 変数)](../../adr/0069-fab-overlay-layout-and-visualviewport-lift.md)
- [ADR 0070 — fontSize 永続化 + parse / clamp 厳密化](../../adr/0070-fontsize-persist-clamp.md)
- [ADR 0071 — touch gesture arbitration + long-press 選択 (term.select)](../../adr/0071-touch-gesture-arbitration-and-long-press-selection.md)
- [ADR 0072 — Coachmark dismiss (tap or 5s) + hintSeen 冪等書込](../../adr/0072-coachmark-dismiss-and-once.md)
- [ADR 0073 — AriaLive debounce + JumpFAB seed-gating](../../adr/0073-arialive-debounce-and-jump-fab-seed-stability.md)
- [ADR 0074 — Migration: PC-only → PC + Mobile (baseline test + gate rollback)](../../adr/0074-migration-pc-only-to-pc-plus-mobile.md)
- [ADR 0075 — Pattern adoption (tmux / Termius / Slack / Material / iOS / WAI-ARIA + IconButton)](../../adr/0075-pattern-adoption-mode-affordances.md)

## Goal

Web UI TerminalPane (xterm.js 5.5.0 + addon-fit) のモバイル UX を `ux.md` の 7 flow / 26 UAC で固定された観察契約どおりに、PC (pointer:fine) **完全現状維持** + 既存 ADR 0029 / 0030 / 0034 / 0059 / 0063 / 0064 / 0065 / 0066 衝突回避を絶対制約として実装する技術計画と、UX Open Questions 1〜4 をすべて plan-how 段階で決着させる 9 件の ADR (gate / mode 分離 + focus-block + zoom-guard / FAB overlay + visualViewport lift / fontSize 永続化 / touch gesture arbitration + long-press 選択 / coachmark dismiss / aria-live debounce + jumpFAB seed gating / migration / pattern adoption) を確定する。

ATDD は **vitest + happy-dom + @testing-library/react** を harness とし、Playwright 不在経路 (実 soft keyboard / 実 pinch / 実 long-press / 実 VoiceOver) は実機検証チェックリストへ振り分ける。

## Scope

### In Scope

- `src/client/web/src/components/TerminalPane.tsx` のモバイル経路追加 (`useMobileGate` true 分岐のみ条件 render、PC path 1 bit 不変)
- `src/client/web/src/css/app.css` と `view.css` の mobile gate scope 内 CSS 追加 (`touch-action:pan-y` / `.terminal-fab-layer` / `.xterm-helper-textarea font-size:16px !important` / FAB overlay layout / CSS custom property `--terminal-fab-offset` / ADR 0064 reduced-motion guard 末尾追記)
- 新規 hook (10 個): `useMobileGate` / `useInputMode` / `useHostPointerInterceptor` / `useTerminalTouchGestures` / `useFontSize` / `useJumpToLatest` / `useVisualViewportLift` / `useCoachmarkOnce` / `usePersistedValue` / `useAnnouncer`
- 新規 component (7 個): `IconButton` primitive / `KeyboardFAB` / `JumpToLatestFAB` / `FontSizeControl` + disclosure popover / `Coachmark` / `PinchIndicator` (Toast 再利用 wrap) / `AriaLiveStatus`
- localStorage 永続化 (`arc.web.term.fontSize` / `arc.web.term.hintSeen`) を `usePersistedValue` adapter で集約
- vitest + happy-dom + @testing-library/react ベースの UAC test suite (TouchEvent / Pointer / matchMedia / visualViewport の合成 harness を chunk-01 で確立)
- 実機 iOS Safari 17+ / iPadOS 17+ / Android Chrome 最新の手動検証チェックリスト (visualViewport-lift / iOS focus-zoom / OS keyboard blur / long-press OS feedback / VoiceOver / TalkBack)
- 新規 ADR 9 件 (ADR 0067 〜 0075)
- ADR 0064 (reduced-motion 単一 guard) への追記 (smooth scroll / FAB fade / Coachmark fade / PinchIndicator fade を即時化)
- chunk-01 で PC behavior baseline test を CI 必須化 (`FR-PC-PRESERVE-001/002/003` を machine-checkable に昇格)

### Out of Scope

- PC (`pointer:fine`) パスの一切の振る舞い変更 — wheel scroll / click→focus / ドラッグ選択コピー / `touch-action` 未指定は legacy 完全維持
- サーバ側 (`src/server/web/wire.go` 等) の wire 変更 — `conn.send k:i / k:r / k:o` / `conn.onOutput` は legacy 踏襲、本タスク **client-only**
- xterm renderer 切り替えそのものを新規 ADR にすること (long-press 検証で標準 `term.select()` 成立を確定したため canvas/webgl addon 採択は不要)
- 専用モバイルビュー (`ux.md` `reference_ux` で rejected の方針 c) — 既存 `TerminalPane.tsx` を別ビューへ置換しない
- PC のキーボードショートカット追加・新規 wire メッセージ追加・session state 拡張
- scrollback 容量変更 (xterm scrollback:10000 固定、ADR 0066 維持)
- 既存 ADR 0029 / 0030 / 0034 / 0059 / 0063 / 0065 / 0066 の改廃 (本タスクで参照のみ)
- App-level `useAnnouncer` への即時移行 (ADR 0057 拡張) — fast-follow note のみ、本タスクは TerminalPane local で完結
- Playwright の導入 (現状未導入 / 導入予定なし、実 soft keyboard 経路は実機チェックリストで対応)
- アクセシビリティの全 ARIA pattern 適用 (本タスクは `aria-live polite` + `aria-pressed` + `aria-label` + `role=button` に限定)

## EARS Requirements

EARS 件数: 39 件 (State-driven 15 / Event-driven 19 / Unwanted 5 / Ubiquitous 0 / Optional 0)。

### State-driven (状態駆動)

> **FR-MOB-GATE-002** (`state_driven`) — TerminalPane が mount している間、システムは matchMedia の change イベントを購読し続け、gate true→false 遷移時には現在の入力モード state を破棄し、helper textarea の readonly を解除し、全 overlay (FAB / Coachmark / PinchIndicator / AriaLiveStatus / .terminal-fab-layer) を unmount し、visualViewport listener を unsubscribe してから入力モード state 破棄を行う順序を守らなければならない。
>
> *Rationale*: ux edge case『デバイス回転で 767px 境界をまたぐ』+ visualViewport listener leak 防止。順序保証が無いと回転後に listener が leak し PC path で動作する。

> **FR-MOB-MODE-001** (`state_driven`) — gate true でセッション初期描画が完了している間、システムは terminal-host ラッパの data-input-active 属性を文字列 'false' で開始し、.xterm-helper-textarea に readonly 属性を付与し、document.activeElement が .xterm-helper-textarea でない状態を維持しなければならない。
>
> *Rationale*: UAC-001 — invariant 初期状態 (entry observation) を ubiquitous/state_driven で固定し『何もしないとき常に成立する』振る舞いを EARS 化。

> **FR-MOB-MODE-002** (`state_driven`) — gate true かつ閲覧モード (data-input-active='false') の間、システムは xterm 表示領域 (.xterm-helper-textarea 以外) への touchstart→touchend に対し、helper textarea へ focus イベントを 0 回 dispatch し、document.activeElement を変えない状態を維持しなければならない (useHostPointerInterceptor の capture-phase pointerdown.preventDefault() が load-bearing 機構、readonly は defense-in-depth)。
>
> *Rationale*: UAC-002/009 counterexample『touchend で term.focus() 直後に setTimeout で blur するチラ見せ実装』が focus 発火数 0 assertion で fail することを保証する判別力を確保する。

> **FR-MOB-MODE-007** (`state_driven`) — gate true である間、システムは .xterm-helper-textarea の computed font-size を常時 16px 以上に保ち (mobile gate scope の CSS で `.xterm-helper-textarea { font-size: 16px !important }` を適用)、グリッド .xterm-rows の computed font-size は usePinchFontSize / FontSizeControl の clamp [8,28] に従わせなければならない。
>
> *Rationale*: ux edge case『iOS focus-zoom 抑止』が load-bearing。否定役指摘により上書き機構 (CSS !important) を ADR で確定。grid 描画は .xterm-rows 側で別系統のため独立。

> **FR-MOB-SCROLL-001** (`state_driven`) — gate true かつ閲覧モードである間、システムは .xterm-viewport に `touch-action: pan-y` を適用し続けなければならない (PC path では touch-action 未指定で xterm default 挙動を維持)。
>
> *Rationale*: UAC-007/008 — invariant な CSS 宣言を state_driven で固定。

> **FR-MOB-SCROLL-003** (`state_driven`) — gate true かつ閲覧モードで縦 swipe が連続発生している間、システムは helper textarea への focus イベントを 0 回維持し data-input-active='false' を保たなければならない。
>
> *Rationale*: UAC-009 — swipe の touchstart を tap と誤区別して focus する counterexample を排除。

> **FR-MOB-JUMP-001** (`state_driven`) — gate true かつ .xterm-viewport.scrollTop が (scrollHeight - clientHeight) と ±2px の近接にある間、システムは aria-label='最新へスクロール' の <button> 要素を DOM に存在させてはならない (条件 render により完全に DOM から除外)。
>
> *Rationale*: UAC-012 invariant 末尾状態。CSS opacity:0 隠蔽 counterexample 排除。

> **FR-MOB-STEPPER-001** (`state_driven`) — gate true である間、システムは FontSizeControl を disclosure popover (Aa アイコン → tap で popover 露出) で常時到達可能にし、popover 内の +/-/Reset 3 ボタンがそれぞれ role=button / getBoundingClientRect().width≥44 / height≥44 / 非空 aria-label を持つ状態を維持しなければならない。+ activate で 2px 増加 / - activate で 2px 減少 / Reset activate で 14px に戻し、いずれの activate でも scheduleFit (ADR 0034) を invoke する。
>
> *Rationale*: UAC-020 — VoiceOver/TalkBack は 2 指ジェスチャを自前に奪うため pinch 不能ユーザー向け非 pinch 代替を invariant 提供。

> **FR-MOB-FAB-001** (`state_driven`) — gate true である間、システムは KeyboardFAB を IconButton primitive で wrap した <button type='button'> 要素として render し、getBoundingClientRect().width≥44 / height≥44 / 非空 aria-label / aria-pressed を useInputMode state と同期 (false 時 aria-label='キーボードを開く', true 時 'キーボードを閉じる') する状態を維持しなければならない。
>
> *Rationale*: UAC-024/026 invariant a11y 契約。IconButton primitive 内で 44px / aria-pressed を集約。

> **FR-MOB-FAB-003** (`state_driven`) — gate true である間、システムは全 FAB の position 計算に env(safe-area-inset-*) を加算してはならず (.app-shell が四辺で適用済み / FR-LAYOUT-004 single-source)、terminal-slot 内側からの 16px offset で配置しなければならない。
>
> *Rationale*: FR-LAYOUT-004 + ux assumption §9 — safe-area 二重計上禁止 invariant。

> **FR-MOB-FAB-004** (`state_driven`) — gate true である間、システムは KeyboardFAB を下端 bottom: var(--terminal-fab-offset, 16px) 位置に、JumpToLatestFAB をその 8px gap 上 (bottom: calc(var(--terminal-fab-offset, 16px) + 52px)) に、FontSizeControl を別位置 (top-right inset 内 16px) に固定スタックで配置し、既存 .notification-toast layer とは別 portal で z-index と DOM 位置を分離しなければならない。
>
> *Rationale*: UAC F-007 step 4 — 固定スタック順 + Toast との衝突回避。

> **FR-MOB-VVP-001** (`state_driven`) — gate true かつ入力モードかつ window.visualViewport API が存在する間、システムは visualViewport の resize / scroll を購読し続け、.terminal-fab-layer の inline CSS custom property `--terminal-fab-offset` を `(window.innerHeight - visualViewport.height - visualViewport.offsetTop + 16)px` で更新し続けなければならない (各 FAB は CSS で `bottom: var(--terminal-fab-offset, 16px)` を参照し React 再 render を発生させない)。
>
> *Rationale*: ux edge case『iOS soft keyboard が dvh を縮めない』+ 最適化案『CSS 変数で 1→多 fan-out』採用。

> **FR-PC-PRESERVE-001** (`state_driven`) — gate false である間、システムは KeyboardFAB / JumpToLatestFAB / FontSizeControl / Coachmark / PinchIndicator / AriaLiveStatus / .terminal-fab-layer のいずれも DOM に存在させず、terminal-host に data-input-active 属性を付与せず、.xterm-helper-textarea に readonly 属性を付与してはならない。
>
> *Rationale*: UAC-021 invariant PC preserve。条件 render の機構的保証。

> **FR-PC-PRESERVE-002** (`state_driven`) — viewport が 700px かつ pointer:fine (narrow desktop window + マウス) である間、システムは gate を false と評価し desktop 扱いとし、terminal 表示領域への click で .xterm-helper-textarea が focus されて legacy 入力受付が成立する状態を維持し、KeyboardFAB を DOM 不在にし terminal-host に data-input-active 属性を付与してはならない。
>
> *Rationale*: UAC-022 — 幅のみ gate counterexample (700px+fine が mobile 化して全 mobile/PC scenario を green のまま PC regression だけ取りこぼす) を唯一判別する invariant。

> **FR-PC-PRESERVE-003** (`state_driven`) — gate false である間、システムは .xterm-viewport の touch-action 指定を行わず (xterm default 挙動を維持)、wheel up イベントで .xterm-viewport.scrollTop が減少する legacy 経路を維持しなければならない。
>
> *Rationale*: UAC-023 invariant — touch-action やカスタム scroll handler を全環境に適用する counterexample 排除。

### Event-driven (イベント駆動)

> **FR-MOB-GATE-001** (`event_driven`) — セッションが mount される時、システムは useMobileGate hook を介して matchMedia('(max-width: 767px) and (pointer: coarse)').matches を boolean 真実源として評価し、matches=true の場合のみ terminal-host に data-input-active 属性を付与し、KeyboardFAB / JumpToLatestFAB / FontSizeControl / Coachmark / PinchIndicator / AriaLiveStatus / .terminal-fab-layer を条件 render (CSS display:none による隠蔽は禁止) しなければならない。
>
> *Rationale*: UAC-001/021/022 — JS gate を真実源とし条件 render で a11y tree からも完全に除外する。CSS display:none は querySelector で取得可能で UAC-012/021 counterexample を通すため禁忌。

> **FR-MOB-MODE-003** (`event_driven`) — gate true かつ閲覧モードで KeyboardFAB が tap される時、システムは data-input-active='true' へ遷移し、helper textarea の readonly を外し helper textarea を focus し、KeyboardFAB の aria-pressed='true' / aria-label='キーボードを閉じる' に同期し、tap 後 200ms 経過後も data-input-active='true' を維持しなければならない (enter/exit race が起きてはならない)。
>
> *Rationale*: UAC-003 — readonly 外し + focus + aria 同期 + enter/exit race 不在の 4 観察契約を 1 EARS で固定。

> **FR-MOB-MODE-004** (`event_driven`) — gate true かつ入力モードで同じ KeyboardFAB が再 tap される時、システムは data-input-active='false' に戻し、helper textarea を blur し readonly を再付与し、KeyboardFAB aria-pressed='false' / aria-label='キーボードを開く' に同期しなければならない。
>
> *Rationale*: UAC-004 — toggle 契約 + counterexample『単発トリガー (toggle でない)』を排除。

> **FR-MOB-MODE-005** (`event_driven`) — gate true かつ入力モードで terminal 表示コンテンツ部 (.xterm-helper-textarea / [data-overlay] 配下を除く) が tap される時、システムは data-input-active='false' に戻し KeyboardFAB aria-pressed='false' に同期しなければならない。
>
> *Rationale*: UAC-005 — outside-tap 経路を useHostPointerInterceptor に集約。target.closest('[data-overlay]') で FAB / Coachmark / FontSizeControl 誤発火を除外。

> **FR-MOB-MODE-006** (`event_driven`) — gate true かつ入力モードで helper textarea が blur される時または document に Escape の keydown が dispatch される時、システムは data-input-active='false' に戻し、KeyboardFAB aria-pressed='false' に同期し、AriaLiveStatus に『閲覧モードに戻りました』のテキストを useAnnouncer 経由で 1 回 setText しなければならない (同一テキスト連続 1.5s デバウンスで重複抑止)。
>
> *Rationale*: UAC-006 + 否定役指摘『連続 emit で SR ear-fatigue』対応 — debounce contract を EARS に組み込む。

> **FR-MOB-SCROLL-002** (`event_driven`) — gate true かつ閲覧モードで .xterm-viewport 上に縦方向 touch swipe (touchstart→touchmove×N→touchend) が発生する時、システムは .xterm-viewport.scrollTop をスワイプ方向に追従して変化させなければならない (browser ネイティブ scroll に委ねる)。
>
> *Rationale*: UAC-007/008 — swipe scroll の観察契約。

> **FR-MOB-SELECT-001** (`event_driven`) — gate true かつ閲覧モードで terminal コンテンツ上に約 500ms の静止 dwell (移動 threshold 8px 未満) 保持後のドラッグが発生する時、システムは useTerminalTouchGestures の state machine で longpress-drag に遷移し xterm 標準 term.select(startCol, startRow, length) API を programmatic に呼び、term.getSelection() が非空 / .xterm-selection-layer に選択矩形が描画される状態にし、かつ document.activeElement が .xterm-helper-textarea でなく data-input-active='false' のままにしなければならない。
>
> *Rationale*: UAC-010 — 標準 API 経由の長押し選択 (依存追加 0)。否定役指摘で『POC 先送り禁止』のため ADR-touch-gesture-arbitration で決定確定。

> **FR-MOB-SELECT-002** (`event_driven`) — gate true かつ閲覧モードで dwell を伴わない 200px 縦 swipe (touchstart 後 8px 超移動が 500ms 以内) が発生する時、システムは term.getSelection() を空のまま .xterm-viewport.scrollTop のみを変化させなければならない。
>
> *Rationale*: UAC-011 — dwell で swipe/選択を判別する判別力を EARS で固定。

> **FR-MOB-JUMP-002** (`event_driven`) — gate true で scrollTop が (scrollHeight - clientHeight) ±2px から離れる時 (閲覧モード/入力モード非依存)、システムは aria-label='最新へスクロール' の <button> を overlay に出現させ、getBoundingClientRect().width≥44 / height≥44 / 非空 aria-label の条件を満たさせなければならない。
>
> *Rationale*: UAC-013/015 — mode 非依存契約と 44×44 a11y 契約を 1 EARS に。

> **FR-MOB-JUMP-003** (`event_driven`) — JumpToLatestFAB が tap される時、システムは term.scrollToBottom() を呼び .xterm-viewport.scrollTop を (scrollHeight - clientHeight) ±2px と一致させ、その後 JumpToLatestFAB を DOM から除外しなければならない (FR-MOB-JUMP-001 invariant に復帰)。
>
> *Rationale*: UAC-014 — scrollTop=末尾 + FAB unmount の連動契約。

> **FR-MOB-JUMP-004** (`event_driven`) — JumpToLatestFAB が初めて出現する時、システムは AriaLiveStatus に『最新へ移動できます』のテキストを useAnnouncer 経由で 1 回 setText しなければならない (同一テキスト連続 1.5s デバウンスで慣性 scroll による mount/unmount 反復時の連続 emit を抑止)。
>
> *Rationale*: UAC-013 + 否定役指摘『慣性 scroll で aria-live polite 連続 emit が SR ear-fatigue』対応。

> **FR-MOB-PINCH-001** (`event_driven`) — gate true で touches.length=2 の連続 touchmove が .xterm-viewport 上に発生する時、システムは 2 指間距離 d_now / d_start の比率を term.options.fontSize の base 値に乗じ整数 px に丸めた値を [8,28] に clamp して連続適用し、fontSize 変更ごとに scheduleFit (ADR 0034 rAF coalesce) を invoke しなければならない。
>
> *Rationale*: UAC-016 — 比率追従 + clamp + refit の 3 観察契約を 1 EARS に。

> **FR-MOB-PINCH-003** (`event_driven`) — touchstart で touches.length が 1 から 2 へ遷移する時、システムは useTerminalTouchGestures の state machine を pinch へ遷移させ、進行中の swipe handler を中断し、入力モードへの遷移を発生させてはならない。
>
> *Rationale*: ux edge case『1 指→2 指の pinch 開始』+ 否定役指摘『xterm 内部 touch handler との競合』対応。

> **FR-MOB-PINCH-004** (`event_driven`) — pinch が継続している時、システムは既存 Toast primitive (ADR 0063) を `ariaHidden=true` prop 拡張で再利用した PinchIndicator を画面中央に render し現在 fontSize を表示し、touchend 約 800ms 後に fade させ、indicator tap で useFontSize.reset(14) と scheduleFit() を呼ばなければならない。
>
> *Rationale*: ux F-005 step 3 — 純視覚 indicator は aria-hidden で Toast layer 再利用 (新規 primitive を作らない)。

> **FR-MOB-PERSIST-001** (`event_driven`) — pinch の touchend または FontSizeControl 操作で fontSize が確定する時、システムは usePersistedValue adapter を介して localStorage キー arc.web.term.fontSize に整数値を文字列で書き込まなければならない (try/catch で例外を握りつぶし、書込失敗時もメモリ上の state は更新)。
>
> *Rationale*: UAC-018 + ux edge case『private mode で degrade のみ』対応。

> **FR-MOB-PERSIST-002** (`event_driven`) — セッション初期 mount で localStorage キー arc.web.term.fontSize を読む時、システムは parseInt の結果が NaN なら default 14px へ fallback し、parseInt 成功かつ Number.isFinite が真なら値を [8,28] に clamp して採用しなければならない (例: '999' は parse 成功 + finite で 28 に clamp / '' / 'foo' / null は NaN で 14 へ fallback)。
>
> *Rationale*: UAC-019 counterexample『999 が 28 に clamp されない』を排除。否定役指摘で『parse 失敗 → 14 / parse 成功 + 範囲外 → clamp』を分離して厳密化。

> **FR-MOB-VVP-003** (`event_driven`) — gate true→false (デバイス回転) または入力モード→閲覧モード遷移が起きる時、システムは visualViewport listener を unsubscribe してから入力モード state を破棄しなければならない (順序保証で listener leak を防止)。
>
> *Rationale*: 否定役指摘『listener leak の余地』対応 — unsubscribe 順序を契約化。

> **FR-MOB-COACH-001** (`event_driven`) — gate true で初回閲覧モード突入時に usePersistedValue<boolean>('arc.web.term.hintSeen') が null/false を返す時、システムは Coachmark を 1 回 render し、同時に hintSeen='1' を localStorage に書き込まなければならない (tap/auto dismiss を待たない冪等性確保)。
>
> *Rationale*: ux edge case + ADR-coachmark-dismiss-and-once 決定 — 起動直後離脱時の未閲覧トレードオフを承認した上で冪等性優先。

> **FR-MOB-COACH-002** (`event_driven`) — Coachmark が render 中に tap が発生するか 5 秒経過するかのいずれか早い方が起きる時、システムは Coachmark を fade-out して unmount しなければならない (fade-out 250ms は ADR 0064 reduced-motion guard 内で reduce 時即時化)。
>
> *Rationale*: UX OQ4 Option C 採用 — tap or 5s 早い方。

### Unwanted (禁止挙動 / If-Then)

> **FR-MOB-FAB-PD-001** (`unwanted`) — もし gate true で KeyboardFAB に pointerdown イベントが発生したなら、システムは pointerdown.preventDefault() を呼び、pointerdown 前後で document.activeElement が変化せず helper textarea への focus イベントが 0 回 dispatch される状態を保たなければならない。
>
> *Rationale*: UAC-003 counterexample (B)『FAB が pointerdown で focus を奪い blur-exit listener が発火して enter/exit race』を排除する load-bearing 機構を独立 EARS 化 (否定役 blocker)。

> **FR-MOB-JUMP-005** (`unwanted`) — もし ADR 0066 (tmux-style scrollback) の 2 段 seed frame flush が完了する前なら、システムは JumpToLatestFAB を強制的に DOM 不在 (shouldShowFab=false) に保ち、seed 完了後の初回 scroll イベントが届くまで suppress しなければならない。
>
> *Rationale*: 否定役指摘『seed 完了前後で scrollHeight 動的変化により FAB が即出現→即 unmount のちらつき』対応。

> **FR-MOB-JUMP-006** (`unwanted`) — もし prefers-reduced-motion:reduce が真なら、システムは term.scrollToBottom() を smooth でなく即時ジャンプで実行し、JumpToLatestFAB および全 FAB の fade も即時化しなければならない (新規 motion は ADR 0064 の view.css 末尾 @media (prefers-reduced-motion: reduce) 単一 guard block への追記で実現)。
>
> *Rationale*: ux edge case + ADR 0064 single guard 維持。

> **FR-MOB-PINCH-002** (`unwanted`) — もし clamp 適用後の fontSize 計算結果が 8 未満になる pinch in 比率が発生したなら、システムは term.options.fontSize を 8px に張り付かせ 8 未満にしてはならない。
>
> *Rationale*: UAC-017 — 下限 clamp 契約。NaN cols / 読めない font 防止。

> **FR-MOB-VVP-002** (`unwanted`) — もし window.visualViewport API が存在しない環境なら、システムは CSS custom property `--terminal-fab-offset` の default 16px を自動 fallback として適用し、JS から CSS 変数を書き換えてはならない。
>
> *Rationale*: 古い browser fallback contract。CSS default が役割を担うため JS 介入不要。

## UAC ↔ EARS 対応表

`ux.md` の 26 件 UAC それぞれをカバーする EARS ID 群。`(D)` = 機構保証として直接の UAC 対応はないが下支えする EARS。

| UAC ID (ux.md) | カバーする EARS ID 群 |
|---|---|
| UAC-001 | `FR-MOB-GATE-001`, `FR-MOB-MODE-001` |
| UAC-002 | `FR-MOB-MODE-002` (load-bearing: `FR-MOB-FAB-PD-001` (D), ADR 0068 focus-block) |
| UAC-003 | `FR-MOB-MODE-003`, `FR-MOB-FAB-PD-001` (counterexample B 排除) |
| UAC-004 | `FR-MOB-MODE-004` |
| UAC-005 | `FR-MOB-MODE-005` |
| UAC-006 | `FR-MOB-MODE-006` |
| UAC-007 | `FR-MOB-SCROLL-001`, `FR-MOB-SCROLL-002` |
| UAC-008 | `FR-MOB-SCROLL-001`, `FR-MOB-SCROLL-002` |
| UAC-009 | `FR-MOB-MODE-002`, `FR-MOB-SCROLL-003` |
| UAC-010 | `FR-MOB-SELECT-001` |
| UAC-011 | `FR-MOB-SELECT-002` |
| UAC-012 | `FR-MOB-JUMP-001` |
| UAC-013 | `FR-MOB-JUMP-002`, `FR-MOB-JUMP-004` |
| UAC-014 | `FR-MOB-JUMP-003` |
| UAC-015 | `FR-MOB-JUMP-002` |
| UAC-016 | `FR-MOB-PINCH-001` |
| UAC-017 | `FR-MOB-PINCH-002` |
| UAC-018 | `FR-MOB-PERSIST-001` |
| UAC-019 | `FR-MOB-PERSIST-002` |
| UAC-020 | `FR-MOB-STEPPER-001` |
| UAC-021 | `FR-MOB-GATE-001`, `FR-PC-PRESERVE-001` |
| UAC-022 | `FR-MOB-GATE-001`, `FR-PC-PRESERVE-002` |
| UAC-023 | `FR-PC-PRESERVE-003` |
| UAC-024 | `FR-MOB-FAB-001` |
| UAC-025 | `FR-MOB-FAB-002` |
| UAC-026 | `FR-MOB-FAB-001` |

### 機構保証のみ (直接 UAC 対応なし) の EARS

- `FR-MOB-GATE-002` — デバイス回転 (gate true→false) の listener / state 破棄順序 (ux edge case)
- `FR-MOB-MODE-007` — helper textarea 16px CSS `!important` (iOS focus-zoom 抑止、ux edge case)
- `FR-MOB-JUMP-005` — ADR 0066 seed flush 完了まで FAB 強制不在 (late-join FAB ちらつき排除)
- `FR-MOB-JUMP-006` — prefers-reduced-motion 即時化 (ADR 0064 単一 guard 維持)
- `FR-MOB-PINCH-003` — 1→2 finger 遷移で swipe 中断 + 入力モード非遷移 (ux edge case)
- `FR-MOB-PINCH-004` — PinchIndicator を Toast primitive 再利用 (F-005 step 3 視覚 indicator)
- `FR-MOB-FAB-003` — safe-area 二重計上禁止 (`FR-LAYOUT-004` invariant)
- `FR-MOB-FAB-004` — 固定スタック順 + Toast 別 portal (F-007 step 4)
- `FR-MOB-VVP-001` — visualViewport 連動 CSS 変数更新 (iOS sticky toolbar、ux edge case)
- `FR-MOB-VVP-002` — visualViewport 不在環境 fallback
- `FR-MOB-VVP-003` — listener unsubscribe 順序 (回転時 leak 防止)
