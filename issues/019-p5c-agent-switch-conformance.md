# 019: claude-app-server — token usage, approval/sandbox posture, agent-switch end-to-end

- **Phase**: P5c ([plans/04-phases.md#p5-claude-app-server-shim](../plans/04-phases.md))
- **Status**: Open
- **Depends on**: 018 (shim 単線通電)
- **Blocks**: M2 完成（claude / codex 切替）

## Background

018 で claude-app-server が 1 turn を中継できる。本 issue で **SPEC §10.4 event の取りこぼしを埋め**、token usage を出し、approval/sandbox posture を documented にし、**WORKFLOW.md の `codex.command` で agent を切替えられる**ことを end-to-end で確認して M2 を完成させる。

## Tasks

### A. event マッピング拡充 (§10.4)

- [ ] tool 呼び出しを item event に: claude の **全 tool_use を一律 `DynamicToolCallThreadItem`（codex v2 schema の generic dynamic tool-call item）** として emit する。`assistant:tool_use` → `item/started`（tool 名 + raw args をそのまま載せる）、対応する `user:tool_result` → `item/completed`（tool_use の id で相関）
  - **ツール名ヒューリスティック分岐はしない**（Bash→command_execution / Edit→file_change 等への振り分けは採らない）。理由: (1) orchestrator(013) は item event を消費せず **conformance/observability 専用**、(2) codex の `command_execution`/`file_change` item は exit code・diff 等の固有必須フィールドを持ち、claude の tool_use から推測補完すると不正確・lossy。generic item なら忠実かつ fabrication 不要
- [ ] `assistant` の途中 text を `item/agentMessage/delta` で逐次送出（018 の最小版を整理）
- [ ] Claude に対応のない event（`turn_input_required` 等）は **不発火**で良いことを明記（plans/03 の表）。orchestrator(013 handler) が解釈できる範囲に限定

### B. token usage (§13.5)

- [ ] 017 の usage 抽出を使い `result` から per-turn の input/output/total を取り出す。claude は per-turn 報告なので、**shim 内で running cumulative thread total に積み上げ**、turn 完了時に **cumulative absolute total** として emit（codex の累積 absolute と同一セマンティクスにそろえる。021 の metrics が last-reported 差分で二重計上回避できる前提）。codexschema の usage を載せる経路。helper が無ければ `EmitNotification` で補う
- [ ] orchestrator 側 RunAttempt の `TotalInputTokens`/`TotalOutputTokens`（P6=021 で本格集計）に載る形を確認（本 issue は emit までで可）

### C. approval / sandbox posture (§10.5 / §15)

- [ ] `turn/start` 等で受け取る `approval_policy` / `thread_sandbox` / `turn_sandbox_policy` は **shim では強制しない**。受領値を warn ログに記録（意図と異なる可能性をオペレータに伝える）
- [ ] 安全境界は devcontainer（016）が担う前提を [plans/05-conformance.md](../plans/05-conformance.md) に **documented posture** として明記（shim は approval を行わない / sandboxed container 内起動が前提）
- [ ] approval を全許可で都度通すなら `approval_auto_approved` 相当を emit するか方針を doc 化（plans/03 の表）

### D. agent 切替 end-to-end

- [ ] WORKFLOW.md `codex.command: claude-app-server` を指定 → orchestrator が codex と同じ event 列を受ける（013 の runner が無改変で動く）ことを確認
- [ ] `codex.command: codex app-server`（純正）と切替えても orchestrator 側の挙動が変わらないことを確認（agent 非依存性の実証）
- [ ] WORKFLOW.md の設定例（claude / codex）を docs か README に追記

### E. テスト (§17.5)

- [ ] tool_use/tool_result が item event に変換される
- [ ] usage が turn 完了 event に載る（input/output/total）
- [ ] approval/sandbox フィールド受領時に warn ログが出る（強制しない）
- [ ] WORKFLOW.md agent 切替の結合テスト（fake claude / fake codex で event 列が同型）

## Acceptance Criteria

- claude-app-server が §10.4 の主要 event（thread/turn/item/usage）を codex 互換で送出
- token usage が turn 完了で取得できる
- approval/sandbox posture が 05-conformance に明文化（deviation/extension として）
- `codex.command` 切替で orchestrator が agent 非依存に動く（M2 達成）
- `go test ./cmd/claude-app-server/ ./orchestrator/...` 緑、lint 緑

## References

- [Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §10.4 (Event Types), §10.5 (client-side tools / posture), §13.5 (Token accounting), §15 (Security posture), §17.5
- [plans/03-agent.md](../plans/03-agent.md)（変換表、approval/sandbox 方針、token 抽出）、[plans/05-conformance.md](../plans/05-conformance.md)
- [017](017-p5a-claude-streamjson.md)、[018](018-p5b-claude-app-server.md)
