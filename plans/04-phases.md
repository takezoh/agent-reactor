# Phase 計画

## 全体俯瞰

| Phase | 内容 | 主な書き先 | 規模 |
|---|---|---|---|
| **P0a** | 物理移動: roost → `client/`、共有資産 → `platform/`、main → `cmd/roost/` | 全 | 中 |
| **P0b** | `agentlaunch/` を `client/runtime/` から `platform/` へ抽出 | platform | 中 |
| **P0c** | `platform/agent/codexclient/` を `client/runtime/subsystem/stream/` から抽出 + schema pin | platform | 中 |
| **P0d** | `cmd/orchestrator/` `cmd/claude-app-server/` 雛形配置、Makefile 整備 | cmd | 小 |
| **P1** | `WORKFLOW.md` loader + wfconfig + preflight + stub scheduler | orchestrator | 小 |
| **P2** | Linear adapter を `platform/tracker/linear/` に移植 + workspace manager + 4 hooks | platform + orchestrator | 中 |
| **P3** | scheduler core (poll/dispatch/retry/reconcile) + 直接 codex app-server 起動の単線 | orchestrator | 大 |
| **P4** | agent invocation を `platform/agent/codexclient` 経由に切替 + sandbox 配線 (`agentlaunch/`) | orchestrator | 中 |
| **P5** | `claude-app-server` shim 実装 | cmd/claude-app-server | 中 |
| **P6** | continuation turn + stall detection + reconciliation + token accounting | orchestrator + platform | 大 |
| **P7** | HTTP server (`/`, `/api/v1/state`, `/api/v1/<id>`, `/api/v1/refresh`) | orchestrator | 中 |
| **P8** | WORKFLOW.md hot reload + `linear_graphql` tool (codex native `item/tool/call`) | orchestrator | 中 |
| **P9** | SPEC §17 conformance test 群 + orchestrator 位置付け doc | orchestrator + docs | 中 |

P0a-P3 で **Linear → workspace → agent 起動 → 1 turn** の単線が通る。
P4-P5 で container 内 agent と claude/codex 切替が動く。
P6 で本格的 orchestrator になる。

## Phase 詳細

### P0a: 物理移動

**目的**: ディレクトリ構造を [02-layout.md](02-layout.md) に揃える。挙動変更ゼロ。

**作業**:
1. `src/main.go` → `src/cmd/roost/main.go`
2. 共有候補を `src/platform/` 配下に移動:
   - `src/sandbox/` → `src/platform/sandbox/`
   - `src/hostexec/` → `src/platform/hostexec/`
   - `src/mcpproxy/` → `src/platform/mcpproxy/`
   - `src/lib/pathmap/` → `src/platform/pathmap/`
   - `src/lib/{git,github,claude}/` → `src/platform/lib/{git,github,claude}/`
   - `src/logger/` → `src/platform/logger/`
   - `src/features/` → `src/platform/features/`
   - `src/config/` の共有部 → `src/platform/config/` (SandboxResolver, DataDir 等)
3. roost 専用を `src/client/` 配下に移動:
   - `src/{state,runtime,proto,tui,tools,tmux,driver,connector,event,uiproc,cli,procio,winexec}/` → `src/client/<同名>/`
   - `src/config/` の roost 専用部 → `src/client/config/` (TUI 設定 / driver 設定 等)
4. 全 import path 更新
5. Makefile 修正 (build target は `roost` のまま)
6. `depguard` ルールを `client/` ↛ `orchestrator/`、`platform/` ↛ `{client,orchestrator}/` で更新

**成功条件**:
- `make build` `make vet` `make lint` `cd src && go test ./...` がすべて通る
- 既存 roost の動作は変わらない

**留意点**:
- 1 PR で済む規模ではない。move-only と import-update を分ければ safe
- 移動時 git rename detection を効かせるため **PR 内で他の変更を混ぜない**

### P0b: `agentlaunch/` 抽出

**目的**: `client/runtime/sandbox_dispatcher.go` 等を runtime 非依存に切り出し、orchestrator からも使えるようにする。

**作業**:
1. `platform/agentlaunch/` package 新設
2. 以下を runtime から移動:
   - `SandboxDispatcher`
   - `DirectLauncher`, `DevcontainerLauncher` 相当
   - `WrappedLaunch{Command,Args,Env,Mounts,ContainerSockDir,Cleanup}` 型
   - `LaunchPlan{Command,Args,Env,StartDir}` 型 (Subsystem 由来の hint を持たない pure data)
3. `client/runtime/` 側は新パッケージを呼ぶ薄いアダプタに変える
4. `LaunchPlan` から tmux 由来 field を除去 (subsystem 側で wrap する)

**新 API シグネチャ**:

```go
package agentlaunch

type LaunchPlan struct {
    Command  string
    Args     []string
    Env      map[string]string
    StartDir string
}

type Mount struct { Source, Target string; ReadOnly bool }

type WrappedLaunch struct {
    Command          string
    Args             []string
    Env              map[string]string
    Mounts           []Mount
    ContainerSockDir string
    Cleanup          func(context.Context) error
}

type Dispatcher interface {
    Wrap(ctx context.Context, project string, plan LaunchPlan) (WrappedLaunch, error)
}
```

**成功条件**:
- roost の挙動変更ゼロ
- orchestrator から `agentlaunch.Dispatcher` を import して `WrappedLaunch` を取得できる

### P0c: `codexclient/` 抽出

**目的**: codex app-server stdio protocol 実装を `runtime/subsystem/stream/` から `platform/agent/codexclient/` に抽出。schema pin を確立。

**作業**:
1. `platform/agent/codexschema/` 新設、Codex app-server JSON Schema を pin (commit 済み JSON ファイル)
2. `platform/agent/codexclient/` 新設、以下を含む:
   - stdio JSON-RPC framing (client-side)
   - stdio JSON-RPC framing (server-side) ← shim 用
   - message 型 (schema-pinned)
   - transport timeout
3. `client/runtime/subsystem/stream/` を新パッケージを使う薄い実装に書き換え
4. CI に `codex app-server generate-json-schema` 出力との diff 検出 step を追加

**成功条件**:
- roost stream subsystem が従来通り codex app-server と通信
- orchestrator / claude-app-server からも `codexclient` を import 可能

### P0d: `cmd/` 整備

**目的**: 残り 2 バイナリのエントリ雛形と Makefile target。

**作業**:
1. `src/cmd/orchestrator/main.go` 雛形 (stub: 引数解析 + WORKFLOW.md パスチェックのみ)
2. `src/cmd/claude-app-server/main.go` 雛形 (stub: stdio で initialize に応答するだけ)
3. Makefile target 追加:
   ```
   build-orchestrator:
   build-claude-app-server:
   build-all:
   ```

**成功条件**:
- 3 バイナリが build できる
- stub バイナリが起動して exit する

### P1: workflow loader / wfconfig / preflight

**目的**: SPEC §5-§6 を実装し、`WORKFLOW.md` を読んで preflight validation が通る状態を作る。

**作業**:
1. `orchestrator/workflowfile/` で YAML front matter + Markdown body 分離 parser
2. `orchestrator/wfconfig/` で typed getter + 各 field default + `$VAR` 解決 + `~` 展開
3. preflight validation (SPEC §6.3): workflow file 読込 / `tracker.kind` / `tracker.api_key` / `tracker.project_slug` / `codex.command`
4. stub scheduler: WORKFLOW.md を読んで preflight OK なら poll interval ごとに「dispatch しません」とログするだけ

**成功条件**:
- `orchestrator --workflow ./WORKFLOW.md` で起動・loop・SIGTERM で graceful shutdown
- 不正な WORKFLOW.md で `missing_workflow_file` / `workflow_parse_error` / etc を返す
- SPEC §17.1 の test 項目を pass

### P2: Linear adapter + workspace + hooks

**目的**: SPEC §9 + §11 を実装。1 issue を fetch して workspace を作って hook を実行できる。

**作業**:
1. `platform/tracker/linear/` 新設（Linear GraphQL adapter を Go で新規実装）:
   - `FetchCandidateIssues()` (active states, project slug filter, pagination)
   - `FetchIssuesByStates(stateNames)` (startup terminal cleanup 用)
   - `FetchIssueStatesByIDs(ids)` (reconciliation 用)
   - Linear GraphQL schema を pin (drift detection 不要、network timeout 30s)
2. `orchestrator/tracker/` で linear client を業務観点で wrap (active state set 等)
3. `orchestrator/workspace/`:
   - workspace key sanitization (`[A-Za-z0-9._-]` 以外を `_`)
   - workspace root containment check
   - 4 種 hooks (`after_create`/`before_run`/`after_run`/`before_remove`) を `sh -lc` + timeout で実行
4. `client/connector/linear/` (現状の roost Linear connector) を `platform/tracker/linear/` を使うように書き換え

**成功条件**:
- `orchestrator --once` で 1 tick だけ poll → candidate を log 出力
- workspace 作成と `after_create` hook が動く
- SPEC §17.2 §17.3 の test 項目を pass

### P3: scheduler core

**目的**: SPEC §7-§8 の orchestrator (scheduling brain) を実装。codex app-server を直接起動する形で end-to-end の単線を通す。

**作業**:
1. `orchestrator/scheduler/` で SPEC §7.1 の state machine 実装 (Unclaimed/Claimed/Running/RetryQueued/Released)
2. SPEC §8.1 poll loop (reconcile → validate → fetch → sort → dispatch)
3. SPEC §8.2 candidate eligibility (active state, blocker(Todo) rule)
4. SPEC §8.3 global + per-state concurrency
5. SPEC §8.4 retry/backoff (`min(10000*2^(n-1), max)` + 連続 retry 1s)
6. SPEC §8.5 reconciliation の Part A (stall detection) と Part B (tracker state refresh)
7. SPEC §8.6 startup terminal workspace cleanup
8. **agent 起動はまず simple な `exec.Command(bash, "-lc", codex.command)` で実装** (sandbox 配線は P4)
9. SPEC §10.4 event の最低限を直接 stdio parse して emit (`session_started` / `turn_completed` / `turn_failed`)

**成功条件**:
- 1 issue を fetch → workspace 作成 → codex app-server 起動 → 1 turn → workspace 残存
- retry backoff / per-state concurrency / stall timeout の動作確認
- SPEC §17.4 の test 項目を pass

### P4: codexclient 経由 + sandbox 配線

**目的**: P3 の生 stdio を `platform/agent/codexclient/` 経由に置き換え、`platform/agentlaunch/` を介して container 内で agent を起動。

**作業**:
1. P3 で書いた生 stdio parse を `codexclient.Client.RunTurn(...)` に置き換え
2. `orchestrator/agent/` で codexclient を Issue 単位で使う wrapper
3. `agentlaunch.Dispatcher.Wrap(...)` を介して `WrappedLaunch` を取得 → `exec.Cmd` 生成
4. container 内で codex app-server が動く構成 (devcontainer mode の場合)

**成功条件**:
- direct mode と devcontainer mode の両方で 1 issue end-to-end
- container 内から host MCP server 等が見える (mcpproxy / hostexec が動く)

### P5: claude-app-server shim

**目的**: [03-agent.md](03-agent.md) の設計に沿って claude shim を実装。

**作業**:
1. `cmd/claude-app-server/main.go`:
   - `codexclient.Server` で stdio JSON-RPC 受信
   - `initialize` / `v2/start_thread` / `v2/start_turn` / `cancel_turn` / `v2/stop_thread` 実装
2. `platform/lib/claude/streamjson/` 新設、claude の stream-json reader
3. `thread_id → claude_session_id` map、`claude -p --resume` invocation
4. stream-json → SPEC §10.4 event 変換
5. token 抽出
6. approval/sandbox field は受け取って警告ログのみ (実体は sandbox 側に委ねる)

**成功条件**:
- WORKFLOW.md で `codex.command: claude-app-server` を指定 → orchestrator から見て codex と同じ event 列が流れる
- container 内で claude が動く
- 1 issue end-to-end (orchestrator → claude-app-server → claude -p)

### P6: continuation + reconciliation + metrics

**目的**: SPEC §7.1 後半 + §8.5 + §13.5 を完成。

**作業**:
1. continuation turn loop (SPEC §16.5):
   - turn 完了後に tracker 状態再確認
   - active なら同一 thread で次 turn
   - `max_turns` まで継続
   - worker 正常終了後の **1s 連続 retry** で再 dispatch 機会
2. stall detection (SPEC §8.5 Part A):
   - `last_codex_timestamp` ベースの elapsed check
   - `stall_timeout_ms` 超過で worker を kill + retry queue
3. tracker state refresh (SPEC §8.5 Part B):
   - terminal なら worker kill + workspace 削除
   - non-active なら worker kill (workspace 残存)
   - active なら issue snapshot 更新
4. `platform/metrics/`:
   - input/output/total tokens (absolute thread totals vs delta の判別)
   - runtime seconds aggregate
   - rate-limit snapshot
5. SPEC §10.4 event の漏れを埋める (`turn_input_required`, `unsupported_tool_call`, etc.)

**成功条件**:
- 1 issue で複数 turn を回せる
- stall すると retry に落ちる
- terminal 遷移で worker と workspace が消える
- token / runtime / rate-limit が正確に集計される

### P7: HTTP server

**目的**: SPEC §13.7 を必須実装として完成。

**作業**:
1. `orchestrator/httpserver/`:
   - `GET /` (dashboard、server-rendered HTML)
   - `GET /api/v1/state` (running / retrying / codex_totals / rate_limits)
   - `GET /api/v1/<issue_identifier>` (per-issue 詳細)
   - `POST /api/v1/refresh` (immediate tick trigger)
   - `405 Method Not Allowed` / error envelope `{"error":{"code","message"}}`
2. loopback デフォルト bind (`127.0.0.1`)
3. CLI `--port` が `server.port` 設定を上書き
4. `platform/httpsurface/` を作るかは P7 で判断:
   - 共通 middleware (logging / panic recovery) を切り出す価値があれば platform へ
   - 1 binary で完結なら orchestrator/httpserver/ 内のみ

**成功条件**:
- ブラウザで dashboard 表示
- curl で API 操作可能
- SPEC §13.7.2 のサンプル response shape と一致

### P8: hot reload + linear_graphql tool

**目的**: SPEC §6.2 + §10.5 を完成。

**作業**:
1. `WORKFLOW.md` fsnotify watch (`platform/lib/` の fsnotify 利用パターン参考)
2. config change の **live re-apply**:
   - poll interval / concurrency / active-terminal state set
   - codex settings は次回 dispatch から反映
   - 不正な reload は last known good を保持 + operator-visible warn
3. `linear_graphql` tool:
   - Linear MCP server を `platform/mcpproxy/` 経由で host 起動
   - container 内 agent から MCP 経由で見える
   - SPEC §10.5 が要請する input shape (`query` + `variables`) を確保
   - success/errors の判別を結果に反映

**成功条件**:
- WORKFLOW.md を save すると orchestrator が再読込
- agent が `linear_graphql` tool で Linear に query / mutation を発行できる

### P9: conformance tests + docs

**目的**: SPEC §17 conformance test を埋め、orchestrator サービスの位置付けを doc 化。

**作業**:
1. SPEC §17.1-§17.7 (Core Conformance) を埋める test を `orchestrator/*` 配下に追加
2. SPEC §17.8 (Real Integration Profile) は `LINEAR_API_KEY` 有無で skip/run
3. `docs/technical/orchestrator/symphony-conformance.md` で SPEC 各項目と test の対応表を作成
4. agent-roost の AGENTS.md / ARCHITECTURE.md に orchestrator サービスの位置付けを追記

**成功条件**:
- SPEC §17 のチェック項目がテストでカバー
- conformance 表が SPEC 各 § と紐づいて閲覧可能
- orchestrator サービスの位置付け（3 バイナリ・三層・責務）が doc から辿れる

## 依存関係

```
P0a ──┬─ P0b ──┐
      │        ├── P0d ── P1 ── P2 ── P3 ── P4 ── P5
      └─ P0c ──┘                            │
                                            └── P6 ── P7 ── P8 ── P9
```

- P0a-P0d は順序自由 (依存少ない)
- P3 は P0d + P2 が前提
- P4 は P3 + P0b + P0c が前提
- P5 は P0c + P4 が前提
- P6-P9 は P5 完了後の独立な進行が可能

## マイルストーン

| マイルストーン | Phase | 意義 |
|---|---|---|
| **M0: 構造分離完了** | P0a-P0d | 後続の物理基盤確立 |
| **M1: 最小単線通電** | P3 | 1 issue → codex app-server で 1 turn |
| **M2: 多 agent 対応** | P5 | claude vs codex 切替 |
| **M3: SPEC 機能完成** | P8 | SPEC §1-§16 を満たす |
| **M4: conformance 確認** | P9 | SPEC §17 test pass + 位置付け doc |
