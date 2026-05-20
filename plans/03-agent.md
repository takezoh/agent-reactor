# Agent 抽象戦略

## 方針

**Codex app-server stdio protocol を agent との唯一の interface とする** (SPEC §10 を厳守)。
Go interface ベースの Backend 抽象は採用しない。
agent 切替は SPEC §10.1 の `codex.command` 経由で行い、新しい agent は **shim binary を 1 個追加するだけ** で接続できる構造にする。

## 設計判断

| 候補案 | 採否 | 理由 |
|---|---|---|
| **A. Go interface (`Backend`)** で複数 agent を抽象化 | × | orchestrator が複数の Go 実装を抱えるため SPEC §10.1 の `codex.command` 拡張点が活用できない。conformance が崩れる |
| **B. stdio shim 方式** (各 agent が独立 binary、Codex protocol を喋る) | ◯ | SPEC §10.1 を素直に活用。orchestrator は **Codex protocol しか知らない**。agent 追加は binary 増加だけで済む |
| **C. HTTP/gRPC で agent と通信** | × | SPEC §10.1 が stdio + `bash -lc` を規定している。逸脱 |

## バイナリ構成

```
WORKFLOW.md の codex.command:        起動されるプロセス:
─────────────────────────────       ─────────────────────
codex.command: codex app-server     → codex app-server (OpenAI 純正)
codex.command: claude-app-server    → claude-app-server (本リポジトリ)
codex.command: <future-agent>       → <future-agent> (将来追加)
```

orchestrator から見ると **どの起動も同じ stdio JSON-RPC client**。背後で何が走っているかを知らない。

## Codex stdio protocol の取り扱い

### Schema pin

SPEC §5.3.6 は「targeted Codex app-server version の `generate-json-schema` 出力に従え」「pass-through Codex config として扱え」と規定している。

実装方針:

1. **特定の Codex app-server schema version を pin する**
2. `platform/agent/codexschema/` に JSON Schema を commit
3. CI で `codex app-server generate-json-schema --out <dir>` の出力との diff を検出 (drift detection)
4. `platform/agent/codexclient/` と `cmd/claude-app-server/` の両方が同一 schema を参照
5. version bump は明示的な PR で行う

### `platform/agent/codexclient/` の役割

| 機能 | 用途 |
|---|---|
| stdio JSON-RPC framing (client side) | orchestrator が agent process と通信 |
| stdio JSON-RPC framing (server side) | claude-app-server shim が orchestrator と通信 |
| schema-pinned message 型 | 双方 |
| transport timeout (`codex.read_timeout_ms`) | client 側 |

→ client 専用にせず **server-side helper も提供** することで shim binary との重複実装を防ぐ。

## `claude-app-server` shim の設計

### 役割

stdin/stdout で **Codex app-server stdio protocol** を喋り、内部で `claude -p` を呼び続けることで thread / turn セマンティクスを再現する。

### Codex の主要メッセージ → claude 動作のマッピング

| Codex protocol method | claude shim の挙動 |
|---|---|
| `initialize` | shim 自身の capability を宣言。supported tool list 含む |
| `v2/start_thread` (cwd, sandbox, approval) | `thread_id` を UUIDv7 で発行。**`claude` 起動はしない**。最初の `start_turn` まで遅延 |
| `v2/start_turn` (thread_id, input, cwd) | `claude -p --output-format stream-json [--resume <claude_session_id>] <prompt>` を起動。初回は new session, 以降は `--resume` |
| stream-json の `assistant`/`tool_use`/`user`/`result` 等 | Codex 風 event (`turn_started`, `tool_call`, `notification`, `turn_completed`, etc.) に逐次変換 |
| `cancel_turn` | 実行中の claude プロセスへ SIGTERM、graceful 待ち→SIGKILL |
| `v2/stop_thread` | thread map から削除。claude session 自体は resume 不要状態に (no-op で可) |
| approval / sandbox policy fields | Claude に対応概念がない。shim 内で allow-list or pass-through。詳細は後述 |

### thread_id / turn_id / session_id

| ID | 発行元 | 保持先 |
|---|---|---|
| `thread_id` | shim (UUIDv7) | shim 内 map `thread_id → claude_session_id` |
| `claude_session_id` | claude (`-p` 初回起動で割当) | 上記 map に格納 |
| `turn_id` | shim (UUIDv7、各 turn 開始時) | shim 内 turn state |
| `session_id` (SPEC §4.2) | `<thread_id>-<turn_id>` 形式で組成 | event 送出時 |

### stream-json → SPEC §10.4 event の変換

claude stream-json の event 型を SPEC §10.4 が列挙する event 名に写像:

| claude stream-json | shim emit (SPEC §10.4) |
|---|---|
| `system: init` | `session_started` |
| `assistant: message` (途中) | `notification` (text 抜粋) |
| `assistant: tool_use` | `tool_call` — codex v2 では **generic `DynamicToolCallThreadItem`** に統一（tool 名 + raw args をそのまま）。Bash→command_execution 等のヒューリスティック分岐はしない（item event は orchestrator 非消費の conformance/observability 用途、固有フィールド fabrication を避ける） |
| `user: tool_result` | `tool_result` — 対応 `tool_use` の id で相関し item/completed |
| `result: success` | `turn_completed` + `usage` (input/output/total tokens) |
| `result: error` | `turn_failed` |
| stderr abnormal | `turn_ended_with_error` |
| (該当なし) | `turn_input_required` — Claude は明示的 user input 要求がないため不発火 |
| (該当なし) | `approval_auto_approved` — shim 設定で全許可なら都度 emit |

### approval / sandbox policy の扱い

Codex の `approval_policy` (AskForApproval) / `thread_sandbox` (SandboxMode) / `turn_sandbox_policy` (SandboxPolicy) は Claude には対応概念がない。

shim の方針:

- **shim 自身は approval を行わない** (Claude が即実行する前提)
- SPEC §10.5 が要請する「documented approval/sandbox posture」として、**shim を sandboxed container 内で起動する** ことを前提とする
- 実際の安全境界は `platform/sandbox/devcontainer/` が提供する
- 受け取った `approval_policy` 値はログに記録し、警告フラグを付与する (本来の意図と異なる可能性をオペレータに伝える)

### token 抽出

SPEC §13.5 が要請する `input_tokens` / `output_tokens` / `total_tokens` を claude stream-json の `result` event から抽出:

```
result.usage.input_tokens   → input_tokens
result.usage.output_tokens  → output_tokens
result.usage.total_tokens   → total_tokens (= input + output、なければ算出)
```

per-turn の usage を **running cumulative thread total に積み上げ**、cumulative absolute total として emit する（codex の累積 absolute と同一セマンティクス）。SPEC §13.5 は absolute thread totals を使い `last_token_usage` 等の delta 形式 payload は無視する方針（delta フォールバックは持たない）。orchestrator 側 metrics(021) は last-reported との差分で二重計上を回避する。

### 実装規模

- stream-json reader + event mapper: ~150 LOC
- Codex stdio JSON-RPC server (codexclient の server helper 利用): ~100 LOC
- thread / turn lifecycle 管理: ~100 LOC
- claude プロセス起動 + cancel: ~50 LOC
- **合計**: ~400 LOC

## 将来の agent 追加

新規 agent を加える際の手順:

1. `cmd/<agent>-app-server/main.go` を追加
2. `platform/agent/codexclient/` の server-side helper を使って stdio JSON-RPC を喋る
3. agent CLI の出力を SPEC §10.4 event に変換
4. WORKFLOW.md の `codex.command` 設定例を更新

orchestrator 側の変更は **不要**。

## SPEC §10.5 `linear_graphql` tool

orchestrator が advertise する optional client-side tool。SPEC は agent process がこれを呼ぶことを規定する。

### 実装方針

- **自作の薄い in-repo MCP サーバ** を host で起動し、`platform/mcpproxy/` で container 内 agent に relay する。既製 `@anthropic-ai/linear-mcp-server` は採用しない（高水準 tool 群を出すため §10.5 の raw `query`+`variables` passthrough 形状に合わず、httptest モック/token 非ログのテスト要件も満たせず、node/npx の host 依存が増える）
- サーバは **`linear_graphql` tool 1 個だけ**を出す: `query` + `variables` を受け Linear GraphQL に POST、success/errors を判別して返す。wire 層は既存の `codexclient.Conn`（stdio JSON-RPC）を再利用し、Linear POST は stdlib `net/http`
- orchestrator が WORKFLOW.md の Linear auth を env 経由で渡してサーバを起動 (token は env のみ、ログ禁止)
- agent (codex / claude-app-server) は MCP tool として `linear_graphql` を見る
- SPEC §10.5 が要請する input/output 形式 (query + variables、success/errors の判別) は **この in-repo サーバ**で確保。httptest で Linear をモックして単体テスト可能

→ orchestrator binary は Linear API を **2 系統**持つ:
1. tracker 用 (`platform/tracker/linear/` 経由で dispatch decisions)
2. agent tool 用 (`platform/mcpproxy/` 経由で agent に提供)

## roost との関係

`platform/agent/codexclient/` は roost の `client/runtime/subsystem/stream/` からも利用される (現在の codex app-server 接続を再利用)。

- roost: agent が `codex app-server` を直接実行する従来動作を維持
- roost: 将来 `claude-app-server` を stream subsystem 配下で使うことも可能 (orchestrator と同じ shim を流用)

これにより **agent 切替の恩恵が roost 側にも波及**する。

## 開発順序

[04-phases.md](04-phases.md) を参照:

1. **P0c** で `platform/agent/codexclient/` を `client/runtime/subsystem/stream/` から抽出 (client/server helper の整理を含む)
2. **P0c** で `platform/agent/codexschema/` を新設し schema を pin
3. **P4** で orchestrator が `codexclient` を使って codex app-server を起動する単線を通す
4. **P5** で `claude-app-server` shim を実装し、orchestrator が agent を切替可能にする
