# Spec — Web UI コマンドパレット 再設計 (TUI 移植からの Web 最適化, 案 A 最小)

- **作成日**: 2026-06-25
- **ブランチ**: `main`
- **ux**: [ux.md](./ux.md)
- **plan**: [plan.md](./plan.md)
- **ADRs (新規, 本 spec 範囲)**: [0054](../../adr/0054-palette-cursor-identity-by-tool-id.md), [0055](../../adr/0055-palette-submit-freeze-via-lift-state.md), [0056](../../adr/0056-palette-store-slice-composition.md), [0057](../../adr/0057-palette-single-aria-live-slot.md), [0058](../../adr/0058-palette-active-context-data-path.md)
- **ADRs (既存・ux で生成済み)**: [0050](../../adr/0050-palette-scope-unify-with-disabled-policy.md), [0051](../../adr/0051-palette-hover-follow-single-cursor.md), [0052](../../adr/0052-palette-active-context-header-with-change-feedback.md), [0053](../../adr/0053-palette-chip-toggle-keybinding-redesign.md)
- **ADRs (制約として参照)**: ADR-0036 (store 純粋性), ADR-0040 (composing guard), ADR-0046 (client-local activeSessionID), ADR-0047 (disabledReason single source), ADR-0049 (ASCII-only)

## Goal

accepted な ux.md (UAC-001〜UAC-018, F-001〜F-008) と ADR-0050〜0053 を入力に、src/client/web/ 配下の command palette を「scope 統合 + disabled visible + hover follow + active context header + chip 3 経路 toggle + push 送信先明示 toast + submit 中凍結」へ再設計する。本 spec はコードを書かず、EARS-format FR / NFR / Acceptance と新規 ADR 5 本 (cursor identity / submit freeze / store slice 分割 / 単一 aria-live / active context data path) で再設計 1 PR 範囲の観測可能な振る舞いと内部設計判断を確定する。

実装は plan.md の m1〜m5 milestones 順 (pure helpers → store slices → presentation listbox → chips + freeze → scope 削除 + tests) で進める。既存 ADR (0036 / 0040 / 0046 / 0047 / 0049) の不変条件 (store は DOM/HTTP 非保有 / composing 中 short-circuit / activeSessionID は client-local / disabledReason は single source / 表面文言は英語のみ) を破らない。

## Scope

### In Scope

- src/client/web/ 配下の CommandPalette / ToolSelectPhase / ParamSelectPhase / palette store / lib/tools / 関連 hook のみを変更する
- ScopeSegment.tsx および ScopeSegment.test.tsx の撤去 (ADR-0050)
- palette store の paletteScope / setScope / initialScope (PaletteScope 型を含む) の撤去と関連 test の置換 (ADR-0050)
- 統合 1 listbox + 有効 → separator → disabled の安定 sort + disabled 行 inline 表示 (UAC-004, UAC-015)
- selectedToolId を cursor identity の単一情報源とし、paramCursor は paramSelect phase 専用に責務縮約 (FR-026)
- hover と keyboard が単一 cursor state を共有する single-cursor highlight (UAC-006, UAC-007, UAC-008)
- ActiveContextHeader (client-local activeSessionID から projBase / sid8 を導出 + 変化時 600ms flash + 単一 aria-live announce slot) (UAC-001, UAC-002, UAC-013)
- Worktree / Host chip の role=switch + pointer click + Alt+W / Alt+H (event.code='KeyW'/'KeyH') + Tab→Space + chip Enter (4 経路) toggle (UAC-008, UAC-009, UAC-010)
- paramless push 送信時 info toast (projBase + sid8 monospace + title=full path + full sessionID) — 既存 notifications API の title 引数を利用 (UAC-011, UAC-012)
- disabled 行を ↑↓ で skip しつつ visible に残す cursor navigation (UAC-015)
- list の operable 行 0 件異常状態の status badge slot (Loading commands… / No commands available) — ただし standard new-session を含めて enabled が常に 1 件以上ある前提を spec で明示し、precondition を成立可能な状態 (sessionConfig=null の hydrate 前 + standard tools が未登録の場合) に絞る (FR-024 / Assumptions に整理)
- disabled 行を Enter / pointer click した場合の inline status (kind=warning, role=status, 単一 aria-live, 4s auto-clear + seq 増分で再 announce) + 行 flash (toast は出さない) (UAC-005)
- submit in-flight 中の palette UI 凍結を CommandPalette 1 箇所に lift state する形で実装 (Active context 行 / sorted list / status badge を frozen snapshot props で受ける) (UAC-017, UAC-018)
- selectedToolId ベースの cursor 再計算ルール (同 id 残存 → そのまま、同 id 消失 → cursor の旧 index 起点で 前方優先 nearest enabled)
- IME composing guard を全新規入力経路 (pointer click on row, chip pointer click, chip Space, chip Enter, Alt+W, Alt+H) に拡張 (ADR-0040 延長)
- projBase basename 抽出 pure helper (/ 終端 fallback / 同名 disambiguator は active header のみで適用 / Windows path `\` 対応) (FR-027)
- ParamTextInput.tsx の Tab/Shift+Tab hijack 撤去と表示テキスト 'Tab to toggle' / 'Shift+Tab to toggle' 削除 (ADR-0053)
- ToolListGrouped は単一の role=listbox 内で aria-activedescendant を sortedList の論理 index ベースで採番し、separator は role=presentation の <li> として cursor 算出から除外する設計の明文化
- Alt+W/H は event.code='KeyW'/'KeyH' で physical key 判定 (NFR / Open Question 整理)
- 新規 Vitest test 追加 + 既存 test 移植 + 削除の方針表 (UAC ↔ test file mapping table)
- no-japanese.test.ts pass (ADR-0049)

### Out of Scope

- global header (palette 外) の active session 常時表示
- Stop session lifecycle action の復活
- Cmd+N / Cmd+P 等の追加グローバル hotkey
- recently used / frequency-based dynamic sort
- destructive push の確認モーダル
- mobile / touch-first レイアウト最適化
- breadcrumb / favorites UI
- TUI 側 palette (src/client/tui/palette*.go) の変更
- platform / orchestrator 層への変更
- wire-format (server 側 API / JSON schema) の変更 — SessionInfo に project 識別子が不足する場合は active header を sid8 のみ fallback 表示し別 PR で対応
- 新規 npm 依存の追加
- Playwright e2e の本格導入 (Alt+W/H 実機検証は manual QA 扱い)
- 新規 NotificationsApi action の追加 (既存 add()/success() の title 引数を利用するに留める)

## Functional Requirements (EARS)

### FR-001 — 単一 listbox 構造 (ScopeSegment 撤去)
- **type**: ubiquitous
- **statement**: The palette shall render exactly one role='listbox' (no ScopeSegment) on phase='toolSelect', listing standard tools first followed by push tools in the registry order.
- **rationale**: ADR-0050 (scope 統合)。invariant 系で常時の構造を固定。
- **traces**: UAC-004, F-002

### FR-002 — 有効/separator/disabled の 2 段グループ sort
- **type**: state_driven
- **statement**: While the listbox contains both enabled and disabled tools, the palette shall render enabled tools above a single role='presentation' separator and disabled tools below, preserving registry order within each group.
- **rationale**: UAC-004 / UAC-015。継続中の表示順序を invariant 化。
- **traces**: UAC-004, UAC-015, F-002

### FR-003 — disabled 行の inline 表示 (single source)
- **type**: ubiquitous
- **statement**: The system shall render each disabled tool row with a warning icon and the verbatim string returned by scopeDisabledReason(tool, daemon) on the same row as its label, without applying any string transformation.
- **rationale**: ADR-0047 (single source)。
- **traces**: UAC-004, UAC-005

### FR-004 — disabled skip cursor navigation
- **type**: event_driven
- **statement**: When the user presses ArrowDown, ArrowUp, Ctrl+N, or Ctrl+P on the input while phase='toolSelect', the system shall move the cursor to the nearest enabled row in that direction, skipping disabled rows while leaving them visible.
- **rationale**: UAC-015。
- **traces**: UAC-015, F-002

### FR-005 — disabled 行 Enter/click 時の inline status + flash
- **type**: event_driven
- **statement**: When the user presses Enter or fires mousedown on a disabled row, the system shall keep the palette open, play a single shake/flash animation on that row, and emit an inline status `"<label>" is unavailable: <reason>` into the single aria-live='polite' announce slot, without emitting any toast.
- **rationale**: UAC-005。
- **traces**: UAC-005, F-003

### FR-006 — enabled 行 hover で cursor 追従
- **type**: event_driven
- **statement**: When the user fires pointermove over an enabled row, the system shall set the cursor to that row's logical index and update aria-activedescendant accordingly.
- **rationale**: UAC-006 / UAC-008。
- **traces**: UAC-006, UAC-008, F-002

### FR-007 — disabled 行 hover は cursor 不変
- **type**: event_driven
- **statement**: When the user fires pointermove over a disabled row, the system shall apply a subtle hover style to that row only and shall not move the cursor.
- **rationale**: UAC-007。
- **traces**: UAC-007

### FR-008 — mouseleave で cursor 維持
- **type**: event_driven
- **statement**: When the pointer leaves the listbox (mouseleave), the system shall keep the cursor at its last value so that the visual highlight remains at the most recent keyboard or pointer position.
- **rationale**: UAC-008。hover で動かない invariant を保つ。
- **traces**: UAC-008

### FR-009 — Active context header 表示 (3 ブランチ)
- **type**: ubiquitous
- **statement**: The palette header shall display an Active context row sourced from client-local activeSessionID (ADR-0046), showing "Active: <projBase> / <sid8>" when an active session resolves to a known project, "Active: ??? / <sid8>" when sessionID is set but project resolution fails, and an icon plus the text "— No active session" otherwise.
- **rationale**: UAC-001 / UAC-013。SessionInfo.projectPath 欠落時の fallback も含む。
- **traces**: UAC-001, UAC-013, F-001

### FR-010 — Active context 変化時 flash + announce
- **type**: event_driven
- **statement**: When the client-local activeSessionID changes while the palette is open and submitting=false, the system shall update the Active context row text, flash its background for 600ms via a CSS animation, and announce "Active session changed to <projBase> / <sid8>" exactly once into the single aria-live='polite' slot.
- **rationale**: UAC-013。flash 発火条件は activeSessionID 文字列比較に限定 (projBase の disambiguator 変動では発火しない)。
- **traces**: UAC-013, F-001

### FR-011 — push tool の enabled/disabled 遷移時 flash
- **type**: event_driven
- **statement**: When an active-session change causes a push tool row to transition from disabled to enabled or vice versa, the system shall move that row to the new group, play a single flash animation on it, and add or remove its inline disabled markings accordingly.
- **rationale**: UAC-014。
- **traces**: UAC-014, F-002

### FR-012 — submit 中 UI freeze (lift state)
- **type**: state_driven
- **statement**: While submitting=true, the system shall freeze the Active context row text, sorted-list grouping, status badge text, and chip visibility by reading from a snapshot captured at submit start, and shall set aria-disabled=true on the listbox.
- **rationale**: UAC-017。freeze は CommandPalette が presentation 層で lift-state し、子コンポーネントは props 経由で frozen snapshot を受ける。
- **traces**: UAC-017, F-006

### FR-013 — submit 解決時の palette 後処理
- **type**: event_driven
- **statement**: When submit resolves (success or terminal error) after a view-update arrived during the frozen window, the system shall close the palette on success or surface the error inline, and the next palette open shall reflect the latest snapshot.
- **rationale**: UAC-017 後段。
- **traces**: UAC-017, F-006

### FR-014 — paramless push 成功時の info toast
- **type**: event_driven
- **statement**: When a paramless push tool submit succeeds, the system shall emit a single info-level toast notification with text "Sent '<command>' → <projBase> · <sid8>" by calling the existing notifications add() with title set to the full project path and the full sessionID joined by a newline; the snapshot used for projBase and sid8 shall be the snapshot captured at submit start, not the latest view-update.
- **rationale**: UAC-011 / UAC-018。既存 NotificationsApi の title 引数を利用 (新 API 追加なし)。
- **traces**: UAC-011, UAC-018, F-005

### FR-015 — push toast の非対話性
- **type**: ubiquitous
- **statement**: The push toast shall not contain interactive elements (no undo, no session-switch link) and shall auto-dismiss per the existing notifications policy.
- **rationale**: UAC-012。
- **traces**: UAC-012

### FR-016 — chip の role=switch + 物理キーヒント
- **type**: ubiquitous
- **statement**: On phase='paramSelect' the system shall render Worktree and Host chips as role='switch' with aria-checked reflecting the current toggle state, displaying '[W]' and '[H]' physical-key hint glyphs to the left of each chip.
- **rationale**: UAC-009 / NFR a11y。
- **traces**: UAC-009, F-007

### FR-017 — chip pointerdown による toggle (focus 保持)
- **type**: event_driven
- **statement**: When the user fires pointerdown on a chip, the system shall toggle the chip's aria-checked, preventDefault to keep the command input focus, and shall not submit the form.
- **rationale**: UAC-010。
- **traces**: UAC-010, F-007

### FR-018 — Alt+W / Alt+H ホットキー
- **type**: event_driven
- **statement**: When the user presses Alt+W (resp. Alt+H) and event.code is 'KeyW' (resp. 'KeyH') while palette is open, phase='paramSelect', the command input is rendered and visible, the corresponding chip is visible, and composing=false, the system shall toggle the Worktree (resp. Host) chip's aria-checked without losing input focus.
- **rationale**: UAC-009。event.code 指定で macOS Option+W = ∑ の OS 依存差異を回避。
- **traces**: UAC-009, F-007

### FR-019 — chip 上 Space で toggle
- **type**: event_driven
- **statement**: When a chip has DOM focus and the user presses Space, the system shall toggle that chip's aria-checked.
- **rationale**: UAC-010 (Tab→Space)。
- **traces**: UAC-010, F-007

### FR-020 — chip 上 Enter で toggle + submit 阻止
- **type**: event_driven
- **statement**: When a chip has DOM focus and the user presses Enter, the system shall toggle that chip's aria-checked and shall preventDefault so that the form does not submit.
- **rationale**: UAC-010 / F-008。
- **traces**: UAC-010, F-007, F-008

### FR-021 — Tab/Shift+Tab natural traversal + hijack 撤去
- **type**: ubiquitous
- **statement**: Tab and Shift+Tab shall function as natural focus-trap traversal across the palette without intercepting them for chip toggles, and the previous ParamTextInput Tab/Shift+Tab hijack and its 'Tab to toggle' / 'Shift+Tab to toggle' hint text shall be removed.
- **rationale**: ADR-0053 (Tab 撤去)。
- **traces**: UAC-010

### FR-022 — chip visibility 消失時の focus fallback
- **type**: event_driven
- **statement**: When the selected project's isGit or isSandboxed transitions to false and DOM focus is on the corresponding chip, or when selectedProject becomes null while focus is on either chip, the system shall move focus back to the command input.
- **rationale**: UAC-010 fallback。
- **traces**: UAC-010

### FR-023 — composing guard 全経路拡張
- **type**: unwanted
- **statement**: If composing=true (IME middle), then the system shall guard every input path including setQuery, moveCursor, confirmTool, submit, pointer click on row, chip pointer click, chip Space, chip Enter, Alt+W, Alt+H by short-circuiting before mutating state or preventDefault.
- **rationale**: ADR-0040 拡張。capture-phase での順序は (1) palette 条件判定 (2) composing 判定 (3) preventDefault (4) state 変更 とする。
- **traces**: UAC-010, F-007

### FR-024 — 0 件状態の status badge
- **type**: state_driven
- **statement**: While phase='toolSelect' and the sorted-list enabled-rows count equals 0, the system shall display 'Loading commands…' in the status badge slot when sessionConfig is null, 'No commands available' when sessionConfig is hydrated, and shall treat Enter as a no-op.
- **rationale**: UAC-016。new-session standard tool が常に enabled なため通常運用では到達しないが、Test/edge での到達条件 (standard tools 未登録 + sessionConfig=null) を invariant として固定。
- **traces**: UAC-016

### FR-025 — ctx=null 時の責務境界
- **type**: unwanted
- **statement**: If httpFactory validation fails (ctx=null), then the system shall not render the Active context row and shall display 'Unavailable' in the status badge slot; the ctx state shall be propagated from CommandPalette to ActiveContextHeader by conditional render at the CommandPalette level.
- **rationale**: 責務境界: ActiveContextHeader は ctx を知らず、CommandPalette が条件 render する。
- **traces**: UAC-001 (fallback)

### FR-026 — cursor 再計算 (selectedToolId anchor)
- **type**: event_driven
- **statement**: When the sorted list changes due to a view-update, the system shall recompute the cursor by selectedToolId: if the previously selected tool id exists in the new sorted list and is enabled, the cursor shall land on that tool; otherwise the cursor shall land on the nearest enabled row scanned forward from the previous logical index, falling back to scan backward if no forward enabled row exists.
- **rationale**: silent footgun 回避。selectedToolId が cursor identity の単一情報源。
- **traces**: UAC-006, UAC-014, F-002

### FR-027 — projBase basename pure helper
- **type**: ubiquitous
- **statement**: The system shall derive projBase via a pure helper that (a) returns the input as-is if it is empty or ends with '/' or '\\' after trim, (b) handles both '/' and '\\' as separators, (c) appends ' (under <parent>)' to the basename only when the active-context resolution detects a collision with another project in the current snapshot, and shall apply the disambiguator only on the Active context header (not on push tool rows).
- **rationale**: 表示一貫性。
- **traces**: UAC-001, UAC-013

### FR-028 — sid8 計算と full sessionID title
- **type**: ubiquitous
- **statement**: The system shall compute sid8 as the first 8 characters of activeSessionID without uniqueness guarantee, and shall always carry the full sessionID as the title attribute of the rendering element.
- **rationale**: UAC-001 / UAC-013 / UAC-018 で full sessionID にアクセス可能にする。
- **traces**: UAC-001, UAC-013, UAC-018

### FR-029 — 英語 only + 色非依存 (a11y)
- **type**: ubiquitous
- **statement**: All new strings emitted by the palette UI (Active context row, push toast, inline status, status badge text) shall be English-only and shall not signal state by color alone (icon plus prefix character required).
- **rationale**: ADR-0049 / WCAG 1.4.1。
- **traces**: UAC-001, UAC-005, UAC-009, UAC-011, UAC-016

### FR-030 — Enter target = cursor 指す tool
- **type**: event_driven
- **statement**: When the user presses Enter while the cursor is on a disabled row, the system shall route the disabled-attempt handler (cursor identity is selectedToolId; hover does not override Enter target since hover only moves cursor on enabled rows per FR-006/FR-007).
- **rationale**: 否定役 #6 解消。Enter の判定対象は cursor が指す tool。
- **traces**: UAC-005, UAC-015

### FR-031 — inline-status replace + seq 増分 re-announce
- **type**: event_driven
- **statement**: When a new inline-status message is emitted while a previous one is still visible, the system shall replace the previous message, increment a sequence counter so that screen readers re-announce, and start a fresh 4-second auto-clear timer; if the same message text would be emitted twice in a row, the sequence counter shall still increment to force re-announce.
- **rationale**: 否定役 #9 解消。
- **traces**: UAC-005

### FR-032 — active flash の race / cancel
- **type**: event_driven
- **statement**: When the activeSessionID changes twice within the 600ms flash window, the system shall clear the previous flash timer in the useEffect cleanup, restart the 600ms timer for the new change, and announce the new change exactly once (cancelling the previous announce if not yet flushed).
- **rationale**: 否定役 #8 解消。
- **traces**: UAC-013

### FR-033 — 単一 aria-live slot へ announce 集約 (submit 中含む)
- **type**: state_driven
- **statement**: While submitting=true, the system shall route all aria-live announces through the same single aria-live='polite' announce slot used by the inline status and Active context header, so that screen readers experience a deterministic announce order.
- **rationale**: 最適化役 #12 解消。
- **traces**: UAC-017

## Non-Functional Requirements (NFR)

- **NFR-001** *(maintainability / size)*: palette store の composition root (palette.ts) は ≤500 行を維持する。本 PR の追加で超過する場合は ADR-0056 の slice 分割で吸収。各 React component は 500 行以内、各 function は 80 行以内 (project default)。
- **NFR-002** *(maintainability / deps)*: 新規 npm 依存を追加しない。既存 zustand / React / Vitest / Testing-Library のみで完結する。
- **NFR-003** *(usability / a11y)*: WCAG 1.4.1 (Use of Color) に従い、状態は色のみで signal しない (icon + prefix character 必須)。ARIA は combobox/listbox/option/separator/switch/role=status aria-live='polite' を使用する。
- **NFR-004** *(maintainability / i18n)*: src/client/web/ 配下の .ts/.tsx は no-japanese.test.ts (ADR-0049) を pass する。
- **NFR-005** *(compatibility)*: 既存 ADR (0036 store 純粋性 / 0040 composing guard / 0046 client-local activeSessionID / 0047 disabledReason single source / 0049 ASCII-only) の不変条件を維持する。
- **NFR-006** *(reliability / test env)*: Vitest + jsdom の制約上、Alt+W/H は KeyboardEvent dispatch (event.code='KeyW'/'KeyH', altKey=true) の unit test で担保し、cross-OS 実機検証は manual QA で plan.md Verification に明記する。
- **NFR-007** *(reliability / test retention)*: 既存 Vitest test は retain。scope (paletteScope / setScope / initialScope) 系 test は ScopeSegment 撤去に伴って sortToolsForList の test へ置換する移行を許容する (resolved_issues 否定役 #2 / 否定役 #17 と同根)。
- **NFR-008** *(performance)*: pointermove → setCursor の頻度は tens of rows 規模では throttle 不要を default invariant とする。profile で jank が観測された場合のみ後続 PR で requestAnimationFrame coalesce を導入する (Open Question)。
- **NFR-009** *(security)*: 新規外部 API 呼び出し / 新規 wire-format を導入しない。push toast の表示テキストは既存 SessionInfo / projects[] の値を XSS-safe な React text として render する。
- **NFR-010** *(maintainability / single source)*: scopeDisabledReason(tool, daemon) の戻り文字列は UI で加工せず逐語表示する (ADR-0047)。

## Assumptions

- SessionInfo.projectPath の wire 上の存在は実装 m1 着手時に src/client/web/lib/daemon.ts / wire/ の型定義で検証する。欠落していれば ADR-0058 の fallback (sid8 のみ表示) で本 PR を完結させ、wire 拡張は別 PR で起票する。
- pushCommands の運用件数は典型 5〜20 件。100 件超は将来 ADR 候補 (Open Questions)。
- standard 系 new-session tool は registry に常に登録されており、通常運用では enabled が常に 1 件以上ある。FR-024 の precondition は test/edge 状況 (standard tools 未登録 + sessionConfig=null) に限定される。
- Alt+W/H の cross-OS 実機検証は jsdom 範囲外。NFR-006 / plan.md Verification の manual QA 手順で担保する。
- pointermove の頻度は tens of rows 規模を想定。100 件超の listbox で jank が発生した場合は NFR-008 / Open Questions の rAF coalesce で別 PR 対応。
- frozenSnapshot は CommandPalette useRef で保持し、store には持たせない (ADR-0055 / ADR-0036 維持)。

## Acceptance (UAC ↔ FR ↔ test mapping)

各 UAC を spec の FR 群と Vitest test ファイルに対応付ける。test 種別: U=unit (pure helper), S=store slice test, C=component test, I=integration test (CommandPalette 結合), M=manual QA。

| UAC | 概要 | FR | test file | test case 名 (例) | 種別 |
|-----|------|-----|-----------|-------------------|------|
| UAC-001 | Active context header 3 ブランチ表示 | FR-009, FR-025, FR-027, FR-028, FR-029 | palette_active_context.test.ts / ActiveContextHeader.test.tsx | deriveActiveContext: resolves projBase from projects[] / fallback to ???/sid8 when projectPath missing / icon + No active session when activeSessionID=null | U+C |
| UAC-002 | Active context header 視覚的存在 (常時 1 行) | FR-009 | ActiveContextHeader.test.tsx | always renders single row regardless of state | C |
| UAC-003 | new-session 完走 → palette 閉 + focus 復元 | FR-001, FR-002, FR-006, FR-014 (success path), FR-021 | CommandPalette.test.tsx | full new-session flow closes overlay / focus returns to opener element (TerminalPane) / Enter from input submits when cursor is on enabled row | I |
| UAC-004 | 統合 1 listbox + enabled→separator→disabled | FR-001, FR-002, FR-003 | palette_helpers.test.ts / ToolSelectPhase.test.tsx | sortToolsForList: enabled before separator before disabled / listbox renders separator role=presentation | U+C |
| UAC-005 | disabled 行 Enter/click → inline status + flash + 単一 aria-live | FR-005, FR-029, FR-030, FR-031, FR-033 | InlineStatus.test.tsx / CommandPalette.test.tsx | disabled-row Enter routes through emitDisabledFeedback / aria-live container text replaces with seq increment / same message re-announces | C+I |
| UAC-006 | enabled 行 hover で cursor 追従 | FR-006, FR-026 | ToolSelectPhase.test.tsx / palette_helpers.test.ts | pointermove on enabled row updates cursor and aria-activedescendant / resolveCursorBySelectedToolId nearest enabled forward fallback | C+U |
| UAC-007 | disabled 行 hover で cursor 不変 | FR-007 | ToolSelectPhase.test.tsx | pointermove on disabled row does not move cursor | C |
| UAC-008 | mouseleave で cursor 維持 + single-cursor highlight | FR-006, FR-007, FR-008 | ToolSelectPhase.test.tsx | mouseleave preserves cursor / single cursor shared between hover and keyboard | C |
| UAC-009 | Worktree/Host chip role=switch + Alt+W/H | FR-016, FR-018, FR-029 | ChipSwitch.test.tsx / useChipHotkey.test.ts / ParamSelectPhase.test.tsx | renders role=switch with aria-checked / Alt+W (event.code=KeyW + altKey) toggles Worktree without losing input focus | C+U |
| UAC-010 | chip 4 経路 toggle + focus 保持 + Tab natural | FR-017, FR-019, FR-020, FR-021, FR-022, FR-023 | ChipSwitch.test.tsx / ParamSelectPhase.test.tsx | pointerdown preventDefault keeps focus / Space toggles when focused / Enter preventDefault + toggle / chip visibility lost → focus returns to input | C |
| UAC-011 | paramless push 成功時 info toast (projBase + sid8) | FR-014, FR-015, FR-029 | CommandPalette.test.tsx | submit success emits notify.add with message and title containing projBase·sid8 / fullPath\nfullSessionID | I |
| UAC-012 | push toast 非対話 + auto-dismiss | FR-015 | CommandPalette.test.tsx | toast contains no interactive elements / auto-dismiss policy unchanged | I |
| UAC-013 | Active context 変化時 flash + announce | FR-010, FR-027, FR-028, FR-032, FR-033 | ActiveContextHeader.test.tsx / CommandPalette.test.tsx | flash class toggles for 600ms on activeSessionID string change / announces "Active session changed to ..." once / disambiguator change does not flash | C+I |
| UAC-014 | push tool enabled/disabled 遷移 flash + 再 sort | FR-011, FR-026 | ToolSelectPhase.test.tsx / palette_helpers.test.ts | row transitions group with flash class / sort recomputes preserving selectedToolId identity | C+U |
| UAC-015 | disabled skip cursor (↑↓) + visible | FR-002, FR-003, FR-004, FR-030 | ToolSelectPhase.test.tsx | ArrowDown/Up skips disabled rows / disabled rows remain visible / Enter on disabled routes feedback | C |
| UAC-016 | 0 件異常状態の status badge | FR-024, FR-029 | CommandPalette.test.tsx | enabled count=0 + sessionConfig=null → "Loading commands…" / hydrated → "No commands available" / Enter is no-op | I |
| UAC-017 | submit 中 UI freeze (lift state) + 単一 aria-live | FR-012, FR-013, FR-033 | CommandPalette.test.tsx | submitting=true freezes header/list/badge from snapshot props / daemon mutation during submit does not affect frozen UI / submit resolve releases frozen snapshot | I |
| UAC-018 | submit 中 active 変更でも toast は frozen snapshot 参照 | FR-014, FR-028 | CommandPalette.test.tsx | toast text references snapshot at submit start, not latest view-update | I |

phase 別 test ファイル mapping:
- **phase A (pure helpers)** — palette_helpers.test.ts (sortToolsForList / resolveCursorBySelectedToolId) / palette_active_context.test.ts (deriveActiveContext + projBase pure helper)
- **phase B (store slices)** — palette store slice test (active_context flash / inline_status seq / freeze 補助 state)
- **phase C (presentation)** — ToolSelectPhase.test.tsx / ChipSwitch.test.tsx / ParamSelectPhase.test.tsx / ActiveContextHeader.test.tsx / InlineStatus.test.tsx
- **phase D (integration)** — CommandPalette.test.tsx (active flash / submit freeze / push toast / single aria-live / ctx=null Unavailable) / useChipHotkey.test.ts (Alt+W/H jsdom dispatch by event.code)
- **phase E (cleanup)** — ScopeSegment.test.tsx 削除 + palette.test.ts の scope 系 test を sortToolsForList の test へ置換

## Open Questions

> 実装後 follow-up または別 spec で扱う事項。本 PR の設計判断ではない。

- **[spec]** SessionInfo の現状 wire-format に projectPath field が含まれているかは本 plan 起票時点では未検証。実装フェーズ m1 着手時に src/client/web/lib/daemon.ts / wire/ の型定義で確認し、欠落していれば ADR-0058 の fallback (sid8 のみ) で本 PR を完結させ、wire 拡張は別 PR で起票する。
- **[spec]** pushCommands の運用件数は典型 5〜20 件を想定する。100 件超になった場合の listbox 体験 (仮想スクロール / 検索ファースト UI) は本 spec の対象外。将来 ADR 候補として残す。
