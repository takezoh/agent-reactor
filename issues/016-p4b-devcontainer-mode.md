# 016: orchestrator/agent — devcontainer/sandbox mode + host↔container path translation

- **Phase**: P4b ([plans/04-phases.md#p4-codexclient-経由--sandbox-配線](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 015 (Dispatcher seam)、P0b (merged; `platform/agentlaunch` の `DevcontainerLauncher`)
- **Blocks**: M2 (多 agent 対応の前提)、P5 (claude-app-server shim を container 内で動かす)

## Background

015 で launch が `agentlaunch.Dispatcher` 経由になった。本 issue は **devcontainer モード**を配線し、codex app-server を container 内で起動する。`DevcontainerLauncher.Wrap` は `docker exec ...` 文字列を `WrappedLaunch.Command` に返し、cwd を **container パスに変換**（`pathmap` 経由）し、`ContainerSockDir` / `Mounts` で host サービス（hostexec/mcpproxy/credproxy）への到達を提供する。

`DevcontainerLauncher` 構築シグネチャ（roost と共有）:

```go
func NewDevcontainerLauncher(
    mgr sandbox.Manager[*sandboxdc.ContainerState],
    resolveSandbox func(string) config.SandboxConfig,
    resolveProjectScope func(string) *config.SandboxConfig,
    proxy *credproxy.Runner,
    dataDir string,
) *DevcontainerLauncher
```

## 設計判断（要決定 — 実装前に PR で確定）

**devcontainer/sandbox 設定の出どころ**。`wfconfig`（WORKFLOW.md）には sandbox/devcontainer フィールドが無く、SPEC は container 隔離を §15 の RECOMMENDED hardening（SPEC 外の拡張）として扱う。候補:

1. **roost の `~/.roost/` project config を再利用**（`config.SandboxConfig` / `config.DevcontainerConfig` を project path で解決）— **推奨**。platform を再利用し roost と同じ devcontainer 解決にそろう。[plans/05-conformance.md](../plans/05-conformance.md) に「devcontainer-default は SPEC §15 準拠の拡張」として deviation を明記する
2. orchestrator 専用の config ファイルを新設（WORKFLOW.md とは別。設定の二重管理になる）
3. WORKFLOW.md に拡張フィールド追加（SPEC 非準拠の front matter 拡張になる。tracker が読む repo-owned policy に launch 詳細を載せる是非）

→ 既定は **1**。`project` は workspace が属する repo root を解決して渡す。

## Tasks

### A. Dispatcher 構築とモード選択

- [ ] `cmd/orchestrator` で `Mode`（`direct` | `devcontainer`）を選択（CLI flag もしくは roost config）。既定は `direct`（015 のまま）
- [ ] devcontainer モード時、上記決定 1 に従い `sandbox.Manager` + `credproxy.Runner` + config resolver + `dataDir` を組み立て `agentlaunch.NewDevcontainerLauncher(...)` を構築し `agent.New(..., dispatcher)` へ注入
- [ ] 起動時に `dispatcher.EnsureProject(ctx, projectPath)` を呼び warm up。`ColdStartAware` を満たす場合は cold-start ウィンドウで `BeginColdStart`/`EndColdStart` を呼ぶ

### B. host↔container path 変換（§9.5 / §10.2 の整合）

- [ ] codex の thread cwd に **container パス**を渡す: `codexclient.StartTurn(conn, "", wrapped.StartDir, ...)`（現状は host の `wsPath` を渡している。`DevcontainerLauncher.Wrap` が `pathmap.ToContainer` 済みの `StartDir` を返す）
- [ ] `workspace.VerifyCWD`（§9.5 Inv1）は **host パス**で実施（host 側 invariant）。container 側 cwd は launcher が保証する
- [ ] direct mode では `wrapped.StartDir == wsPath`（host）のまま — 分岐不要で両モードが同一コードで通る

### C. sock/mounts 配線

- [ ] `WrappedLaunch.ContainerSockDir` と `Mounts` を agent runner で受け取り、container 内 agent が host サービス（hostexec broker / mcpproxy / credproxy）へ到達できるよう sock bridge を配線（roost の bridge パターン参照: `agentlaunch.ContainerBridgeSpec` / `InstallSockBridgeInRunDir`）
- [ ] `Mounts`（host↔container ペア）が IPC 境界の path 変換に必要なら runner/observability に保持

### D. ティアダウン

- [ ] `WrappedLaunch.Cleanup` が frame release / container frame ティアダウンを行う（015 で配線済みの cleanup 経路を devcontainer でも検証）

### E. テスト

- [ ] fake `sandbox.Manager` で `DevcontainerLauncher.Wrap` を回し、`StartTurn` に渡る cwd が **container パス**であることを検証
- [ ] direct mode では cwd が host パス（`wsPath`）のまま回帰しないこと
- [ ] `EnsureProject` が起動時に呼ばれること
- [ ] `Cleanup` が frame を release すること（fake Manager の acquire/release を観測）

## Acceptance Criteria

- devcontainer モードで 1 issue end-to-end: container 内で codex app-server 起動 → container cwd で 1 turn → workspace 残存
- direct モードが 015 と等価に回帰なし
- container 内 agent から host MCP/hostexec が見える（mcpproxy/hostexec/credproxy が動く）
- §15 isolation の posture と devcontainer-default deviation を [plans/05-conformance.md](../plans/05-conformance.md) に明記
- `go test ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.1 (Launch Contract), §10.2 (cwd = workspace), §9.5 (Inv1 cwd 検証), §15 (Security and Operational Safety — container/VM 隔離は RECOMMENDED hardening), §16.5
- [plans/04-phases.md#p4](../plans/04-phases.md)、[plans/05-conformance.md](../plans/05-conformance.md)（deviation: devcontainer-default）
- `platform/agentlaunch`（`DevcontainerLauncher` / `SandboxDispatcher` / `pathmap` 変換 / `ContainerSockDir`）、`platform/sandbox`、`platform/mcpproxy`、`platform/hostexec`
