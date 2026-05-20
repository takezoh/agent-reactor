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

## 直近 issue 一覧 (P1 batch)

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [005](005-p1a-workflowfile.md) | WORKFLOW.md loader (front matter + body 分離) | P1a | Open | P0 (merged) |
| [006](006-p1b-wfconfig.md) | wfconfig — typed config view (default/$VAR/~/検証) | P1b | Open | 005 |
| [007](007-p1c-preflight-stub-scheduler.md) | dispatch preflight + stub scheduler loop | P1c | Open | 006 |

## 依存関係グラフ

```
  005 (P1a) ── 006 (P1b) ── 007 (P1c)
  loader       wfconfig      preflight + stub loop
```

- P0 と異なり P1 は **直列**。各段が前段の出力を入力に取るため
- **005** loader が front matter map を返す → **006** が typed config に解決 → **007** が config を preflight 検証し loop に配線
- 005 → 006 → 007 を 3 PR で順に積むか、規模次第で 005+006 を 1 PR にまとめ 007 を別 PR でも可

## 完了済み (archive)

P0 batch (M0: 構造分離) は完了し [.archive/](.archive/) に移動:

- [001](.archive/001-p0a-physical-move.md) P0a 物理移動 / [002](.archive/002-p0b-agentlaunch.md) P0b agentlaunch / [003](.archive/003-p0c-codexclient.md) P0c codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) P0d cmd 雛形

## 次の batch (P2 以降)

- P2: Linear adapter (`platform/tracker/linear/`) + workspace manager + 4 hooks
- P3: scheduler core (poll/dispatch/retry/reconcile) + 生 codex 単線

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
