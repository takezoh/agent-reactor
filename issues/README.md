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

### P4 batch (agent launch の sandbox 配線) — M2 前半

| ID | タイトル | Phase | Status | Depends on |
|---|---|---|---|---|
| [015](015-p4a-agentlaunch-seam.md) | launch を `agentlaunch.Dispatcher` 経由に (direct mode) | P4a | Open | 013 (merged), P0b (merged) |
| [016](016-p4b-devcontainer-mode.md) | devcontainer モード + host↔container path 変換 | P4b | Open | 015 |

## 依存関係グラフ

```
  M1 (Done):  005─006─007 ─┐
              008─009─010 ─┼─ 011 ─┬─ 012 ─┬─ 013 (agent runner)
                           │       └─ 014  │
                           └──────────────→ M1 単線通電 ✅

  P4:  013 (merged) ──► 015 ──► 016
       agent runner     Dispatcher    devcontainer
       (P0b agentlaunch)  seam        + path 変換
       (direct mode)
```

- **P4a (015)** が P4 のルート。前提は **013**（agent runner、merged）+ **P0b**（`platform/agentlaunch`、merged）。launch を `Dispatcher.Wrap` 経由にするだけで挙動は direct のまま（回帰なし）
- **P4b (016)** は **015 に直列依存**。015 で入れた seam に `DevcontainerLauncher` を差し込み、container 内 launch・cwd の host↔container 変換・sock/mounts を配線
- **016 には設計判断**（devcontainer 設定の出どころ）があり、実装前に PR で確定する（推奨: roost `~/.roost/` config 再利用）
- P4 完了で **direct / devcontainer 両モード**で 1 issue end-to-end。続く **P5**（claude-app-server shim）で agent 切替に進み M2 完成

## 完了済み (archive)

完了 issue は [.archive/](.archive/) に移動（記録として保持）:

- **M0 / P0 batch** (構造分離): [001](.archive/001-p0a-physical-move.md) 物理移動 / [002](.archive/002-p0b-agentlaunch.md) agentlaunch / [003](.archive/003-p0c-codexclient.md) codexclient / [004](.archive/004-p0d-cmd-scaffolding.md) cmd 雛形
- **M1 / P1 batch** (loader→config→scheduler): [005](.archive/005-p1a-workflowfile.md) loader / [006](.archive/006-p1b-wfconfig.md) wfconfig / [007](.archive/007-p1c-preflight-stub-scheduler.md) preflight+stub loop
- **M1 / P2 batch** (tracker/workspace): [008](.archive/008-p2a-linear-tracker.md) linear adapter / [009](.archive/009-p2b-orchestrator-tracker.md) tracker wrapper / [010](.archive/010-p2c-workspace-manager.md) workspace manager
- **M1 / P3 batch** (scheduler core): [011](.archive/011-p3a-scheduler-state.md) state machine / [012](.archive/012-p3b-dispatch-tick.md) dispatch tick / [013](.archive/013-p3c-agent-runner.md) agent runner / [014](.archive/014-p3d-reconciliation.md) reconciliation

## 次の batch (P5 以降)

- P5: `claude-app-server` shim 実装（container 内で claude を codex protocol で喋らせる、要 016）
- P6 以降: continuation turn + stall + metrics / HTTP server / hot reload + linear_graphql / conformance test

詳細は [plans/04-phases.md](../plans/04-phases.md) / [plans/roadmap.md](../plans/roadmap.md) を参照。

## ライフサイクル

- 着手時に `Status: Open` → `Status: In Progress`
- PR open 時に PR 番号を Status 横に併記
- merge 後に `Status: Done`、関連 PR と完了日を記録
- 別 issue で blocked になったら `Status: Blocked` + 理由
