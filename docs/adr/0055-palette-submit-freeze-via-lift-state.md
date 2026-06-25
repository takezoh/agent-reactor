# ADR 0055 — submit in-flight 中の palette UI freeze は CommandPalette の lift-state で実装する

Status: Accepted

Related: [spec](../specs/2026-06-25-web-palette-redesign/spec.md), [plan](../specs/2026-06-25-web-palette-redesign/plan.md), [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: FR-012, FR-013, FR-014, FR-018, FR-033

## Context

FR-012〜014 / UAC-017〜018 は submit 中に Active context / sorted list / status badge / chip visibility / toast 送信先がすべて「submit 開始時点の snapshot」に凍結されることを要求する。現状 ToolSelectPhase / ParamSelectPhase / ActiveContextHeader はそれぞれ useDaemonStore の primitive selector を購読しており、子コンポーネントが独立に re-render するため、CommandPalette 内で useRef を作っても freeze は成立しない。

freeze を成立させるには (a) 子の購読を止めて CommandPalette からの props 経由に切り替える (lift-state) (b) store に frozenDaemonSnapshot を持たせる (c) view-update 自体を一時 unsubscribe する のいずれかが必要。ADR-0036 (store 純粋性) を維持しながら、submit 中の体験を決定論化する方針を確定する必要がある。

## Decision

submitting transition (false→true) で CommandPalette が現スナップショット (DaemonSnapshot + activeContextSnapshot + sortedList) を frozenSnapshotRef に capture し、submitting=true の間は ToolSelectPhase / ActiveContextHeader / StatusBadge に frozen snapshot を props で渡す。子コンポーネントはこれら snapshot props を最優先で参照し、submitting=true の間は自前で daemon selector を購読しない (props 優先モード)。submit resolve (true→false) で frozenSnapshotRef を解放し、子は通常の selector 購読に戻る。

store には freeze 関連 state を持たせない (ADR-0036 store 純粋性維持)。送信 toast の projBase / sid8 も同じ frozen snapshot を ctx 拡張経由で lib/tools.ts に渡す。これにより toast と send 先が同一 snapshot を参照することが構造的に保証される (FR-014 / UAC-018)。

## Consequences

- positive: store 純粋性 (ADR-0036) を維持しつつ、freeze の責務が CommandPalette 1 箇所に集約される
- positive: freeze 中に suppress すべき経路 (子の selector 購読) が enumerate しやすく、test で全経路を網羅できる (CommandPalette.test.tsx 1 ケース: submitting=true 中の daemon mutation が header text / list 順序を変えないことを assert)
- positive: toast 送信先 (UAC-018) も同じ snapshot を使うため、submit 中 active 変更で toast が反転するバグを構造的に防げる
- negative: ToolSelectPhase / ActiveContextHeader / StatusBadge の signature が拡張され、props 経由のスナップショットと自前 selector の両モードを持つことになる (frozen props が undefined のときのみ自前 selector を購読)。test 上は両モードを別ケースで検証する必要がある
- neutral: store には freeze 関連 state を持たないため、store の type 表面積は最小化される

## Alternatives Considered

### store に frozenDaemonSnapshot 場を追加し submitting 中はそれを返すセレクタを提供

palette store が daemon store の DaemonSnapshot 型に依存することになり、layer の依存方向が逆転する。store 純粋性 (ADR-0036) と矛盾する。

### view-update 自体を一時 unsubscribe する

daemon store の購読を palette が制御するのは副作用範囲が広く、別 component への影響が予測困難。

### submitting 中だけ children を unmount / remount

input focus と composing 状態が失われ、ADR-0040 の composing guard と矛盾する。
