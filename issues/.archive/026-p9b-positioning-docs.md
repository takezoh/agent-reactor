# 026: orchestrator サービスの位置付けを agent-roost doc に追記

- **Phase**: P9b ([plans/04-phases.md#p9-conformance-tests--docs](../plans/04-phases.md))
- **Status**: Done (2026-05-21)
- **Depends on**: M3 機能完成（P6–P8a 済）。[025](025-p9a-conformance-suite.md)（conformance 表）先行が望ましいが独立着手可
- **Blocks**: M4 完成

## Background

orchestrator は agent-roost 同一リポジトリ内に同居する Symphony SPEC 実装バイナリだが、リポジトリ正本の `AGENTS.md` / `ARCHITECTURE.md` には orchestrator サービスの位置付け（3 バイナリ構成・三層境界・責務）が明記されていない。新規参加者やツールが orchestrator の役割と build/test 導線を辿れるよう、本 issue で doc を整備する。

## Tasks

### A. `AGENTS.md` に orchestrator サービスの節を追加

- [ ] 3 バイナリ（`roost` / `orchestrator` / `claude-app-server`）の役割を明記
- [ ] orchestrator が Symphony SPEC（poll/dispatch/reconcile + observability HTTP）の実装である旨
- [ ] build/test 導線（既存 `make build-all` / `make build-orchestrator` / `make build-claude-app-server` 等）への参照

### B. `ARCHITECTURE.md` に三層と orchestrator 責務を追記

- [ ] 三層境界（`platform/`（共有基盤）/ `client/`（roost）/ `orchestrator/`（Symphony））と depguard による実効化
- [ ] orchestrator の責務（single-authority な poll/dispatch/reconcile + read-only observability HTTP）
- [ ] SPEC §3.1 の 8 component と実装 package の対応は [plans/05-conformance.md](../plans/05-conformance.md) の用語対応表へリンク（重複記述を避ける）

### C. provenance / 配置の正本

- [ ] `docs/orchestrator/` 配下（025 が作る conformance 表の隣）に、SPEC component ↔ Go package の対応表を置く or conformance doc 内に節を設ける（plans/05 の用語対応表を正本として参照）

## Acceptance Criteria

- `AGENTS.md` / `ARCHITECTURE.md` から orchestrator サービスの役割・三層境界・build/test 導線が辿れる
- SPEC component ↔ package 対応が doc から追える
- doc のみの変更で `make vet` / `make lint` に影響なし

## References

- [plans/04-phases.md#p9](../plans/04-phases.md)（orchestrator 位置付け doc の追記）
- [plans/00-overview.md](../plans/00-overview.md)（ゴール / 決定事項）、[plans/02-layout.md](../plans/02-layout.md)（リポジトリ構成）
- [plans/05-conformance.md](../plans/05-conformance.md)（SPEC 用語 ↔ 実装名 対応表）、[025](025-p9a-conformance-suite.md)（conformance 表）
