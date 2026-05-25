# 018: cmd/claude-app-server — Codex stdio shim driving `claude -p`

- **Phase**: P5b ([plans/04-phases.md#p5-claude-app-server-shim](../plans/04-phases.md))
- **Status**: Closed
- **Depends on**: 017 (streamjson reader)、P0c (merged; `platform/agent/codexclient` server helper + `codexschema`)
- **Blocks**: 019 (agent 切替 end-to-end / conformance)

## Background

[plans/03-agent.md](../plans/03-agent.md) の設計に沿って、stdin/stdout で **Codex app-server stdio protocol** を喋り内部で `claude -p` を回す shim を実装する。現状 `cmd/claude-app-server/main.go` は stub（initialize に応答するだけ）。これを実体化し、orchestrator から見て **codex app-server と同じ event 列**が流れるようにする。

**プロトコルは codexschema の pin 済み定数が正**（plans/03 の `v2/start_*` 表記は旧称）。orchestrator(013/`codexclient` client)が送るのは:

- `initialize`（`codexclient.Initialize`）
- `turn/start`（`codexclient.StartTurn(threadID, startDir, stdin=prompt)`）
- 継続時 `thread/resume`（`codexclient.ResumeThread`）

shim が emit するのは `codexclient.Server` の helper:
`EmitThreadStarted` / `EmitTurnStarted` / `EmitAgentMessageDelta` / `EmitTurnCompleted` / `EmitTurnFailed`（`codexschema.Method*` に対応）。

## ID 設計（plans/03 §thread_id/turn_id/session_id）

| ID | 発行元 | 保持 |
|---|---|---|
| `thread_id` | shim (UUIDv7) | `thread_id → claude_session_id` map |
| `claude_session_id` | claude (`system:init`) | 上 map |
| `turn_id` | shim (UUIDv7、turn ごと) | turn state |
| `session_id` | `<thread_id>-<turn_id>` | event 送出時 |

## Tasks

### A. stdio server ループ

- [x] `main.go` を `codexclient.NewServer(codexclient.NewConn(codexclient.DefaultStdioTransport(), ...))` ベースに置換。`Conn.Run` で受信し handler に dispatch（stub の挙動は drop）
- [x] `initialize` 要求に capability 応答（supported tool list 含む。最小で可）
- [x] graceful shutdown（stdin EOF / SIGTERM）で実行中の claude 子プロセスを確実に停止（プロセスグループ kill か ctx 連動）

### B. thread / turn ライフサイクル

- [x] `turn/start` 受信: thread 未作成なら `thread_id` 発行 + `EmitThreadStarted`。`turn_id` 発行 + `EmitTurnStarted`。prompt（StartTurn の stdin）と cwd を取り出す
- [x] `thread/resume` 受信: 既存 `thread_id` の `claude_session_id` で継続（`--resume`）
- [x] cancel は **プロトコル method ではなくプロセス kill で実現**（orchestrator の `Worker.Kill` が shim プロセスを落とす）。shim は子 claude を道連れに落とす（プロセスグループ／ctx 伝播）。明示 `cancel_turn` 方式は採らない

### C. `claude -p` 起動

- [x] `turn/start` で `claude -p --output-format stream-json [--resume <claude_session_id>] <prompt>` を cwd で起動（初回 new session、以降 `--resume`）
- [x] stdout を 017 の streamjson reader で逐次読む
- [x] `system:init` の `session_id` を thread map に格納（継続 turn の `--resume` に使う）

### D. event 変換（最小通電）

- [x] streamjson event → Codex notification:
  - `system:init` → `EmitThreadStarted` + `EmitTurnStarted`（または turn 開始時に発行済みなら整合させる）
  - `assistant` text → `EmitAgentMessageDelta`（途中経過）
  - `result:success` → `EmitTurnCompleted`（+ token は 019）
  - `result:error` / 異常 exit → `EmitTurnFailed`
- [x] tool_use/tool_result の item event（`item/started`/`item/completed`）は 019 で拡充（本 issue は単線通電優先）

### E. テスト (§17.5)

- [x] fake claude（stream-json を吐く test double スクリプト or 注入した proc）で 1 turn を回し、`thread/started`→`turn/started`→…→`turn/completed` が emit される
- [x] orchestrator の `codexclient` client（013 で使う Conn）を相手に in-memory pipe で結合し、`session_id = <thread_id>-<turn_id>` を検証
- [x] `result:error` で `turn/failed` emit
- [x] shim プロセス kill で claude 子が停止する（プロセスグループ／ctx 伝播の確認）

## Acceptance Criteria

- `claude -p` を起動して 1 turn を codex event 列として中継できる
- `session_id` が `<thread_id>-<turn_id>`、継続 turn が `--resume <claude_session_id>` を使う
- orchestrator から見て codex app-server と同型の event 列（013 の handler が解釈できる）
- `go test ./cmd/claude-app-server/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10 (Agent Runner Protocol), §10.1 (Launch Contract — `bash -lc`), §10.4 (Event Types), §16.5
- [plans/03-agent.md](../plans/03-agent.md)（method マッピング表、ID 設計、stream-json 変換）
- `platform/agent/codexclient`（`Server` emit helper / `codexschema.Method*`）、[017](017-p5a-claude-streamjson.md)（streamjson reader）
