# Overview

## ゴール

Symphony SPEC 準拠の自立型開発パイプラインバイナリ (`orchestrator`) を agent-roost と同一リポジトリ内に構築する。roost の環境基盤実装 (sandbox / hostexec / mcpproxy / agentlaunch 等) を **`platform/`** 配下に共有資産として整理し、roost / orchestrator の両バイナリから利用する。

## 決定事項

| # | 決定 | 帰結 |
|---|---|---|
| D1 | 環境基盤は `platform/` に集約し共有 | `sandbox/` `hostexec/` `mcpproxy/` `lib/` 等を移動 |
| D2 | roost 専用コードは `client/` に集約 | `state/` `runtime/` `tui/` `tmux/` `proto/` `driver/` `connector/` 等 |
| D3 | Symphony 実装は `orchestrator/` に集約 | 新規ツリー |
| D4 | バイナリは 3 種類 (`roost` / `orchestrator` / `claude-app-server`) | `cmd/<name>/main.go` 配置 |
| D5 | 全コンポーネントは Go で新規実装し、SPEC.md を正本とする | Linear adapter 含め後方互換は持たず SPEC に合わせる |
| D6 | orchestrator は **TUI 非使用**。観測は HTTP server (SPEC §13.7) | dashboard `/` + `/api/v1/*` |
| D7 | agent は **Codex 専用にしない**。`claude -p` を Codex stdio protocol で wrap する shim を実装 | `cmd/claude-app-server/` |
| D8 | agent 切替は SPEC §10.1 の `codex.command` 経由 | `codex.command: claude-app-server` で claude が走る |
| D9 | 新規実装でも roost / orchestrator 双方で活用できるものは `platform/` に検討 | 詳細は [02-layout.md](02-layout.md) |
| D10 | 永続 DB なし (SPEC §14.3 準拠) | restart 復旧は tracker 再 poll + workspace 残存 |
| D11 | Linear status の state machine 化は **しない** | SPEC §11.5 — workflow phase は agent prompt 側に押し込む |
| D12 | git worktree 前提を捨てる (SPEC §9.3) | workspace 作成は単純な mkdir、git 拡張は hook で |

## 非ゴール

- 多テナント制御プレーン
- 汎用ワークフローエンジン
- 既存 roost の TUI / tmux 設計を変更すること
- 既存 roost の単一イベントループ純粋性を緩めること

## 範囲

### この計画に含む

- `platform/` `client/` `orchestrator/` の物理分離
- `runtime/` から `platform/agentlaunch/` の抽出
- `runtime/subsystem/stream/` から `platform/agent/codexclient/` の抽出
- SPEC §1-§17 の **Core Conformance** 項目の実装
- `claude-app-server` shim 実装
- HTTP observability server (SPEC §13.7)

### この計画には含まない (将来検討)

- SPEC Appendix A の SSH worker extension
- Linear 以外の tracker adapter
- 永続化 (SPEC §18.2 の TODO 項目)
- orchestrator service 内部での tracker write API (SPEC §11.5 で agent 側に委譲)
- ultra-light agent backend (web-only, no local exec) 等の異種 agent

## 設計原則 (引き継ぎ)

agent-roost の [ARCHITECTURE.md](../ARCHITECTURE.md) の設計原則は `client/` 内では従来通り厳守する。`platform/` `orchestrator/` 配下にも以下を適用:

- **Functional Core / Imperative Shell** — orchestrator の scheduler も pure な状態遷移関数として書く (SPEC §16 の reference algorithm が pure)
- **No fallbacks** — tracker fetch 失敗時に隠れたデフォルトに falling back しない (SPEC §11.4 と整合)
- **Driver/Subsystem 隔離** — `platform/agent/codexclient/` が agent backend の唯一の知識点。orchestrator は backend を識別しない

新たに加える原則:

- **SPEC が決めたものは SPEC に従う** — 命名・状態名・event 名は SPEC §17 の test matrix と突き合わせて conformance 可能な形にする
- **逸脱は明示** — SPEC から外れる選択は [05-conformance.md](05-conformance.md) に記録する

## 関連資料

- [Symphony SPEC.md](https://github.com/openai/symphony/blob/main/SPEC.md) (v1 Draft, language-agnostic)
- agent-roost [ARCHITECTURE.md](../ARCHITECTURE.md)
- agent-roost [docs/technical/platform/sandbox.md](../docs/technical/platform/sandbox.md), [docs/technical/client/ipc.md](../docs/technical/client/ipc.md)
