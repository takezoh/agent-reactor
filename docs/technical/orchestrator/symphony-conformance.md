# Symphony SPEC Conformance

Symphony SPEC v1 Draft への conformance の正本ドキュメント。  
`plans/05-conformance.md` が working doc であり、本ファイルは M4 時点でのスナップショットとして機能する。

---

## SPEC §17 ↔ テスト対応表

各 §17.x チェック項目に対し、canonical `TestSPEC_*` マーカーまたは per-phase テストを対応させる。  
命名規約: `TestSPEC_<section>_<short_name>` (詳細は `plans/05-conformance.md#conformance-test-の整備-p9`)。

### §17.1 Workflow and Config Parsing

| チェック項目 | テスト |
|---|---|
| Workflow file path precedence (explicit > cwd default) | `TestSPEC_17_1_WorkflowFilePathPrecedence` (`cmd/orchestrator`) |
| Workflow file changes trigger re-read without restart | `TestWatchWorkflowSignalsOnWrite`, `TestWatchWorkflowSignalsOnWrite_Coalesces` (`scheduler`) |
| Invalid reload keeps last-known-good config, emits operator-visible error | `TestSPEC_17_1_LastKnownGoodOnInvalidReload` (`scheduler`) / `TestDispatchGatingOnBadReload` / `TestDegradedWarnEmittedOnce` |
| Missing WORKFLOW.md returns typed error | `TestRunMissingWorkflow` (`cmd/orchestrator`) / `TestLoad_MissingFile` (`workflowfile`) |
| Invalid YAML front matter returns typed error | `TestLoad_InvalidYAML` (`workflowfile`) |
| Front matter non-map returns typed error | `TestLoad_FrontMatterNotMap` (`workflowfile`) |
| Config defaults apply for OPTIONAL values | `TestResolve_AppliesAllDefaults` (`wfconfig`) |
| `tracker.kind` validation enforces supported kind | `TestNew_UnsupportedKind` (`orchestrator/tracker`) / `TestPreflightValid` (`scheduler`) |
| `tracker.api_key` works including `$VAR` indirection | `TestResolve_VarExpansion_APIKey` (`wfconfig`) |
| `$VAR` resolution for tracker API key and path values | `TestResolve_VarExpansion_*` (`wfconfig`) |
| `~` path expansion | `TestResolve_TildeExpansion_WorkspaceRoot` (`wfconfig`) |
| `codex.command` preserved as shell command string | `TestResolve_CodexCommandPreserved` (`wfconfig`) |
| Per-state concurrency override map normalizes state names | `TestResolve_PerStateConcurrencyNormalized` (`wfconfig`) |
| Prompt template renders `issue` and `attempt` | `TestRender_interpolatesIssue`, `TestRender_interpolatesAttempt` (`prompt`) |
| Prompt rendering fails on unknown variables (strict mode) | `TestSPEC_17_1_StrictTemplateUnknownVarErrors` (`prompt`) / `TestRender_unknownVariableErrors` |

### §17.2 Workspace Manager and Safety

| チェック項目 | テスト |
|---|---|
| Deterministic workspace path per issue identifier | `TestPath_Deterministic` (`workspace`) |
| Missing workspace directory is created | `TestEnsure_CreatesDirectory` (`workspace`) |
| Existing workspace directory is reused | `TestEnsure_ReusesExistingDirectory` (`workspace`) |
| Existing non-directory path handled safely | `TestEnsure_NonDirectoryFails` (`workspace`) |
| `after_create` hook runs only on new workspace creation | `TestEnsure_AfterCreate_NewOnly` (`workspace`) |
| `before_run` hook runs before each attempt; failure aborts | `TestBeforeRun_FailureReturnsError`, `TestBeforeRun_Timeout` (`workspace`) |
| `after_run` hook runs after each attempt; failure ignored | `TestAfterRun_FailureIgnored`, `TestAfterRun_TimeoutIgnored` (`workspace`) |
| `before_remove` hook runs on cleanup; failure ignored | `TestRemove_BeforeRemoveFailureIgnored` (`workspace`) |
| Workspace path sanitization and root containment enforced before agent launch | `TestSPEC_17_2_WorkspaceKeySanitized` (`workspace`) / `TestPath_Sanitize_*` |
| Agent launch uses per-issue workspace cwd; out-of-root rejected | `TestSPEC_17_2_CwdEqualsWorkspaceRoot` (`workspace`) / `TestVerifyCWD_*` |

### §17.3 Issue Tracker Client

| チェック項目 | テスト |
|---|---|
| Candidate issue fetch uses active states and project slug | `TestSPEC_17_3_CandidateFetchUsesActiveStates` (`platform/tracker/linear`) |
| Linear query uses slugId project filter field | `TestSPEC_17_3_LinearProjectFilterUsesSlugId` |
| Empty `fetch_issues_by_states([])` returns empty without API call | `TestSPEC_17_3_FetchIssuesByStates_EmptyStates_NoAPICall` |
| Pagination preserves order across multiple pages | `TestSPEC_17_3_PaginationPreservesOrder` |
| Blockers normalized from inverse relations of type `blocks` | `TestSPEC_17_3_BlockedByFromBlocksInverseRelation` |
| Labels normalized to lowercase | `TestSPEC_17_3_LabelsLowercase` |
| Issue state refresh by ID returns minimal normalized issues | `TestSPEC_17_3_FetchIssueStatesByIDsUsesIDType` |
| Error mapping for request errors, non-200, GraphQL errors, malformed payloads | `TestErrorMapping_*` (`platform/tracker/linear`) |

### §17.4 Orchestrator Dispatch, Reconciliation, and Retry

| チェック項目 | テスト |
|---|---|
| Dispatch sort order: priority then oldest creation time | `TestSortCandidates` (`scheduler`) |
| `Todo` issue with non-terminal blockers is not eligible | `TestEligible_RequiredFields`, `TestEligible_BlockerRule` (`scheduler`) |
| `Todo` issue with terminal blockers is eligible | `TestEligible_BlockerRule` (`scheduler`) |
| Active-state issue refresh updates running entry state | `TestReconcileRefresh_ActiveUpdatesSnapshot` (`scheduler`) |
| Non-active state stops running agent without workspace cleanup | `TestReconcileRefresh_IntermediateKillsNoWorkspaceRemove` (`scheduler`) |
| Terminal state stops running agent and cleans workspace | `TestReconcileRefresh_TerminalKillsAndCleansWorkspace` (`scheduler`) |
| Normal worker exit schedules short continuation retry (attempt 1) | `TestHandleWorkerExit_Normal_ReleasesSlotAndSchedulesContinuation` (`scheduler`) |
| Abnormal worker exit increments retries with exponential backoff | `TestHandleWorkerExit_Abnormal_EnqueuesBackoffRetry` (`scheduler`) |
| Retry backoff capped by `agent.max_retry_backoff_ms` | `TestSPEC_17_4_RetryBackoffCapHonored` (`scheduler`) / `TestBackoffDelay` |
| Per-state concurrency cap applied independently from global | `TestSPEC_17_4_PerStateConcurrency` (`scheduler`) / `TestAvailablePerStateSlots` |
| Continuation uses fixed 1s delay | `TestSPEC_17_4_ContinuationFixed1s` (`scheduler`) / `TestContinuationDelay` |
| Stall detection kills stalled sessions and schedules retry | `TestReconcileStall_KillsAndEnqueuesRetry` (`scheduler`) |

### §17.5 Coding-Agent App-Server Client

| チェック項目 | テスト |
|---|---|
| Launch command uses workspace cwd and invokes `bash -lc <codex.command>` | `TestSpawn_sessionStartedAndTurnCompleted` (`agent`) |
| Thread/turn identities extracted and used to emit `session_started` | `TestSPEC_17_5_SessionIDFormat` (`agent`) / `TestShim_SessionID` (`cmd/claude-app-server`) |
| Request/response read timeout enforced | `TestSpawn_turnTimeoutKillsAndFails` (`agent`) |
| Unsupported dynamic tool calls rejected without stalling | `TestHandleToolCall_unknownTool_replyError` (`agent`) |
| Usage and rate-limit telemetry extracted | `TestTurnHandler_UsageUsesTotalIgnoresLastPayload`, `TestTurnHandler_RateLimitReported` (`agent`) |
| Token absolute totals used; same absolute value not double-counted | `TestSPEC_17_5_AbsoluteTokenNoDoubleCount` (`platform/metrics`) / `TestAccumulator_SingleThread_NoDoubleCount` |
| Agent-switch event parity: shim emits §10.4 protocol method names | `TestSPEC_17_5_AgentSwitchEventParity` (`cmd/claude-app-server`) / `TestShim_ConformanceEventOrder` |
| `thread/start` sends `approvalPolicy`, `sandbox`, `serviceName` per §10.2 | `TestSPEC_17_5_ThreadStartSendsApprovalPolicy`, `TestSPEC_17_5_ThreadStartSendsSandboxMode`, `TestSPEC_17_5_ThreadStartSendsServiceName` (`agent`) |
| `turn/start` sends `approvalPolicy`, `sandboxPolicy` per §10.2 | `TestSPEC_17_5_TurnStartSendsApprovalPolicy`, `TestSPEC_17_5_TurnStartSendsSandboxPolicy` (`agent`) |
| Empty policy config omits optional fields from wire | `TestSPEC_17_5_EmptyPolicyFieldsOmitted` (`agent`) |

### §17.6 Observability

| チェック項目 | テスト |
|---|---|
| `/api/v1/state` response contains required top-level fields | `TestSPEC_17_6_StateShape` (`orchestrator/httpserver`) / `TestStateEndpoint_EmptySnapshot` |
| 405 Method Not Allowed uses standard error envelope | `TestSPEC_17_6_MethodNotAllowedEnvelope` (`orchestrator/httpserver`) / `TestMethodNotAllowed_405` |
| Snapshot timeout returns 503 with `snapshot_timeout` code (§13.3 RECOMMENDED) | `TestSPEC_17_6_SnapshotTimeout` (`orchestrator/httpserver`) / `TestSnapshotCtx_Timeout` (`scheduler`) |
| Orchestrator unavailable returns 503 with `orchestrator_unavailable` code (§13.3 RECOMMENDED) | `TestSPEC_17_6_OrchestratorUnavailable` (`orchestrator/httpserver`) / `TestScheduler_SnapshotCtx_Unavailable` (`scheduler`) |
| Logging sink failures do not crash orchestration | `TestRunContinuesAfterTickPreflightFailure` (`cmd/orchestrator`) |

### §17.7 CLI and Host Lifecycle

| チェック項目 | テスト |
|---|---|
| CLI accepts positional `--workflow` argument | `TestSPEC_17_1_WorkflowFilePathPrecedence` (`cmd/orchestrator`) |
| CLI uses `./WORKFLOW.md` when no workflow path provided | `TestSPEC_17_1_WorkflowFilePathPrecedence` (cwd sub-test) |
| CLI errors on nonexistent explicit workflow path | `TestRunMissingWorkflow` (`cmd/orchestrator`) |
| CLI exits 0 on normal shutdown | `TestSPEC_17_7_GracefulShutdownExitsZero` (`cmd/orchestrator`) / `TestRunGracefulShutdown` |
| CLI exits nonzero on startup failure | `TestRunPreflightFailure`, `TestRunConfigResolveFailure` (`cmd/orchestrator`) |
| Secret (`tracker.api_key`) never appears in logs or stderr | `TestSPEC_17_7_SecretNeverLogged` (`cmd/orchestrator`) |
| Root prefix containment invariant enforced before agent launch | `TestSPEC_17_7_RootPrefixCheck` (`workspace`) / `TestPath_EscapeRoot_*` |

### §17.8 Real Integration Profile

| チェック項目 | テスト |
|---|---|
| `FetchCandidateIssues` with real API key succeeds | `TestSPEC_17_8_RealLinearFetchCandidates` (`platform/tracker/linear`) — env-gated |
| `FetchIssuesByStates` with real API succeeds | 同上 (3-op chain 内) |
| `FetchIssueStatesByIDs` with real API succeeds | 同上 (3-op chain 内) |
| Skipped when credentials absent; not silently passed | env 未設定で `t.Skip` → `--- SKIP` として報告 |

**実行方法** (§17.8):
```sh
LINEAR_API_KEY=<key> LINEAR_PROJECT_SLUG=<slug> \
  go test -run TestSPEC_17_8 -v ./platform/tracker/linear/
```

オプション env: `LINEAR_TRACKER_ENDPOINT`（既定: `https://api.linear.app/graphql`）、`LINEAR_ACTIVE_STATES`（カンマ区切り、既定: `Todo,In Progress`）。

---

## SPEC §3.1 Component ↔ Go package 対応

詳細は [`plans/05-conformance.md#SPEC-用語と実装名の対応`](../../../plans/05-conformance.md) が正本。以下は要約。

| SPEC §3.1 Component | Go package |
|---|---|
| Workflow Loader (§3.1.1) | `orchestrator/workflowfile/` |
| Config Layer (§3.1.2) | `orchestrator/wfconfig/` |
| Issue Tracker Client (§3.1.3) | `orchestrator/tracker/` (→ `platform/tracker/linear/`) |
| Orchestrator (§3.1.4) — SPEC の scheduler 相当 | `orchestrator/scheduler/` |
| Workspace Manager (§3.1.5) | `orchestrator/workspace/` |
| Agent Runner (§3.1.6) | `orchestrator/agent/` (→ `platform/agent/codexclient/`) |
| Status Surface (§3.1.7) | `orchestrator/httpserver/` |
| Logging (§3.1.8) | `platform/logger/` |

**注**: SPEC §3.1.4 コンポーネント名 "Orchestrator" はサービス全体名と衝突するため、実装では `orchestrator/scheduler/` に rename。`plans/02-layout.md#naming` 参照。

---

## 厳守する項目 (抜粋)

完全な一覧は `plans/05-conformance.md#厳守する項目` を参照。代表的な厳守項目:

- **§4.2**: session_id = `<thread_id>-<turn_id>`、workspace key sanitize 正規表現 `[A-Za-z0-9._-]`
- **§8.4**: continuation retry は固定 1s、失敗 retry は `min(10000×2^(n-1), max)` ms
- **§11.2**: Linear slugId フィルタ、ID 型 `[ID!]`、pagination 50/page、timeout 30s
- **§11.3**: labels lowercase、blockers from `blocks` 反転、priority int-only、ISO-8601
- **§13.5**: absolute thread totals 優先、delta フォールバック禁止
- **§15.3**: `tracker.api_key` 等の secret はログ出力禁止（存在チェックのみ可）

---

## 逸脱 / 拡張する項目

完全な一覧は `plans/05-conformance.md#逸脱--拡張する項目` を参照。主要な逸脱:

| SPEC § | SPEC | 我々の選択 |
|---|---|---|
| §10 | Codex app-server 専用 | `codex.command` 経由の stdio shim で複数 agent 対応 (`claude-app-server`) |
| §10.5 `linear_graphql` | OPTIONAL extension | codex native `item/tool/call` で実装; advertise は pinned codex 0.133.0 で blocked (→ [issues/024](../../../issues/024-p8b-linear-graphql-tool.md) §B) |
| §13.7 | HTTP server は OPTIONAL | **必須として実装** — orchestrator は TUI を持たない |
| §3.3 | sandbox は impl-defined | devcontainer mode をデフォルト推奨 |
| §9.3 | workspace population は impl-defined | `after_create` hook で `git worktree add` を強く推奨 |
| §15.5 | harness hardening は文書化のみ | devcontainer + credproxy + mcpproxy がデフォルト |
| §18.2 | persistence / tracker write / pluggable tracker | 実装しない (SPEC §14.3 in-memory 設計維持) |

---

## Documented Posture (SPEC §10.5 / §15.5 要請)

### approval / sandbox policy (§10.5)

- `codex.command: codex app-server` の場合: Codex 側の auto-approve policy を WORKFLOW.md で指定。
- `codex.command: claude-app-server` の場合: claude には approval 概念がないため shim が**即実行**。受け取った `approvalPolicy`/`sandboxPolicy` 値は `slog.Warn` に記録するのみ（オペレータへの逸脱通知）。実際の安全境界は container が担う。

### user-input-required (§10.5)

user input を要求する turn は **fail 扱い**する（自動運転前提）。

### harness hardening (§15.5)

default で適用: devcontainer 隔離、credproxy、mcpproxy whitelist、hostexec allow-list、secret 非ログ、hook script 出力 truncate。

### §10.5 `linear_graphql` advertise blocked (最新逸脱)

`DynamicToolSpec` が codex schema 上 orphan のため tool 宣言の wire 経路が無い。handler は `orchestrator/lineargql/` に実装済だが、advertise は pinned codex 0.133.0 が schema bump するまで不可。`codex.command: claude-app-server` 経由では `item/tool/call` で call は到達するが codex 正規経路では blocked。詳細: [issues/024](../../../issues/024-p8b-linear-graphql-tool.md) §B。
