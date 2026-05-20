# Roadmap

Symphony SPEC 実装の全体ロードマップと進捗。設計の詳細は [04-phases.md](04-phases.md)、
個別の作業単位は [issues/](../issues/) を参照。

更新日: 2026-05-20

## 現在地

**M0 (構造分離)・M1 (最小単線通電) ともに完了。** P1 (005–007)・P2 (008–010)・P3 (011–014) を全て実装・
レビュー・merge 済み（007/011–014 のレビュー修正は `3c659e0`）。1 issue → workspace → codex app-server で
1 turn が direct mode で通電する。次は **M2 / P4 (agent 起動の sandbox 配線)** を 2 issue (015–016) に分解して起票。
015 で launch を `agentlaunch.Dispatcher` 経由にし、016 で devcontainer モードを配線する。

## Phase 進捗

| Phase | 内容 | 状態 | issue |
|---|---|---|---|
| P0a | 物理移動 (`platform/`/`client/`/`cmd/`) | ✅ Done | [001](../issues/.archive/001-p0a-physical-move.md) |
| P0b | `agentlaunch/` を `platform/` へ抽出 | ✅ Done | [002](../issues/.archive/002-p0b-agentlaunch.md) |
| P0c | `codexclient/`+`codexschema/` 抽出・schema pin | ✅ Done | [003](../issues/.archive/003-p0c-codexclient.md) |
| P0d | `cmd/orchestrator`/`cmd/claude-app-server` 雛形 | ✅ Done | [004](../issues/.archive/004-p0d-cmd-scaffolding.md) |
| P1a | WORKFLOW.md loader | ✅ Done | [005](../issues/.archive/005-p1a-workflowfile.md) |
| P1b | wfconfig typed config | ✅ Done | [006](../issues/.archive/006-p1b-wfconfig.md) |
| P1c | preflight + stub scheduler loop | ✅ Done | [007](../issues/.archive/007-p1c-preflight-stub-scheduler.md) |
| P2a | `platform/tracker` Linear adapter | ✅ Done | [008](../issues/.archive/008-p2a-linear-tracker.md) |
| P2b | `orchestrator/tracker` config wrapper | ✅ Done | [009](../issues/.archive/009-p2b-orchestrator-tracker.md) |
| P2c | `orchestrator/workspace` manager + hooks | ✅ Done | [010](../issues/.archive/010-p2c-workspace-manager.md) |
| P3a | scheduler state machine + runtime state (§7) | ✅ Done | [011](../issues/.archive/011-p3a-scheduler-state.md) |
| P3b | poll/dispatch tick — eligibility/sort/concurrency/retry (§8) | ✅ Done | [012](../issues/.archive/012-p3b-dispatch-tick.md) |
| P3c | agent runner — prompt + codex 1 turn + events (§10/§16.5) | ✅ Done | [013](../issues/.archive/013-p3c-agent-runner.md) |
| P3d | reconciliation + startup cleanup (§8.5/§8.6) | ✅ Done | [014](../issues/.archive/014-p3d-reconciliation.md) |
| P4a | launch を `agentlaunch.Dispatcher` 経由に (direct mode) | ▶ Next | [015](../issues/015-p4a-agentlaunch-seam.md) |
| P4b | devcontainer モード + host↔container path 変換 | ⬜ Open | [016](../issues/016-p4b-devcontainer-mode.md) |
| P5 | `claude-app-server` shim 実装 | ⬜ Pending | — |
| P6 | continuation turn + stall + reconciliation + metrics | ⬜ Pending | — |
| P7 | HTTP server (`/`, `/api/v1/*`) | ⬜ Pending | — |
| P8 | WORKFLOW.md hot reload + `linear_graphql` tool | ⬜ Pending | — |
| P9 | SPEC §17 conformance test + loki retirement | ⬜ Pending | — |

## マイルストーン

| | Phase | 意義 | 状態 |
|---|---|---|---|
| **M0** 構造分離完了 | P0a–P0d | 後続の物理基盤確立 | ✅ Done |
| **M1** 最小単線通電 | P1–P3 | 1 issue → codex app-server で 1 turn | ✅ Done |
| **M2** 多 agent 対応 | P4–P5 | sandbox 配線 + claude / codex 切替 | ▶ 進行中 |
| **M3** SPEC 機能完成 | P6–P8 | SPEC §1–§16 を満たす | ⬜ |
| **M4** conformance 確認 | P9 | SPEC §17 test pass + loki retire | ⬜ |

## P0 で確立した基盤 (現状)

- **三層境界**: `platform/` (共有基盤) ↛ `client/`/`orchestrator/`、`client/` ↛ `orchestrator/` を depguard で実効化
- **agentlaunch**: `platform/agentlaunch/` の `Dispatcher` を import すれば sandbox 配線済みで agent を起動可能。client 固有概念 (FrameID/SandboxOverride) は adapter で遮断
- **codexclient**: transport 非依存の JSON-RPC framing (`Conn`)。ws (roost) と stdio (shim/orchestrator) の両 transport。server helper を shim に提供
- **codexschema**: codex-cli 0.128.0 で pin、CI で drift 検出
- **3 バイナリ**: `roost` / `orchestrator` (stub) / `claude-app-server` (stub) が同一 module から build

## issue の置き場

- [issues/](../issues/) — 進行中・未着手の作業単位
- [issues/.archive/](../issues/.archive/) — 完了済み issue (記録として保持)
