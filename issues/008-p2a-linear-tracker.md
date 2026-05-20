# 008: platform/tracker — Linear GraphQL adapter

- **Phase**: P2a ([plans/04-phases.md#p2-linear-adapter--workspace--hooks](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: P0 batch (merged) — `platform/logger/` 前提。P1 には依存しない (純粋な HTTP/GraphQL client)
- **Blocks**: 009 (orchestrator/tracker wrapper)、P3 (scheduler の poll/reconcile)

## Background

SPEC §11 の Issue Tracker 統合を `platform/` に実装する。**正規化済み Issue ドメインモデル (§4.1.1)** と **tracker adapter interface (§11.1)**、その Linear 実装 (§11.2) を提供する。orchestrator から使い回すため platform に置く (platform ↛ orchestrator 境界を維持し、設定値は引数で受け取る — `wfconfig.Config` を import しない)。

roost 側に Linear connector は存在しない (新規実装)。loki `clients/linear.py` は移植参考に留め、SPEC が source of truth。

## Tasks

### A. Issue ドメインモデル (§4.1.1)

- [ ] `src/platform/tracker/` 新設 (`package tracker`)
- [ ] `Issue` struct を §4.1.1 の全 field で定義 (id / identifier / title / description / priority / state / branch_name / url / labels / blocked_by / created_at / updated_at)
- [ ] `Blocker` struct (id / identifier / state、いずれも nullable)
- [ ] adapter interface (§11.1):

```go
type Adapter interface {
    FetchCandidateIssues(ctx context.Context) ([]Issue, error)
    FetchIssuesByStates(ctx context.Context, stateNames []string) ([]Issue, error)
    FetchIssueStatesByIDs(ctx context.Context, issueIDs []string) ([]Issue, error)
}
```

### B. Linear 実装 (§11.2)

- [ ] `src/platform/tracker/linear/` 新設 (`package linear`)
- [ ] `New(endpoint, apiKey, projectSlug string, activeStates []string) *Client`
  - **active states の注入経路 (確定)**: §11.1 の `FetchCandidateIssues()` は引数なしで「**configured** active states」を使うため、active states は接続レベル設定として `New` に束ねる。`FetchCandidateIssues(ctx)` は `c.activeStates` を使用
  - **terminal states は `New` に持たせない**: `FetchIssuesByStates(ctx, stateNames)` は汎用の「指定 state で取得」op。terminal cleanup 時に orchestrator/tracker (009) が `cfg.Tracker.TerminalStates` を引数で渡す
  - `wfconfig.Config` は import しない (platform ↛ orchestrator 境界)。設定値は plain な引数で受ける
- [ ] GraphQL over HTTP POST (`{query, variables}`):
  - [ ] `Authorization` header に token
  - [ ] candidate query は `project: { slugId: { eq: $projectSlug } }` で project filter
  - [ ] issue-state refresh は GraphQL issue ID を変数型 `[ID!]` で送る
  - [ ] **pagination 必須** (page size default `50`、`endCursor` 追跡、順序保持)
  - [ ] network timeout `30000 ms`
- [ ] query 構築を 1 箇所に隔離 (§11.2「keep query construction isolated」)

### C. 正規化 (§11.3)

- [ ] `labels` → lowercase
- [ ] `blocked_by` → relation type が `blocks` の inverse relation から導出
- [ ] `priority` → integer のみ (非整数は null)
- [ ] `created_at` / `updated_at` → ISO-8601 parse

### D. エラー分類 (§11.4)

- [ ] typed error を公開 (`errors.Is` 判別可能): `unsupported_tracker_kind` / `missing_tracker_api_key` / `missing_tracker_project_slug` / `linear_api_request` / `linear_api_status` / `linear_graphql_errors` / `linear_unknown_payload` / `linear_missing_end_cursor`

### E. ライブラリ選定

- [ ] **`net/http` + `encoding/json` (stdlib)** を採用。GraphQL は `{query, variables}` の JSON POST であり、SPEC §11.2 が「exact query fields/types をテストせよ」と要求するため query を手書きで完全制御する
  - 代替: `hasura/go-graphql-client` (struct-tag 駆動でフィールド制御が弱い)、`machinebox/graphql` (薄いが新規依存)。AGENTS.md「wire-format は stdlib」「既存依存を優先」に従い stdlib を選択

### F. テスト (§17.3)

- [ ] candidate fetch が active states + project slug を使う
- [ ] Linear query が `slugId` filter を使う
- [ ] `FetchIssuesByStates([])` は API 呼び出しせず空を返す
- [ ] pagination が複数ページ跨ぎで順序保持
- [ ] blocker が `blocks` inverse relation から正規化
- [ ] labels が lowercase
- [ ] state refresh が `[ID!]` 型を使う
- [ ] error mapping (request error / non-200 / GraphQL errors / malformed) を `httptest.Server` で網羅

## Acceptance Criteria

- `platform/tracker/linear` を import して 3 つの required op を呼べる
- `go list -deps ./platform/tracker/...` に `orchestrator/` `client/` が出ない
- §17.3 の test 項目を pass、`go test ./platform/tracker/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §4.1.1 (Issue model), §11 (Tracker Integration), §17.3
- [plans/02-layout.md](../plans/02-layout.md), [plans/04-phases.md#p2](../plans/04-phases.md)
- `/workspace/loki/loki2/clients/linear.py` (移植参考、非規範)
