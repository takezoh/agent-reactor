# 002: agentlaunch を runtime/ から platform/ へ抽出

- **Phase**: P0b ([plans/04-phases.md#p0b-agentlaunch-抽出](../plans/04-phases.md))
- **Status**: Closed ✓
- **Depends on**: [001](001-p0a-physical-move.md)
- **Blocks**: 後続の orchestrator 実装で sandbox 配線が必要な Phase (P4 以降)

## Background

`client/runtime/` に存在した `SandboxDispatcher` / `DirectLauncher` / `DevcontainerLauncher` 等を
`platform/agentlaunch/` へ切り出し、orchestrator binary から `Dispatcher.Wrap(...)` を呼べる形にした。

実装は 3 ステップで構成される:

1. **Step 1 — config 分割**: sandbox 設定群 (`SandboxConfig`, `SandboxResolver`, `MergeSandbox` 等) を `client/config` から `platform/config` へ移設。
2. **Step 2 — platform/sandbox 脱 state**: `sandbox.Manager.BuildLaunchCommand` の `state.LaunchPlan` 引数を `sandbox.LaunchSpec{Command, StartDir}` に置換し、depguard blanket 除外を完全削除。
3. **Step 3 — agentlaunch 抽出**: launcher 群・credproxy・rundir を `platform/agentlaunch/` / `platform/credproxy/` へ移設。`client/runtime/launcher.go` を `dispatcherAdapter` に書き換え。

## Tasks

### A. 新パッケージ作成

- [x] `src/platform/agentlaunch/` を新設
- [x] 以下の型を定義 (最終型は以下):

```go
package agentlaunch

type LaunchPlan struct {
    Command   string
    Env       map[string]string
    StartDir  string
    Project   string
    ForceHost bool  // state.SandboxOverrideHost の代替
}

type Mount struct { Host, Container string }

type WrappedLaunch struct {
    Command, StartDir string
    Env               map[string]string
    Cleanup           func(context.Context) error
    ContainerSockDir  string
    Mounts            []Mount
}

type Dispatcher interface {
    Wrap(ctx context.Context, frameID string, plan LaunchPlan) (WrappedLaunch, error)
    AdoptFrame(ctx context.Context, frameID, projectPath string) (func(context.Context) error, []Mount, error)
    EnsureProject(ctx context.Context, projectPath string) error
    IsContainer(projectPath string) bool
}
```

### B. runtime からの移動

- [x] `SandboxDispatcher` → `platform/agentlaunch/dispatcher_mode.go`
- [x] `DirectLauncher` → `platform/agentlaunch/direct.go` (`DirectDispatcher` に改名)
- [x] `DevcontainerLauncher` → `platform/agentlaunch/devcontainer.go`
- [x] `WrappedLaunch` / `LaunchPlan` / `Mount` → `platform/agentlaunch/types.go`
- [x] `CredProxyRunner` → `platform/credproxy/credproxy.go`
- [x] `rundir.go` / `bridge.go` → `platform/agentlaunch/`
- [x] `state.LaunchPlan` 依存を `agentlaunch.LaunchPlan` + `ForceHost bool` で置換

### C. client/runtime/ 側のアダプタ化

- [x] `client/runtime/launcher.go` を `dispatcherAdapter` に書き換え
- [x] `runtime.NewDispatcherAdapter(d agentlaunch.Dispatcher) AgentLauncher` を提供
- [x] `cmd/roost/coordinator.go` を `agentlaunch.*` + `credproxy.Start` で配線

### D. テスト

- [x] `platform/agentlaunch/{devcontainer_test,devcontainer_flow_test,dispatcher_mode_test}.go` を新設
- [x] `platform/credproxy/credproxy_test.go` を新設
- [x] 全テスト通過 (`go test ./...` 緑)

### E. boundary

- [x] depguard blanket 除外 (`platform/sandbox/`, `platform/hostexec/`, `platform/mcpproxy/`) を完全削除
- [x] `platform-no-client-or-orchestrator` が包括除外なしで緑
- [x] `grep -rn "client/" src/platform/` が空 (test 含む)

## Acceptance Criteria

- [x] roost の挙動変更ゼロ (warm/cold start、direct/devcontainer 両 mode で動作)
- [x] `platform/agentlaunch/` を import するだけで `Dispatcher.Wrap(...)` が呼べる
- [x] `go list -deps ./platform/agentlaunch ./platform/credproxy` に `client/` が出ない
- [x] depguard / lint が緑

## Implementation Notes

- **前提の訂正**: issue 本文の「`SandboxResolver` は `platform/config` にある」「token/sockdir 生成は `platform/sandbox` にある」は誤りで、いずれも `client/config` / `client/runtime` にあった。config 分割 (Step 1) が前提タスクとなった。
- **`ForceHost`**: `state.SandboxOverrideHost` は client 固有概念なので agentlaunch には持ち込まず `ForceHost bool` で写像。
- **credproxy cycle 回避**: `platform/credproxy` はコンテナパスを `Paths{RunDir, BinPath, MCPSock}` 引数で受け取り、`platform/agentlaunch` への循環 import を回避。
- **Subsystem/Stream 削除**: `WrappedLaunch.Subsystem`/`.Stream` は spawn consumer (`interpret.go`) で未使用であることを確認の上削除。
- **`ResolveFrameContext` 公開**: platform 側でテスト可能にするため `resolveFrameContext` → `ResolveFrameContext` にエクスポート。
- **`ContainerExecInfo` 追加**: `stream_backend.go` が必要とする `mgr.EnsureInstance` アクセスを公開 API `DevcontainerLauncher.GetContainerExecInfo(ctx, project)` 経由に置換。

## References

- [plans/02-layout.md#共有実装候補の判定表](../plans/02-layout.md)
- [plans/04-phases.md#p0b-agentlaunch-抽出](../plans/04-phases.md)
- `src/platform/agentlaunch/` (実装)
- `src/platform/credproxy/` (実装)
- `src/client/runtime/launcher.go` (adapter)
