# 005: WORKFLOW.md loader (front matter + body 分離)

- **Phase**: P1a ([plans/04-phases.md#p1-workflow-loader--wfconfig--preflight](../plans/04-phases.md))
- **Status**: Done
- **Depends on**: P0 batch (merged) — `cmd/orchestrator/`, `platform/logger/` 前提
- **Blocks**: 006, 007 (および P3 以降の dispatch 全般)

## Background

SPEC §5.1–§5.2 の `WORKFLOW.md` ローダを `orchestrator/workflowfile/` に実装する。
このフェーズは **生のパースまで** が範囲。typed getter / default 適用は 006 (wfconfig)、
dispatch preflight は 007 で行う。prompt template の **レンダリング** (Liquid, §5.4) は P3 以降に延期し、
ここでは body を `prompt_template` 文字列として保持するだけ。

## Tasks

### A. パッケージ新設

- [x] `src/orchestrator/workflowfile/` を新設 (`package workflowfile`)
- [x] 返却型を定義:

```go
type Workflow struct {
    Config         map[string]any // front matter root object (config キー直下ではない)
    PromptTemplate string         // trim 済み Markdown body
}
```

### B. ローダ実装 (SPEC §5.1–§5.2)

- [x] `Load(path string) (Workflow, error)`:
  - [x] 先頭が `---` なら次の `---` までを YAML front matter として decode
  - [x] 残りを prompt body とし、trim して `PromptTemplate` に格納
  - [x] front matter が無ければ全体を body 扱い、`Config` は空 map
  - [x] YAML front matter が map/object に decode できない場合はエラー
- [x] path 解決の precedence は呼び出し側 (007/cmd) の責務。本パッケージは受け取った path を読むだけ

### C. typed error (SPEC §5.5)

- [x] error class を sentinel/typed error として公開:
  - [x] `missing_workflow_file` (read 失敗)
  - [x] `workflow_parse_error` (YAML decode 失敗)
  - [x] `workflow_front_matter_not_a_map` (front matter が非 map)
- [x] `errors.Is` で判別可能にする (例: `var ErrMissingWorkflowFile = errors.New(...)`)

### D. ライブラリ選定

- [x] YAML パーサは **`gopkg.in/yaml.v3`** を採用 (go.mod に既存 / AGENTS.md「既存依存を優先」)
  - 代替: `go.yaml.in/yaml/v3` (fork, 既存だが indirect)、`goccy/go-yaml` (高速だが新規依存)。indirect → direct への昇格のみで済む yaml.v3 を選択
- [x] front matter は `map[string]any` に decode (typed 化は 006 の責務)

## Acceptance Criteria

- `Load` が front matter あり/なし両方で正しく `Config` と `PromptTemplate` を返す
- 3 つの typed error が `errors.Is` で判別できる
- SPEC §17.1 のうち以下を test で pass:
  - missing `WORKFLOW.md` → typed error
  - invalid YAML front matter → typed error
  - front matter non-map → typed error
  - body trim / front matter なし時の挙動
- `go test ./orchestrator/workflowfile/` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §5.1, §5.2, §5.5, §17.1
- [plans/04-phases.md#p1](../plans/04-phases.md)
- [plans/06-orchestrator-migration.md](../plans/06-orchestrator-migration.md) — `orchestrator/workflowfile/` の位置付け
