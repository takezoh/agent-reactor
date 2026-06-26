# ADR 0029 — `.terminal-host` の高さを flex で確定させる (Web UI 問題4 の真因対処)

Status: Accepted

Related: [spec](../specs/2026-06-24-web-ui-fixes/spec.md), [plan](../specs/2026-06-24-web-ui-fixes/plan.md), [ADR 0034](./0034-refit-raf-coalesce-and-test-infra.md)
Related requirements: FR-006, FR-007, FR-008

## Context

`src/client/web/src/css/app.css` の `.terminal` は grid row (`1fr`) 内の flex-column で、`DriverViewPanel` / `LogTabs` / `TerminalPane` が縦に並ぶ。`.terminal-host` は

```css
.terminal-host {
  width: 100%;
  height: 100%;
}
```

のみで flex-basis / flex-grow を持たない。flex コンテナ内の `%` 高さは親の確定値が無いと安定して解決されず、兄弟パネル (`DriverViewPanel` / `LogTabs`) の出現消滅で host の実高さが不定になる。この状態では `ResizeObserver` を足しても `FitAddon.fit()` が 0 もしくは過大値を返し続け、Web UI 問題4 (0 サイズ初期化・非追従) は直らない。

plan-how の否定役は当初案 (ResizeObserver 追加が主・CSS は「必要に応じて」) に対し **blocker** を立てた。真因が CSS 構造側にあり、ResizeObserver は従属対策に過ぎないという指摘。

## Decision

`.terminal-host` を

```css
.terminal-host {
  flex: 1 1 0;
  min-height: 0;
  width: 100%;
}
```

とし、flex 上で確定した残余高さを得るよう CSS を修正することを **主対策** とする。必要に応じて親 `.terminal` にも `min-height: 0` を入れる。`ResizeObserver` / `scheduleFit` ([ADR 0034](./0034-refit-raf-coalesce-and-test-infra.md)) は確定した box size の変化に追従する **従属対策** と位置づける。

## Consequences

- host が flex 残余高さを確定して得るため `fit()` が実サイズを返し、FR-008 (0 サイズ回避) と FR-006/007 (追従) の前提が成立する
- `ResizeObserver` は「サイズ変化への追従」に責務が限定され、レイアウト確定の責務を負わない (主従が正される)
- `min-height: 0` を入れないと flex item が内容で潰れない既定挙動が残るため、回帰確認 (host が画面残余を占める) を手動 Verification に含める必要がある
- CSS のみの変更で JS 側の複雑度は増えない

## Alternatives Considered

### `ResizeObserver` だけ追加し CSS は「必要に応じて」後回し (初稿の主従)

却下: host が flex 上で確定高さを得られない限り `fit()` が安定値を返さず、`ResizeObserver` を入れても FR (0 サイズ回避・追従) を満たせない。真因が CSS 構造側にある。

### `.terminal-host` を grid row の sub-grid / 絶対配置で高さ確定

却下: 既存 grid/flex レイアウトの作り直しになり変更面積が増える。`flex: 1 1 0` + `min-height: 0` の最小修正で十分。
