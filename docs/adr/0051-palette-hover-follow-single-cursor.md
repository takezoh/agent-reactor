# ADR 0051 — listbox の pointer hover と keyboard cursor を単一 highlight state に統一 (有効行のみ cursor 移動)

Status: Accepted

Related: [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: F-003 (UAC-006〜UAC-007), F-007

## Context

commit 9287c7f の CommandPalette は keyboard cursor (ToolSelectPhase / ParamSelectPhase の `cursor` state) と pointer hover (CSS `:hover`) を独立に扱う。keyboard で 4 行目に cursor を置いたあと pointer で 6 行目に hover すると、画面上には 2 つの highlight (`aria-selected=true` の 4 行目 + hover style の 6 行目) が同時に見える。Web の listbox UX 標準 (Raycast / Linear / VSCode Quick Pick) では hover が cursor を follow し highlight は 1 つに保たれる。

さらに critical な footgun として、Enter の発火対象が「keyboard cursor 位置」のままなので、ユーザーが pointer で hover している行と発火する行が乖離する。ユーザーは hover した行が選ばれると認識するが、実際は keyboard cursor 行が発火する。これは『見えているもの ≠ 起きること』の典型で、特に paramless push のような不可逆操作で誤送信を誘発する。

ADR 0050 で disabled 行を visible + skip にしたことで、新たに「disabled 行 hover で cursor を奪うべきか」という設計判断が発生する。disabled 行に cursor を移すと keyboard ↑↓ の skip ルール (有効行間移動) と矛盾する。本 ADR でこの境界を統合的に決定する。なお spec.md / plan.md は後続 plan-how フェーズで生成される。

## Decision

(1) cursor state を 1 ソース (`paletteCursor` / palette store の `cursor` field) で持ち、pointer hover と keyboard 操作の双方がこの単一 state を更新する。`aria-activedescendant` も同じ cursor を参照する。

(2) pointer の `pointermove` (mousemove) が**有効行**の上を通過したとき、cursor state を該当 index に更新する (hover follow)。keyboard ↓↑ も同じ cursor state を更新するため、両者の整合は構造的に保証される。

(3) **disabled 行への hover は cursor state を更新しない**。代わりに subtle hover style (background tint のみで focus ring は出さない) を CSS `:hover` で適用し、『情報を読みに来た』表現に留める。これにより keyboard ↑↓ skip ルール (ADR 0050) と pointer hover ルールが一貫する。

(4) `mouseleave` で pointer が listbox 領域外に出たら、cursor state は最後の keyboard cursor 位置を保持し、その行の visual highlight を確実に復元する (listbox 外に出た瞬間に highlight が消えるバグを構造的に防ぐ)。

(5) Enter は常に「現 cursor 位置」で発火する。disabled 行を hover 中に Enter を押しても cursor は有効行のままなので、有効行が発火する (誤発火しない)。これは UAC-007 の構造的保証である。

(6) keyboard 操作直後に pointer が静止している場合の hover suppression (一定時間 mousemove を ignore する) は本 ADR では導入しない。導入すると state machine が複雑化し、テストコストが増える。現状の単一 cursor + hover follow で footgun は無いため、UX 監視対象として open_questions に残す可能性を許容する。

## Consequences

- **positive**: highlight 競合 (keyboard / pointer 2 重) が構造的に排除される。画面上の highlight は常に 1 つで `aria-activedescendant` と visual が一致する。
- **positive**: Enter 誤発火 (hover している行と発火する行が異なる) が構造的に防止される。disabled 行を hover していても安全に Enter を押せる。
- **positive**: ADR 0050 の keyboard skip ルールと pointer hover ルールが「disabled 行は cursor を奪わない」という単一原則で統合される。実装と test の両方で重複が減る。
- **positive**: screen reader (NVDA / VoiceOver) は `aria-activedescendant` だけを追えば cursor 位置を正しく告知できる。
- **negative**: pointer hover で cursor が動くため、keyboard 中心のユーザーが意図せず cursor を奪われる可能性がある (例: ↓で 5 行目まで動かし思考中に pointer がたまたま 7 行目を通過すると cursor が 7 行目に飛ぶ)。keyboard 直後の hover suppression は本 ADR では入れず UX 監視対象とする。
- neutral: pointer ユーザーが ↑↓ を併用しても highlight は単一なので操作系混在は safe。
- neutral: hover follow は CSS `:hover` だけで実現できない (cursor state 更新が必要) ため、ToolSelectPhase 内に `onPointerMove` handler を 1 箇所追加する小規模変更が入る。

## Alternatives Considered

### pointer と keyboard で別 highlight を出し、最後の操作系で visual を切り替える

state 数が 2 倍になり (`keyboardCursor` + `pointerCursor`)、操作系判定 (`lastInputKind`) も追加実装する必要がある。テストケースも 2 倍 (両方の組み合わせ) に膨らむ。Q2 (本タスク resolved_issues) で『単一 highlight 採択』に確定済み。

### 有効行 hover でも cursor を更新せず、別 hover style だけ出す

`aria-activedescendant` (cursor) と visual highlight が乖離し、screen reader と sighted user の認識が食い違う。Enter の対象が「いま highlight が見えている行」ではなく「過去に keyboard で動かした行」になり直感に反する。

### disabled 行 hover でも cursor を奪う (skip ルールを hover では適用しない)

keyboard ↑↓ で disabled を skip するルールと、pointer hover で cursor が disabled に着地するルールが食い違う。Enter が disabled 行で発火し ADR 0050 の inline status feedback が走るが、結果的に「disabled なら何もできない」のは同じで、cursor を奪う意味が無い。
