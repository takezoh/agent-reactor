# 017: platform/lib/claude/streamjson — `claude -p` stream-json reader

- **Phase**: P5a ([plans/04-phases.md#p5-claude-app-server-shim](../plans/04-phases.md))
- **Status**: Closed
- **Depends on**: なし（leaf。P0c の codexschema/codexclient は不要）
- **Blocks**: 018 (claude-app-server shim が本 reader を消費する)

## Background

P5 の claude-app-server shim は `claude -p --output-format stream-json` の **NDJSON 出力**を読んで Codex protocol event に変換する（[plans/03-agent.md](../plans/03-agent.md) §stream-json → §10.4）。本 issue はその **入力側の純粋パーサ**を `platform/lib/claude/streamjson/` に新設する。shim 本体（018）から切り離し、fixture で単体テストできる leaf にする。

**layering**: roost には `client/lib/claude/transcript`（TUI 用の `.jsonl` transcript レンダラ）が既にあるが、これは表示用で stream-json reader ではない。orchestrator/shim が client に依存しないよう、新規実装は **`platform/`** に置く（[[feedback-role-coordinator]] と 02-layout の platform=共有基盤方針）。

## stream-json の形（`claude -p --output-format stream-json`）

行区切り JSON。代表的な `type`:

- `system` (`subtype: "init"`) — `session_id` を含む。セッション開始
- `assistant` — message。content に `text` / `tool_use` ブロック
- `user` — `tool_result` ブロック
- `result` (`subtype: "success"|"error_*"`) — 終了。`usage.input_tokens` / `output_tokens` / 累計、`result` テキスト、`is_error`

## Tasks

### A. 型と Parser

- [x] `platform/lib/claude/streamjson/` 新設。`Event` を type 判別した typed struct に分解する `Parse(line []byte) (Event, error)`（または `io.Reader` を行ストリームで読む `Scanner`）
- [x] 対応イベント: `SystemInit{SessionID}` / `AssistantMessage{Text, ToolUses []ToolUse}` / `ToolResult{...}` / `Result{Usage, ResultText, IsError, Subtype}`
- [x] 未知 `type` は `Unknown`（無視可能）として返し、reader を止めない（claude のマイナー版差異に耐える）
- [x] 大きい行（巨大 diff 等）に耐える buffer（`bufio.Scanner` の上限を引き上げ。codexclient stdio transport と同様 64MiB 目安）

### B. usage 抽出 (§13.5)

- [x] `Result.Usage` から `input_tokens` / `output_tokens` / `total_tokens`（無ければ input+output で算出）を取り出す helper
- [x] absolute thread totals として扱える形で返す（018/scheduler が累計表示に使う）

### C. テスト (§17.5 系)

- [x] 各 `type` の fixture 行をパースし typed event になる
- [x] `system:init` から `session_id` を取り出せる
- [x] `result:success` から usage（input/output/total）を取り出せる、`total` 欠落時は算出
- [x] `result:error` / `is_error` を失敗として判別
- [x] 未知 type・空行・壊れた JSON 行で reader が落ちない（壊れた行はスキップまたは typed error）

## Acceptance Criteria

- `claude -p --output-format stream-json` の代表出力を fixture で網羅パース
- `session_id` と usage が取り出せる
- `go test ./platform/lib/claude/streamjson/` 緑、lint 緑、新規依存なし（stdlib `encoding/json` + `bufio`）

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.4 (Agent Event Types — 変換先)、§13.5 (Token/runtime accounting)、§17.5
- [plans/03-agent.md](../plans/03-agent.md)（stream-json → §10.4 マッピング表、token 抽出）
- 参考（再利用はしない）: `client/lib/claude/transcript/transcript_usage.go`（usage 解析の既存例）
