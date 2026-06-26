# ADR 0067 — モバイル UX gate を `matchMedia('(max-width: 767px) and (pointer: coarse)')` の AND 契約に固定する

Status: Accepted

Related: [ADR 0029](./0029-terminal-host-flex-height.md), [ADR 0030](./0030-keyed-remount.md), [ADR 0034](./0034-refit-raf-coalesce.md), [ADR 0065](./0065-terminal-slot-absolute-overlay.md), [ADR 0066](./0066-terminal-scrollback-via-vt-buffer.md), [ADR 0074](./0074-migration-pc-only-to-pc-plus-mobile.md)
Related code: `src/client/web/src/components/TerminalPane.tsx`, `src/client/web/src/hooks/useMobileGate.ts` (new)
Related spec: [Web Terminal Mobile UX spec.md](../specs/web-terminal-mobile-ux/spec.md) — `FR-MOB-GATE-001/002`, `FR-PC-PRESERVE-001/002/003`

## Context

`ux.md` assumption §1 で gate の確定値は与えられていたが、CSS `@media` と JS 判定のどちらを真実源にするかが未確定だった。CSS `display:none` 切替では a11y tree から overlay を除外できず、また UAC-022 counterexample (幅のみ gate で narrow desktop window が誤 mobile 化する PC regression) が他全シナリオを green のまま通り抜ける判別不能性が `ux.md` で固定された。回転 (デバイス向き変更) で 767px 境界をまたぐ際の入力モード state 破棄経路も load-bearing として確定する必要がある。F-006 (PC 非影響) は保護フローのため `vs_legacy: must-pass` の例外承認も本 ADR に含める。

## Decision

(1) `matchMedia('(max-width: 767px) and (pointer: coarse)').matches` を真実源とし、`useMobileGate` hook が boolean を返す。

(2) `MediaQueryList.change` を購読し、`true→false` 遷移時には callback で **(a)** 入力モード state を破棄、**(b)** helper textarea の `readonly` を解除、**(c)** overlay 群を unmount する。

(3) SSR / `matchMedia` 不在環境 (古い browser) は `false` 固定で fallback。

(4) CSS の `@media (max-width:767px)` は補助 (`touch-action` 等の宣言用) で、要素の存在/不在は条件 render で決定する。`display:none` による隠蔽は禁止 (querySelector / a11y tree に残り counterexample が通る)。

(5) **F-006 全 UAC が `vs_legacy:must-pass` であることは PC 保護フローの例外として承認**。他全フロー (F-001〜F-005, F-007) は `must-fail` を 1 件以上保持し regression discriminator が確保されているため、F-006 のみ `must-fail` を要求しない判断を本 ADR で明文化し、後段 plan-impl で『must-fail を追加すべき』という二重審査が起きないようにする。

## Alternatives Considered

### 幅のみ gate `matchMedia('(max-width: 767px)')`

UAC-022 (700px + `pointer:fine` narrow desktop window) が mobile 化し PC 入力 regression を全シナリオ green のまま通り抜ける。**却下**。

### `pointer:coarse` のみ gate

外付け touch screen 付き PC や iPad pen 接続時に誤 mobile 化、PC ユーザーが意図せず閲覧モードに入る。**却下**。

### userAgent sniff

brittle / 将来の OS 更新で破綻 / iPadOS が macOS UA を返すなど exception 多発。**却下**。

### CSS `display:none` による隠蔽

要素が DOM / a11y tree に残り UAC-012 / UAC-021 counterexample (querySelector で取得可能) で fail、screen reader にも幽霊ボタンとして読まれる。**却下**。

### F-006 にも `must-fail` を追加して plan-how で再審査

保護フローの『現実装で書いたら違反する EARS』を要求すると逆に PC 改変を促す悪い圧力。UAC-021 / UAC-022 / UAC-023 が他フローからの誤伝播を判別する役割を既に果たしている。**却下**。

## Consequences

- PC narrow-window (≤767px + `pointer:fine`) が誤 mobile 化する UAC-022 counterexample が AND 契約で構造的に排除される
- デバイス回転で gate が `true→false` に変化した瞬間に入力モード state が破棄され、helper textarea readonly が解除されて legacy PC 経路へ即時復帰する
- 条件 render により mobile overlay は PC の a11y tree に一切現れない (querySelector でも取得できず screen reader にも読まれない)
- CSS `@media` と JS gate の二重判定は『JS が真実源』ルールで一意化され、`display:none` による隠蔽実装が code review で禁忌として識別可能
- F-006 全 must-pass の例外承認が ADR で明文化され、後段 plan-impl で『must-fail を 1 件追加すべき』という二重審査が起きない

## Related Requirements

- `FR-MOB-GATE-001` — matchMedia AND 契約での gate true 判定 + 条件 render
- `FR-MOB-GATE-002` — gate true→false 遷移時の state 破棄順序保証
- `FR-PC-PRESERVE-001` — gate false で overlay 全不在
- `FR-PC-PRESERVE-002` — 700px + pointer:fine narrow-window が PC として動作
- `FR-PC-PRESERVE-003` — gate false で touch-action / wheel scroll の legacy 維持
