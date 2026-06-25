# Plan — Web UI コマンドパレット バグ修正

- **spec**: [spec.md](./spec.md)
- **ADRs**: [0048](../../adr/0048-paramselect-display-layer-materialize.md), [0049](../../adr/0049-palette-ui-english-only.md)

## Goal

commit 9287c7f で導入された Web UI コマンドパレットの実害 (New Session 不全 / 撤去予定 stop-session の残存 / 日本語混在 / navigator.platform 依存) を 1 PR で解消する。具体的には (A) ParamDef を判別共用体化し、dynamic-options 系 param を ParamSelectPhase の純関数 materializeOptions(param, snapshot) で表示層 materialize して New Session を完走可能にする、(B) stop-session を ToolDef registry / palette UI から撤去し HTTP/server route は残置する、(C) パレット表面に露出する固定文字列を英語に統一し ASCII-only メタテストで回帰を防ぐ、(D) Mac 判定を lib/platform.ts:isMacPlatform() の純関数に集約し navigator.userAgentData → navigator.platform → navigator.userAgent → undefined-safe の順でフォールバックする、を観測可能な振る舞いとして実現する。IME 日本語対応と既存 ADR (0030/0033/0036/0044) を破らない。

## Components

| Component | Responsibility | Depends on |
|-----------|----------------|------------|
| ParamSelectPhase (React) | listbox 描画分岐で param.kind === 'dynamic-options' のとき純関数 materializeOptions(param, snapshot) を呼び materializeKey で dispatch (現状 'projects' のみ)。options.length>=1 で先頭プリセット useEffect 発火 (dynamic-options 限定、value===undefined のときのみ)。options.length===0 で ParamEmptyState を描画し以降の params をレンダリングしないため submit 不到達。Enter / ↑↓ / IME ガードは既存維持。 | `lib/tools.ts:projectOptions`, `lib/tools.ts:ParamDef`, `components/palette/ParamEmptyState`, `store/palette.ts`, `store/daemon.ts:selectDaemonSnapshot` |
| ParamEmptyState (React, 独立ファイル) | 汎用 empty-state コンポーネント。props {message: string} を受け取り role=status で描画。Enter ハンドラ非搭載でキーイベントを吸わない。ParamSelectPhase.tsx の 500 行上限を超えないよう独立ファイル化 (components/palette/ParamEmptyState.tsx)。 | — |
| lib/tools.ts (ToolDef registry) | ParamDef を判別共用体化 (text / static-options / dynamic-options)。newSessionTool.params.project を dynamic-options + materializeKey:'projects' に変更し options 静的宣言を廃止。newSessionTool.label / 全 params.label を英語化 (New Session / Project / Command)、notify.success を 'Session created' に変更。stopSessionTool 定義と sessionOptions エクスポートを削除、listTools() から stop-session を除外。scopeDisabledReason の戻り文字列を英語化。projectOptions(snapshot) は維持 (materializeKey 'projects' の唯一の実装)。 | `api/sessions.ts:SessionsApi`, `wire/server.ts:SessionInfo` |
| lib/platform.ts (新規) | isMacPlatform(): boolean を純関数として export。typeof navigator==='undefined' → false。navigator.userAgentData?.platform に 'macOS' を含む → true。navigator.platform.toUpperCase() に 'MAC' を含む → true。navigator.userAgent.toUpperCase() に 'MAC' を含む → true。いずれも未確定なら false。declare global で UADataValues 型を補強し any キャスト回避。 | — |
| App.tsx (Header) | lib/platform.ts:isMacPlatform を import し Header の '⌘K'/'Ctrl+K' 表示と aria-label='Open command palette (⌘K / Ctrl+K)' を実装。New Session ボタン onClick で openPalette({preselectToolId:'new-session', daemonSnapshot, opener}) を呼ぶ (daemonSnapshot を必ず渡す)。'session-config の取得に失敗しました:' toast 文言を 'Failed to load session config:' に置換。 | `lib/platform.ts:isMacPlatform`, `store/palette.ts:openPalette`, `store/daemon.ts:selectDaemonSnapshot` |
| hooks/useGlobalHotkey.ts | 既存 isMacPlatform() ローカル実装を削除し lib/platform.ts:isMacPlatform を import に切り替え。同名異実装ドリフトを解消 (FR-D3)。キー処理ロジックは不変。 | `lib/platform.ts:isMacPlatform` |
| store/palette.ts | openPalette の preselectToolId 経路で scope filter を bypass し、明示 preselect は scope='standard' に正規化して resolve する (Header CTA の scope mismatch 失敗を排除)。stop-session 撤去に伴い clearActiveIf を rg で参照ゼロ確認後に PaletteActions / ToolStoreCtx から削除。submit の auth/http/unknown エラートースト文字列と confirmTool unknown id エラー文言を英語化。state 形状とそれ以外のアクション意味論は不変。setParam の IME ガード追加は本 PR スコープ外 (FR-IME は既存の setQuery/moveCursor/submit/confirmTool composing ガードを維持確認)。 | `lib/tools.ts:listTools`, `store/palette_helpers.ts`, `store/daemon.ts` |
| store/palette_helpers.ts | classifySubmitError の auth/http/unknown ブランチ既定英語メッセージを整理 (HTTP server メッセージは透過、auth は固定英語 'Authentication required')。initialScope / findToolForSubmit / isParamless は不変。 | — |
| components/palette/ToolSelectPhase.tsx | placeholder='コマンドを検索…' を 'Search commands…' に置換。store ctx 構築から clearActiveIf を削除 (型変更追従)。stop-session 関連表示が listbox から消えていることを確認。 | `store/palette.ts`, `lib/tools.ts` |
| components/palette/CommandPalette.tsx | aria-label='戻る'/'閉じる' を 'Back'/'Close' に置換。不可用時のエラー文言を 'Command palette unavailable (http client invalid)' に置換。REQUIRED_HTTP_METHODS から 'deleteSession' を削除 (本 PR で UI から呼ばない)。store ctx 構築から clearActiveIf を削除 (型変更追従)。 | `store/palette.ts`, `lib/tools.ts`, `api/sessions.ts` |
| components/palette/ScopeSegment.tsx | 既存英語ラベル (standard / push) と aria-label='palette scope' を維持。scopeDisabledReason から英語サブテキストが流れて来ることを描画確認のみ。 | `lib/tools.ts:scopeDisabledReason`, `store/palette.ts`, `store/daemon.ts` |
| test/fixtures/daemon.ts (新規) | mkSnapshot({projects: [...]}) helper を集約。projects=0/1/N+ ケースを共通生成し ParamSelectPhase.test.tsx / CommandPalette.test.tsx / tools.test.ts から利用。wire shape (SessionInfo / ProjectInfo) 変更時の修正範囲を 1 箇所に局所化。 | `wire/server.ts`, `store/daemon.ts` |
| __meta__/no-japanese.test.ts (新規) | src/client/web/src/ 配下の .ts/.tsx を glob し /[぀-ヿ一-龯]/ をスキャン。allowlist (ファイルパス → 検出許容パターン) を 1 箇所に集約し、本テスト自身と必要な日本語テストデータの False Positive を構造的に防ぐ。 | — |
| Vitest 更新 (既存テスト群) | ParamSelectPhase.test.tsx: materialize / 先頭プリセット (waitFor) / empty-state / dynamic-options 限定 effect / describe.each で preselectToolId 直接入口と toolSelect 経由の DOM 同一性 (FR-Det)。tools.test.ts: stop-session/sessionOptions 削除確認 + 英語ラベル + ParamDef 判別共用体形状。palette.test.ts: clearActiveIf 削除追従 + 英語エラートースト + preselect scope bypass 動作。App.test.tsx: Header の '⌘K'/'Ctrl+K' 表示が lib/platform を呼ぶこと + daemonSnapshot が openPalette に渡ることを確認。CommandPalette.test.tsx: aria-label/title 英語 + REQUIRED_HTTP_METHODS 変更追従。ToolSelectPhase.test.tsx: placeholder 英語 + stop-session 不在。useGlobalHotkey.test.ts: lib/platform 経由動作確認。sessions.test.ts: deleteSession テストを残置 (FR-B3 担保)。 | `ParamSelectPhase`, `lib/tools.ts`, `store/palette.ts`, `App.tsx`, `components/palette/CommandPalette`, `hooks/useGlobalHotkey`, `test/fixtures/daemon` |
| docs/specs/2026-06-25-web-ui-palette-bugfix/ | spec.md (背景・観測可能な振る舞いとしての要件・受け入れ条件) と plan.md (Implementation Sequence / Targets / Verification) を新規作成。既存 web-ui-fixes spec とは別ディレクトリ。relations で 2 本の新規 ADR を partOf 参照。 | `adr-20260625-paramselect-display-layer-materialize`, `adr-20260625-palette-ui-english-only` |

## Build Sequence (chunks 依存順)

依存方向: `m1-platform-foundation → m2-tools-registry-rewire → m3-empty-state-and-fixtures → m4-paramselect-materialize → m5-palette-store-rewire → m6-header-integration → m7-english-only-gate`

### Chunk: `m1-platform-foundation`

- **Depends on**: (なし、起点)

- **Members**:
  - component:lib/platform.ts (新規)
  - component:hooks/useGlobalHotkey.ts
  - req:FR-D1
  - req:FR-D2
  - req:FR-D3

### Chunk: `m2-tools-registry-rewire`

- **Depends on**: (なし、起点)

- **Members**:
  - component:lib/tools.ts (ToolDef registry)
  - req:FR-B1
  - req:FR-B2
  - req:FR-C1
  - req:FR-C2
  - req:FR-C4
  - adr: [0048-paramselect-display-layer-materialize](../../adr/0048-paramselect-display-layer-materialize.md)

### Chunk: `m3-empty-state-and-fixtures`

- **Depends on**: `m2-tools-registry-rewire`

- **Members**:
  - component:ParamEmptyState (React, 独立ファイル)
  - component:test/fixtures/daemon.ts (新規)

### Chunk: `m4-paramselect-materialize`

- **Depends on**: `m2-tools-registry-rewire`, `m3-empty-state-and-fixtures`

- **Members**:
  - component:ParamSelectPhase (React)
  - req:FR-A1
  - req:FR-A4
  - req:FR-Det
  - req:FR-IME

### Chunk: `m5-palette-store-rewire`

- **Depends on**: `m2-tools-registry-rewire`, `m4-paramselect-materialize`

- **Members**:
  - component:store/palette.ts
  - component:store/palette_helpers.ts
  - component:components/palette/ToolSelectPhase.tsx
  - component:components/palette/CommandPalette.tsx
  - component:components/palette/ScopeSegment.tsx
  - req:FR-A2
  - req:FR-A3
  - req:FR-B3
  - req:FR-C3
  - req:FR-C5

### Chunk: `m6-header-integration`

- **Depends on**: `m1-platform-foundation`, `m5-palette-store-rewire`

- **Members**:
  - component:App.tsx (Header)
  - req:FR-A2
  - req:FR-D1
  - req:FR-D2

### Chunk: `m7-english-only-gate`

- **Depends on**: `m2-tools-registry-rewire`, `m4-paramselect-materialize`, `m5-palette-store-rewire`, `m6-header-integration`

- **Members**:
  - component:__meta__/no-japanese.test.ts (新規)
  - component:Vitest 更新 (既存テスト群)
  - req:FR-C6
  - adr: [0049-palette-ui-english-only](../../adr/0049-palette-ui-english-only.md)

## Verification

- 静的: `cd src && go vet ./... && make lint` / `cd src/client/web && pnpm biome check && pnpm tsc --noEmit`
- テスト: `cd src/client/web && pnpm vitest run` (新規 __meta__/no-japanese.test.ts 含む) / 既存 Go テスト不変
- 手動: `make build && ./arc` で実機起動、Cmd/Ctrl+K → New Session → Project 選択 → command 入力 → 完走確認

## Open Questions (実装後 follow-up)

- notify level (success vs info) の UI マッピングが現状 success → info に縮退している。本 PR では FR-C3 の文言 assertion のみで担保するが、success 専用の見た目 (緑系 / ✓ icon) が必要かは別 task で再設計する。性質は plan の不確実性 (実装) のため plan.md の Open Questions に振り分け。
- push 機能語の語選 (push / send / broadcast) は本 PR スコープ外。既存ラベル 'push' を維持しつつ将来の UX 議論に委ねる。性質は spec の不確実性 (要件) のため spec.md の Open Questions に振り分け。
- 本 PR で停止する stop-session の代替 UI (SessionList × ボタン等) の設計は未着手。deleteSession HTTP クライアントと server route を残置する根拠 (FR-B3) は将来 UI の存在を前提としているため、別 spec で設計する必要がある。性質は spec の不確実性のため spec.md の Open Questions に振り分け。
- ASCII-only gate を Biome カスタムルール化するか Vitest メタテストのままにするか。本 PR は Vitest で実装するが、Biome 化が完了すればメタテストは削除可能。性質は plan の不確実性のため plan.md の Open Questions に振り分け。
- ParamSelectPhase.tsx が ParamEmptyState 抽出 + materialize ロジック追加で 500 行近辺になる見込み。実装中に超過した場合のさらなる抽出単位 (例: useDynamicParamPreset hook 化) は実装後の lint 結果次第。性質は plan の不確実性のため plan.md の Open Questions に振り分け。
