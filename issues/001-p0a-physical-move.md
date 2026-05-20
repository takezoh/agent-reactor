# 001: roost を client/ に、共有を platform/ に物理移動

- **Phase**: P0a ([plans/04-phases.md#p0a-物理移動](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: —
- **Blocks**: 002, 003, 004 (および後続全 Phase)

## Background

[plans/02-layout.md](../plans/02-layout.md) のディレクトリ構造に揃える物理移動。**挙動変更ゼロ** が絶対条件。後続全 Phase の前提となるため最優先で実施する。

src/ 直下に並んでいるパッケージを以下に振り分ける:

- 共有基盤 → `src/platform/<同名>/`
- roost 専用 → `src/client/<同名>/`
- バイナリエントリ → `src/cmd/roost/main.go`

## Tasks

### A. ファイル移動

- [x] `src/main.go` → `src/cmd/roost/main.go`
- [x] 共有基盤を `src/platform/` 配下に移動:
  - [x] `src/sandbox/` → `src/platform/sandbox/`
  - [x] `src/hostexec/` → `src/platform/hostexec/`
  - [x] `src/mcpproxy/` → `src/platform/mcpproxy/`
  - [x] `src/lib/pathmap/` → `src/platform/pathmap/`
  - [x] `src/lib/git/` → `src/platform/lib/git/`
  - [x] `src/lib/github/` → `src/platform/lib/github/`
  - [x] `src/lib/claude/` → `src/platform/lib/claude/` (core のみ; transcript は client/lib/claude/transcript/ へ)
  - [x] (他 lib/<tool>/ も同様: codex core, gemini, notify, openurl, plastic, tmux, vcs, wsl)
  - [x] `src/logger/` → `src/platform/logger/`
  - [x] `src/features/` → `src/platform/features/`
- [x] `src/config/` を分割:
  - [ ] 共有部 (SandboxResolver) → `src/platform/config/` — **P0b に延期** (SandboxConfig が client/config に依存するため循環回避)
  - [x] roost 専用部 → `src/client/config/` (sandbox_resolver.go を含む全体)
- [x] roost 専用を `src/client/` 配下に移動:
  - [x] `src/state/` → `src/client/state/`
  - [x] `src/runtime/` → `src/client/runtime/`
  - [x] `src/proto/` → `src/client/proto/`
  - [x] `src/tui/` → `src/client/tui/`
  - [x] `src/tools/` → `src/client/tools/`
  - [x] `src/driver/` → `src/client/driver/`
  - [x] `src/connector/` → `src/client/connector/`
  - [x] `src/event/` → `src/client/event/`
  - [x] `src/uiproc/` → `src/client/uiproc/`
  - [x] `src/cli/` → `src/client/cli/`
  - [x] `src/procio/` → `src/client/procio/`
  - [x] `src/winexec/` → `src/client/winexec/`
  - [x] `src/lib/peers/` → `src/client/lib/peers/` (client/event に依存するため platform 不可)
  - [x] `src/lib/claude/transcript/` → `src/client/lib/claude/transcript/` (client/state に依存)
  - [x] `src/lib/codex/transcript/` → `src/client/lib/codex/transcript/` (client/state に依存)

### B. import path 一括更新

- [x] 全 `.go` ファイルの import を新パスに更新 (sed + gofmt)
- [x] `go.mod` の module path は据え置き、subpackage path のみ書き換え

### C. Makefile / ビルド調整

- [x] `make build` の対象を `cmd/roost/` に変更
- [x] `make vet` `make lint` が全 packages を対象に変更
- [x] `make test` 相当が `go test ./...` で通ることを確認

### D. depguard / lint ルール更新

- [x] `.golangci.yml` の import boundary を更新:
  - `platform/*` は `client/*` `orchestrator/*` を import 禁止
  - `client/*` は `orchestrator/*` を import 禁止 (逆も)
  - `cmd/<name>/main.go` のみが各層を自由に wiring 可能
  - 注: platform/{sandbox,hostexec,mcpproxy} は client 型に依存するため depguard 除外扱い (P0b で解消予定)
- [x] 既存 depguard rule を新パスに追随

### E. ドキュメント追随

- [x] `ARCHITECTURE.md` 内の "Layer Structure" を新構成 (`platform/` `client/`) に書き換え
- [x] `AGENTS.md` 内の build コマンドは `make build` 抽象化済みのため変更不要
- [x] `docs/interfaces.md` 内のパス参照を更新

## Acceptance Criteria

- `make build` `make vet` `make lint` がすべて通る
- `cd src && go test ./...` が緑
- 既存 roost binary の挙動は変わらない (warm start / cold start / palette / IPC 全て)
- import boundary 違反が depguard で検出可能になっている
- `git mv` で rename detection が効き、blame が保たれている

## Notes

- 1 PR で全てやるか、(1) ファイル移動のみ (2) import path 更新 に分けるかは規模次第。**git rename detection を効かせるため、移動と内容変更は分離する**
- 移動時 PR には **他の変更を一切混ぜない** (review 容易性のため)
- 一時的に build が壊れる中間 commit を許す場合は CI を skip するブランチで作業

## References

- [plans/02-layout.md](../plans/02-layout.md) — ターゲットの完成形
- [plans/04-phases.md#p0a-物理移動](../plans/04-phases.md)
- [ARCHITECTURE.md](../ARCHITECTURE.md) — 現状の層構造
