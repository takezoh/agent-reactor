# Web UI Terminal Mobile UX — Implementation Plan

Status: Draft
Spec: [./spec.md](./spec.md)
Upstream UX: [./ux.md](./ux.md)
Related ADRs: ADR 0067 〜 ADR 0075 ([spec.md](./spec.md) 冒頭参照)

## Goal

Web UI TerminalPane (xterm.js 5.5.0 + addon-fit) のモバイル UX を `ux.md` の 7 flow / 26 UAC で固定された観察契約どおりに、PC (`pointer:fine`) 完全現状維持 + 既存 ADR 衝突回避を絶対制約として実装する。UX Open Questions 1〜4 はすべて plan-how 段階で決着 (POC 先送り禁止) — 詳細は ADR 0067 〜 0075。

ATDD は **vitest + happy-dom + @testing-library/react** を harness とし、Playwright 不在経路 (実 soft keyboard / 実 pinch / 実 long-press / 実 VoiceOver) は実機検証チェックリストへ振り分ける。

## Components

| Component | Responsibility | Depends on |
|---|---|---|
| `useMobileGate` (hook) | `matchMedia('(max-width: 767px) and (pointer: coarse)').matches` を boolean 真実源として返す。`MediaQueryList.change` を購読し true→false 遷移で『rotation transition callback』を発火して入力モード state 破棄 + overlay unmount + helper textarea readonly 解除を順序保証で実行。SSR / matchMedia 不在は false 固定 fallback。 | `window.matchMedia` |
| `IconButton` (primitive component) | 全 mobile FAB / FontSizeControl 内ボタンが薄く wrap する単一 primitive。`<button type='button'>` で 44×44px 以上 / 非空 `aria-label` / `aria-pressed` (optional) / `pointerdown.preventDefault()` による focus 奪取抑止 / 既存 theme token (`--accent` / `--surface-*`) と ADR 0059 連動を集約。既存 SessionDrawer close / CommandSearchTrigger のスタイル言語に揃える。 | theme tokens |
| `useInputMode` (hook + reducer) | `data-input-active` state (boolean) と AriaLive メッセージを管理。`toggle / enter / exit(reason)` の transition (FAB 再 tap / blur / Esc / outside-tap / gate false 遷移) を pure reducer 化。helper textarea ref を受けて focus/blur + readonly 属性付与/解除を担う。 | `useMobileGate`, helper textarea ref |
| `useHostPointerInterceptor` (hook) | host (terminal-host) capture-phase `pointerdown` listener を 1 系統だけ attach。閲覧モード時は `preventDefault()` で focus 移動をブロック、入力モード時は `target.closest('[data-overlay]')` / helper textarea を除外して outside-tap を判定し `useInputMode.exit('outside-tap')` を呼ぶ。focus-block と outside-tap を 1 listener に集約 (race 防止)。 | `useInputMode`, host ref |
| `useTerminalTouchGestures` (hook + state machine) | `.xterm-viewport` 上の `touchstart/touchmove/touchend` を 1 listener で受け、gesture state machine (`idle / swipe / dwell / longpress-drag / pinch`) で arbitration。500ms dwell 後の `touchmove` で `term.select()` を programmatic 起動 (依存追加 0)。`touches.length=2` で pinch に遷移し fontSize ratio 反映。`preventDefault` は dwell 成立後 + pinch 中のみ。 | term ref, `.xterm-viewport` ref, `useFontSize`, `scheduleFit` |
| `usePersistedValue<T>` (adapter hook) | localStorage 永続化の共通 adapter。`{key, parse, serialize, fallback, validate}` を受け、`try/catch` / `parseInt` / `Number.isFinite` / clamp / write を 1 箇所に集約。テスト時は in-memory `Map` を inject 可能 (DI)。`useFontSize` と `useCoachmarkOnce` が共有。 | localStorage (or injected storage) |
| `useFontSize` (hook) | fontSize の in-memory state と persist callback を保持。`usePersistedValue` を経由して `arc.web.term.fontSize` を扱う。pinch と FontSizeControl の双方から書き込まれる。 | `usePersistedValue` |
| `useJumpToLatest` (hook) | `.xterm-viewport` の scroll を購読し `scrollTop` と `(scrollHeight-clientHeight)` の差分 ±2px で末尾判定。`shouldShowFab` と `jumpToBottom` callback (smooth / reduced-motion 即時化切替) を返す。ADR 0066 seed flush 完了 signal を gate に取り、seed 完了前は強制 suppress。FAB 出現時に AriaLive へ debounce 経由で emit。 | `.xterm-viewport` ref, `term.scrollToBottom`, `useReducedMotion`, ADR 0066 seed-flush signal, `useAnnouncer` |
| `useVisualViewportLift` (hook) | `window.visualViewport` の `resize/scroll` を購読し、`.terminal-fab-layer` の inline CSS custom property `--terminal-fab-offset` を `(window.innerHeight - visualViewport.height - visualViewport.offsetTop + 16)px` で更新。React 再 render を発生させずに CSS 経由で全 FAB の `bottom` を 1→多 fan-out。listener は入力モード突入時に subscribe、退出 + rotation で unsubscribe を順序保証。visualViewport 不在は CSS default `16px` が自動 fallback。 | `window.visualViewport`, `useInputMode`, `.terminal-fab-layer` ref |
| `useCoachmarkOnce` (hook) | `usePersistedValue<boolean>` で `arc.web.term.hintSeen` を読み、null/false なら初回閲覧モード突入で Coachmark を 1 回 render + 同時に `hintSeen='1'` 書込 (冪等性確保)。dismiss callback (tap or 5s timer) を提供。 | `usePersistedValue`, `useInputMode` |
| `useAnnouncer` (hook + Context) | TerminalPane local `AriaLiveStatus` への `setText` API を Context 経由で配る。同一テキスト連続 1.5s デバウンスを内部実装し `last-text` / `last-emit-ts` で重複抑止 (SR ear-fatigue 防止)。 | `AriaLiveStatus` ref |
| `KeyboardFAB` (component) | `IconButton` primitive を wrap し `aria-pressed` / `aria-label` を `useInputMode` state と同期。click で `useInputMode.toggle()`。`pointerdown.preventDefault` は IconButton 内で処理。 | `IconButton`, `useInputMode`, `useVisualViewportLift` (CSS 変数経由間接) |
| `JumpToLatestFAB` (component) | `useJumpToLatest.shouldShowFab=true` の時のみ render。`aria-label='最新へスクロール'` / `IconButton` wrap / click で `jumpToBottom()`。reduced-motion 時は即時 scroll。 | `IconButton`, `useJumpToLatest` |
| `FontSizeControl` (component + disclosure popover) | Aa アイコン (`IconButton` 1 個) を表示し tap で popover を開き +/-/Reset 3 ボタン (各 `IconButton` wrap, `role=button` / 44×44 / `aria-label`) を露出。+ で 2px 増加 / - で 2px 減少 / Reset で 14px、いずれも `useFontSize.set()` + `scheduleFit()` を呼ぶ。iOS Safari Reader / Kindle / VS Code mobile の Aa popover 慣習に倣う。 | `IconButton`, `useFontSize`, `scheduleFit` |
| `Coachmark` (component) | `useCoachmarkOnce` が gate する。`KeyboardFAB` 隣の dismissible 一行注釈。tap or 5s で fade-out + unmount。既存 Tooltip / Popover primitive が存在すれば extend、無ければ最小 `<div role='status'>` で実装 (chunk-07 着手時に grep 調査)。`aria-live` は使わない (AriaLiveStatus と直交)。 | `useCoachmarkOnce` |
| `PinchIndicator` (component, reuses Toast) | 既存 `.notification-toast` primitive を `ariaHidden=true` prop 拡張で再利用し画面中央に現在 fontSize を表示。touchend 800ms 後 fade。indicator tap で `useFontSize.reset(14)`。Toast layer は別 portal で AriaLive / Coachmark と z-index 分離。 | 既存 Toast primitive (ADR 0063), `useFontSize` |
| `AriaLiveStatus` (component) | `terminal-slot` 直下 visually-hidden `<div aria-live='polite'>` 1 個を提供し、子は `useAnnouncer` 経由で `setText` (Context or ref forwarding)。ADR 0057 palette `aria-live` single slot とは role 分離 (mode 変化通知用)。 | — |
| `TerminalPane.tsx` (orchestrator) | 既存の Terminal 構築 / `conn.onOutput` / `scheduleFit` / subscribe (ADR 0030 keyed remount) を踏襲しつつ、`useMobileGate` true 分岐で上記 hook 群と overlay 子コンポーネントを mount。host element に `data-input-active` 属性を付ける。PC path は条件分岐の else 側で 1 bit も変えない。AGENTS.md の 500 行 / 関数 80 行制約を維持するため touch / pinch / gate / fontSize / fab / coachmark を独立 hook へ分離。 | (上記すべて) |

## Dependency-ordered Chunks

### Chunk: `chunk-01-pc-baseline-and-touch-harness`

- **Depends on**: (none — root chunk)
- **Members**:
  - `req:FR-PC-PRESERVE-001`
  - `req:FR-PC-PRESERVE-002`
  - `req:FR-PC-PRESERVE-003`
- **Acceptance (UAC)**: UAC-021, UAC-022, UAC-023 (PC 1 bit 不変の machine-checkable 化)
- **Test approach**:
  - **vitest + happy-dom**: 既存 TerminalPane の mount / click→focus→input / wheel scroll / 選択コピー / `data-*` 属性なし / `readonly` なし を snapshot + 関数 assertion で固定 (PC baseline as CI gate)
  - **touch-event shim harness** を確立: `tapAt(x,y)` / `swipeFromTo(start,end,duration)` / `pinchByRatio(r)` / `longPressAndDrag(x,y,dx,dy)` / `mockMatchMedia(query, matches)` / `mockVisualViewport({height, offsetTop})`
  - **happy-dom 限界の実証** (open question): `matchMedia('(pointer: coarse/fine)')` の mock / `TouchEvent` `touches.length=2` 合成 / `.xterm-viewport.scrollTop` 計算 / `getBoundingClientRect()` 44×44 — 不足分は実機チェックリストへ移送

### Chunk: `chunk-02-mobile-gate-and-iconbutton-primitive`

- **Depends on**: `chunk-01-pc-baseline-and-touch-harness`
- **Members**:
  - `component:useMobileGate`
  - `component:IconButton`
  - `req:FR-MOB-GATE-001`
  - `req:FR-MOB-GATE-002`
  - `adr:adr-20260626-mobile-gate-matchmedia` ([ADR 0067](../../adr/0067-mobile-gate-matchmedia.md))
  - `adr:adr-20260626-pattern-adoption-mode-affordances` ([ADR 0075](../../adr/0075-pattern-adoption-mode-affordances.md))
- **Acceptance (UAC)**: UAC-001 (gate true 時の初期), UAC-021 (gate false で overlay 全不在), UAC-022 (narrow-window + pointer:fine = PC 扱い), UAC-024 / UAC-026 (44×44 / aria-label / aria-pressed primitive)
- **Test approach**:
  - **vitest + happy-dom**: `mockMatchMedia` で AND 契約の 4 組合せ網羅。change イベント発火で gate true→false 遷移時の overlay unmount / readonly 解除 / state 破棄を assertion。IconButton primitive の `pointerdown.preventDefault` / 44×44 を unit test。
  - **実機**: matchMedia 評価は実機 (iOS Safari 17+ / iPadOS / Android Chrome) で確認 — happy-dom の `pointer:coarse` mock 限界は chunk-01 で実証する

### Chunk: `chunk-03-mode-separation-focus-block-and-arialive`

- **Depends on**: `chunk-02-mobile-gate-and-iconbutton-primitive`
- **Members**:
  - `component:useInputMode`
  - `component:useHostPointerInterceptor`
  - `component:AriaLiveStatus`
  - `component:useAnnouncer`
  - `component:KeyboardFAB`
  - `req:FR-MOB-MODE-001..007`
  - `req:FR-MOB-FAB-PD-001`
  - `req:FR-MOB-FAB-001`
  - `adr:adr-20260626-mode-separation-focus-block-and-zoom-guard` ([ADR 0068](../../adr/0068-mode-separation-focus-block-and-zoom-guard.md))
  - `adr:adr-20260626-arialive-debounce-and-jump-fab-seed-stability` ([ADR 0073](../../adr/0073-arialive-debounce-and-jump-fab-seed-stability.md))
- **Acceptance (UAC)**: UAC-001 ('false' invariant), UAC-002 / UAC-009 (focus 発火 0), UAC-003 / UAC-004 (toggle), UAC-005 (outside-tap), UAC-006 (blur/Esc + AriaLive), UAC-024 / UAC-026 (KeyboardFAB a11y)
- **Test approach**:
  - **vitest + happy-dom**: capture-phase `pointerdown` listener が 1 系統のみ attach されることを spy で確認。閲覧モード tap で `focus` event 数 0 / `activeElement` 不変を assertion。toggle / blur / Esc / outside-tap 4 経路の reducer 遷移を pure unit test。debounce 1.5s を fake timer で検証。
  - **実機**: iOS Safari 17+ で helper textarea 16px `!important` による focus-zoom 抑止確認 (CSS 計算は happy-dom で不十分)、VoiceOver / TalkBack で『閲覧モードに戻りました』が 1 回のみ通知されることを確認

### Chunk: `chunk-04-touch-gestures-arbitration`

- **Depends on**: `chunk-03-mode-separation-focus-block-and-arialive`
- **Members**:
  - `component:useTerminalTouchGestures`
  - `req:FR-MOB-SCROLL-001..003`
  - `req:FR-MOB-SELECT-001/002`
  - `req:FR-MOB-PINCH-001..004` (FR-MOB-PINCH-004 の実 wire は chunk-05 で完成)
  - `adr:adr-20260626-touch-gesture-arbitration-and-long-press-selection` ([ADR 0071](../../adr/0071-touch-gesture-arbitration-and-long-press-selection.md))
- **Acceptance (UAC)**: UAC-007 / UAC-008 (pan-y swipe scroll), UAC-009 (swipe 中 focus 0), UAC-010 (500ms dwell + term.select 非空), UAC-011 (dwell 不在 swipe は scroll のみ), UAC-016 (pinch 比率追従 + clamp + refit)
- **Test approach**:
  - **vitest + happy-dom**: state machine を pure reducer として TouchEvent shim で単体 test (`idle / swipe / dwell / longpress-drag / pinch` 5 状態の transition table)。`preventDefault` 呼出が dwell 成立後 + pinch 中のみであることを spy。`term.select()` の programmatic call 引数を mock で検証。
  - **実機**: 500ms dwell + drag で iOS native selection menu / Android long-press menu の挙動整合 (happy-dom 不可)。`.xterm-viewport` の実 scroll 慣性、2 指 pinch の実体験。Android Chrome で haptic feedback 不在を承認確認。

### Chunk: `chunk-05-fontsize-persist-and-control`

- **Depends on**: `chunk-04-touch-gestures-arbitration`
- **Members**:
  - `component:usePersistedValue`
  - `component:useFontSize`
  - `component:FontSizeControl`
  - `component:PinchIndicator`
  - `req:FR-MOB-PERSIST-001/002`
  - `req:FR-MOB-STEPPER-001`
  - `adr:adr-20260626-fontsize-persist-clamp` ([ADR 0070](../../adr/0070-fontsize-persist-clamp.md))
- **Acceptance (UAC)**: UAC-016 (pinch → fontSize 変更後 PinchIndicator), UAC-017 (8 下限 clamp), UAC-018 (20px 永続化), UAC-019 (999 → 28 clamp), UAC-020 (FontSizeControl +/-/Reset a11y)
- **Test approach**:
  - **vitest + happy-dom**: `usePersistedValue` を in-memory Map に DI して 4 ケース (`'999'` → 28 / `''` → 14 / `'foo'` → 14 / `null` → 14) を網羅。`localStorage` throw 時の degrade も assertion。FontSizeControl disclosure popover の +/-/Reset 3 ボタン a11y (`role=button` / 44×44 / `aria-label`) を `getBoundingClientRect()` で。PinchIndicator が Toast primitive を `ariaHidden=true` で再利用していることを props assertion。
  - **実機**: private mode (Safari Private / Chrome Incognito) で起動が壊れず default 14 で degrade のみ確認

### Chunk: `chunk-06-jump-to-latest-fab-and-seed-gate`

- **Depends on**: `chunk-03-mode-separation-focus-block-and-arialive`
- **Members**:
  - `component:useJumpToLatest`
  - `component:JumpToLatestFAB`
  - `req:FR-MOB-JUMP-001..006`
- **Acceptance (UAC)**: UAC-012 (末尾で DOM 不在), UAC-013 (離脱で出現 + polite emit), UAC-014 (tap で末尾 + unmount), UAC-015 (44×44 / aria-label)
- **Test approach**:
  - **vitest + happy-dom**: `.xterm-viewport.scrollTop` を直接書き換えて scroll event 合成、末尾 ±2px の boundary で `shouldShowFab` boolean が切替わることを確認。ADR 0066 seed flush 完了 signal を mock で未完了状態にして FAB が強制 DOM 不在になることを assertion。慣性 scroll の mount/unmount 反復で AriaLive が 1.5s デバウンス通り 1 回のみ emit を fake timer で。
  - **実機**: 末尾判定 ±2px の Retina (DPR=2/3) + Browser zoom 110-150% での fluctuation 検証。慣性 scroll の FAB ちらつき有無。VoiceOver で 1 回のみ通知。

### Chunk: `chunk-07-visualviewport-lift-coachmark-and-integration`

- **Depends on**: `chunk-05-fontsize-persist-and-control`, `chunk-06-jump-to-latest-fab-and-seed-gate`
- **Members**:
  - `component:useVisualViewportLift`
  - `component:useCoachmarkOnce`
  - `component:Coachmark`
  - `component:TerminalPane.tsx`
  - `req:FR-MOB-FAB-002/003/004`
  - `req:FR-MOB-VVP-001/002/003`
  - `req:FR-MOB-COACH-001/002`
  - `adr:adr-20260626-fab-overlay-layout-and-visualviewport-lift` ([ADR 0069](../../adr/0069-fab-overlay-layout-and-visualviewport-lift.md))
  - `adr:adr-20260626-coachmark-dismiss-and-once` ([ADR 0072](../../adr/0072-coachmark-dismiss-and-once.md))
  - `adr:adr-20260626-migration-pc-only-to-pc-plus-mobile` ([ADR 0074](../../adr/0074-migration-pc-only-to-pc-plus-mobile.md))
- **Acceptance (UAC)**: UAC-025 (terminal-host box 不変), F-001 step 9 (visualViewport-lift), UAC F-007 step 4 (固定スタック順 + Toast 別 portal), F-001 step 3 (Coachmark 1 回 + dismiss)
- **Test approach**:
  - **vitest + happy-dom**: `mockVisualViewport({height, offsetTop})` で `.terminal-fab-layer` の inline `--terminal-fab-offset` が更新されることを `getComputedStyle` で確認。listener subscribe / unsubscribe 順序を spy で検証。Coachmark hintSeen 初回 render 時の冪等書込を `usePersistedValue` mock で。tap or 5s 早い方で unmount を fake timer。Coachmark 用既存 primitive grep 調査 (chunk 着手前) で extend 方針確定。
  - **実機**: iOS soft keyboard 表示で FAB が visualViewport 上端より上に追従、address bar 表示/非表示の追従挙動許容、デバイス回転で gate true→false 時の listener leak なし、VoiceOver で Coachmark popup が自然に読まれ AriaLive と二重通知にならない
  - **AGENTS.md 制約検証**: TerminalPane.tsx の行数 / 関数長を計測し 500 行 / 80 行制約に収まることを確認。万一超過時は orchestration を sub-component に分割する選択肢を plan-impl 段階で検討。

## Test Plan

### Automated (vitest + happy-dom + @testing-library/react)

happy-dom + touch shim harness で再現可能な UAC:

| UAC | EARS | 自動テスト観点 |
|---|---|---|
| UAC-001 | FR-MOB-GATE-001, FR-MOB-MODE-001 | matchMedia mock + `data-input-active='false'` / `readonly` / `activeElement≠helper` |
| UAC-002 | FR-MOB-MODE-002 | tap での `focus` event 発火数 = 0 / `activeElement` 不変 |
| UAC-003 | FR-MOB-MODE-003, FR-MOB-FAB-PD-001 | FAB tap → `aria-pressed='true'` / readonly 解除 / focus 移動 / pointerdown.preventDefault spy |
| UAC-004 | FR-MOB-MODE-004 | FAB 再 tap → toggle off の assertion |
| UAC-005 | FR-MOB-MODE-005 | terminal 内部 tap で `[data-overlay]` 除外による outside-tap exit |
| UAC-006 | FR-MOB-MODE-006 | blur / Escape 経路で polite emit 1 回 (debounce fake timer) |
| UAC-007 / 008 | FR-MOB-SCROLL-001/002 | `touch-action:pan-y` CSS / scrollTop 追従 |
| UAC-009 | FR-MOB-MODE-002, FR-MOB-SCROLL-003 | swipe 中の focus 発火 0 |
| UAC-010 | FR-MOB-SELECT-001 | 500ms dwell + drag で `term.select()` call + `getSelection()` 非空 |
| UAC-011 | FR-MOB-SELECT-002 | dwell 不在 swipe で `getSelection()` 空 + scrollTop 変化 |
| UAC-012 / 013 / 014 / 015 | FR-MOB-JUMP-001..004 | scrollTop ±2px boundary / mount-unmount / 44×44 / polite emit |
| UAC-016 / 017 | FR-MOB-PINCH-001/002 | pinch 比率による fontSize 変更 + [8,28] clamp |
| UAC-018 / 019 | FR-MOB-PERSIST-001/002 | localStorage 4 ケース (`'999'`/`''`/`'foo'`/`null`) + try/catch degrade |
| UAC-020 | FR-MOB-STEPPER-001 | FontSizeControl popover 内 +/-/Reset の `role=button` / 44×44 / aria-label |
| UAC-021 | FR-MOB-GATE-001, FR-PC-PRESERVE-001 | gate false で全 overlay DOM 不在 |
| UAC-022 | FR-PC-PRESERVE-002 | 700px + pointer:fine で PC 扱い |
| UAC-023 | FR-PC-PRESERVE-003 | gate false で wheel scroll legacy |
| UAC-024 / 025 / 026 | FR-MOB-FAB-001/002 | FAB 44×44 / aria-label / aria-pressed / terminal-host box 不変 |

### Manual on Device (実機検証チェックリスト)

対象環境: **iOS Safari 17+** / **iPadOS 17+** / **Android Chrome 最新**

検証項目 (`open_questions` の実機検証チェックリスト記述から):

- [ ] **visualViewport-lift** — 入力モード突入で FAB が visualViewport 上端より上に追従、address bar 表示/非表示の追従挙動を許容
- [ ] **iOS focus-zoom 抑止** — helper textarea 16px CSS `!important` で focus 時の viewport auto-zoom が起きないこと
- [ ] **long-press 500ms dwell + `term.select()`** — iOS native selection menu / Android long-press menu と整合
- [ ] **Android Chrome haptic feedback 不在** — iOS は出るが Android Chrome は出ない差を承認
- [ ] **VoiceOver / TalkBack で polite emit** — 『閲覧モードに戻りました』『最新へ移動できます』が二重通知にならないこと、debounce 1.5s が効くこと
- [ ] **Coachmark popup の SR 読み上げ** — `<div role='status'>` が SR で自然に読まれ AriaLive と直交すること
- [ ] **末尾判定 ±2px の DPR fluctuation** — Retina (DPR=2/3) + Browser zoom 110-150% で JumpToLatestFAB がチラつかないこと。fluctuation 発生時は margin を +1〜+3px 増やして再評価
- [ ] **デバイス回転 (gate true→false)** — visualViewport listener leak なし、入力モード state 破棄、PC path 即時復帰
- [ ] **private mode** (Safari Private / Chrome Incognito) — localStorage 例外で起動が壊れず default 14 で degrade のみ
- [ ] **ADR 0066 seed flush** — late-join 初期の JumpToLatestFAB ちらつきがないこと

## Existing ADR Conflict Audit

既存 ADR との衝突点 audit (PLAN JSON の `resolved_issues` および各 ADR の Consequences から):

| 既存 ADR | 本計画での扱い | 衝突有無 |
|---|---|---|
| [ADR 0029](../../adr/0029-terminal-host-flex-height.md) (terminal-host `flex:1 1 0` / dvh) | `.terminal-fab-layer` は terminal-slot 直下の absolute 兄弟で sizing 非影響 ([ADR 0069](../../adr/0069-fab-overlay-layout-and-visualviewport-lift.md)) | なし |
| [ADR 0030](../../adr/0030-keyed-remount.md) (keyed remount) | 入力モード state は TerminalPane 内 local で session 切替時に keyed remount で破棄。別 `TerminalPaneMobile.tsx` を新設しないため subscribe 所有権と整合 ([ADR 0074](../../adr/0074-migration-pc-only-to-pc-plus-mobile.md)) | なし |
| [ADR 0034](../../adr/0034-refit-raf-coalesce.md) (refit rAF coalesce) | fontSize 変更も `scheduleFit` 経由で coalesce。pinch / FontSizeControl / Reset の 3 経路すべて経由 | なし |
| [ADR 0057](../../adr/0057-palette-single-aria-live.md) (palette single aria-live) | TerminalPane local `AriaLiveStatus` と role 分離 (palette = palette 開閉 / terminal = mode 変化通知)。同時 emit は現状アーキテクチャで発生しない ([ADR 0073](../../adr/0073-arialive-debounce-and-jump-fab-seed-stability.md)) | なし |
| [ADR 0059](../../adr/0059-theme.md) (theme) | fontSize は theme と独立 key (`arc.web.term.fontSize`) で衝突しない | なし |
| [ADR 0063](../../adr/0063-notification-toast-primitive.md) (Toast primitive) | `PinchIndicator` は Toast primitive を `ariaHidden=true` prop 拡張で再利用 (新規 primitive を作らない) | なし |
| [ADR 0064](../../adr/0064-reduced-motion-single-guard.md) (reduced-motion 単一 guard) | 新規 animation (smooth scroll / FAB fade / Coachmark fade / PinchIndicator fade) は view.css 末尾の単一 guard ブロックに追記 | なし |
| [ADR 0065](../../adr/0065-terminal-slot-absolute-overlay.md) (terminal-slot absolute overlay) | `.terminal-fab-layer` は terminal-slot 直下、box 不変 (UAC-025) | なし |
| [ADR 0066](../../adr/0066-terminal-scrollback-via-vt-buffer.md) (tmux-style scrollback) | `JumpToLatestFAB` は seed flush 完了まで suppress (`FR-MOB-JUMP-005` / [ADR 0073](../../adr/0073-arialive-debounce-and-jump-fab-seed-stability.md)) | なし |

`FR-LAYOUT-004` (safe-area single-source via `.app-shell`) との衝突: `FR-MOB-FAB-003` で FAB 側の `env(safe-area-inset-*)` 加算を禁止する invariant を契約化し、二重計上を構造的に防止 ([ADR 0069](../../adr/0069-fab-overlay-layout-and-visualviewport-lift.md))。

## Open Questions

PLAN JSON の `open_questions` を以下の chunk タイミングで確定する。

1. **実機検証チェックリストの実施計画** — iOS Safari 17+ / iPadOS 17+ / Android Chrome 最新で (a) visualViewport-lift、(b) iOS focus-zoom 抑止、(c) long-press dwell + `term.select()` と OS selection menu の整合、(d) Android Chrome の haptic 不在承認、(e) VoiceOver / TalkBack polite emit / Coachmark popup 読み上げを確認。**確定タイミング: chunk-07 完了時の最終確認**。本 plan.md 上記「Manual on Device」セクションが手順正本。

2. **happy-dom harness 限界の実証検証** — chunk-01 で (a) `matchMedia('(pointer: coarse/fine)')` mock、(b) `TouchEvent` `touches.length=2` 合成、(c) `.xterm-viewport.scrollTop` / `scrollHeight` 計算、(d) `getBoundingClientRect()` 44×44 を実証。harness 不足が判明した UAC は (i) `jest-matchmedia-mock` 等の追加で吸収 or (ii) 実機チェックリストへ移送。**確定タイミング: chunk-01 着手中**。

3. **Coachmark 用既存 primitive の調査結果** — chunk-07 着手時に `src/client/web/src/components` 配下の Tooltip / Popover / Hint / Snackbar 系 primitive を grep 調査。再利用候補があれば extend (dismissible variant 追加)、無ければ最小 `<div role='status'>` で実装。**確定タイミング: chunk-07 着手時**。

4. **末尾判定 ±2px の実機 fluctuation 検証** — Retina (DPR=2/3) + Browser zoom 110-150% で sub-pixel `scrollTop` fluctuation が出て FAB がチラつくケースが実機で発生したら margin を +1px 〜 +3px 増やして再評価。**確定タイミング: chunk-06 完了後の実機チェックリスト**。

5. **AGENTS.md 制約遵守の検証** — `TerminalPane.tsx` は現状 167 行で、本計画では orchestration 責務に絞り 9 hook + 5 overlay を独立 file へ分離する。chunk-07 完了時点で `TerminalPane.tsx` 行数 / 関数長を計測し 500 行 / 関数 80 行制約に収まることを確認。万一超過時は orchestration を sub-component (`TerminalPaneOrchestratorMobile` / `TerminalPaneOrchestratorPC`) に分割する選択肢を plan-impl 段階で検討。**確定タイミング: chunk-07 完了時**。
