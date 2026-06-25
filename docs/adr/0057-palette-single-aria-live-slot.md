# ADR 0057 — palette 内のすべての status announce は単一の aria-live='polite' slot を経由する

Status: Accepted

Related: [spec](../specs/2026-06-25-web-palette-redesign/spec.md), [plan](../specs/2026-06-25-web-palette-redesign/plan.md), [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: FR-005, FR-010, FR-024, FR-025, FR-029, FR-031, FR-033

## Context

InlineStatus (disabled-attempt feedback) / ActiveContextHeader (active session 変化) / StatusBadge (Sending… / Unavailable) はそれぞれ独立に role='status' aria-live='polite' を持たせる候補があったが、aria-live region が複数共存すると screen reader の announce 順序が UA 依存になり、UAC-001 / UAC-005 / UAC-013 / UAC-017 の体験が決定論的に保証できない。

NFR (ARIA) は「何を announce するか」までしか規定しておらず、「どの slot を経由するか」が未整理だった。連続発火 (active 変化 + disabled attempt が同 frame) や submitting=true 中の announce 順序が UA ごとに異なると、a11y 体験が不安定になる。

## Decision

palette 内の polite アナウンスは InlineStatus.tsx が提供する単一の aria-live='polite' region に集約する。ActiveContextHeader / StatusBadge は aria-live を持たず、announce が必要な遷移 (sessionID 変化 / Sending… 遷移 / Unavailable 遷移) で store の announce slot (palette_active_context slice の announceSeq) に文字列を投入する。InlineStatus はこの slot の seq 変化を購読して 1 領域内に message を流す。

announce 順序は seq の単調増加で決定論化する。submitting=true 中の announce も同じ slot を経由する (FR-033)。

## Consequences

- positive: screen reader の announce 順序が UA 依存にならず、test (Vitest + jsdom) で 'aria-live container の最新 text content' を assert することで成功条件を観測できる
- positive: aria-live region が 1 つに集約されるため、a11y review (axe 等) の差分が縮小する
- positive: 連続発火 (active 変化 + disabled attempt) の順序が seq の単調増加で決まり、test 容易性が高い
- negative: ActiveContextHeader / StatusBadge / 各 announce 元と InlineStatus が store の announce slot を介して結合する間接性が生まれる。直接 aria-live を持つ実装より trace が一段長い
- neutral: visually-hidden な aria-live container を 1 つ専用に持つことになり、DOM 構造が 1 行増える

## Alternatives Considered

### 各 component が独立に aria-live='polite' を持つ

複数 polite region の announce 順序が UA 依存。連続発火 (active 変化 + disabled attempt 同時) で体験が不安定。

### role='log' aria-live='polite' で履歴を保持

履歴は要求されておらず冗長。UAC-005 (1 回読み上げて消える) と矛盾。

### aria-live='assertive' に格上げ

disabled attempt や active 変化は緊急情報ではなく、screen reader user の作業を中断させる必要がない。
