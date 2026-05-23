# Roadmap

Symphony SPEC 実装の全体ロードマップと進捗。設計の詳細は [04-phases.md](04-phases.md)、
個別の作業単位は [issues/](../issues/) を参照。

更新日: 2026-05-21

## 現在地

**M0–M4 完了。SPEC §1–§17 を満たす実装が一通り揃った。唯一の残件は P8b の tool advertise（外部要因で blocked）。**
M2 で claude-app-server shim が完成し `codex.command` での agent 切替を実証。M3 で continuation loop (020)、
metrics+stall (021)、observability HTTP server (022)、WORKFLOW.md hot reload (023) を実装・archive 済み。
M4 (P9) で SPEC §17 conformance test 群（canonical `TestSPEC_*` マーカー + §17.8 実 Linear profile）と
`docs/orchestrator/symphony-conformance.md`（SPEC §17 ↔ test 対応表 + posture）、orchestrator サービスの
位置付け doc（AGENTS.md / ARCHITECTURE.md）を整備（025/026、archive 済み）。
**唯一の残件は P8b `linear_graphql` (024) の tool advertise**: handler + wiring は実装済だが、pinned codex 0.133.0 で
`DynamicToolSpec` が schema 上 orphan（request からの `$ref` 参照ゼロ）のため宣言できず、実機 codex から到達不能。
codex schema bump が入れば forward-compatible な handler がそのまま機能する（外部要因待ち）。

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
| P4a | launch を `agentlaunch.Dispatcher` 経由に (direct mode) | ✅ Done | [015](../issues/.archive/015-p4a-agentlaunch-seam.md) |
| P4b | devcontainer モード + host↔container path 変換 | ✅ Done | [016](../issues/.archive/016-p4b-devcontainer-mode.md) |
| P5a | `claude -p` stream-json reader (`platform/lib/claude/streamjson`) | ✅ Done | [017](../issues/.archive/017-p5a-claude-streamjson.md) |
| P5b | `claude-app-server` shim — codex stdio + `claude -p` 中継 | ✅ Done | [018](../issues/.archive/018-p5b-claude-app-server.md) |
| P5c | token usage + approval posture + agent 切替 end-to-end | ✅ Done | [019](../issues/.archive/019-p5c-agent-switch-conformance.md) |
| P6a | continuation multi-turn loop + worker-exit→state | ✅ Done | [020](../issues/.archive/020-p6a-continuation-loop.md) |
| P6b | token/runtime 集計 + codex activity (stall) tracking | ✅ Done | [021](../issues/.archive/021-p6b-metrics.md) |
| P7 | observability HTTP server (`/`, `/api/v1/*`) | ✅ Done | [022](../issues/.archive/022-p7-http-server.md) |
| P8a | WORKFLOW.md hot reload (§6.2) | ✅ Done | [023](../issues/.archive/023-p8a-hot-reload.md) |
| P8b | `linear_graphql` agent tool (native `item/tool/call`, §10.5) | ⚠ Partial — advertise が schema 制約で blocked | [024](../issues/024-p8b-linear-graphql-tool.md) |
| P9a | SPEC §17 conformance test 群 + conformance 表 | ✅ Done | [025](../issues/.archive/025-p9a-conformance-suite.md) |
| P9b | orchestrator サービスの位置付け doc (AGENTS.md/ARCHITECTURE.md) | ✅ Done | [026](../issues/.archive/026-p9b-positioning-docs.md) |

## マイルストーン

| | Phase | 意義 | 状態 |
|---|---|---|---|
| **M0** 構造分離完了 | P0a–P0d | 後続の物理基盤確立 | ✅ Done |
| **M1** 最小単線通電 | P1–P3 | 1 issue → codex app-server で 1 turn | ✅ Done |
| **M2** 多 agent 対応 | P4–P5 | sandbox 配線 + claude / codex 切替 | ✅ Done |
| **M3** SPEC 機能完成 | P6–P8 | SPEC §1–§16 を満たす | ✅ 実質完了（P8b advertise のみ codex schema 制約で blocked）|
| **M4** conformance 確認 | P9 | SPEC §17 test pass + 位置付け doc | ✅ Done |

## P0 で確立した基盤 (現状)

- **三層境界**: `platform/` (共有基盤) ↛ `client/`/`orchestrator/`、`client/` ↛ `orchestrator/` を depguard で実効化
- **agentlaunch**: `platform/agentlaunch/` の `Dispatcher` を import すれば sandbox 配線済みで agent を起動可能。client 固有概念 (FrameID/SandboxOverride) は adapter で遮断
- **codexclient**: transport 非依存の JSON-RPC framing (`Conn`)。ws (roost) と stdio (shim/orchestrator) の両 transport。server helper を shim に提供
- **codexschema**: codex-cli 0.133.0 で pin、CI で drift 検出
- **3 バイナリ**: `roost` / `orchestrator`（poll/dispatch/reconcile/HTTP 稼働）/ `claude-app-server`（codex protocol shim 稼働）が同一 module から build

## issue の置き場

- [issues/](../issues/) — 進行中・未着手の作業単位
- [issues/.archive/](../issues/.archive/) — 完了済み issue (記録として保持)
