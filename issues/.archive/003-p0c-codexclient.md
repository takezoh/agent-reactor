# 003: codexclient と codexschema を stream/ から platform/agent/ へ抽出

- **Phase**: P0c ([plans/04-phases.md#p0c-codexclient-抽出](../plans/04-phases.md))
- **Status**: Closed
- **Depends on**: [001](001-p0a-physical-move.md)
- **Blocks**: orchestrator agent 統合 (P4)、claude-app-server shim (P5)

## Background

現在 `client/runtime/subsystem/stream/` (旧 `src/runtime/subsystem/stream/`) に codex app-server stdio JSON-RPC のプロトコル実装が埋め込まれている。これを **roost (client/) と orchestrator 両方が使い回せる platform 層** に抽出し、`claude-app-server` shim でも server-side 実装として利用できる形にする。

同時に Codex app-server JSON Schema を **特定 version で pin** し、drift detection を CI に組み込む ([plans/03-agent.md#schema-pin](../plans/03-agent.md))。

## Tasks

### A. codexschema 新設

- [x] `src/platform/agent/codexschema/` 新設
- [x] 現在 link している Codex app-server schema (version) を確認し、`codex app-server generate-json-schema --out <dir>` で取得した JSON を pin commit:
  - [x] `v2/ThreadStartParams.json`
  - [x] `v2/TurnStartParams.json`
  - [x] 関連 enum (AskForApproval, SandboxMode, SandboxPolicy 等)
- [x] Go 構造体に decode する型を生成 (手書き or codegen)
- [x] `codexschema/README.md` に pin している version と更新手順を明示

### B. codexclient 抽出

- [x] `src/platform/agent/codexclient/` 新設
- [x] `client/runtime/subsystem/stream/` 内のプロトコル層を抽出:
  - [x] stdio JSON-RPC framing (client-side)
  - [x] message 型 (codexschema を参照)
  - [x] request/response timeout (`codex.read_timeout_ms`)
  - [x] turn event stream の reader
- [x] **server-side helper も同時に提供**:
  - [x] incoming JSON-RPC のデコード
  - [x] event emission helper (`turn_completed`, `turn_failed`, etc.)
  - [x] claude-app-server shim から利用するための API
- [x] subsystem 固有 (BindFrame, ReleaseFrame, frame ID 等) は **抽出対象外**。runtime 側に残す

### C. client/runtime/subsystem/stream/ のアダプタ化

- [x] `stream/` を `platform/agent/codexclient/` を使う薄い実装に書き換え
- [x] frame ↔ thread の対応付けは引き続き `stream/` 内で管理
- [x] sockbridge / shared codex backend の概念は `stream/` に残す

### D. drift detection CI

- [x] CI step を追加:
  ```sh
  codex app-server generate-json-schema --out /tmp/codex-schema-out
  diff -r platform/agent/codexschema/<schema-dir>/ /tmp/codex-schema-out/
  ```
- [x] diff 検出時は CI fail させる (schema bump は明示的 PR で)
- [x] `Makefile` に `codex-schema-update` target を追加 (手元で pin を更新する手順)

### E. boundary

- [x] depguard ルール:
  - `platform/agent/codexclient/` は `platform/agent/codexschema/` `platform/logger/` のみ依存可
  - `client/*` `orchestrator/*` を import 禁止
- [x] roost の `runtime/isolation_test.go` 相当を更新

### F. テスト

- [x] codexclient のプロトコル framing test (round-trip)
- [x] server-side helper の test (claude-app-server shim から使うシナリオを模倣)
- [x] 既存 stream subsystem の test が通ることを確認

## Acceptance Criteria

- roost の挙動変更ゼロ。`codex app-server` を spawn して thread/turn を回せる
- `platform/agent/codexclient/` を import するだけで stdio JSON-RPC client が利用可能
- `platform/agent/codexclient/` の server helper を使って codex-protocol を喋るプロセスが書ける (shim の前提)
- codex schema は pin され、drift で CI が落ちる

## Notes

- Codex app-server protocol は schema 経由で型安全に扱う。**string で method 名を散らかさない**
- `claude-app-server` shim から `codexclient.Server` を使うため、API を export する設計
- schema version は最初の pin として「現時点で `codex app-server` が出力するもの」を採用。version 文字列を README に明記

## References

- [plans/02-layout.md](../plans/02-layout.md)
- [plans/03-agent.md#codex-stdio-protocol-の取り扱い](../plans/03-agent.md)
- [plans/04-phases.md#p0c-codexclient-抽出](../plans/04-phases.md)
- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10 (Agent Runner Protocol), §5.3.6 (`codex` config)
- `client/runtime/subsystem/stream/` (現状実装)
- `docs/state-monitoring.md` の codex 関連記述
