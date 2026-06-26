# ADR 0030 — セッション切替時の stale render を keyed remount で解消する (Web UI 問題3)

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md)
Related requirements: FR-005

## Context

`src/client/web/src/components/TerminalPane.tsx` は **単一 xterm インスタンスを全セッションで共有** している。コメント L84-92 に

> when sessionId changes, we don't reset xterm — TerminalPane is keyed on sessionId by parent if a full reset is desired. β: single shared term.

と明記されているが、`src/client/web/src/App.tsx` は `<TerminalPane conn={conn} sessionId={activeSessionID} />` で **key を付けていない**。よって切替時に前セッションの buffer が xterm に残り、ユーザは前セッションの画面の上に新セッションの出力が重なるように見える (Web UI 問題3)。

`OutputFrame` は `frame[3] === sessionRef.current` で filter 済みなので別セッション出力の混入は防げているが、xterm の既存描画はクリアされない。

代替案として `useEffect` で sessionId 変化時に `term.clear() / term.reset()` を呼ぶ初稿があったが、plan-how の否定役は **major** を指摘した:

- clear エフェクトと subscribe エフェクトの React 実行順依存が生まれる
- `sessionRef.current` の同期更新と clear のエフェクト遅延の境界で、新セッション先頭出力が clear に巻き込まれる順序競合がある
- happy-dom 上の `FakeTerminal` mock は `clear()` / `reset()` 未実装で test-setup 拡張も要る

加えて、購読 (`conn.subscribe` / `unsubscribe`) が `SessionList.onClick` と `TerminalPane` の `useEffect` の **両方で走り二重化** している (購読所有者の重複)。これも `unsubscribe(old) → subscribe(new)` の順序競合の根。

## Decision

`App.tsx` で

```tsx
<TerminalPane key={activeSessionID ?? "none"} conn={conn} sessionId={activeSessionID} />
```

として **keyed remount** し、切替ごとに新しい空 term を構築することで、stale buffer / scrollback 残留 / clear-vs-write 競合を React の key 機構に委ねて機械的に解消する。`TerminalPane` に sessionId-clear `useEffect` は新設しない。

あわせて **購読の所有者を `TerminalPane` (マウント / アンマウント) に一本化** し、`SessionList.onClick` は `selectSession(s.id)` のみとする。これにより:

- 旧 `TerminalPane` の cleanup で `conn.unsubscribe(旧 sid)`
- 新 `TerminalPane` の mount で `conn.subscribe(新 sid)`

が React のマウントライフサイクルに沿って実行され、購読源の二重化と順序競合の根を断つ。

## Consequences

- 新セッションは必ず空 term から始まり、FR-005 (旧出力が残らない) を分岐ロジック無しで満たす
- `onOutput` 単一スロットの所有権が「1 マウント = 1 conn ライフサイクル」に揃い、StrictMode 二重マウントも cleanup で `onOutput = undefined` → 再設定の対で閉じる (否定役の所有権懸念が縮小)
- 切替直後はスクロールバック履歴が失われ画面は新規出力到着まで空になる (履歴保持は connection / サーバ拡張が要りスコープ外)
- 切替ごとに xterm 生成破棄コストが発生するが、人間操作のセッション切替頻度では無視できる
- 購読源が `SessionList` と `TerminalPane` に二重化していた状態を解消し、`unsubscribe(old)` / `subscribe(new)` 順序競合の根を断つ

## Alternatives Considered

### 単一 term 維持 + sessionId 変化 effect で `clear()` / `reset()` + `fit()` (初稿)

却下: clear エフェクトと subscribe エフェクトの順序競合で新セッション先頭出力が消えうる。`FakeTerminal` mock 拡張も要る。keyed remount が競合自体を消す。

### 切替時にスナップショット再取得して再描画

却下: `connection.ts` は出力スナップショット再送機構を持たず connection / サーバ拡張が必要でスコープ外。

### 購読を `SessionList` 側に残したまま keyed remount だけ行う

却下: 購読源が二重化したままで `unsubscribe(old)` / `subscribe(new)` の順序競合と二重購読が残る。購読所有者を `TerminalPane` に一本化する。
