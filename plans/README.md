# Plans

Symphony SPEC ([openai/symphony](https://github.com/openai/symphony)) を agent-roost のリポジトリ内に同居する別バイナリとして実装するための計画群。

## 背景

- 自立型開発パイプラインを **agent-roost の環境基盤を共有**しつつ Symphony SPEC として実装する
- roost TUI は使わず、観測は HTTP server で行う
- agent は Codex 専用にせず、`claude -p` を **Codex app-server stdio protocol** で wrap する shim を作って選択可能にする

## ドキュメント構成

| ファイル | 内容 |
|---|---|
| [roadmap.md](roadmap.md) | 全体ロードマップと進捗 (現状を反映する living tracker) |
| [00-overview.md](00-overview.md) | ゴール / 決定事項 / 非ゴール / 範囲 |
| [01-spec-gap.md](01-spec-gap.md) | Symphony SPEC.md と既存資産 (roost) の gap 分析 |
| [02-layout.md](02-layout.md) | リポジトリ構成 / バイナリ / パッケージ境界 / 共有実装方針 |
| [03-agent.md](03-agent.md) | Agent 抽象戦略 (stdio shim) と `claude-app-server` 設計 |
| [04-phases.md](04-phases.md) | フェーズ別実装計画 (P0a-P9) |
| [05-conformance.md](05-conformance.md) | SPEC との逸脱 / 厳守項目の明示 |

## 用語

- **orchestrator (service)** — 本計画で構築する Symphony 実装バイナリの総称。`cmd/orchestrator/` および `orchestrator/` ディレクトリ
- **orchestrator (SPEC component)** — SPEC §3.1.4 が定義する scheduling brain (poll/dispatch/retry/reconcile)。実装上は `orchestrator/scheduler/` package に格納し、サービス全体と区別する
- **harness** — 旧称。`orchestrator` に統一済み
- **claude-app-server** — `claude -p` を Codex app-server stdio protocol で wrap する shim binary
- **WORKFLOW.md** — SPEC §5 が定義する repo-owned な policy file
