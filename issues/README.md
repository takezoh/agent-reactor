# Issues

[plans/](../plans/) で定めた計画を実行可能な単位に分解した issue 群。

## 表記規約

各 issue ファイルは以下のセクションを持つ:

```markdown
# <ID>: <タイトル>

- **Phase**: P0a / P0b / ... ([plans/04-phases.md](../plans/04-phases.md))
- **Status**: Open / In Progress / Blocked / Done
- **Depends on**: 他 issue ID または PR
- **Blocks**: 他 issue ID

## Background
## Tasks
## Acceptance Criteria
## References
```

**SPEC 参照は必須**: 全 issue は `References` に該当する [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) のセクション (§番号付き) を含める。SPEC が source of truth であり、実装判断はここに帰着させる。直接の SPEC 要件が無い基盤作業でも、それが実現する SPEC コンポーネント (例: §3 System Overview) を明示する。

全体の進捗は [plans/roadmap.md](../plans/roadmap.md) を参照。

## 直近 issue 一覧

### P1 batch (loader → config → scheduler)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [005](005-p1a-workflowfile.md) | WORKFLOW.md loader (front matter + body 分離) | P1a | Done | P0 (merged) |
| [006](006-p1b-wfconfig.md) | wfconfig — typed config view (default/$VAR/~/検証) | P1b | Done | 005 |
| [007](007-p1c-preflight-stub-scheduler.md) | dispatch preflight + stub scheduler loop | P1c | Open | 006 |

### P2 batch (tracker / workspace)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [008](008-p2a-linear-tracker.md) | `platform/tracker` Linear GraphQL adapter | P2a | Open | P0 (merged) |
| [009](009-p2b-orchestrator-tracker.md) | `orchestrator/tracker` config wrapper | P2b | Open | 008, 006 |
| [010](010-p2c-workspace-manager.md) | `orchestrator/workspace` manager + hooks + safety | P2c | Done | 006 |

## 依存関係グラフ

```
  P1 (直列):  005 ── 006 ── 007
              loader wfconfig preflight+stub loop
                       │
                       ├──────────────┐  (006 完了済み)
  P2:                  ▼              ▼
              008 ──── 009          010
              linear   tracker      workspace
              adapter  wrapper      manager
```

- **P1** は直列。各段が前段の出力を入力に取る (loader→config→preflight)
- **P2** は 006 (完了) を前提に並行可能。**008** (純 HTTP/GraphQL client) と **010** (workspace、tracker 非依存) は即着手可、**009** は 008 を待つ
- P2 は P1c (007) に依存しない — tracker/workspace は scheduler が P3 で使うライブラリで、stub loop とは独立

## 完了済み (archive)

P0 batch (M0: 構造分離) は完了し [.archive/](.archive/) に移動:

- [001](.archive/001-p0a-physical-move.md) P0a 物理移動 / [002](.archive/002-p0b-agentlaunch.md) P0b agentlaunch / [003](.archive/003-p0c-codexclient.md) P0c codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) P0d cmd 雛形

## 次の batch (P3 以降)

- P3: scheduler core (poll/dispatch/retry/reconcile) + 生 codex 単線 — 007 + 009 + 010 が前提
- P4: agent 起動を codexclient 経由に + sandbox 配線

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
