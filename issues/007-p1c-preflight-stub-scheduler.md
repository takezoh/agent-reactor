# 007: dispatch preflight + stub scheduler loop

- **Phase**: P1c ([plans/04-phases.md#p1-workflow-loader--wfconfig--preflight](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: [006](006-p1b-wfconfig.md) (typed config を検証する)
- **Blocks**: P2 (Linear adapter)、P3 (scheduler core が本 loop を肉付けする)

## Background

SPEC §6.3 の dispatch preflight と、SPEC §16.2 の骨格となる **stub scheduler loop** を実装し、
`cmd/orchestrator/main.go` を loader → wfconfig → preflight → loop で配線する。
本フェーズの loop は **dispatch しない** — poll interval ごとに「候補を取得せず dispatch もしない」旨を
log するだけ。実際の poll/dispatch は P3 で肉付けする。

## Tasks

### A. preflight validation (SPEC §6.3)

- [ ] `src/orchestrator/scheduler/` を新設 (`package scheduler`)
- [ ] `Preflight(cfg wfconfig.Config) error` を実装。検証項目:
  - [ ] workflow file が load/parse 済み (loader 段で担保 → 呼び出し順で保証)
  - [ ] `tracker.kind` が present かつ supported (`linear`)
  - [ ] `tracker.api_key` が `$` 解決後に present
  - [ ] `tracker.project_slug` が present (kind=linear で REQUIRED)
  - [ ] `codex.command` が present かつ非空
- [ ] 失敗は **operator-visible error** (slog.Error + stderr) として表面化

### B. stub scheduler loop (SPEC §16.2 骨格)

- [ ] `Scheduler.Run(ctx) error`:
  - [ ] **起動時 preflight** → 失敗なら起動失敗 (非ゼロ exit) + operator-visible error
  - [ ] `polling.interval_ms` ごとに tick
  - [ ] 各 tick で **per-tick preflight 再検証** → 失敗なら dispatch skip + reconcile 継続 (現状 reconcile は no-op) + operator-visible error。成功時は「dispatch なし」を log
  - [ ] `ctx.Done()` で graceful shutdown
- [ ] dispatch / candidate fetch は **未実装** (P3)。loop だけ回す

### C. cmd/orchestrator 配線

- [ ] `cmd/orchestrator/main.go` を更新:
  - [ ] workflow path precedence (SPEC §5.1): `--workflow` 明示 > cwd の `WORKFLOW.md`
  - [ ] `workflowfile.Load` → `wfconfig.Resolve` → `scheduler.New(cfg)` → `scheduler.Run(ctx)`
  - [ ] 既存の SIGTERM/SIGINT graceful shutdown を維持
  - [ ] `--port` は P7 まで保持 (未使用)

### D. テスト (SPEC §17.1, §17.7)

- [ ] preflight の各失敗ケース (kind 欠落/未対応、api_key 欠落、project_slug 欠落、command 空) の test
- [ ] loop の tick → graceful shutdown を fake clock / 短 interval + context cancel で test
- [ ] `cmd/orchestrator` の起動失敗パス (preflight NG で非ゼロ exit) の test

## Acceptance Criteria

- `orchestrator --workflow ./WORKFLOW.md` が起動 → loop → SIGTERM で graceful shutdown
- 不正 WORKFLOW.md / 設定で `missing_workflow_file` / `workflow_parse_error` / preflight 失敗を operator-visible に返し、起動失敗する
- 起動後の WORKFLOW.md 不正化は当該 tick の dispatch を skip (本フェーズは dispatch 自体が no-op だが skip 経路を通す)
- SPEC §17.1 の preflight 系・§17.7 の CLI lifecycle 項目を test で pass
- `go test ./orchestrator/scheduler/ ./cmd/orchestrator/` 緑、lint 緑

## Notes

- hot reload (fsnotify, SPEC §6.2 / §17.1「changes detected without restart」) は **P8**。本フェーズは tick ごとの再 load/再 preflight までで、ファイル監視はしない
- reconcile の中身は P3/P6。ここでは「dispatch skip しても reconcile 経路は止めない」構造だけ用意

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §6.3, §16.2, §17.1, §17.7
- [plans/04-phases.md#p1](../plans/04-phases.md)
- [006](006-p1b-wfconfig.md) — 入力となる typed config
