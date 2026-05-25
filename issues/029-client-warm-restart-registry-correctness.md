# 029: client/runtime — warm-restart container registry correctness (token-without-mounts window + Save/Delete race)

- **Phase**: client-runtime follow-up（single-writer port 由来。Symphony SPEC 範囲外）
- **Status**: Open
- **Depends on**: orchestrator-migration → main マージ（single-writer port）
- **Blocks**: なし

## Background

warm restart（daemon 再起動でも container が生存している経路）の登録順に、
spawn 経路で閉じた token-without-mounts の窓が残っている。

`RecoverSandboxFrames`（`bootstrap.go`）の流れ:

1. `recoverWarmTokens` が全 live frame の token を**先に** `frameReg.Register`（token のみ、mounts なし）。
2. per-frame ループで `AdoptFrame` → `frameReg.StoreMounts` → `startContainerEndpointIfNeeded`。

同一 project の 2 個目以降の frame は、1 個目の反復で endpoint が起動済みなのに、
自身の `StoreMounts` がまだ走っていない窓を持つ。warm の container は生存しており
即座に hook を送り得るため、その hook は `Lookup(token)` 成功 / `GetMounts(frame)` 失敗
となり、`translatePayloadPaths` が変換をスキップして **container 相対パスが下流へ漏れる**。
spawn 経路は `RegisterWithMounts` で token+mounts を atomic 登録するが、warm 経路は
未対応（orchestrator-migration から既存）。本 port の目的（token-without-mounts の窓を
閉じる）が warm 経路では未達。

### 関連 (F4): warm Save/Delete のファイル競合

`registerContainerFrame` の `go func(){ wfStore.Save(wf) }`（非同期）と
`executeKillSessionWindow` の同期 `warmFrames.Delete(frame)` が同一 warm ファイルを
競合し得る。spawn 直後に kill すると Save が Delete を追い越し、死んだ frame の warm
ファイルが残る。影響は限定的（次回 warm 起動の `recoverWarmTokens` orphan pruning で
除去される）が、順序保証がない。

### 関連 (F8): framereg の token 衝突非検出

`framereg.Register` / `RegisterWithMounts` は別 frame に同一 token を登録した際の衝突を
検出しない（`tokenToFrame` が黙って rebind、片方の `frameToToken` が orphan 化）。
唯一の現実的経路は warm の永続 token 再利用だが、token は 32byte 乱数で実質非到達。
旧 `tokenStore` からの latent。

## Tasks

- [ ] warm path で token+mounts を atomic に登録する: per-frame ループで `AdoptFrame`
      後に該当 warm token を引いて `frameReg.RegisterWithMounts(frame, token, mounts)`
      を使う、または全 frame の adopt+登録を終えてから endpoint を起動する second pass
      に分離する。`recoverWarmTokens` の先行一括 Register を見直す。
- [ ] (F4) warm `Save`/`Delete` の順序保証（Save 同期化 vs loop ブロック回避のトレードオフ
      を検討）か、Save を frame-id 単位で直列化。
- [ ] (F8) 必要なら同一 token の重複登録に対する防御（debug assert / log）を追加。
- [ ] test: warm restart で同一 project 複数 frame を復元し、後続 frame の早期 hook が
      `GetMounts` 成功（host 変換済み）になること。

## Acceptance Criteria

- warm restart 後、同一 project の複数 frame いずれの早期 container hook も host 変換済み
  パスで届く（token-without-mounts の窓なし）。
- spawn 直後 kill で warm ファイルが残留しない（または次回起動で確実に pruning される
  ことの明示）。
- `go test ./client/runtime/...` 緑（`-race` 含む）、lint 緑。

## References

- roost client runtime — Symphony SPEC 範囲外。source of truth は
  [ARCHITECTURE.md](../ARCHITECTURE.md) "Core principles → Single-writer event loop"。
- `src/client/runtime/bootstrap.go`（`RecoverSandboxFrames`, `recoverWarmTokens`）、
  `src/client/runtime/framereg/registry.go`、`src/client/runtime/cleanup.go`
  （`registerContainerFrame`）、`src/client/runtime/warm_state.go`
- 由来: 1179fcf single-writer port の code review（F6 / F4 / F8）。
