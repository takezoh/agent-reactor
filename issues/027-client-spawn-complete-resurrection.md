# 027: client/runtime — handleSpawnComplete resurrects a frame killed mid-spawn

- **Phase**: client-runtime follow-up（orchestrator-migration → main マージ後の single-writer port 由来。Symphony SPEC 範囲外）
- **Status**: Open
- **Depends on**: orchestrator-migration → main マージ（runtime spawn パスの single-writer port）
- **Blocks**: なし（既存の稀な leak。即時のビルド/機能影響なし）

## Background

single-writer port で `spawnTmuxWindow`（`interpret.go`）は loop 外の goroutine で
subsystem ensure → `BindFrame` → `WrapLaunch` → `SpawnWindow` を実行する。これらは
worktree 作成や container 起動を含み**数百 ms〜秒**かかり得る。完了は
`internalSpawnComplete` で loop に渡り、`handleSpawnComplete` が状態を登録する。

この実行中に session / frame が削除されると競合が起きる:

1. `EffKillSessionWindow` が先に loop で処理される。その時点で `frameSubsystems` /
   `frameSubsystemIDs` / `sandboxCleanups` はまだ空（`handleSpawnComplete` 未実行）の
   ため、`executeKillSessionWindow` は何も解放しない（`frameReg.Delete` も no-op）。
2. 後着の `handleSpawnComplete` が**死んだ frame に対して**
   `subsystems` / `frameSubsystems` / `frameSubsystemIDs` / `sandboxCleanups` を書き戻し、
   `token != ""` なら `registerContainerFrame` で token+mounts 登録・container endpoint
   起動・warm-frame save まで行う。
3. `EvTmuxPaneSpawned` の reducer は session 不在で no-op。よってこれらは二度と解放
   されず **leak** する（subsystem backend プロセス、container/worktree、endpoint
   listener、warm file、cleanup closure）。

旧 orchestrator-migration の `spawnTmuxWindowAsync` も map を goroutine 内で直接書いて
いたため同種の leak（かつ data race 付き）があった。本 port は data race を解消した一方、
kill-before-complete の resurrection leak は残っている。`internalSpawnComplete` の
delivery を reliable 化した（drop しない）ことで、kill 先行時に必ず resurrection 経路を
通るようになった点も併せて要対処。

## Tasks

- [ ] `handleSpawnComplete` の冒頭で frame/session が現存するか確認する
      （`r.state.Sessions[e.effect.SessionID]` とその frame の存在）。
- [ ] 不在なら状態登録を行わず、goroutine が取得済みのリソースを解放する:
      `e.cleanup()`（sandbox/container release）、`e.sub` の停止または
      `ReleaseFrame`、spawn 済み tmux pane（`e.paneID`）の kill。loop を
      ブロックしないよう必要に応じて goroutine 化する。
- [ ] 二重解放にならないこと（token は未登録のまま破棄、`frameReg` には書かない）を保証。
- [ ] test: in-flight spawn 中に `EffKillSessionWindow` が先行した場合、
      `handleSpawnComplete` が登録を skip し cleanup を呼ぶこと（fake subsystem で
      Stop/ReleaseFrame と cleanup closure の呼び出しを観測）。

## Acceptance Criteria

- spawn 実行中に session を削除しても、subsystem / container / endpoint / warm file が
  leak しない。
- 通常の spawn（kill なし）の挙動は不変。
- `go test ./client/runtime/...` 緑（`-race` 含む）、lint 緑。
- 実機: container session を spawn 中に削除 → container/worktree が解放されることを確認。

## References

- roost client runtime — Symphony SPEC 範囲外。source of truth は
  [ARCHITECTURE.md](../ARCHITECTURE.md) "Core principles → Single-writer event loop"。
- `src/client/runtime/interpret.go`（`spawnTmuxWindow`, `handleSpawnComplete`,
  `executeKillSessionWindow`, `reapSubsystemIfLast`）
- `src/client/runtime/cleanup.go`（`registerContainerFrame`, `invokeFrameCleanup`）
- 由来: orchestrator-migration マージ後の 1179fcf single-writer port の code review（F2）。
