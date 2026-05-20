# 010: orchestrator/workspace — workspace manager + hooks + safety

- **Phase**: P2c ([plans/04-phases.md#p2-linear-adapter--workspace--hooks](../plans/04-phases.md))
- **Status**: Done (2026-05-20)
- **Depends on**: 006 (merged; `wfconfig.Config` の workspace/hooks)。008/009 には依存しない
- **Blocks**: P3 (Agent Runner が workspace を作って agent を起動する)

## Background

SPEC §9 の Workspace Manager を `orchestrator/workspace/` に実装する。per-issue ディレクトリの生成/再利用、4 hooks の実行、そして **最重要の安全不変条件 (§9.5)** を担う。tracker とは独立。

## Tasks

### A. パッケージ + パス計算 (§9.1–§9.2)

- [x] `src/orchestrator/workspace/` 新設 (`package workspace`)
- [x] `Manager` を `wfconfig.Config` (workspace.root + hooks) から構築
- [x] `sanitizeKey(identifier)` — `[A-Za-z0-9._-]` 以外を `_` に置換 (§9.5 Invariant 3)
- [x] per-issue path = `<workspace.root>/<sanitized_key>`
- [x] `Ensure(ctx, issue)`:
  - [x] ディレクトリを冪等に作成し、**今回作成したときだけ `created_now=true`** (§9.2)
  - [x] `created_now` のときのみ `after_create` hook を実行
  - [x] 既存が非ディレクトリなら安全に fail (policy: fail)

### B. 安全不変条件 (§9.5) — 最重要

- [x] **Invariant 2**: workspace path を絶対化し、`workspace_root` を prefix に持つことを要求。root 外を reject
- [x] **Invariant 1**: agent 起動前に `cwd == workspace_path` を検証する API を提供 (起動自体は P3)
- [x] root 外/sanitize 回避 (`..` 等) を test で明示的に弾く

### C. hooks 実行 (§9.4)

- [x] 4 hooks (`after_create` / `before_run` / `after_run` / `before_remove`) を **`sh -lc <script>`**、cwd = workspace、timeout = `hooks.timeout_ms` で実行
- [x] hook の start / failure / timeout を slog に記録 (§9.4)
- [x] failure semantics (§9.4):
  - [x] `after_create` 失敗/timeout → workspace 作成を fatal に
  - [x] `before_run` 失敗/timeout → 当該 run attempt を fatal に
  - [x] `after_run` 失敗/timeout → log して無視
  - [x] `before_remove` 失敗/timeout → log して無視 (削除は続行)
- [x] 空文字 hook は no-op

### D. クリーンアップ (§9.2 notes / §9.4)

- [x] `Remove(ctx, issue)` → `before_remove` hook (存在時) → ディレクトリ削除
- [x] 成功 run では自動削除しない (§9.1: workspace は永続) — 削除は terminal issue のみ (呼び出しは P6)

### E. テスト (§17.2)

- [x] identifier→path が決定的、欠落ディレクトリ作成、既存再利用
- [x] 非ディレクトリ path の安全処理
- [x] `after_create` が新規作成時のみ発火
- [x] `before_run` 失敗が attempt を中断、`after_run`/`before_remove` 失敗は無視
- [x] sanitize と root containment が起動前に強制される (root 外 path を reject)

## Acceptance Criteria

- `wfconfig.Config` を渡すと per-issue workspace を冪等に用意し、4 hooks を契約通り実行できる
- §9.5 の 3 不変条件が test で強制される (特に root containment と sanitize)
- §17.2 の test 項目を pass、`go test ./orchestrator/workspace/` 緑、lint 緑

## Notes

- VCS/repo bootstrap は実装しない (§9.3: implementation-defined、hooks に委ねる)
- hook timeout は `context.WithTimeout` + `exec.CommandContext` で実装

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §9 (Workspace Management & Safety), §17.2
- [plans/04-phases.md#p2](../plans/04-phases.md)
