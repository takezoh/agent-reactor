# 028: client/runtime — container endpoint/token registered after agent spawn (early-hook loss window)

- **Phase**: client-runtime follow-up（single-writer port 由来。Symphony SPEC 範囲外）
- **Status**: Open
- **Depends on**: orchestrator-migration → main マージ（single-writer port）
- **Blocks**: なし。まず**実機検証で実害の有無を判定**する investigate 系 issue

## Background

orchestrator-migration では `wrapWithContainerToken` が `SpawnWindow`（= container
agent の起動）の**前に** token+mounts を登録し container endpoint を起動していた。
single-writer port（1179fcf）でこれらの状態変更は `handleSpawnComplete`（spawn 完了後・
event loop 上）へ移動した。`registerContainerFrame` は `SpawnWindow` 後に走る。

結果、container 内 agent が起動直後に hook を送ると、

- container endpoint がまだ listen していない（`startContainerEndpointIfNeeded` 未実行）→ dial 失敗、または
- listener はあるが token 未登録（`frameReg.RegisterWithMounts` 未実行）→ `Lookup` 失敗で `invalid token`

となり得る窓がある。container 側 sender（`src/client/event`）は **retry せず warning
log のみで drop** するため、その hook event は失われる。

窓は短い（`SpawnWindow` 返却 → `internalSpawnComplete` → loop が `handleSpawnComplete`
処理）一方、agent の起動から最初の hook までは通常それより長いため、1179fcf でも実用上
問題にならなかったと推測される。ただし orchestrator-migration（spawn 前登録）からの
**ordering 変更**であり、early-hook 取りこぼしの実害は実機で確認すべき。

single-writer 原則（状態変更は loop 所有）と「endpoint を agent 起動前に用意」を両立する
には、loop 上で token を**先行生成・先行登録**し endpoint を spawn goroutine 起動前に
立てる設計が要る。ただし mounts は `WrapLaunch` 後にしか判明しないため、token-without-mounts
の TOCTOU（`RegisterWithMounts` の atomic 性で閉じたもの）と再びトレードオフになる。

## Tasks

- [ ] **実機検証**（plan の roost 実機検証と統合）: cold-start で devcontainer-sandbox
      frame を起動し、in-container agent の**最初の** hook（SessionStart 等）が
      host 変換済み `cwd`/`transcript_path` で届くか、取りこぼし窓が顕在化するかを確認。
- [ ] 実害が確認された場合の設計検討:
      - loop 上で token 生成 + `frameReg.Register`(token) + `startContainerEndpointIfNeeded`
        を spawn goroutine 起動前に実行し、mounts は `handleSpawnComplete` で
        `StoreMounts` で後追い（warm path と同型）。
      - もしくは agent 起動を endpoint 準備完了まで遅延させる barrier。
      - いずれも 027 / 029 の atomic 登録方針と整合させる。
- [ ] 採用案に test を追加。

## Acceptance Criteria

- 実機: cold-start container frame の**最初の** hook が host 変換済みパスで daemon に届く
  （取りこぼしなし）。
- single-writer 原則を破らない（状態変更は loop 上のまま）。
- 既存 spawn 挙動・テストにリグレッションなし。

## References

- roost client runtime — Symphony SPEC 範囲外。source of truth は
  [ARCHITECTURE.md](../ARCHITECTURE.md) "Core principles → Single-writer event loop"。
- `src/client/runtime/interpret.go`（`spawnTmuxWindow`, `handleSpawnComplete`）、
  `src/client/runtime/cleanup.go`（`registerContainerFrame`）、
  `src/client/runtime/ipc_container.go`（`startContainerEndpointIfNeeded`）、
  `src/client/event`（container 側 hook sender / retry なし）
- 由来: 1179fcf single-writer port の code review（F3）。
