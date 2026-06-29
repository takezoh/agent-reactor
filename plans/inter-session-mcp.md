# Plan - Agent セッション間コミュニケーション MCP

- **作成日**: 2026-06-29
- **ステータス**: draft (検証済み仮説の設計化 / 未実装)
- **影響範囲**: `client/state`, `client/runtime`, `client/runtime/subsystem/stream`, `platform/mcpproxy` もしくは新規 MCP server、`server/web` の許可設定 UI/API
- **目的**: agent-grid がホストする agent セッション同士が、明示的な権限と監査の下で安全に message / prompt を配送できる MCP tool を提供する。

## 1. 前提と検証結果

この plan でいう「起動中の対話セッション」は、任意の外部 terminal で動く CLI ではなく、**agent-grid が起動し、agent-grid daemon が管理している session / frame** を指す。

検証済みの事実:

1. **Codex session は app-server 管理**
   - Codex driver は `LaunchSubsystemStream` を選ぶ。
   - stream backend が `codex app-server` を起動し、`thread/start` / `thread/resume` で app-server thread を bind する。
   - 表示用 frame は `codex --remote unix://<sock> ...` で同じ app-server に接続する。
   - したがって Codex への prompt 配送は PTY 注入ではなく app-server API を使うべき。

2. **Codex app-server API**
   - `turn/start` は idle thread に新しい user input を送る。
   - `turn/steer` は active turn に追加入力を送る。
   - 現行 schema の入力形式は `message` ではなく `input: [{ "type": "text", "text": "..." }]`。
   - `turn/steer` は `expectedTurnId` を必須 precondition とする。

3. **Claude session は PTY 管理**
   - Claude Code の `--remote-control` は今回の要件とは別物として扱う。
   - agent-grid 管理下の Claude は PTY master を daemon が保持するため、配送は `termvt.Session.WriteInput` 相当の surface input 経路を使う。
   - 単体検証では、PTY master に文字列を書いた場合 Claude の対話入力欄に反映された。

4. **任意外部プロセスへの後付け注入は対象外**
   - `/dev/pts/N` の slave へ write しても stdin 注入にはならない。
   - `TIOCSTI` は環境依存で、この環境では失敗した。
   - 本 plan は agent-grid が master / app-server connection を保持している session のみを対象にする。

## 2. 目的

agent が他の agent session に安全に依頼・返信・引き継ぎできる仕組みを提供する。

達成したいこと:

1. agent session から MCP tool 経由で、許可された target session に message を送れる。
2. 必要な場合だけ、message を target agent の prompt として配送できる。
3. Codex と Claude の配送実装差を daemon 側で吸収する。
4. hard gate により、agent が勝手に他 session を操作したり user / system を偽装したりできないようにする。
5. 全配送を監査可能にする。

## 3. 非目的

- 任意の外部 terminal / tmux / shell 上の CLI へ後付け入力する。
- agent が raw PTY bytes や任意 control sequence を送れるようにする。
- broadcast / swarm coordination を初期実装で提供する。
- agent が target session の full transcript を自由に読む。
- human approval や policy を prompt instruction だけで代替する。

## 4. 基本モデル

MCP server は「直接注入器」ではなく、agent-grid daemon の **session messaging broker** への入口にする。

既定経路は inbox delivery:

```text
source agent
  -> MCP tool
  -> agent-grid daemon broker
  -> target session inbox
  -> target agent / UI が読む
```

例外経路として prompt delivery:

```text
source agent
  -> MCP tool
  -> daemon hard gate
  -> driver-specific delivery
       Codex: app-server turn/start or turn/steer
       Claude: PTY surface input
```

prompt delivery は強い権限を要求し、default deny とする。

## 5. MCP Tool 案

### 5.1 `agent_sessions.list`

通信可能な session のみを返す。

返す情報:

- `sessionId`
- `driver`: `codex` / `claude` / other
- `project`
- `status`
- `capabilities`: `inbox`, `promptStart`, `promptSteer`, `ptySubmit` など
- `policySummary`: agent に見せてよい範囲の許可概要

### 5.2 `agent_sessions.read`

target session の公開 inbox / summary / status を読む。

原則:

- raw transcript は返さない。
- `messageId` 単位で既読管理する。
- target session が公開を許可した metadata のみ返す。

### 5.3 `agent_sessions.send_message`

target session の inbox に message を追加する。

入力案:

```json
{
  "targetSessionId": "sess-b",
  "topic": "review-api",
  "body": "直近の API 設計を確認してください",
  "priority": "normal"
}
```

これは prompt 注入ではない。初期実装の安全な既定経路とする。

### 5.4 `agent_sessions.deliver_prompt`

target session に prompt として配送する。

入力案:

```json
{
  "targetSessionId": "sess-b",
  "body": "直近の差分をレビューし、懸念点だけ返してください",
  "delivery": "prompt",
  "submit": true
}
```

戻り値案:

```json
{
  "accepted": false,
  "reason": "target_not_idle",
  "fallbackMessageId": "msg-123"
}
```

失敗時に inbox fallback するかは policy で決める。無断 fallback は避け、tool 入力または policy に明示する。

### 5.5 `agent_sessions.reply`

受信 message に対する reply を作る。

入力:

- `messageId`
- `body`
- `resolution`: `answered` / `declined` / `needs_info`

### 5.6 `agent_sessions.request_handoff`

作業移譲を提案する。初期実装では direct prompt delivery ではなく inbox message として扱う。

## 6. Hard Gate

hard gate は MCP server ではなく daemon 側で強制する。agent が tool input で偽装できない情報を authority とする。

### 6.1 Source Session 認証

- MCP request の認証 token から source session / frame を解決する。
- `sourceSessionId` は tool input で受け取らない。
- container / frame に配布する token は session-scoped か frame-scoped にする。

### 6.2 Target Allowlist

- target session は inter-session communication を default deny にする。
- 許可は session 作成 option、UI 操作、workflow policy のいずれかで明示する。
- 許可は `inbox` と `prompt` を分ける。

### 6.3 Direction Policy

通信方向を `source -> target` で評価する。

候補:

- same project only
- same task group only
- explicit pair only
- orchestrator-created cohort only
- user-approved once / always

### 6.4 Delivery Mode Policy

配送 mode ごとに gate 強度を分ける。

| Mode | 既定 | Gate |
|---|---|---|
| `inbox` | 条件付き許可 | allowlist + rate limit |
| `prompt_start` | deny | target idle + prompt permission |
| `prompt_steer` | deny | active turn id match + steer permission |
| `pty_submit` | deny | target idle + human approval 推奨 |

### 6.5 Target State Gate

Codex:

- `threadStatus == idle` なら `turn/start`。
- active turn があり、`activeTurnID` が記録されていて steerable なら `turn/steer`。
- `turn/steer` では `expectedTurnId` に daemon が保持する `activeTurnID` を入れる。
- app-server が `activeTurnNotSteerable` や `no active turn to steer` を返した場合は reject として扱う。

Claude:

- target が idle / input 待ち相当であること。
- driver status、OSC/prompt event、terminal tail のいずれで判定するかは別途実装時に決める。
- status が不明なら reject。

### 6.6 Human Approval Gate

以下は human approval を要求できる policy にする。

- 初回 `source -> target`
- cross-project delivery
- prompt delivery
- active turn steer
- Claude PTY submit
- high priority / large payload

承認は daemon state に記録し、agent の prompt instruction で代替しない。

### 6.7 Provenance Envelope

prompt delivery では daemon が必ず envelope を付ける。source agent は user / system を偽装できない。

例:

```text
[agent-grid inter-session message]
from: sess-a
to: sess-b
message-id: msg-123
delivery: prompt
---
<sender body>
```

Codex app-server delivery でも Claude PTY delivery でも同じ envelope を入れる。

### 6.8 Sanitization

- MCP から raw control bytes は受けない。
- `body` は UTF-8 text として扱う。
- NUL、OSC、CSI、terminal control sequence は拒否または escape する。
- Claude PTY delivery で Enter を送る場合も daemon が `\r` を付与する。agent が任意 control sequence を送れないようにする。

### 6.9 Rate / Size Limit

- per source session の送信回数制限。
- per target session の受信回数制限。
- message size 上限。
- fanout / broadcast は初期実装では禁止。

### 6.10 Audit Log

記録するもの:

- timestamp
- source session / frame
- target session / frame
- tool name
- delivery mode
- gate decision
- reason
- message hash
- human approval id

本文保存は設定で選択する。default は hash + metadata のみが望ましい。

## 7. Driver-specific Delivery

### 7.1 Codex

Codex は stream backend が app-server connection と thread binding を持つため、ここに delivery method を追加する。

必要な内部情報:

- `frameBinding.threadID`
- `frameBinding.activeTurnID`
- `frameBinding.threadStatus`
- `frameBinding.waitApproval`

配送:

- idle: `turn/start`
- running: `turn/steer`

注意:

- 現在の `codexclient.StartTurn` は現行 schema とずれている可能性があるため、`input` 配列形式へ更新する。
- `turn/steer` client helper を追加する。
- app-server response / error を gate result として上位へ返す。

### 7.2 Claude

Claude は PTY surface input を使う。

配送:

- `submit=false`: text を入力欄に挿入するだけ。
- `submit=true`: text + Enter を送る。

初期実装では `submit=true` のみでもよいが、誤送信のリスクがあるため `human approval` と `target idle` gate を必須にする。

## 8. 状態と Wire 型

追加候補:

- `InterSessionMessage`
- `InterSessionDeliveryRequest`
- `InterSessionDeliveryDecision`
- `InterSessionPolicy`
- `InterSessionAuditRecord`

永続化:

- inbox message は session snapshot とは別ファイルにする候補がある。
- audit log は append-only が望ましい。
- persistence 型は stdlib-only を維持する。

## 9. 実装フェーズ

### Phase 1: Inbox-only

- MCP tools: `list`, `send_message`, `read`, `reply`
- direct prompt delivery なし。
- allowlist、direction policy、audit log の最小実装。

完了条件:

- source session を token から解決できる。
- 許可された target にだけ message が届く。
- unauthorized target は拒否される。
- audit record が残る。

### Phase 2: Codex app-server prompt delivery

- Codex stream backend に delivery method を追加。
- `turn/start` / `turn/steer` helper を現行 schema で実装。
- target state gate を実装。

完了条件:

- idle Codex session に `turn/start` で prompt が届く。
- active Codex turn に `turn/steer` で追加入力できる。
- `expectedTurnId` mismatch は拒否される。
- TUI 側にも配送結果が反映される。

### Phase 3: Claude gated PTY delivery

- Claude target state 判定を実装。
- sanitized text のみ PTY に送る。
- human approval policy を接続。

完了条件:

- idle Claude session にだけ prompt を submit できる。
- running / unknown 状態では拒否される。
- control sequence は拒否される。

### Phase 4: UI / Policy Management

- session ごとの communication opt-in。
- source-target allowlist 管理。
- audit viewer。
- pending approval UI。

## 10. Test Plan

Unit:

- gate reducer: allow / deny matrix。
- source token spoofing rejection。
- target allowlist。
- delivery mode policy。
- sanitization。
- rate limit。

Codex integration:

- fake app-server connection に `turn/start` payload が現行 schema で送られる。
- `turn/steer` に daemon-held `activeTurnID` が使われる。
- app-server error が delivery rejection に変換される。

Claude integration:

- fake PTY に sanitized prompt + Enter が書かれる。
- control sequence を含む body は拒否される。
- target non-idle では PTY write が発生しない。

MCP:

- tool schema。
- unauthorized source。
- unauthorized target。
- audit emission。

## 11. 未解決事項

1. Claude の「入力待ち」判定を何で確定するか。
2. inbox を session snapshot に含めるか、別 store にするか。
3. human approval の UX を既存 approval surface と統合するか。
4. Codex `turn/start` 実行時に active TUI がどの通知をどう描画するかの追加検証。
5. orchestrator が作る session cohort を direction policy にどう渡すか。

