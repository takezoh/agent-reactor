# Plan — Web UI コマンドパレット 再設計

- **作成日**: 2026-06-25
- **spec**: [spec.md](./spec.md)
- **ux**: [ux.md](./ux.md)
- **ADRs (新規)**: [0054](../../adr/0054-palette-cursor-identity-by-tool-id.md), [0055](../../adr/0055-palette-submit-freeze-via-lift-state.md), [0056](../../adr/0056-palette-store-slice-composition.md), [0057](../../adr/0057-palette-single-aria-live-slot.md), [0058](../../adr/0058-palette-active-context-data-path.md)
- **ADRs (ux で生成済み)**: [0050](../../adr/0050-palette-scope-unify-with-disabled-policy.md), [0051](../../adr/0051-palette-hover-follow-single-cursor.md), [0052](../../adr/0052-palette-active-context-header-with-change-feedback.md), [0053](../../adr/0053-palette-chip-toggle-keybinding-redesign.md)

## Goal

accepted な ux.md (UAC-001〜UAC-018, F-001〜F-008) と ADR-0050〜0053 を入力に、src/client/web/ 配下の command palette を「scope 統合 + disabled visible + hover follow + active context header + chip 3 経路 toggle + push 送信先明示 toast + submit 中凍結」へ再設計する。本 plan は (1) zustand StateCreator slice 分割 (active_context / inline_status / freeze) で 500 行制約を吸収しつつ、(2) selectedToolId を cursor identity の単一情報源化、(3) submit freeze を CommandPalette の lift-state + frozen snapshot props で実装、(4) 単一 aria-live slot に announce 集約、を 1 PR の m1〜m5 milestones で実装する手順を確定する。

## Components

### `src/client/web/store/palette.ts` (modified, ≤500 lines)
composition root。既存 phase / transition と新 slice (active_context / inline_status / freeze) を StateCreator パターンで合成する。scope / setScope / initialScope (PaletteScope 型) を撤去 (ADR-0050)。新規 action: setCursor(index), emitDisabledFeedback, setActiveContextSnapshot, freezeForSubmit, unfreezeForSubmit。confirmTool / submit に disabled gate を defensive fail-safe として追加 (presentation 層との二重防御)。store には DOM / HTTP を持たない (ADR-0036 維持) ため、frozenSnapshot 本体は CommandPalette useRef で保持する。

### `src/client/web/store/palette_active_context.ts` (NEW slice)
StateCreator slice。ActiveContextSnapshot 型 + flashSeq state + announceSeq state を保持する。deriveActiveContext(daemon, projects) と projBase basename pure helper (FR-027) をエクスポートする。projBase の disambiguator (`(under <parent>)`) は active header のみで適用する。

### `src/client/web/store/palette_inline_status.ts` (NEW slice)
StateCreator slice。`{message, seq, kind, timerId?}` state + emitDisabledFeedback(toolLabel, reason) action + 4s auto-clear 管理。timerId は setTimeout の id (ADR-0036 は document/window/HTMLElement のみ禁止であり setTimeout は store 内利用を許容)。同一 message でも seq を増分して再 announce する (FR-031)。

### `src/client/web/store/palette_freeze.ts` (NEW slice)
StateCreator slice。submitting flag は既存維持。frozenSnapshot は CommandPalette useRef で保持するため store には持たせない (ADR-0055 Decision)。slice は freeze 関連の副次 state (例: submitStartedAt) のみ保持する。将来 retry count 等が増えた際の追加余地を確保する。

### `src/client/web/store/palette_helpers.ts` (modified)
initialScope を削除する。新規 pure helper を追加する: `sortToolsForList(tools, daemon, fuzzyRanked)` → `{enabled: ToolDef[], disabled: Array<{tool, reason}>, sorted: ToolDef[] (separator なし論理リスト)}`、`resolveCursorBySelectedToolId(prevSelectedId, prevLogicalIndex, sortedList, enabledOnlyIndexSet)` → 新 cursor logical index (FR-026)。projBase pure helper は palette_active_context.ts に移管する。

### `src/client/web/lib/tools.ts` (minor)
listTools / scopeDisabledReason の signature は不変。makePushToolDef.submit 内で submit 開始時点の active context snapshot を引数 (ctx 拡張) で受け取り、`notify.add({level: 'info', title: '<fullPath>\n<fullSessionId>', message: "Sent '<cmd>' → <projBase> · <sid8>"})` を呼ぶ (FR-014)。fuzzyRank は disabled tools を除外しない (UAC-004 invariant)。

### `src/client/web/store/notifications.ts` (no API change)
既存の add({level, message, title, body}) を利用する。新規 API 追加なし (resolved_issues 否定役 #10 / 最適化役 #2 の決定通り)。

### `src/client/web/components/palette/ScopeSegment.tsx + ScopeSegment.test.tsx` (DELETE)
ADR-0050 で撤去する。CommandPalette からの import / 参照を削除する。

### `src/client/web/components/palette/ToolSelectPhase.tsx` (modified)
scope filter を削除し、全 tool を sortToolsForList で 1 list 化する。内部 helper const ToolRow を inline 定義する (新規 .tsx を増やさない / 最適化役 #1)。selectedToolId 起点で cursor を再計算する (FR-026)。enabled rows → role=presentation separator → disabled rows を 1 つの role=listbox 内で render する。aria-activedescendant は sortedList の論理 index ベース (separator 除外) で採番する。pointermove on enabled で setCursor、disabled では cursor 不変。mouseleave で cursor 不変。frozen snapshot props を受けたときは props を最優先で render する (lift-state)。

### `src/client/web/components/palette/ActiveContextHeader.tsx` (NEW)
role='status' を持つが aria-live は持たない (announce は単一 slot 経由 / ADR-0057)。flash class は activeSessionID 文字列変化を trigger に useEffect + setTimeout 600ms で toggle し、cleanup で clearTimeout (FR-032)。No active session 時の icon + 文字列 fallback。ctx 制御は CommandPalette が条件 render する (FR-025)。

### `src/client/web/components/palette/InlineStatus.tsx` (NEW)
単一の aria-live='polite' announce slot を提供する (ActiveContextHeader / 送信中 status もここを経由 / ADR-0057)。store.inlineStatus.seq を購読して replace + 4s auto-clear + 連続発火時の re-announce (FR-031)。

### `src/client/web/components/palette/StatusBadge (inline in CommandPalette.tsx)`
独立 .tsx は作らず CommandPalette 内 conditional render で 'Loading commands…' / 'No commands available' / 'Sending…' (spinner) / 'Unavailable' を排他表示する (最適化役 #8)。既存 palette-progress 'sending…' は StatusBadge の 'Sending…' に統合し DOM 二重描画を回避する。

### `src/client/web/components/palette/ChipSwitch.tsx` (NEW)
role='switch' aria-checked 1 chip。pointerdown preventDefault で input focus 保持 (FR-017)。Space / Enter で toggle (Enter は preventDefault で form submit 阻止 / FR-019, FR-020)。'[W]' / '[H]' icon を hint として表示する (FR-016)。

### `src/client/web/components/palette/ParamSelectPhase.tsx` (modified)
command field の chip 表示を ChipSwitch ベースに差し替える。useChipHotkey を本 phase でのみ mount する。chip visibility 動的変化での focus fallback (FR-022) を実装する。focus 判定は ref.current === document.activeElement で行う。

### `src/client/web/components/palette/ParamTextInput.tsx` (modified)
Tab / Shift+Tab hijack を削除する。'Tab to toggle' / 'Shift+Tab to toggle' hint text を削除する (ADR-0053 / 否定役 #21)。

### `src/client/web/components/palette/CommandPalette.tsx` (modified)
ScopeSegment 撤去。ActiveContextHeader / StatusBadge (inline) / InlineStatus を組み込む。submitting transition の useEffect で frozenSnapshotRef を capture / release し、ToolSelectPhase / ActiveContextHeader / StatusBadge に frozen snapshot props を渡す (freeze は lift-state / ADR-0055)。ctx 構築失敗時の Active 行非表示 + status badge Unavailable を CommandPalette 側で制御 (FR-025)。既存 palette-progress を StatusBadge inline に統合する。

### `src/client/web/hooks/useChipHotkey.ts` (NEW)
document capture-phase keydown listener。判定順序: (1) palette 条件判定 (open + phase=paramSelect + command 入力 visible + chip visible) → (2) composing 判定 → (3) event.code === 'KeyW' / 'KeyH' + altKey 判定 → (4) preventDefault → (5) store.toggleWorktree / toggleHost。useGlobalHotkey とは別 hook で責務分離する。

### `src/client/web/hooks/useGlobalHotkey.ts` (no change)
prefix+p / prefix+C-p の挙動は不変。

### Vitest test 追加 / 移植 (詳細は spec.md Acceptance table)
- **phase A test**: palette_helpers.sortToolsForList / resolveCursorBySelectedToolId / palette_active_context.deriveActiveContext + projBase pure helper
- **phase B test**: palette store slice (active_context flash / inline_status seq / freeze)
- **phase C test**: ToolSelectPhase.test.tsx (group sort / disabled visible / cursor skip / hover follow / aria-activedescendant) / ChipSwitch.test.tsx / ParamSelectPhase.test.tsx (chip 4 経路 + focus 保持) / ActiveContextHeader.test.tsx / InlineStatus.test.tsx
- **phase D test**: CommandPalette.test.tsx (active flash / submit freeze / push toast / single aria-live / ctx=null Unavailable) / useChipHotkey.test.ts (Alt+W/H jsdom dispatch by event.code)
- **phase E test**: ScopeSegment.test.tsx 削除 + palette.test.ts の scope 系 test を sortToolsForList の test へ置換移行

## Implementation Sequence

依存方向: `m1-pure-helpers → m2-store-slices → m3-presentation-listbox → m4-chips-and-freeze → m5-scope-removal-and-tests`

### m1-pure-helpers
- **依存**: なし (起点)
- **Targets**:
  - component: `src/client/web/store/palette_helpers.ts` (modified)
  - component: `src/client/web/store/palette_active_context.ts` (NEW slice の pure helper 部分のみ先行)
  - req: FR-001 (sort 構造の論理 list)
  - req: FR-002 (enabled/separator/disabled の論理順序)
  - req: FR-026 (selectedToolId anchor の cursor 再計算 pure helper)
  - req: FR-027 (projBase basename pure helper)
  - req: FR-028 (sid8 計算)
  - adr: ADR-0054 (cursor identity by tool id)
  - adr: ADR-0058 (active context data path)
- **Verification**:
  - Vitest: `palette_helpers.test.ts` (sortToolsForList の enabled/disabled 分離 + 論理 index 順序 / resolveCursorBySelectedToolId の前方優先 + backward fallback)
  - Vitest: `palette_active_context.test.ts` (deriveActiveContext の 3 ブランチ / projBase の `/` `\` 終端 + Windows path / disambiguator collision)
  - `no-japanese.test.ts` pass
  - `pnpm tsc --noEmit` clean

### m2-store-slices
- **依存**: m1-pure-helpers
- **Targets**:
  - component: `src/client/web/store/palette.ts` (modified, ≤500 lines)
  - component: `src/client/web/store/palette_active_context.ts` (slice の state + action)
  - component: `src/client/web/store/palette_inline_status.ts` (NEW slice)
  - component: `src/client/web/store/palette_freeze.ts` (NEW slice)
  - req: FR-005 (emitDisabledFeedback action)
  - req: FR-023 (composing guard を store action 経路にも適用)
  - req: FR-031 (inline status seq 増分 + 4s auto-clear)
  - req: FR-032 (active flash race / cancel の slice 側 state)
  - adr: ADR-0056 (store slice composition)
- **Verification**:
  - Vitest: palette store slice test (emitDisabledFeedback で message + seq 増分 / 同一 message でも seq 増分 / 4s timer の cleanup / setActiveContextSnapshot で flashSeq + announceSeq 更新 / freezeForSubmit + unfreezeForSubmit の submitStartedAt)
  - palette.ts 行数 ≤500 を `wc -l` で確認
  - `no-japanese.test.ts` pass

### m3-presentation-listbox
- **依存**: m2-store-slices
- **Targets**:
  - component: `src/client/web/components/palette/ToolSelectPhase.tsx` (modified)
  - component: `src/client/web/components/palette/ActiveContextHeader.tsx` (NEW)
  - component: `src/client/web/components/palette/InlineStatus.tsx` (NEW)
  - req: FR-003 (disabled 行 inline 表示)
  - req: FR-004 (disabled skip cursor)
  - req: FR-006 (enabled hover で cursor 追従)
  - req: FR-007 (disabled hover で cursor 不変)
  - req: FR-008 (mouseleave で cursor 維持)
  - req: FR-009 (Active context header 3 ブランチ)
  - req: FR-010 (active 変化時 flash + announce)
  - req: FR-011 (push tool enabled/disabled 遷移 flash)
  - req: FR-015 (push toast 非対話) — render 側の構造担保のみ
  - req: FR-024 (status badge 0 件状態) — InlineStatus を経由する announce 経路の準備
  - req: FR-029 (英語 only + 色非依存)
  - req: FR-030 (Enter target は cursor 指す tool)
  - req: FR-033 (announce 集約 slot)
  - adr: ADR-0057 (single aria-live slot)
- **Verification**:
  - Vitest: `ToolSelectPhase.test.tsx` (separator role=presentation / aria-activedescendant 論理 index / ArrowDown/Up disabled skip / pointermove on enabled → setCursor / pointermove on disabled → cursor 不変 / mouseleave → cursor 維持)
  - Vitest: `ActiveContextHeader.test.tsx` (3 ブランチ表示 / activeSessionID 文字列変化で flash class toggle / disambiguator 変化単独では flash しない / setTimeout 600ms cleanup)
  - Vitest: `InlineStatus.test.tsx` (seq 変化で text replace + re-announce / 4s で clear)
  - `no-japanese.test.ts` pass

### m4-chips-and-freeze
- **依存**: m3-presentation-listbox
- **Targets**:
  - component: `src/client/web/components/palette/ChipSwitch.tsx` (NEW)
  - component: `src/client/web/components/palette/ParamSelectPhase.tsx` (modified)
  - component: `src/client/web/components/palette/ParamTextInput.tsx` (modified)
  - component: `src/client/web/hooks/useChipHotkey.ts` (NEW)
  - component: `src/client/web/components/palette/CommandPalette.tsx` (modified)
  - component: `src/client/web/lib/tools.ts` (minor)
  - req: FR-012 (submit 中 UI freeze)
  - req: FR-013 (submit 解決時の palette 後処理)
  - req: FR-014 (paramless push 成功時 info toast)
  - req: FR-016 (chip role=switch)
  - req: FR-017 (chip pointerdown toggle + focus 保持)
  - req: FR-018 (Alt+W/H ホットキー)
  - req: FR-019 (Space toggle)
  - req: FR-020 (Enter toggle + preventDefault)
  - req: FR-021 (Tab natural + hijack 撤去)
  - req: FR-022 (chip 消失時 focus fallback)
  - req: FR-025 (ctx=null の責務境界)
  - adr: ADR-0055 (submit freeze via lift state)
- **Verification**:
  - Vitest: `ChipSwitch.test.tsx` (role=switch + aria-checked / pointerdown preventDefault / Space toggle / Enter preventDefault + toggle)
  - Vitest: `ParamSelectPhase.test.tsx` (4 経路 toggle / chip visibility 消失 → focus が command input に戻る)
  - Vitest: `useChipHotkey.test.ts` (Alt+W on event.code=KeyW + altKey toggles Worktree / composing=true で no-op / phase!=paramSelect で no-op)
  - Vitest: `CommandPalette.test.tsx` (submitting=true で frozen snapshot props が ToolSelectPhase / ActiveContextHeader / StatusBadge に渡る / daemon mutation during submit does not change DOM / submit 解決後 frozen 解除 / push toast が frozen snapshot の projBase·sid8 / fullPath\nfullSessionId を含む / ctx=null で Active 行非表示 + 'Unavailable')
  - **Manual QA** (NFR-006): macOS Safari + Chrome + Firefox / Windows Chrome / Linux Firefox で Alt+W / Alt+H を実機 dispatch し、Worktree / Host chip の toggle と command input の focus 保持を確認する。Option+W = ∑ の OS 依存差異が起きないこと (event.code 判定によって)。

### m5-scope-removal-and-tests
- **依存**: m4-chips-and-freeze
- **Targets**:
  - component: `src/client/web/components/palette/ScopeSegment.tsx + ScopeSegment.test.tsx` (DELETE)
  - component: Vitest test 群の最終置換 (palette.test.ts の scope 系 test → sortToolsForList の test へ移行 / no-japanese.test.ts の allowlist 更新 / Acceptance table 完成)
- **Verification**:
  - `rg "ScopeSegment|paletteScope|setScope|initialScope|PaletteScope"` で参照 0
  - Vitest: `pnpm vitest run` 全 pass
  - `pnpm biome check` / `pnpm tsc --noEmit` clean
  - `no-japanese.test.ts` pass
  - 静的 lint: `cd src && go vet ./... && make lint` (Go 側不変)
  - 手動: `make build && ./arc` で実機起動、Cmd/Ctrl+K → tool 選択 → push 成功で toast 確認 → active 切替で flash 確認

## Test Strategy

- **Tier scheme** (docs/agent/testing.md 参照): 本 PR の対象は client/web/ なので web client Tier。pure helper は coverage 100% を目標、store slice は coverage ≥ 90%、React component は user-visible 振る舞いに対する behavioral test を中心とする。
- **jsdom + happy-dom 使い分け**: 本 PR では既存通り jsdom を維持する。Alt+W/H の KeyboardEvent dispatch は event.code + altKey の組合せで unit test 可能であり、happy-dom への切替は不要。
- **Acceptance table の test 紐付け**: spec.md の UAC ↔ FR ↔ test mapping table を実装時に追加し、test ファイル先頭コメントで `// UAC-XXX / FR-YYY` を明記する (review 時の trace 容易化)。
- **既存 test の retention**: ScopeSegment.test.tsx は m5 で削除、palette.test.ts の scope 系は sortToolsForList の test に置換 (NFR-007)。それ以外の既存 test は全 retain。
- **Manual QA**: Alt+W/H の cross-OS は m4 末尾で manual QA。手順は plan.md Verification に明記し、結果は PR description に貼る。

## Out of Scope (plan 視点)

- 集約 review round 1/2 で carry over された minor TODO の同時対処 (本 PR は m1〜m5 に集中)
- 既存 docs/specs/2026-06-24-web-ui-fixes/ への merge (関連性は spec.md で言及するが merge は別 PR)
- 仮想スクロール / 検索ファースト UI / dynamic sort (pushCommands 100 件超への備え、将来 ADR)
- SessionInfo.projectPath wire 拡張 (ADR-0058 fallback で本 PR を完結、別 PR で起票)

## Open Questions (実装後 follow-up)

- **[plan]** Alt+W / Alt+H の cross-OS 実機検証 (macOS Option+W = ∑ などの mnemonic 競合) は jsdom test 範囲外。event.code='KeyW'/'KeyH' 採用により physical key 判定で OS 依存差異は最小化されるが、リリース前の manual QA で macOS Safari / Chrome / Firefox + Windows Chrome + Linux Firefox を確認する手順を plan.md Verification に明記する。
- **[plan]** hover follow の pointermove 頻度と setCursor 連発による zustand subscription thrashing。NFR-008 として「tens of rows 規模では throttle 不要」を default invariant とし、profile で jank が観測された場合のみ後続 PR で requestAnimationFrame coalesce を導入する。
