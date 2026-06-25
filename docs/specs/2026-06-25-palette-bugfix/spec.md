# Spec — Web UI コマンドパレット バグ修正 (commit 9287c7f 由来 4 件)

- **作成日**: 2026-06-25
- **ブランチ**: `main`
- **plan**: [plan.md](./plan.md)
- **ADRs**: [0048](../../adr/0048-paramselect-display-layer-materialize.md), [0049](../../adr/0049-palette-ui-english-only.md)
- **前提**: docs/specs/2026-06-24-web-ui-command-palette/ (元 spec)、main HEAD 9287c7f (修正対象実装)

## Goal

commit 9287c7f で導入された Web UI コマンドパレットの実害 (New Session 不全 / 撤去予定 stop-session の残存 / 日本語混在 / navigator.platform 依存) を 1 PR で解消する。具体的には (A) ParamDef を判別共用体化し、dynamic-options 系 param を ParamSelectPhase の純関数 materializeOptions(param, snapshot) で表示層 materialize して New Session を完走可能にする、(B) stop-session を ToolDef registry / palette UI から撤去し HTTP/server route は残置する、(C) パレット表面に露出する固定文字列を英語に統一し ASCII-only メタテストで回帰を防ぐ、(D) Mac 判定を lib/platform.ts:isMacPlatform() の純関数に集約し navigator.userAgentData → navigator.platform → navigator.userAgent → undefined-safe の順でフォールバックする、を観測可能な振る舞いとして実現する。IME 日本語対応と既存 ADR (0030/0033/0036/0044) を破らない。

## Scope

### In Scope

- src/client/web/src/lib/tools.ts: ParamDef を判別共用体化 (kind: 'text' | 'static-options' | 'dynamic-options', dynamic は materializeKey: 'projects' を持つ)。newSessionTool.params.project を kind:'dynamic-options' materializeKey:'projects' に変更し options 配列の静的宣言を廃止
- src/client/web/src/lib/tools.ts: newSessionTool.label と全 params.label を英語化 (New Session / Project / Command)。notify.success を 'Session created' に変更
- src/client/web/src/lib/tools.ts: stopSessionTool ToolDef 定義削除 / listTools から除外 / sessionOptions ヘルパ削除
- src/client/web/src/lib/tools.ts: scopeDisabledReason の戻り文字列を英語化 (No active session / No push-capable driver)
- src/client/web/src/lib/tools.ts: projectOptions(snapshot) を materialize の単一 source として維持 (本 PR の主使用者)
- src/client/web/src/lib/platform.ts (新規): isMacPlatform() 純関数を切り出し、navigator.userAgentData?.platform → navigator.platform → navigator.userAgent → false の順で評価。typeof navigator==='undefined' で false を返す SSR/test 安全
- src/client/web/src/lib/platform.test.ts (新規): 4 ブランチ (userAgentData macOS / platform 'MacIntel' / userAgent 'Macintosh' / undefined navigator) を jsdom mock で網羅
- src/client/web/src/hooks/useGlobalHotkey.ts: 既存 isMacPlatform() を削除し lib/platform.ts:isMacPlatform を import に切り替え (同名異実装ドリフトを解消)
- src/client/web/src/App.tsx: 旧 isMac() ローカル util を削除し lib/platform.ts:isMacPlatform を import。Header の '⌘K'/'Ctrl+K' 表示と aria-label='Open command palette (⌘K / Ctrl+K)' を実装
- src/client/web/src/App.tsx: Header の New Session ボタン onClick を openPalette({preselectToolId:'new-session', daemonSnapshot, opener}) に修正 (daemonSnapshot を必ず渡す)
- src/client/web/src/App.tsx: 'session-config の取得に失敗しました' toast 文言を英語化 ('Failed to load session config:')
- src/client/web/src/store/palette.ts: openPalette の preselectToolId 経路で scope filter を bypass し、明示 preselect は scope を 'standard' に正規化して resolve する (Header CTA の scope mismatch 失敗を排除)
- src/client/web/src/store/palette.ts: stop-session 撤去に伴う clearActiveIf 配線を rg で参照ゼロ確認後に PaletteActions / ToolStoreCtx から削除 (型シグネチャ変更は CommandPalette.tsx / ToolSelectPhase.tsx 側にも反映)
- src/client/web/src/store/palette.ts: submit の auth/http/unknown エラートースト文字列と confirmTool unknown id エラー文言を英語化
- src/client/web/src/store/palette_helpers.ts: classifySubmitError の auth/http/unknown ブランチ既定英語メッセージ整理 (HTTP server メッセージは透過)
- src/client/web/src/components/palette/ParamSelectPhase.tsx: listbox 描画分岐に materializeOptions(param, snapshot) を追加。param.kind==='dynamic-options' && materializeKey==='projects' のとき projectOptions(snapshot) を返す純関数
- src/client/web/src/components/palette/ParamSelectPhase.tsx: dynamic-options かつ options.length>=1 かつ paramValues[param.id]===undefined のときのみ初回 useEffect で setParam(param.id, options[0].value) を発火 (static-options/text には影響させない)。effect dep は materializeKey で memo した options ref を使い identity 不変化
- src/client/web/src/components/palette/ParamSelectPhase.tsx: dynamic-options で options.length===0 のとき ParamEmptyState を描画し、以降の params (command 等) のレンダリングと paramCursor 進行を抑制 (form 全体を empty 状態に切り替え、submit 到達不可)
- src/client/web/src/components/palette/ParamEmptyState.tsx (新規): 汎用 empty-state コンポーネント。props {message: string} を受け取り role=status で描画。Enter ハンドラ非搭載 (キーイベントを吸わない)
- src/client/web/src/components/palette/ToolSelectPhase.tsx: placeholder='コマンドを検索…' を 'Search commands…' に置換。store ctx 構築から clearActiveIf を削除 (型変更追従)。listbox の stop-session 関連表示の除去確認
- src/client/web/src/components/palette/CommandPalette.tsx: aria-label='戻る'/'閉じる' を 'Back'/'Close' に置換、不可用時のエラー文言 'command palette は利用できません (http クライアントが不正)' を 'Command palette unavailable (http client invalid)' に置換、title='Command Palette' は維持。REQUIRED_HTTP_METHODS から 'deleteSession' を削除 (本 PR で UI から呼ばないため)
- src/client/web/src/components/palette/CommandPalette.tsx: store ctx 構築から clearActiveIf を削除 (型変更追従)
- src/client/web/src/components/palette/ScopeSegment.tsx: 既存英語ラベル (standard/push) と aria-label='palette scope' を維持確認
- src/client/web/src/test/fixtures/daemon.ts (新規): mkSnapshot({projects: [...]}) helper を集約。projects=0/1/N+ ケースを共通生成し ParamSelectPhase.test.tsx / CommandPalette.test.tsx / tools.test.ts から利用
- src/client/web/src/__meta__/no-japanese.test.ts (新規): src/client/web/src/ 配下の .ts/.tsx を glob し /[぀-ヿ一-龯]/ をスキャンする ASCII-only assert。allowlist (テスト自身の検出文字列等) を 1 箇所に集約
- Vitest 更新: ParamSelectPhase.test.tsx (materialize / 先頭プリセット (waitFor) / empty-state / dynamic-options 限定 effect / preselectToolId 直接入口と toolSelect 経由の DOM 同一性 describe.each)、tools.test.ts (stop-session/sessionOptions 削除確認 + 英語ラベル + ParamDef 判別共用体形状)、palette.test.ts (clearActiveIf 削除 + 英語エラートースト + preselect scope bypass)、App.test.tsx (Header の '⌘K'/'Ctrl+K' 表示が lib/platform を呼ぶ)、CommandPalette.test.tsx (aria-label/title 英語 + REQUIRED_HTTP_METHODS 変更追従)、ToolSelectPhase.test.tsx (placeholder 英語 + stop-session 不在)、useGlobalHotkey.test.ts (lib/platform 経由動作確認)
- docs/specs/2026-06-25-web-ui-palette-bugfix/ 配下に spec.md と plan.md を新規作成 (本 PR の正本)
- docs/adr/ 配下に新規 ADR 2 本: paramselect-display-layer-materialize / palette-ui-english-only

### Out of Scope

- src/client/web/src/api/sessions.ts:deleteSession HTTP クライアントの削除 (別 UI 用に残置。残置根拠は plan.md に明記し sessions.test.ts の deleteSession テストも残す)
- src/server/web/mux.go DELETE /api/sessions/{id} route の削除 (同上)
- SessionList の × ボタン等の新規 UI (別 spec が必要)
- create-project / detach / shutdown など他ツールの追加
- push 機能語の語選 (push vs send vs broadcast)
- wire shape (src/client/web/src/wire/server.ts) の変更
- i18n フレームワーク導入 (react-intl 等)。英語統一は固定文字列で実施
- Playwright e2e (IME composing 実機検証)。本 PR では Vitest の composing フラグ手動 set 回帰のみで担保
- Biome カスタム lint rule の導入 (no-non-ascii-strings 相当)。本 PR は Vitest メタテストで担保し、将来 Biome 化は別 PR
- ToolDef.optionsFor(snapshot) への大規模リファクタ (ParamDef 判別共用体化に留め、ToolDef は静的宣言性を維持)
- 集約 review round 1/2 で carry over された minor TODO の同時対処
- 既存 docs/specs/2026-06-24-web-ui-fixes/ への merge (関連性は spec.md で言及するが merge は別 PR)
- push スコープ実装 (toolsForPush / push tools) の変更。英語化は scopeDisabledReason 戻り値のみ

## Requirements (EARS)

- **FR-A1** *(event_driven)* — When the user opens the palette via ToolSelectPhase and selects new-session with snapshot.projects containing N>=1 entries, the system shall render the project listbox with N role=option entries, set aria-selected=true on the first option, and point aria-activedescendant at the first option id.
  - *Rationale*: FR-A1 covers the standard path via ToolSelectPhase. Ensures dynamic materialize fires before keyboard navigation.
- **FR-A2** *(event_driven)* — When the user clicks the Header New Session button (preselectToolId='new-session'), the system shall enter paramSelect with the project listbox materialized from the same daemon snapshot path as FR-A1 and present the same DOM shape (role=option count and aria-activedescendant target).
  - *Rationale*: FR-A2 covers the preselect entry point. The two entries must converge on a single materialize code path (NFR-Determinism).
- **FR-A3** *(event_driven)* — When the user presses Down then Enter to select the second option, types 'echo hi' into command, and presses Enter on the final field, the system shall call createSession({project: <second option value>, command: 'echo hi'}) exactly once, then selectSession with the returned id, then notify.success('Session created'), then close the palette, then focus the opener element.
  - *Rationale*: FR-A3 is the end-to-end completion behaviour that commit 9287c7f broke.
- **FR-A4** *(unwanted)* — If snapshot.projects contains 0 entries when entering paramSelect for new-session, then the system shall not render the project listbox and shall render a role=status empty-state showing 'No projects available - add a project first', shall suppress subsequent params (command) from rendering, shall prevent submit from being reachable, and shall accept Esc to return to toolSelect discarding paramValues.
  - *Rationale*: Defines the empty-state form behaviour completely (rendering, focus, key handling, submit blocking).
- **FR-B1** *(ubiquitous)* — The system shall not present 'Stop Session', 'セッションを停止', or any ToolDef with id='stop-session' in the ToolSelectPhase listbox under standard scope.
  - *Rationale*: Removes the half-implemented stop-session UI entry.
- **FR-B2** *(ubiquitous)* — The system shall not export stopSessionTool or sessionOptions from src/client/web/src/lib/tools.ts (static grep yields zero hits).
  - *Rationale*: Garbage-collects orphan symbols once the UI entry is removed.
- **FR-B3** *(ubiquitous)* — The system shall preserve src/client/web/src/api/sessions.ts:deleteSession and the server-side DELETE /api/sessions/{id} route along with their existing tests so a future Session-List UI can reuse the HTTP path.
  - *Rationale*: Out-of-scope retention is part of the contract; explicit so dead-code lint or cleanup PRs do not regress it.
- **FR-C1** *(ubiquitous)* — The system shall present the ToolSelectPhase listbox label as 'New Session' and shall not present '新しいセッション' anywhere.
  - *Rationale*: Per-screen English-only assertion for the tool list.
- **FR-C2** *(ubiquitous)* — The system shall present paramSelect labels as 'Project' and 'Command' and shall not present 'プロジェクト' or 'コマンド' as labels.
  - *Rationale*: Per-screen English-only assertion for params.
- **FR-C3** *(event_driven)* — When new-session submit succeeds, the system shall include the text matching /Session created/i in the resulting toast.
  - *Rationale*: Success path notification text.
- **FR-C4** *(state_driven)* — While no active session exists, the system shall present the push scope tab disabled subtext as 'No active session'.
  - *Rationale*: scopeDisabledReason English fallout.
- **FR-C5** *(ubiquitous)* — The system shall render every palette toast (auth / http / unknown) and inline error in English.
  - *Rationale*: Covers store/palette and store/palette_helpers error surfaces.
- **FR-C6** *(ubiquitous)* — The system shall not contain hiragana, katakana, or CJK ideograph characters in any .ts or .tsx file under src/client/web/src/ outside the allowlist defined in src/client/web/src/__meta__/no-japanese.test.ts.
  - *Rationale*: Regression gate: ASCII-only meta-test prevents future Japanese strings from creeping into the web client.
- **FR-D1** *(state_driven)* — While navigator.userAgentData.platform returns a string containing 'macOS', the system shall display the Header command button label as '⌘K' and include 'Open command palette' in its aria-label.
  - *Rationale*: Primary detection branch via UA Client Hints.
- **FR-D2** *(unwanted)* — If navigator is undefined (SSR/test) or navigator.platform returns an empty string and no userAgentData/userAgent indicates Mac, then the system shall not crash and shall fall back to 'Ctrl+K' without false Mac detection.
  - *Rationale*: Crash safety and false-positive avoidance for the navigator fallback chain.
- **FR-D3** *(ubiquitous)* — The system shall use a single isMacPlatform() implementation from src/client/web/src/lib/platform.ts in both Header rendering (App.tsx) and global hotkey handling (hooks/useGlobalHotkey.ts).
  - *Rationale*: Prevents same-name different-impl drift between Header display and hotkey detection.
- **FR-IME** *(state_driven)* — While IME composition is in progress (composing=true), the system shall treat ParamSelectPhase Enter/ArrowUp/ArrowDown handlers and palette store setQuery/moveCursor/submit/confirmTool actions as no-op.
  - *Rationale*: Regression preservation: existing composing guards must remain after refactor.
- **FR-Det** *(ubiquitous)* — The system shall ensure openPalette(preselectToolId) and ToolSelectPhase-driven entry resolve through the same materializeOptions(param, snapshot) call path and produce identical DOM (role=option count and aria-activedescendant target) for the same snapshot.
  - *Rationale*: NFR-Determinism made testable: a describe.each test enforces single-path materialize.

## Open Questions

> 実装後 follow-up または別 spec で扱う事項。本 PR の設計判断ではない。

- notify level (success vs info) の UI マッピングが現状 success → info に縮退している。本 PR では FR-C3 の文言 assertion のみで担保するが、success 専用の見た目 (緑系 / ✓ icon) が必要かは別 task で再設計する。性質は plan の不確実性 (実装) のため plan.md の Open Questions に振り分け。
- push 機能語の語選 (push / send / broadcast) は本 PR スコープ外。既存ラベル 'push' を維持しつつ将来の UX 議論に委ねる。性質は spec の不確実性 (要件) のため spec.md の Open Questions に振り分け。
- 本 PR で停止する stop-session の代替 UI (SessionList × ボタン等) の設計は未着手。deleteSession HTTP クライアントと server route を残置する根拠 (FR-B3) は将来 UI の存在を前提としているため、別 spec で設計する必要がある。性質は spec の不確実性のため spec.md の Open Questions に振り分け。
- ASCII-only gate を Biome カスタムルール化するか Vitest メタテストのままにするか。本 PR は Vitest で実装するが、Biome 化が完了すればメタテストは削除可能。性質は plan の不確実性のため plan.md の Open Questions に振り分け。
- ParamSelectPhase.tsx が ParamEmptyState 抽出 + materialize ロジック追加で 500 行近辺になる見込み。実装中に超過した場合のさらなる抽出単位 (例: useDynamicParamPreset hook 化) は実装後の lint 結果次第。性質は plan の不確実性のため plan.md の Open Questions に振り分け。

## Resolved Issues (plan-how 統合役による収束)

- **Issue**: blocker: Header New Session ボタンは daemonSnapshot を渡さず initialScope='standard' で resolve するため現状動くが、将来 daemonSnapshot を渡すと active+occupant='frame' で scope='push' になり new-session preselect が console.warn → fallback して FR-A2 が破綻する。
  - **Resolution**: scope の in_scope に『Header の New Session ボタン onClick で daemonSnapshot を渡す』と『openPalette の preselectToolId 経路で scope filter を bypass し scope='standard' に正規化』を追加。ADR-paramselect-display-layer-materialize の Decision (4) に明文化。
- **Issue**: major: 先頭プリセット useEffect の所在 (ParamListbox / ParamSelectPhase) と dep 戦略が文書内で食い違い、無限ループ or 余分な setParam ループの危険。
  - **Resolution**: ParamSelectPhase 側に effect を置き、dynamic-options かつ value===undefined のときのみ発火、effect dep は materializeKey で memo した options ref で identity 不変化、と Decision (5) に明示。最適化案 6 採用。
- **Issue**: major: FR-A1『先頭プリセット選択済みで開く』は同期的に成立せず、useEffect 後の 2nd render で 0 が当たる。Testing-Library で findBy / waitFor が必要。
  - **Resolution**: ADR Consequences に『FR-A1 は最終的な観測として定義する』と明記。テストは waitFor / findBy を使う方針を Vitest 更新 component の責務に記載。
- **Issue**: major: NFR-Determinism と store/palette.ts:openPalette が listTools(snap, snap.pushCommands) を呼ぶ resolve 経路の食い違い (2 つの snapshot 源)。
  - **Resolution**: ADR Decision (4) で『openPalette の preselect resolve は scope filter を bypass し ToolDef 存在性のみを判定する責務に限定する』と分離。FR-Det を新設し describe.each で両入口の DOM 同一性を機械的に保証。最適化案 9 採用。
- **Issue**: blocker: in_scope に含まれない箇所の日本語 (ToolSelectPhase placeholder / CommandPalette aria-label/title / App.tsx toast 等) が ASCII gate で必ず引っかかり CI が塞がる。
  - **Resolution**: in_scope を拡張し ToolSelectPhase.tsx (placeholder)、CommandPalette.tsx (aria-label/title/error)、App.tsx (toast) を本 PR の対象に追加。最適化案 5 で allowlist 構造を __meta__/no-japanese.test.ts に集約し False Positive を構造的に防ぐ。
- **Issue**: major: clearActiveIf 削除の型変更波及先 (CommandPalette.tsx / ToolSelectPhase.tsx) が in_scope に欠落。
  - **Resolution**: in_scope の components 表に CommandPalette.tsx と ToolSelectPhase.tsx を追加 (store ctx 構築から clearActiveIf 削除)。最適化案 4 の機械化チェック (Pre-Impl rg で参照ゼロ確認) も plan の Verification に組み込む。
- **Issue**: major: CommandPalette.REQUIRED_HTTP_METHODS に deleteSession が hard-coded されており、本 PR で UI から使わないため整合が崩れる。
  - **Resolution**: CommandPalette.tsx の責務に『REQUIRED_HTTP_METHODS から deleteSession を削除』を明記。deleteSession HTTP クライアントと server route は FR-B3 で残置確認。
- **Issue**: major: ProjectEmptyState 描画後の form 全体の挙動 (command フィールド表示・focus・submit 到達) が未定義。
  - **Resolution**: FR-A4 を『以降の params (command) のレンダリングと paramCursor 進行を抑制し submit 到達不可』『Esc で paramValues 破棄して toolSelect 戻り』として完全定義。ParamSelectPhase の責務にも明記。
- **Issue**: major: data_flows の openPalette({daemonSnapshot}) と Header の実装変更点 (in_scope) が食い違い、worktree 別 PR で抜け落ちる。
  - **Resolution**: in_scope と App.tsx component 責務に Header onClick の daemonSnapshot 追加を明記。FR-A2 と FR-Det で両入口の DOM 同一性を assert。
- **Issue**: major: useGlobalHotkey.ts に同名 isMacPlatform() が既存し、片方だけ修正すると Header 表示と hotkey 判定で drift する。
  - **Resolution**: FR-D3 を新設 (単一実装 from lib/platform.ts) し、in_scope に useGlobalHotkey.ts の lib/platform import 切り替えを追加。最適化案 3 採用。
- **Issue**: minor: ADR 4 本案 (materialize / stop-session 撤去 / 英語統一 / navigator fallback) は薄い ADR が混ざる。
  - **Resolution**: 最適化案 8 採用。ADR を 2 本 (materialize / english-only) に絞り、stop-session 撤去と navigator fallback は plan.md 本文と実装コメントで扱う (方針反転コストが低いため)。
- **Issue**: minor: empty-state 後続フィールド抑制を ParamSelectPhase 内に置くと NFR-Limit (500 行) リスク。
  - **Resolution**: 最適化案 2 採用。ParamEmptyState を独立ファイル (components/palette/ParamEmptyState.tsx) に切り出し、project 専用名にせず汎用化。
- **Issue**: minor: lib/tools.test.ts の削除粒度 (sessionOptions describe / stop-session.submit describe / listTools deterministic order) が未確定。
  - **Resolution**: Vitest 更新 component 責務に削除対象テスト群を列挙 (stop-session/sessionOptions 削除確認 + 英語ラベル + ParamDef 判別共用体形状)。
- **Issue**: minor: deleteSession 残置の根拠 (dead code 警告 / 残置テスト) が記述されていない。
  - **Resolution**: FR-B3 を新設し sessions.test.ts の deleteSession テストを残す責務を Vitest 更新 component に明記。残置の理由 (将来 SessionList × UI 用) は plan.md の Targets 注記。
- **Issue**: minor: store/palette.ts:openPalette が listTools を呼ぶ resolve 経路の ADR-0036 境界の説明が薄い。
  - **Resolution**: ADR Decision (4) で『preselect resolve は materialize ではなく ToolDef 存在性のみを判定する責務に限定する』と境界を明示。store は依然 DOM/HTTP 非保有を保持。
- **Issue**: minor: 『composing 中は setParam が store ガードで no-op』は事実誤認 (setParam に IME ガードなし)。
  - **Resolution**: store/palette.ts component 責務から該当記述を削除し、FR-IME は既存の setQuery/moveCursor/submit/confirmTool composing ガード維持確認に限定。setParam への IME ガード追加は本 PR スコープ外と明示。
- **Issue**: minor: docs/specs/2026-06-24-web-ui-palette-bugfix/ の日付が本 PR 開始日と合うか不明、既存 web-ui-fixes との関係が未整理。
  - **Resolution**: ディレクトリを 2026-06-25-web-ui-palette-bugfix/ に変更 (本 PR 開始日)。out_of_scope に『既存 web-ui-fixes への merge は別 PR』と明記。最適化案 10 採用。
- **Issue**: minor: notify.success が level=info で表示される success UI の見た目に対する不整合。
  - **Resolution**: 本 PR では FR-C3 の文言 assertion のみを担保 (success UI 見た目は別 task)。open_questions に『notify level の success/info マッピング再設計』を残置。
- **Issue**: improvement: テスト fixture 重複。
  - **Resolution**: 最適化案 7 採用。test/fixtures/daemon.ts に mkSnapshot helper を集約し新規 component として in_scope に追加。
- **Issue**: improvement: 実装順序の依存グラフ明示。
  - **Resolution**: 最適化案 11 採用。chunks を 7 段 (m1-m7) に分解し、(m1, m2) を並列起点、(m3) → (m4) → (m5) → (m6) → (m7) の依存順を明示。
