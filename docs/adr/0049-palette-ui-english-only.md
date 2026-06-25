# ADR 0049 — コマンドパレット UI 表面の固定文字列を英語に統一し i18n 基盤導入を見送る

Status: Accepted

Related: [spec](../specs/2026-06-25-palette-bugfix/spec.md), [plan](../specs/2026-06-25-palette-bugfix/plan.md)
Related requirements: FR-C1, FR-C2, FR-C3, FR-C4, FR-C5, FR-C6

## Context

commit 9287c7f はパレット UI に日本語文字列と英語文字列を混在させた。一方で本リポジトリには i18n 基盤が存在せず、対象ユーザーも開発者向け CLI コンパニオンに限られているため、当面の言語は単一でよい。stop-session 撤去 (本 PR) や Header New Session ボタンの修正で複数ファイルにわたる文言変更が同時発生するため、置換漏れと将来の混入を防ぐ機械的ガードが必要。

## Decision

(1) パレット UI 表面 (ToolSelectPhase / ParamSelectPhase / CommandPalette / ScopeSegment / Header) と関連エラー文言 (store/palette / store/palette_helpers / App.tsx toast) の固定文字列を英語に統一する。(2) i18n フレームワーク (react-intl 等) は導入しない。固定文字列リテラルを直接置換する。(3) 回帰防止として src/client/web/src/__meta__/no-japanese.test.ts (Vitest メタテスト) を 1 本追加し、src/client/web/src/ 配下の .ts/.tsx を glob して /[぀-ヿ一-龯]/ をスキャンする。検出する必要がある日本語テストデータは allowlist (ファイル単位) で除外する。(4) Biome カスタムルール化は本 PR のスコープ外とし将来 PR に委ねる。

## Consequences

- **positive**: 1 PR で文言が単一言語に揃い、レビュー時の認知負荷が下がる。
- **positive**: メタテストにより将来コミットで日本語が混入しても CI で検知できる。
- **positive**: i18n 基盤のオーバーキルを避け、AGENTS.md のライブラリ選定プロセスを発火させない。
- **negative**: 将来 i18n を導入する際は本 ADR を supersede し固定文字列を抽出する手作業が発生する。
- neutral: allowlist の管理コストが残る (現状は no-japanese.test.ts 自身のみ allowlist)。

## Alternatives Considered

### 薄い i18n キーマップ (src/client/web/src/i18n/strings.ts) を導入

将来拡張点は用意できるが本 PR で得る価値が薄く YAGNI。固定文字列置換と等価コストで導入する正当性が薄い。

### react-intl 等の i18n ライブラリを導入

ライブラリ追加は AGENTS.md の選定プロセスを必要とし、本 PR の責務 (バグ修正) を超える。

### ASCII gate を Biome カスタムルールで導入

Biome のカスタムルール導入は既存設定への侵襲が大きい。Vitest メタテストで等価な保証が得られ、必要なら別 PR で Biome 化できる。

### ASCII gate を導入せず文字列置換のみで済ます

将来コミットでの日本語混入を検知できず、本 PR の効果が時間とともに劣化する。
