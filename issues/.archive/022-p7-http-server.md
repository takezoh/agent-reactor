# 022: orchestrator/httpserver — observability HTTP server (§13.7)

- **Phase**: P7 ([plans/04-phases.md#p7-http-server](../plans/04-phases.md))
- **Status**: Done (2026-05-20)
- **Depends on**: 011 (merged; `State.Snapshot`)、009 (merged; tracker)、021 B'' (done; `StateSnapshot.CodexTotals`/`CodexSecondsRunning` 生涯累積)
- **並行可**: P5 と完全独立。既存の scheduler state を **read-only** で公開するだけ。codex/claude いずれでも動く。`codex_totals` の累積は 021 が State に持ち、本 issue は read のみ
- **Blocks**: M3

## Background

SPEC §13.7 は観測用 HTTP server を**必須**実装と規定（roost TUI は使わない方針＝[plans/00-overview.md](../plans/00-overview.md) D6）。orchestrator の running/retrying state・codex totals・rate-limit を read-only で公開する。P5（agent 切替）と無関係に、既存 `scheduler.State.Snapshot()` の上に構築できる。

## Tasks

### A. `orchestrator/httpserver/` 新設

- [x] `GET /` — dashboard（server-rendered HTML。running/retrying 一覧 + lifetime totals。`html/template` で XSS 安全）
- [x] `GET /api/v1/state` — running / retrying / codex_totals / rate_limits の JSON（`State.Snapshot()` を射影）。**`codex_totals` は orchestrator 生涯の累積**（running + ended）を 021 B'' が State に保持する `CodexTotals`/`CodexSecondsRunning` から read のみ（running-only sum ではない、§13.5/§13.7.2）
- [x] `GET /api/v1/<issue_identifier>` — per-issue 詳細（attempt/phase/session_id/last activity）
- [x] `POST /api/v1/refresh` — 即時 tick トリガ（scheduler に refresh signal を送る seam）
- [x] `405 Method Not Allowed` / error envelope `{"error":{"code","message"}}`（§13.7.2 の shape）

### B. 配線と安全

- [x] loopback デフォルト bind（`127.0.0.1`）。CLI `--port`（既に main にフラグあり）が `server.port` を上書き
- [x] scheduler から state を読む経路（`State.Snapshot()` の参照を httpserver に渡す）。書込はしない（read-only）
- [x] `POST /refresh` は scheduler の retry/poll channel に signal を送るだけ（single-authority を侵さない）
- [x] wire-format（JSON response 型）は **stdlib のみ**（AGENTS.md）

### C. テスト (§17.6 系)

- [x] `httptest` で各エンドポイントの shape を検証（§13.7.2 サンプルと一致）
- [x] 未対応 method で 405 + error envelope
- [x] `/api/v1/<id>` の存在/非存在ケース
- [x] `POST /refresh` が refresh signal を送る（fake scheduler seam で観測）

## Acceptance Criteria

- ブラウザで dashboard 表示、curl で API 操作可能
- response shape が SPEC §13.7.2 サンプルと一致
- loopback bind 既定、`--port` で上書き可能
- read-only（state を破壊しない）。`go test ./orchestrator/httpserver/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §13.7 (HTTP Surface), §13.7.2 (response shapes), §13.5 (totals)
- [plans/04-phases.md#p7](../plans/04-phases.md)、[plans/00-overview.md](../plans/00-overview.md)（D6: TUI 不使用・HTTP 必須）、`orchestrator/scheduler`（`State.Snapshot`）
