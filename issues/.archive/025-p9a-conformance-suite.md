# 025: orchestrator — SPEC §17 conformance test 群 + conformance 表

- **Phase**: P9a ([plans/04-phases.md#p9-conformance-tests--docs](../plans/04-phases.md))
- **Status**: Done (2026-05-21)
- **Depends on**: 005–023（merged; 各 Phase が §17.x 系 test を既に持つ）。026 とは独立（並行可）
- **Blocks**: M4 完成

## Background

SPEC §17 (Conformance) は実装が満たすべきチェック項目を §17.1–§17.7 (Core) + §17.8 (Real Integration Profile) に分けて列挙する。各 Phase の issue は既に対応する §17.x 系 test を書いている（[plans/05-conformance.md](../plans/05-conformance.md) の対応表参照）が、**(1) §17 チェック項目に対する網羅監査**、**(2) canonical な命名規約 `TestSPEC_<section>_<name>` での conformance マーカー整備**、**(3) §17.8 実 Linear API プロファイル**、**(4) SPEC § ↔ test の対応表 `docs/orchestrator/symphony-conformance.md`** は未整備。本 issue で M4 (SPEC §17 pass) を満たす。

**方針（重複を作らない）**: 既存の per-phase test は維持し、§17 チェック項目ごとに「既存 test があれば対応表で cite」「無ければ `TestSPEC_<section>_<name>` を補充」する。既存 test の機械的 rename はしない（散在 churn を避ける）。canonical マーカーは public API を黒箱で叩き、SPEC § を test doc コメントに明記する。

## §17.x → 対象 package → 既存 test → 補充する canonical マーカー

| §17.x | 領域 | 主な package | 既存 per-phase test (issue) | 代表 canonical test（補充 or alias） |
|---|---|---|---|---|
| §17.1 | workflow/config/preflight/reload | `workflowfile` `wfconfig` `scheduler` | 005/006/007/023 | `TestSPEC_17_1_WorkflowFilePathPrecedence` / `_StrictTemplateUnknownVarErrors` / `_LastKnownGoodOnInvalidReload` |
| §17.2 | workspace 管理 & safety | `workspace` | 010 | `TestSPEC_17_2_WorkspaceKeySanitized` / `_CwdEqualsWorkspaceRoot` |
| §17.3 | tracker / Linear 正規化 | `orchestrator/tracker` `platform/tracker/linear` | 008/009 | `TestSPEC_17_3_LinearProjectFilterUsesSlugId` / `_PriorityIntOnly` / `_BlockersFromBlocksInverted` |
| §17.4 | scheduler state/dispatch/retry/reconcile | `scheduler` | 011/012/014 | `TestSPEC_17_4_RetryBackoffCapHonored` / `_PerStateConcurrency` / `_ContinuationFixed1s` |
| §17.5 | agent runner/session/event/continuation/metrics/shim | `agent` `cmd/claude-app-server` `streamjson` `metrics` | 013/017/018/019/020/021 | `TestSPEC_17_5_SessionIDFormat` / `_AbsoluteTokenNoDoubleCount` / `_AgentSwitchEventParity` |
| §17.6 | HTTP server response shape | `httpserver` | 022 | `TestSPEC_17_6_StateShape` / `_MethodNotAllowedEnvelope` |
| §17.7 | CLI lifecycle / safety invariant / secret 非ログ / sandbox posture | `cmd/orchestrator` `agent` `platform/logger` | 007 | `TestSPEC_17_7_SecretNeverLogged` / `_RootPrefixCheck` / `_GracefulShutdownExitsZero` |
| §17.8 | Real Integration Profile（実 Linear API） | 新規 integration | — | `TestSPEC_17_8_RealLinearFetchCandidates`（`LINEAR_API_KEY` gated） |

## Tasks

### A. §17.1–§17.7 網羅監査 + canonical マーカー補充

- [ ] SPEC §17.1–§17.7 の各チェック項目を列挙し、既存 test との対応を突き合わせる（gap 表を作る）
- [ ] gap のある項目に `TestSPEC_<section>_<name>` を該当 package の `conformance_test.go` に追加（既存 test と重複しない範囲。内部アクセスが要る項目は同 package 内、黒箱で足りる項目は `_test` package）
- [ ] 各 canonical test の doc コメントに対応 SPEC § を明記（例: `// SPEC §17.4 — continuation retry uses a fixed 1s delay`）
- [ ] 命名規約は [plans/05-conformance.md](../plans/05-conformance.md#conformance-test-の整備-p9) に従う（`TestSPEC_17_4_RetryBackoffCapHonored` 形式）

### B. §17.8 Real Integration Profile

- [ ] `LINEAR_API_KEY`（+ 必要なら `LINEAR_PROJECT`）が**未設定なら `t.Skip`**、設定時のみ実 Linear API を叩く統合 test（`fetch_candidate_issues` / `fetch_issues_by_states` / `fetch_issue_states_by_ids` の 3 操作を read-only で検証）
- [ ] 既定の `go test ./...`（CI sandbox）では skip される（env 無し）。secret はログに出さない（§15.3）
- [ ] 起動オプション or build tag の要否を判断（env gate を第一候補、build tag は補助）

### C. conformance 表 `docs/orchestrator/symphony-conformance.md`

- [ ] SPEC § ↔ test name の対応表を作成（§17.x 行ごとに canonical test + 関連 per-phase test を列挙）
- [ ] [plans/05-conformance.md](../plans/05-conformance.md) の「厳守する項目」「逸脱 / 拡張」「documented posture」を移植/要約（SPEC リリース時に維持する想定の正本）
- [ ] **逸脱の最新化**: §10.5 `linear_graphql` は mcpproxy ではなく **codex native `item/tool/call`**、かつ advertise は pinned codex 0.133.0 で blocked（[024](024-p8b-linear-graphql-tool.md) §B）である旨を posture として明記
- [ ] §13.7 HTTP「必須化」、§3.3 devcontainer-default、§10.1 stdio shim 方式 等の既存 deviation を反映

### D. テスト / CI

- [ ] `go test ./...` 緑（canonical マーカー含む。§17.8 は env 無しで skip）
- [ ] `make vet` / `make lint` 緑
- [ ] conformance 表のリンク（test name）が実在 test と一致することを確認（ずれ検出は将来 CI 化候補）

## Acceptance Criteria

- SPEC §17.1–§17.7 の全チェック項目に対応する test が存在（既存 cite または canonical 補充）し、`docs/orchestrator/symphony-conformance.md` で SPEC § ↔ test が一覧できる
- §17.8 は `LINEAR_API_KEY` 設定時のみ実行、未設定で skip。secret 非ログ
- conformance 表が plans/05 と整合し、§10.5 等の最新 deviation を反映
- `go test ./...`（既定）/ `make vet` / `make lint` 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §17 (Conformance: §17.1–§17.7 Core, §17.8 Real Integration Profile)
- [plans/05-conformance.md](../plans/05-conformance.md)（厳守/逸脱/posture/命名規約）、[plans/04-phases.md#p9](../plans/04-phases.md)
- 既存 §17.x 系 test: [005](.archive/005-p1a-workflowfile.md)/[006](.archive/006-p1b-wfconfig.md)/[007](.archive/007-p1c-preflight-stub-scheduler.md)（§17.1/§17.7）、[008](.archive/008-p2a-linear-tracker.md)/[009](.archive/009-p2b-orchestrator-tracker.md)（§17.3）、[010](.archive/010-p2c-workspace-manager.md)（§17.2）、[011](.archive/011-p3a-scheduler-state.md)/[012](.archive/012-p3b-dispatch-tick.md)/[014](.archive/014-p3d-reconciliation.md)（§17.4）、[013](.archive/013-p3c-agent-runner.md)/[017](.archive/017-p5a-claude-streamjson.md)/[018](.archive/018-p5b-claude-app-server.md)/[019](.archive/019-p5c-agent-switch-conformance.md)/[020](.archive/020-p6a-continuation-loop.md)/[021](.archive/021-p6b-metrics.md)（§17.5）、[022](.archive/022-p7-http-server.md)（§17.6）
