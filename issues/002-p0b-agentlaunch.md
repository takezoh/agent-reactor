# 002: agentlaunch を runtime/ から platform/ へ抽出

- **Phase**: P0b ([plans/04-phases.md#p0b-agentlaunch-抽出](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: [001](001-p0a-physical-move.md)
- **Blocks**: 後続の orchestrator 実装で sandbox 配線が必要な Phase (P4 以降)

## Background

現在 `client/runtime/` (旧 `src/runtime/`) に存在する `SandboxDispatcher` / `DirectLauncher` / `DevcontainerLauncher` 等は **`runtime.Effect` と密結合**しているため、orchestrator binary から再利用できない。

これらを runtime 非依存の純粋関数群に切り出し、`platform/agentlaunch/` を介して **roost (client/runtime/) と orchestrator 両方が使える形** にする。

## Tasks

### A. 新パッケージ作成

- [ ] `src/platform/agentlaunch/` を新設
- [ ] 以下の型を定義:

```go
package agentlaunch

type LaunchPlan struct {
    Command  string
    Args     []string
    Env      map[string]string
    StartDir string
}

type Mount struct {
    Source   string
    Target   string
    ReadOnly bool
}

type WrappedLaunch struct {
    Command          string
    Args             []string
    Env              map[string]string
    Mounts           []Mount
    ContainerSockDir string
    Cleanup          func(context.Context) error
}

type Dispatcher interface {
    Wrap(ctx context.Context, project string, plan LaunchPlan) (WrappedLaunch, error)
}
```

### B. runtime からの移動

- [ ] `client/runtime/sandbox_dispatcher.go` の `SandboxDispatcher` を `platform/agentlaunch/` へ移動
- [ ] `client/runtime/direct_launcher.go` 相当 (DirectLauncher) を移動
- [ ] `client/runtime/devcontainer_launcher.go` 相当 (DevcontainerLauncher) を移動
- [ ] `WrappedLaunch` / `LaunchPlan` の型定義を `platform/agentlaunch/` 配下へ集約
- [ ] tmux 由来の hint (frame, pane, target 等) を `LaunchPlan` から除去。**tmux 知識ゼロ** にする

### C. client/runtime/ 側のアダプタ化

- [ ] `client/runtime/` に薄い adapter を残し、`platform/agentlaunch/` を呼ぶ形へ書き換え
- [ ] `EffSpawnTmuxWindow` の effect 変換は client/runtime/ 側に残す (subsystem 固有の責務)
- [ ] subsystem cli / stream が新 API を経由するように修正

### D. テスト

- [ ] `platform/agentlaunch/` 単独のテスト群を追加 (Dispatcher.Wrap の direct / devcontainer 分岐、Cleanup 呼出順序)
- [ ] 既存 runtime テスト群が通ることを確認 (挙動変更ゼロ)

### E. boundary

- [ ] depguard ルールを更新:
  - `platform/agentlaunch/` は `client/*` `orchestrator/*` を import 禁止
  - `platform/agentlaunch/` は `platform/sandbox/` `platform/config/` `platform/lib/` のみ依存
- [ ] `runtime/isolation_test.go` 相当を更新

## Acceptance Criteria

- roost の挙動変更ゼロ (warm/cold start、direct/devcontainer 両 mode で動作)
- `platform/agentlaunch/` を import するだけで orchestrator から `Dispatcher.Wrap(...)` が呼べる
- `Dispatcher.Wrap(...)` の戻り値 `WrappedLaunch` を `os/exec.Cmd` 化するサンプルを test or godoc で提示
- 単体テストで Cleanup が deferred 呼出順を保証
- depguard / boundary test が緑

## Notes

- `Subsystem` interface は引き続き `client/runtime/subsystem/` に残す。`agentlaunch` は **その下のレイヤ** (低レベル wrap-and-launch primitive)
- `SandboxResolver` (per-project mode 解決) は `platform/config/` 側にすでにあるはずなので、`Dispatcher` 実装が config を受け取る形にする
- bearer token 生成・container socket dir 生成等は `platform/sandbox/` 側に既存

## References

- [plans/02-layout.md#共有実装候補の判定表](../plans/02-layout.md)
- [plans/04-phases.md#p0b-agentlaunch-抽出](../plans/04-phases.md)
- `client/runtime/sandbox_dispatcher.go` (現状実装)
- `docs/sandbox.md`
