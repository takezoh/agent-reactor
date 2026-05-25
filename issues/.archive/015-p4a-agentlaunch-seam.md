# 015: orchestrator/agent — route launch through agentlaunch.Dispatcher (direct mode)

- **Phase**: P4a ([plans/04-phases.md#p4-codexclient-経由--sandbox-配線](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: 013 (merged; agent runner)、P0b (merged; `platform/agentlaunch`)
- **Blocks**: 016 (devcontainer mode はこの seam を前提にする)

## Background

013 の agent runner は codex app-server を生の `exec.Command("bash", "-lc", codex.command)`（`proc.go`）で起動している。P4 の目的は launch を **`platform/agentlaunch.Dispatcher` 経由**にし、後続 016 で sandbox/devcontainer 配線を差し込めるようにすること。

013 で `codexclient` 経由化は完了済みのため、04-phases.md の P4 task 1/2（生 stdio → codexclient 置換、Issue 単位 wrapper）は不要。本 issue は **launch 経路に Dispatcher seam を入れるだけ**で、挙動は direct mode のまま（= 現状と等価）に保つ。

`agentlaunch` の既存 API（roost と共有）:

```go
type LaunchPlan struct { Command string; Env map[string]string; StartDir, Project string; ForceHost bool }
type WrappedLaunch struct { Command, StartDir string; Env map[string]string; Cleanup func(context.Context) error; ContainerSockDir string; Mounts []Mount }
type Dispatcher interface {
    Wrap(ctx, frameID string, plan LaunchPlan) (WrappedLaunch, error)
    AdoptFrame(...); EnsureProject(ctx, projectPath string) error; IsContainer(projectPath string) bool
}
// DirectDispatcher{SockPath} は plan を素通し（Command/StartDir/Env のみ、container env を除去）。
```

`WrappedLaunch` のコメントは既に "a direct stdio exec for the orchestrator" を想定している。

## Tasks

### A. Dispatcher seam を Runner に追加

- [x] `Runner` に `Dispatcher agentlaunch.Dispatcher` フィールドを追加。`New(...)` の既定は `agentlaunch.DirectDispatcher{SockPath: ...}`（passthrough = 挙動不変）
- [x] `frameID` の採番: attempt ごとに一意な識別子（例 `<issue.Identifier>#<attempt>`）。direct mode では Wrap は frameID を使わないが 016/ログのため一貫採番する
- [x] `Project` の決定: direct mode では未使用。016 で project root を入れるため、いまは `cfg.Workspace.Root`（または workspace path）を入れて TODO コメントで 016 を指す

### B. launch を Wrap 経由に変更

- [x] `launchConn`（`runner.go`）で、`r.proc(ctx, wsPath, cfg.Codex.Command)` の前に `LaunchPlan{Command: cfg.Codex.Command, Env: env, StartDir: wsPath, Project: ...}` を構築し `r.Dispatcher.Wrap(ctx, frameID, plan)` を呼ぶ
- [x] `proc` には **解決後の** `WrappedLaunch.Command` / `.StartDir` / `.Env` を渡す（`procFunc` を `(ctx, dir string, env map[string]string, command string)` に変更、もしくは `WrappedLaunch` を渡す）。`realProc` は `bash -lc <command>` を `Dir=StartDir`・環境 `Env` で起動
- [x] `WrappedLaunch.Cleanup`（非 nil のとき）を worker のティアダウンで実行 — turn 解決後の `cancel()` → reap 後に `Cleanup(ctx)` を best-effort で呼ぶ。`Worker` に cleanup を保持させ `Kill` 経路でも実行されるようにする
- [x] env の出所: 現状 env は未設定なので、最低限 `plan.Env` は空 map から始め、`DirectDispatcher` が `ROOST_SOCKET` 等を注入できるようにする（中身は 016 で拡張）

### C. cmd/orchestrator 配線

- [x] `cmd/orchestrator/main.go` で `agentlaunch.DirectDispatcher{SockPath: <orchestrator daemon sock or empty>}` を構築し `agent.New(..., dispatcher)` に渡す
- [x] direct mode が既定（016 でモード選択を追加）

### D. テスト

- [x] fake `Dispatcher` を注入し、`Wrap` が `plan.Command == cfg.Codex.Command` / `plan.StartDir == wsPath` で呼ばれることを検証
- [x] `WrappedLaunch.Cleanup` が turn 完了後（および Kill 経路）で 1 回呼ばれることを検証
- [x] `WrappedLaunch.Command` / `.Env` / `.StartDir` が proc に伝播することを検証
- [x] 既存 013 テスト（session_started/turn_completed、timeout、before_run 失敗）が DirectDispatcher 既定で緑のまま

## Acceptance Criteria

- direct mode で 013 と等価に動作（1 issue → workspace → 1 turn → workspace 残存）
- launch が `Dispatcher.Wrap` を必ず経由し、`Cleanup` がティアダウンで実行される
- `go test ./orchestrator/agent/` 緑、lint 緑、挙動回帰なし

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.1 (Launch Contract), §16.5 (Worker Attempt — agent process 起動), §15 (Security and Operational Safety — direct mode = host launch)
- [plans/04-phases.md#p4](../plans/04-phases.md)、[plans/02-layout.md](../plans/02-layout.md)（platform↛orchestrator 境界）
- `platform/agentlaunch`（`Dispatcher` / `DirectDispatcher` / `LaunchPlan` / `WrappedLaunch`）
