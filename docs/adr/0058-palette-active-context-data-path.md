# ADR 0058 — Active context header の projBase 解決は SessionInfo.projectPath + projects[] 突合で行い、欠落時は sid8 のみ fallback 表示する

Status: Accepted

Related: [spec](../specs/2026-06-25-web-palette-redesign/spec.md), [plan](../specs/2026-06-25-web-palette-redesign/plan.md), [ux](../specs/2026-06-25-web-palette-redesign/ux.md)
Related requirements: FR-009, FR-010, FR-025, FR-027, FR-028

## Context

FR-009 / UAC-001 / UAC-013 は Active context header に "Active: <projBase> / <sid8>" を表示することを要求する。activeSessionID から projBase を引くには、(1) sessions[].sessionID === activeSessionID の SessionInfo を引き、(2) その SessionInfo の projectPath を取り、(3) projects[] と突合してプロジェクトを特定する経路が必要。

現状 SessionInfo の wire-format に projectPath が含まれているかが計画段階では未検証 (out_of_scope: wire-format 変更禁止)。SessionInfo に projectPath が無い、または activeSessionID が sessions[] に存在しない (stale) 場合の fallback を確定する必要がある。本 PR では wire 拡張を行わず、欠落時の fallback で完結させる方針を確定する。

## Decision

projBase 解決は (a) `sessions[].find(s => s.sessionID === activeSessionID) → SessionInfo.projectPath → projects[].find(p => p.path === projectPath)` の経路で行う。

経路が解決できる場合は "Active: <projBase> / <sid8>"。activeSessionID は存在するが SessionInfo.projectPath が空 / sessions[] に該当 session が無い / projects[] に該当 project が無いいずれかの場合は "Active: ??? / <sid8>" (sid8 のみ表示、tooltip に full sessionID)。activeSessionID 自体が null / undefined の場合は icon + "— No active session"。

SessionInfo に projectPath field が現状 wire-format に存在しない場合は、本 PR では sid8 のみ fallback で常時動作させ、wire-format 拡張は別 PR とする。Disambiguator " (under <parent>)" は active header のみで適用し、push tool 行には適用しない (表示一貫性は header だけで確保)。

## Consequences

- positive: wire-format 変更を本 PR の前提から外せるため、scope を 1 PR 内に収める
- positive: fallback (sid8 のみ) が明確に定義されることで、SessionInfo の shape が未確認でも test が書ける ('???' branch / 'No active session' branch / 通常 branch)
- positive: disambiguator を header のみに限定することで push 行の表示が単純化し、cognitive load が下がる
- negative: SessionInfo に projectPath が無い場合は projBase が一切表示されず、UAC-001 の experience がやや劣化する (sid8 のみで判別)。フォローアップ PR で wire 拡張する
- neutral: fallback 経路の test は spec.md の Acceptance table に明記する

## Alternatives Considered

### SessionInfo に projectPath を本 PR で追加 (wire-format 拡張)

out_of_scope (wire-format 変更禁止) と矛盾し、本 PR の規模を肥大化させる。server 側 review も必要になる。

### projBase が引けない場合は Active context 行を非表示にする

FR-009 (Active context 行は常時 1 行存在) と矛盾し、header の高さが揺れる視覚 jank を招く。

### Disambiguator を push 行にも適用

snapshot 全体走査の計算が rendering 経路に乗り、視覚ノイズも増える。
