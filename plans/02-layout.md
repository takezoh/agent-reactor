# Repository Layout

## トップディレクトリ構成

```
src/
  cmd/
    roost/                  # TUI バイナリ
    orchestrator/           # Symphony 実装バイナリ
    claude-app-server/      # claude -p を Codex stdio protocol で wrap する shim

  platform/                 # roost / orchestrator 共有基盤
  client/                   # roost 専用 (TUI / runtime / state 等)
  orchestrator/             # Symphony 実装専用ツリー
```

## `platform/` (共有基盤)

```
platform/
  sandbox/                  # devcontainer 管理（現 src/sandbox/）
  hostexec/                 # SCM_RIGHTS broker（現 src/hostexec/）
  mcpproxy/                 # MCP relay broker（現 src/mcpproxy/）
  agentlaunch/              # LaunchPlan → WrappedLaunch（runtime/ から抽出）
  pathmap/                  # container↔host path 変換（現 src/lib/pathmap/）
  config/                   # SandboxResolver, DataDir 等の共通基盤
  logger/                   # slog 初期化 + key=value helper
  features/                 # runtime / compile-time flag

  lib/                      # 外部ツール統合
    git/                    # git worktree 等
    github/                 # gh CLI wrapper
    claude/                 # transcript / hook bridge
    codex/                  # codex CLI wrapper（あれば）

  tracker/                  # tracker クライアントライブラリ群
    linear/                 # Linear GraphQL client (fetching のみ)

  agent/                    # agent integration
    codexclient/            # Codex app-server stdio JSON-RPC client
    codexschema/            # pin 済み JSON Schema

  httpsurface/              # 共有 HTTP middleware / router ベース（必要が立証されたら追加）
  metrics/                  # token / runtime / rate-limit accounting (共有候補)
```

### 共有判定原則

| 原則 | 適用例 |
|---|---|
| 「現に両方で必要になっている」ものだけ共有 | `sandbox/` `hostexec/` `mcpproxy/` `tracker/linear/` `agent/codexclient/` |
| 共有コストが上回るものは個別実装 | hook script runner (orchestrator 専用)、WORKFLOW.md parser (orchestrator 専用) |
| SPEC で binary 固有として定義されたもの | orchestrator/scheduler/ の state machine、orchestrator/workflowfile/ |
| 実装が薄いものは共有しない | hook 実行 (`os/exec` + timeout) |

## `client/` (roost 専用)

```
client/
  state/                    # 純粋関数核（現 src/state/）
  runtime/                  # imperative shell（現 src/runtime/）
  proto/                    # typed IPC（現 src/proto/）
  tui/                      # Bubbletea UI（現 src/tui/）
  tools/                    # palette tools（現 src/tools/）
  tmux/                     # tmux 操作（現 src/tmux/）
  driver/                   # Driver 実装（現 src/driver/）
  connector/                # Connector 実装（現 src/connector/）
  event/                    # event subcommand（現 src/event/）
  uiproc/                   # UI process 関連（現 src/uiproc/）
  cli/                      # CLI subcommand registry（現 src/cli/）
  procio/                   # process IO（現 src/procio/）
  winexec/                  # window exec（現 src/winexec/）
  config/                   # roost 専用 config (TUI 設定 / driver 設定 等)
```

agent-roost の現行設計原則 (Functional Core / Imperative Shell, Driver/Subsystem 隔離, etc.) は `client/` 内では従来通り厳守する。

## `orchestrator/` (Symphony 実装)

```
orchestrator/
  workflowfile/             # SPEC §3.1.1 §5: WORKFLOW.md parser
  wfconfig/                 # SPEC §3.1.2 §6: typed config + preflight + dynamic reload
  scheduler/                # SPEC §3.1.4 §7-8: poll/dispatch/retry/reconcile
                            #   (SPEC が "Orchestrator component" と呼ぶ scheduling brain)
  tracker/                  # SPEC §3.1.3 §11: platform/tracker/* を業務観点で使う薄い wrapper
  workspace/                # SPEC §3.1.5 §9: sanitize / 再利用 / 4 種 hooks
  agent/                    # SPEC §3.1.6 §10: platform/agent/codexclient を Issue 単位で使う wrapper
  prompt/                   # SPEC §5.4 §12: strict template renderer (issue + attempt)
  httpserver/               # SPEC §3.1.7 §13.7: dashboard + REST API
  metrics/                  # SPEC §13.5: token accounting / rate limits（platform/metrics へ昇格検討）
```

## `cmd/` (バイナリエントリ)

```
cmd/
  roost/
    main.go                 # 現 src/main.go を移動
  orchestrator/
    main.go                 # 新規
  claude-app-server/
    main.go                 # 新規 (shim binary)
```

## パッケージ境界（import 方向）

```
                    ┌─────────────────────────┐
                    │  cmd/{roost,orchestrator,claude-app-server}  │
                    └────────┬───────────┬─────┘
                             │           │
                  ┌──────────▼─┐     ┌───▼──────────┐
                  │  client/   │     │ orchestrator/│
                  └──────┬─────┘     └──────┬───────┘
                         │                  │
                         └────────┬─────────┘
                                  │
                              ┌───▼───┐
                              │platform/│
                              └────────┘
```

ルール:

- `platform/*` は **`client/*` `orchestrator/*` を import しない**
- `client/*` は **`orchestrator/*` を import しない** (逆も同様)
- `cmd/<name>/main.go` のみがバイナリ固有の wiring を持つ
- `platform/agent/codexclient/` は `cmd/claude-app-server/` も使える (server-side helper を含む)

import boundary 違反は `depguard` で検出する (現状 roost 内ルールの拡張)。

## 共有実装候補の判定表

| 候補 | roost で必要? | orchestrator で必要? | 判定 |
|---|---|---|---|
| tracker (Linear GraphQL client) | ◯ connector で Linear 読込 | ◯ dispatch | **`platform/tracker/linear/`**。両者は薄い wrapper |
| Codex stdio client | ◯ stream subsystem | ◯ §10 そのもの | **`platform/agent/codexclient/`**。`runtime/subsystem/stream/` から抽出 |
| Codex JSON Schema pin | ◯ stream で参照 | ◯ shim + client で参照 | **`platform/agent/codexschema/`** |
| pathmap | ◯ 既存 | ◯ container 内 agent で必須 | **`platform/pathmap/`** |
| token accounting | △ 表示用 (現状未実装) | ◯ §13.5 必須 | **`platform/metrics/`**。SPEC の absolute vs delta 判別を共有 |
| HTTP server | △ 将来 web view を作るなら | ◯ §13.7 必須 | **`platform/httpsurface/`** にルータ + middleware のみ。ハンドラは各 binary 側 |
| strict template renderer (Liquid 互換) | × | ◯ §5.4 §12 必須 | **orchestrator 専用** で開始。将来昇格余地 |
| hook script runner (`os/exec` + timeout) | × | ◯ §9.4 必須 | **orchestrator 専用** |
| WORKFLOW.md fsnotify reload | × | ◯ §6.2 必須 | **orchestrator 専用** (パターンは roost の fsnotify 利用を参考) |
| issue 状態機械 (Unclaimed/Claimed/Running/...) | × | ◯ §7.1 | **orchestrator 専用** |
| structured slog 初期化 + key=value 規約 | ◯ 既存 | ◯ §13.1 必須 | **`platform/logger/`** に key=value helper を追加 |
| issue 正規化型 | △ TUI 表示でも有用 | ◯ | **`platform/tracker/linear/types`** に正規化型 |
| claude transcript パーサ | ◯ 既存 (TUI 表示) | × (stream-json を使う) | **`platform/lib/claude/`** に残す。orchestrator 不使用 |

## claude-app-server shim の依存

shim binary は orchestrator とは別 process だが、同じ repo 内で build。以下を共有依存する:

- `platform/agent/codexschema/` (response 構築の schema)
- `platform/agent/codexclient/` の **server-side helper** (incoming JSON-RPC のデコード送信ヘルパ)
- `platform/lib/claude/` (stream-json reader; 必要なら新設)
- `platform/logger/`

→ `codexclient` は **client 専用にせず**、server-side framing 補助も提供する設計とする。

## 命名

| 概念 | 名称 |
|---|---|
| 我々の Symphony 実装全体 | "orchestrator service"、ディレクトリ `orchestrator/`、バイナリ `orchestrator` |
| SPEC §3.1.4 の component | "scheduler"、package `orchestrator/scheduler/` |
| 旧称 | `harness/`、`workflow/` は使わない |
| claude を Codex protocol で動かす shim | `claude-app-server`、binary `claude-app-server` |
| WORKFLOW.md 内 `codex.command` 設定例 | `codex.command: codex app-server` または `codex.command: claude-app-server` |

## Makefile target (想定)

```
make build                  # roost binary
make build-orchestrator     # orchestrator binary
make build-claude-app-server # shim binary
make build-all              # 全 binary
make vet                    # 全 packages
make lint                   # depguard / funlen / staticcheck (boundary 含む)
```
