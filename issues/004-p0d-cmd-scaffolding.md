# 004: cmd/orchestrator/ と cmd/claude-app-server/ の雛形 + Makefile

- **Phase**: P0d ([plans/04-phases.md#p0d-cmd-整備](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: [001](001-p0a-physical-move.md)
- **Blocks**: P1 以降 (orchestrator バイナリへの実装追加)

## Background

3 バイナリ (`roost` / `orchestrator` / `claude-app-server`) を同一 module から build できるようにする。雛形 main.go と Makefile target を整備するのみで、機能実装はまだ含まない。

001 (P0a) で `src/cmd/roost/main.go` は配置済みの前提。本 issue では残り 2 バイナリのエントリを追加する。

## Tasks

### A. orchestrator バイナリ雛形

- [x] `src/cmd/orchestrator/main.go` を作成。最低限:
  - [x] CLI flag: `--workflow <path>` (default: `./WORKFLOW.md`), `--port <int>` (HTTP server port; future)
  - [x] WORKFLOW.md 存在チェック (なければ exit 非ゼロ + operator-visible error)
  - [x] SIGTERM / SIGINT で graceful shutdown
  - [x] **scheduler 実装は stub** (P1 で埋める)。loop は持たず、起動・終了のみ
  - [x] log は `platform/logger/` を使って structured 出力

### B. claude-app-server バイナリ雛形

- [x] `src/cmd/claude-app-server/main.go` を作成。最低限:
  - [x] stdio で JSON-RPC `initialize` メッセージに応答 (capability 宣言)
  - [x] 未実装 method は SPEC §10.4 が要請する形で error 応答
  - [x] SIGTERM で graceful shutdown
  - [x] `platform/agent/codexclient/` の server helper を import するだけ (003 完了が前提)
- [x] 003 がまだ merge されていない場合は **本 issue を 003 の merge 後に着手** (依存解決) → 003 は merge 済みのため直接 codexclient を import

### C. Makefile

- [x] target 追加:

```makefile
build-orchestrator:
\tgo build -C src -o ../orchestrator ./cmd/orchestrator

build-claude-app-server:
\tgo build -C src -o ../claude-app-server ./cmd/claude-app-server

build-all: build build-orchestrator build-claude-app-server
```

- [x] `make build` (既存 roost) は据え置き
- [x] CI で `make build-all` を最低限通す → CI は go build 直接呼び出しで対応 (make build が credproxy/sockbridge の go.work replace を前提とするため)

### D. ドキュメント

- [x] `AGENTS.md` の build セクションに新 target を追加
- [x] README に 3 バイナリの存在と関係を 2-3 行で記載

### E. ディレクトリ pre-cleanup

- [x] `cmd/orchestrator/` `cmd/claude-app-server/` 配下に `README.md` を 1 行で配置
- [x] gitignore で生成 binary を除外 (`orchestrator` `claude-app-server` を追加)

## Acceptance Criteria

- [x] `make build-all` で 3 バイナリが生成される
- [x] `./orchestrator --workflow ./WORKFLOW.md` が起動 → graceful shutdown できる
- [x] `./claude-app-server` を起動して `initialize` JSON-RPC を送ると capability 応答が返る
- [x] CI が全 binary を build できる

## Notes

- 003 がまだ merge されていない場合は claude-app-server は仮 stub (`fmt.Println("not implemented")` + exit) でも可 → 003 merge 済みのため codexclient を実際に使用
- orchestrator の loop / scheduler は実装しない。**雛形のみ**
- WORKFLOW.md parser は P1 で実装するため、本 issue では「ファイル存在チェック」だけで十分

## References

- [plans/02-layout.md](../plans/02-layout.md)
- [plans/04-phases.md#p0d-cmd-整備](../plans/04-phases.md)
- [AGENTS.md](../AGENTS.md) — 既存 build コマンド規約
