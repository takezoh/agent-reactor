# ADR 0048 — ParamSelectPhase で dynamic-options を表示層 materialize する (ParamDef 判別共用体化)

Status: Accepted

Related: [spec](../specs/2026-06-25-palette-bugfix/spec.md), [plan](../specs/2026-06-25-palette-bugfix/plan.md)
Related requirements: FR-A1, FR-A2, FR-A3, FR-A4, FR-Det, FR-IME

## Context

commit 9287c7f は ParamDef.options を静的配列で宣言する設計を導入したが、project options はランタイムの daemon snapshot に依存する。静的宣言では options が常に [] となり、New Session の listbox が無言の空 listbox を描画して FR-A1〜A4 を満たさない。さらに openPalette の preselect resolve (store 層) と paramSelect の表示 (React 層) で snapshot 源が異なり、二重実装 / drift のリスクがある。ADR-0036 で『store は DOM/HTTP を持たない』ことが確定しているため、materialize 責務は store ではなく React 表示層に閉じるべき。

## Decision

(1) ParamDef を判別共用体化する: kind: 'text' | 'static-options' | 'dynamic-options'。dynamic-options は materializeKey: 'projects' のような string registry key を持ち、options 配列は宣言しない。(2) ParamSelectPhase 内の純関数 materializeOptions(param, snapshot) が param.kind==='dynamic-options' のとき materializeKey で dispatch して projectOptions(snapshot) を返す。(3) ToolSelectPhase 経由と openPalette(preselectToolId) 経由の両入口で同じ materializeOptions を通すことを describe.each テスト (FR-Det) で機械的に保証する。(4) openPalette の preselect resolve は scope filter を bypass し、materialize ではなく ToolDef 存在性のみを判定する責務に限定する。(5) dynamic-options かつ value===undefined のとき限定で初回 useEffect が setParam(param.id, options[0].value) を発火し先頭プリセットを成立させる。effect dep は materializeKey で memo した options ref を使い identity を不変化させ無限ループを回避する。

## Consequences

- **positive**: ADR-0036 の境界 (store は DOM/HTTP 非保有) を維持しつつ、materialize 責務を React 表示層の単一純関数に閉じる。
- **positive**: ToolDef は cookie-cutter な静的宣言性を保持できる (snapshot 依存を materializeKey 文字列に外出し)。
- **positive**: materialize 単一経路を describe.each テストで機械的に保証でき、二重実装が紛れ込んだ瞬間に CI が落ちる。
- **negative**: materializeKey の registry 1 箇所が将来 dynamic param 増加に伴って成長する (現状は 'projects' のみ)。
- **negative**: dynamic-options 限定の初回 effect により、初回 render では aria-activedescendant が undefined となり 2nd render で 0 に当たる。テストは findBy / waitFor を使う必要があり、FR-A1 は『先頭プリセット選択済みで開く』の解釈を『最終的な観測』として定義する。

## Alternatives Considered

### ToolDef.optionsFor(snapshot) を ToolDef に持たせ ParamDef.options を廃止

ToolDef が daemon snapshot に依存する責務を負い、現在 cookie-cutter な静的宣言性が崩れる。本 PR スコープを超える大規模リファクタになる。

### palette store の selector で materialize し ParamSelectPhase は store 経由で受け取る

ADR-0036 (store は DOM/HTTP 非保有) と矛盾する派生値を store に持たせることになり、store スタブのテストコストが増える。

### ParamSelectPhase 内で param.id 文字列を直接 dispatch (判別共用体化なし)

param.id 増加時に React 層と ToolDef 層の両方で dispatch が肥大化し、責務境界が曖昧になる。判別共用体化により param.id を React 層が知らずに済む。

### 初回プリセットを effect ではなく『currentIdx<0 のとき視覚的に 0 を見せる』表示専用にする

paramValues.project が undefined のまま最終 Enter まで進み、エラー復元時の挙動が不安定になる。早期に store に true なる初期値を入れる方が submit 経路全体が単純化する。
