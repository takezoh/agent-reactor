# ADR 0070 — `fontSize` の device-scoped 永続化を `usePersistedValue` adapter で集約し parse 失敗と範囲外を別経路でハンドルする

Status: Accepted

Related: [ADR 0059](./0059-theme.md), [ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md), [ADR 0072](./0072-coachmark-dismiss-and-once.md)
Related code: `src/client/web/src/hooks/usePersistedValue.ts` (new adapter), `src/client/web/src/hooks/useFontSize.ts` (new), `src/client/web/src/hooks/useCoachmarkOnce.ts` (new)
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-PERSIST-001/002`, `FR-MOB-COACH-001`

## Context

`ux.md` の UAC-018 (20px 永続化) / UAC-019 (999 → 28 max clamp) / UAC-017 (8 下限) を満たし、private mode の localStorage 例外で起動が壊れない契約が必要。

否定役の指摘により『parse 失敗 → default 14』と『parse 成功 + 範囲外 → clamp で吸収』を**分けて契約化する**必要が判明 (前者解釈の実装が UAC-019 counterexample『28px clamp されない』を許す)。複数 hook (`useFontSize` / `useCoachmarkOnce`) が独自に `try/catch` すると 漏れの温床になる。`web-active-session-ownership` の教訓により session に紐付けず **device-scoped (per-browser)** で扱う。

## Decision

(1) **`createPersistedValue<T>(key, {parse, serialize, fallback, validate})` adapter を 1 個新設**し、`try/catch` / `parseInt` / `Number.isFinite` / clamp / serialize の責務を 1 箇所に集約。テスト時は in-memory `Map` を inject 可能 (DI)。

(2) localStorage key `arc.web.term.fontSize` (number) と `arc.web.term.hintSeen` (boolean) を同 adapter で扱う。

(3) **読み出し経路の厳密化**:
- `parseInt` が `NaN` の場合は default 14 へフォールバック
- `parseInt` 成功 + `Number.isFinite` チェック後に `[8,28]` へ clamp して採用する
- 例:
  - `'999'` → parseInt 成功 → finite → clamp → 28 (UAC-019 満たす)
  - `''` → NaN → 14
  - `'foo'` → NaN → 14
  - `null` → 14

(4) 書き出しは `try/catch` で例外を握りつぶし、書込失敗時もメモリ上の state は更新 (UX は degrade のみ)。

(5) helper textarea の font-size は本 ADR の対象外 ([ADR 0068](./0068-mode-separation-focus-block-and-zoom-guard.md) で CSS `!important` 16px 固定)、grid `.xterm-rows` と独立。

(6) `fontSize` は theme ([ADR 0059](./0059-theme.md)) と独立 key で衝突しない。

## Alternatives Considered

### per-session (`sessionId` に紐付け) で fontSize を保存

`web-active-session-ownership` の教訓に反する / 同一デバイスの好みは全セッション共通が自然 / session 切り替えで毎回 reset で UX 悪化。**却下**。

### IndexedDB で保存

起動時の同期 read 不可で初期描画 race / 過剰 / localStorage で十分。**却下**。

### Cookie で保存

毎リクエストでネットワーク経由 / device-scoped でなくドメイン scope / セキュリティ面で過剰。**却下**。

### parse 不能 / 範囲外を全て default 14 へ fallback (現解釈)

UAC-019 (999 → 28 max clamp) の counterexample を許す。『範囲外なら clamp』と『不正なら fallback』を分けないと判別性が失われる。**却下**。

### 各 hook が独自に `try/catch` + `parseInt` + clamp を書く

DRY 違反 / 漏れの温床 / テスト時の DI が困難 / adapter 1 個で同等の効果。**却下**。

## Consequences

- private mode (localStorage 例外) で起動が壊れず default 14 で degrade のみ
- 起動時 `NaN cols` (`fit()` が 0 以下 fontSize で破綻する経路) が `parseInt` + finite + clamp の 3 段検証で防止される
- UAC-019 (999 → 28 clamp) が parse 成功時のみ clamp 適用ルールで明確に満たされる
- 永続化 adapter が再利用可能になり、将来の preference 追加 (例: 入力履歴) でも同 adapter で型安全に扱える
- テスト時に in-memory storage を inject でき happy-dom の localStorage 実装に依存しない決定的テストが書ける

## Related Requirements

- `FR-MOB-PERSIST-001` — pinch / FontSizeControl 確定時の localStorage 書込 + try/catch degrade
- `FR-MOB-PERSIST-002` — parseInt NaN → 14 / parse 成功 + finite → [8,28] clamp の厳密化
- `FR-MOB-COACH-001` — hintSeen 初回 render 時冪等書込 (同 adapter 共有)
