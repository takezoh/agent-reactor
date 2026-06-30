# Plan — Browser-sourced credential injection (host credproxy への push)

- **作成日**: 2026-06-30
- **ブランチ**: `main`
- **ステータス**: draft (依存: [multi-host-gateway.md](./multi-host-gateway.md) Phase 4 E2E Noise が確立していること)
- **影響範囲**: `client/credproxy/` (browser-push を受領する Source 追加)、`client/web/` (secret store + 配布 UI + outbox)、host channel に frame 追加 (3 種)、ADR (credential source model / browser secret storage / Tier 1 vs Tier 2 / agent ごとの注入経路)
- **関連 ADR (将来起こす)**: (a) credential source model (host-sourced と browser-sourced の併存規約) / (b) browser secret storage (WebCrypto + IndexedDB + WebAuthn PRF or passphrase fallback) / (c) host-side memory-only retention 不変条件 / (d) agent ごとの注入経路 (tmpfile / env / stdin) / (e) sidecar callback proxy (Tier 2、別 plan)

## 0. 用語

- **Browser-sourced credential**: agent (codex / claude / API key 等) 認証情報の出所が browser 側 (IndexedDB / WebCrypto-protected secret store) にある状態。本 plan の対象。
- **Host-sourced credential**: 既存。host process の disk (`~/.codex/auth.json` 等) / env / OAuth flow が出所。本 plan で**廃止しない** (併存)。
- **Tier 1**: 受領した host process が credential plaintext を一時的に process memory で扱う (本 plan の MVP)。
- **Tier 2**: host process も plaintext を見られない。sandbox 内 sidecar proxy が browser に Noise tunnel 経由で callback して必要時のみ復号。**本 plan では構造を開けておくのみ、実装は別 plan**。
- **Distribution**: browser が「ある credential を、特定の host に push する」アクションの単位。配布は per-host opt-in。

## 1. 目的とスコープ

### 1.1 目的

1. credential の**出所を browser** に置けるようにし、multi-host 環境で「host ごとに login 作業を繰り返す」運用を不要にする。
2. credential を**host disk に永続化しない** (Tier 1 不変条件)。host 再起動で消え、browser から再 push する。
3. Tier 2 (host 不可視) への移行構造を**MVP 時点から開けておく** (credential を envelope に包む wire shape と Source interface)。
4. **gateway を一切経由しない**: credential は host channel 上で browser → host 直送のみ ([[feedback-gateway-no-domain]] 準拠)。

### 1.2 非目的

- **Cross-device 同期** (browser A で push した credential を browser B でも使う等)。MVP は per-device。
- **既存 host-sourced credential の廃止**。両者は併存し、host 側で per-session に出所を選ぶ。
- **Gateway 経由配布**。gateway no-domain 原則により credential を gateway 上に乗せない (relay-fallback 経路でも Noise ciphertext で素通すのみ、frame 種としても gateway channel に流れない)。
- **secret rotation の自動化**。MVP は user 手動 rotate のみ。
- **Tier 2 (host 不可視) の実装**。本 plan は構造的に余地を残すだけ。

### 1.3 既存制約 (退行禁止)

- [multi-host-gateway.md](./multi-host-gateway.md) の **host channel (Noise wrapped)** を再利用。新 transport は導入しない。
- [[host-direct-env-inherit]]: host 内 env overlay 不変条件は維持。credproxy が overlay を作るだけで、daemon env (`os.Environ` base) を破壊しない。
- ファイル 500 行 / 関数 80 行、wire 型 stdlib のみ (ADR-0021)、`depguard` 境界。

## 2. アーキテクチャ

### 2.1 全体像

```
Browser                                Host (server process)
┌────────────────────────┐             ┌────────────────────────┐
│ secret store            │             │ credproxy               │
│  (IndexedDB +           │  Noise      │   Source: HostSourced   │
│   WebCrypto / PRF)      │  host       │   Source: BrowserSourced│ ← NEW
│                          │  channel    │     (memory map only)   │
│ Distribution UI          │ ──────────► │     TTL expiration      │
│  (palette command 群)    │  frame:     │                          │
│                          │  cred_push  │  inject to pty env      │
│ Outbox                   │  cred_inv.  │   ([[host-direct-       │
│  (offline 時の queue)    │ ◄────────── │     env-inherit]]       │
│                          │  cred_ack   │    overlay)             │
└──────────────────────────┘             └──────────────────────────┘
            ▲                                       │
            │ (Tier 2 future)                       │
            └────── callback proxy ◄──── sidecar in sandbox
```

### 2.2 不変条件

1. **Browser secret は disk から離れて移動しない**。WebCrypto `extractable: false` key で wrap → IndexedDB 永続。host 側はその plaintext を memory に短期保持するだけで disk には書かない。
2. **Host 側は memory-only**。process kill / restart で消え、browser が再 push する。SIGTERM 時も disk flush しない。
3. **配布粒度は per-host opt-in**。browser が「どの host に push するか」を user に明示的に選ばせる。default broadcast はしない (multi-host での意図しない credential leak 防止)。
4. **Wire 上に credential が現れるのは host channel (Noise 内) のみ**。gateway / control plane / signaling / metrics / audit のどこにも漏れない (gateway no-domain と整合、regression guard test で固定)。
5. **Credential ID 空間は browser-local 一意**。host 側は `(credential_id, browser_pubkey_fp)` の組で受領元を識別。host を跨いで同じ id を別 browser が使っても衝突しない。
6. **Source 選択は session 単位で fix**。session 中途で host-sourced ↔ browser-sourced を切り替えない (新 session で反映)。

## 3. wire 型差分 (host channel 専用)

### 3.1 追加フレーム

```ts
// Browser → Host
type HostCredentialPushFrame = {
  k: "host_cred_push";
  host_id: string;
  credential_id: string;        // browser-local UUID
  scope: "agent" | "api_key" | "oauth_token";
  agent?: string;               // "codex" / "claude" / etc (scope=agent のみ)
  payload_b64: string;          // Tier 1: agent-specific serialized secret (例: codex auth.json 形式 JSON を base64)
  ttl_sec?: number;             // host が memory に保持してよい上限 (default 3600)
  tier?: 1 | 2;                 // 既定 1。Tier 2 への将来拡張余地
};

// Host → Browser
type HostCredentialAckFrame = {
  k: "host_cred_ack";
  host_id: string;
  credential_id: string;
  status: "accepted" | "rejected" | "expired" | "renewed";
  renewed_ttl_sec?: number;     // status="renewed" のみ
  reason?: string;              // rejected の理由 (例: "scope_unsupported", "agent_unknown")
};

// Browser → Host
type HostCredentialInvalidateFrame = {
  k: "host_cred_invalidate";
  host_id: string;
  credential_id: string;
};
```

### 3.2 既存フレームへの追加

- `RespErrFrame.code` 拡張: `"credential_required"` (session create 時に host が該当 agent の credential を持っていない)、`"credential_expired"` (session 中に TTL 切れ)。
- `SessionInfo` に `credential_source?: "host" | "browser:<credential_id>"` を追加 (read-only、UI 表示用)。

### 3.3 Channel 不変条件 (codec test で固定)

- 上記 3 frame は **host channel にしか乗らない**。gateway channel に push されたら codec layer で `RespErr{code:"frame_channel_mismatch"}` で reject。
- gateway audit log / metrics / registry に credential 関連の event が**一切**現れない (multi-host-gateway.md §6.5 / §13.5 の regression guard と同じ枠で test を増設)。

## 4. Browser 側

### 4.1 Secret store

- **保管先**: IndexedDB の `credentials` object store。値は WebCrypto `AES-GCM` で暗号化。
- **保管鍵 (key-wrapping key)**: WebAuthn PRF extension (passkey 派生) を一次案。fallback は user passphrase + PBKDF2 derive。Decision は ADR (Open Q C1)。
- **鍵 export 不可**: wrapping key は `extractable: false` で生成。unwrap した plaintext credential も即時 Noise frame に流して破棄、JavaScript heap への滞留を最小化。
- **disk 永続要請**: `navigator.storage.persist()` を初回利用時に要求 (granted されないと eviction の可能性をユーザに警告)。

### 4.2 配布 UI (palette)

新 palette command (ADR-0050 palette phase composition 踏襲):

- `Credentials > Set agent token` — credential を入力 / paste、agent kind を選択、store に保存。
- `Credentials > Distribute to hosts` — credential を選び、push 先 host を multi-select。default は**何も選ばれていない** (誤配布防止)。`HostSelectPhase` (multi-host-gateway.md §10.1) の multi-select 版を再利用。
- `Credentials > Revoke from host` — 配布履歴から特定 host に invalidate frame を送る。
- `Credentials > Forget locally` — browser store から削除 (配布済 host への invalidate 通知は別操作、ただし default で「同時に revoke も送る」を提案)。
- `Credentials > Distribute to all online hosts` — 明示的な macro。auto-broadcast は無く user gesture が必須。

### 4.3 配布ポリシーと Outbox

- **配布履歴**: `credentials[id].distributedTo: { pubkey_fp, last_ack_at, status }[]` を IndexedDB に保持。host 側は持たない (host 再起動で消えても browser が再 push)。
- **Outbox**: offline host への push / invalidate を queue。host channel が確立次第 flush。ack 受領後にエントリ削除。
- **TTL 切れ時の自動再 push**: host から `credential_expired` 受領 → browser が同 credential を再 push (user 介入不要)。Notification は出すが modal にしない (silent ではないが intrusive でもない)。
- **`credential_required` (session create 失敗) 時**: palette が inline で「この host にはまだ credential が無い、配布しますか?」を prompt。承認で push → session create を再試行。

### 4.4 状態スライス

```ts
// src/client/web/src/state/credentialsSlice.ts (新設)
type CredentialsState = {
  store: Record<string, CredentialEntry>;     // credential_id → entry
  outbox: PendingOp[];                         // push / invalidate の queue
};

type CredentialEntry = {
  id: string;
  scope: "agent" | "api_key" | "oauth_token";
  agent?: string;
  ciphertextRef: string;                       // IndexedDB key (plaintext は state に置かない)
  createdAt: number;
  distributedTo: Record<string, DistributionStatus>;  // pubkey_fp → status
};
```

state slice には **plaintext を持ち上げない** (IndexedDB から復号して即 wire に送る)。

## 5. Host 側

### 5.1 Source interface

```go
// client/credproxy/source.go (新規)
type Source interface {
    Resolve(ctx context.Context, agent string) (Secret, error)
    Kind() string  // "host" | "browser"
}

type HostSourced struct { /* 既存実装 */ }

type BrowserSourced struct {
    store *MemoryStore  // process memory only
}
```

- pty session create 時、agent kind に応じて Source を選択。Default 優先順: `BrowserSourced` (該当 agent の credential が memory に存在) → `HostSourced` (fallback)。優先順は user setting で逆転可。
- 注入経路は既存通り。Browser-sourced であることが env overlay 内部の差分を変えない。

### 5.2 Memory-only 保持

```go
// client/credproxy/store.go (新規)
type MemoryStore struct {
    mu      sync.RWMutex
    entries map[CredentialKey]*entry  // key = (credential_id, browser_pubkey_fp)
}

type entry struct {
    secret    []byte    // plaintext (memory only)
    agent     string
    expiresAt time.Time
}
```

- TTL expiration goroutine が `ttl_sec` 経過で削除 + browser に `host_cred_ack{status:"expired"}` 送信。
- process restart で全消失。SIGTERM 時も flush しない (disk に書かない不変条件)。
- `entry.secret` には `MarshalJSON` / `String` / `GoString` を生やさない。logger field に渡しても plaintext が出ないよう **型レベルで防御**。

### 5.3 注入経路 (agent ごと)

Decision は ADR (Open Q C2) で決める。一次案:

| agent | 経路 | 備考 |
|---|---|---|
| codex | tmpfile (`auth.json` 同形式) + `CODEX_AUTH_FILE` env で指す | tmpfile は `O_TMPFILE` + memfd で disk path を持たせない (Linux)、session 終了で自動 unlink |
| claude | env (`ANTHROPIC_API_KEY`) | API key 1 つで完結、env overlay に直接乗る |
| 任意 API key | env (key 名は agent registry で指定) | 同上 |

- いずれも child process が leak しても server process の memory までは含まれない (FD/env 経由で渡し、server heap の参照は session create 後すぐ破棄)。

### 5.4 セキュリティ境界の補強

- core dump 抑制 (`prctl(PR_SET_DUMPABLE, 0)` Linux / `setrlimit(RLIMIT_CORE, 0)`)。実装可否と粒度は ADR。
- `mlock` で swap への漏出を防ぐ案もあるが、rlimit 制約で MVP は努力目標に留める (Tier 2 で根治)。

## 6. セキュリティ

### 6.1 脅威モデル差分 (multi-host-gateway.md §6.1 を継承)

| 脅威 | 対策 |
|---|---|
| Gateway 侵害 | 不変。host channel が Noise wrap、push frame は relay-fallback でも ciphertext。Gateway は credential frame の**存在自体**を観測不能 (frame 種が gateway channel に出ない、relay は host channel 単位の byte counter のみ) |
| Host process 侵害 | **Tier 1 では memory plaintext が漏れる**。受容範囲とし、Tier 2 で sidecar proxy 化することで解消する経路を別 plan で用意。Tier 1 でも core dump 抑制 + 型レベルの logger leak 防御で leak surface を縮小 |
| Browser device 紛失 | user が credential を browser secret store から削除 → `Credentials > Forget locally` が default で各 host へ invalidate を送付。Push 済 host が offline なら outbox に queue され次回接続で再送。**user は credential rotation も別途行うべき** (失効は browser → host の通知に依存するため、host channel が攻撃者に届く前提でも安全にするには rotation 必須) |
| Browser → 偽 host への push | host TOFU (multi-host-gateway.md §6.3.3) で防止。pubkey rotation 後の新 fp は別 host 扱い → 自動再 push しない (user が明示配布) |
| Tier 1 で host disk 書き出し漏れ (debug log 等) | `MemoryStore.entry.secret` 型に `MarshalJSON` を生やさず、logger フィールドに渡せない unexported byte slice にする (静的に守る)。logger 規約に「`secret` 名の field は redact」を追加 |
| Browser store の plaintext export | WebCrypto non-extractable key で技術的に禁止。devtools からの読み出しも IndexedDB 値が ciphertext のみ |

### 6.2 失効フロー

1. Browser 側で credential 削除 (`Credentials > Forget locally`)。
2. Outbox に invalidate frame を queue。
3. 配布履歴 (`distributedTo`) の host ごとに host channel が確立次第 invalidate を送信。
4. Host 受領 → memory map から削除。**進行中 session の credential は continue** (走っている pty を kill しない、設計選択 — kill が必要なら user が明示 stop)。
5. Browser 側 outbox は ack 受領後にエントリ削除。

### 6.3 Tier 2 への移行構造

本 plan では Tier 2 を実装しないが、wire shape はそのまま再利用できる。Tier 2 plan は host 側に「memory に置かない、sidecar proxy 起動 + callback channel 開設」の選択肢を追加するのみ。Browser 側変更は最小 (`HostCredentialPushFrame.tier = 2` 指定 + callback channel handshake 1 段)。

## 7. 段階導入

### Phase 0 — 設計合意 (本 plan)

- 必要 ADR: credential source model、browser secret storage、Tier 境界、agent ごとの注入経路。
- wire shape 確定 (§3)。

### Phase 1 — Browser secret store + 配布 UI (wire 配線なし)

- IndexedDB + WebCrypto store。
- palette command 4 種。
- 配布履歴 / outbox slice。
- 配布アクションは "registered (not yet delivered)" 状態のまま。

### Phase 2 — Host channel push 配線 (Tier 1)

- `host_cred_push` / `host_cred_ack` / `host_cred_invalidate` を host channel に乗せる。
- Host 側 `BrowserSourced` Source 実装。codex か claude のいずれか 1 つを first-class で対応。
- regression guard test: gateway audit / metrics に credential event が現れないこと、credential frame が gateway channel に乗らないこと。

### Phase 3 — Agent 拡張と運用

- 残り agent (codex / claude / 任意 API key) を順次対応。
- TTL 自動再 push、outbox / invalidate 配送保証、`credential_required` 自動 prompt。
- 失効フロー (§6.2) の e2e test。

### Phase 4 — Tier 2 sidecar (別 plan)

- Sandbox 内 proxy + callback channel。本 plan の外。

## 8. Open Questions

| # | 問い | 影響 | 候補 |
|---|---|---|---|
| C1 | Browser secret store の鍵派生: WebAuthn PRF / passphrase / 両方 | UX、device 移行 | MVP: WebAuthn PRF (passkey 必須環境前提)、fallback で passphrase。Decision → ADR |
| C2 | Agent ごとの注入経路 (tmpfile vs env vs stdin) | secret leak リスク、agent 互換性 | agent ごとに最適路を ADR で決める。codex は memfd + `CODEX_AUTH_FILE`、claude は env が一次案 |
| C3 | TTL default 値 | UX vs リスク | 3600s (1h) を一次案。session 進行中は host が `cred_ack{status:"renewed"}` で TTL 延長して expire を防ぐ |
| C4 | 配布履歴の per-device backup | device 紛失時の復旧 | encrypted bundle export/import コマンド。MVP scope か後段 plan か |
| C5 | Tier 2 (host 不可視) の sidecar 実装方式 | 別 plan の前提 | sandbox 内 unix socket proxy + Noise inner tunnel が一次案。本 plan では構造のみ確認 |
| C6 | host-sourced と browser-sourced の優先順位 | 既存運用との両立 | default は browser 優先 (新 use-case を活かす)。user setting で逆転可能。session 単位で固定 (中途切替なし) |
| C7 | `Credentials > Forget locally` で auto-revoke を default にするか | UX vs 安全 | Default で「同時に revoke も送る」 (off にする選択肢あり)。silent leave-behind を防ぐ |
| C8 | mlock / core dump 抑制をどこまでやるか | leak surface vs 実装複雑度 | core dump 抑制は必須、mlock は努力目標 (rlimit 不足時 fallback)。Tier 2 で根治 |

## 9. リスクと緩和

| リスク | 影響 | 緩和 |
|---|---|---|
| Tier 1 で host memory が swap に出る | secret leak | core dump 抑制 + swap 無効化を推奨 docs に記載。Tier 2 で根治 |
| Browser WebAuthn PRF 非対応環境 | UX 退行 | passphrase fallback、対応環境を ADR に明示 |
| Push frame 漏れによる stale credential | session 失敗 | TTL renew + outbox 再送、host が `credential_required` を返したら browser が再 push |
| 配布粒度が細かすぎて UX 摩擦 | "全 host に配布したい" use-case との衝突 | palette `Distribute to all online hosts` macro を提供 (明示 macro のみ、auto-broadcast 無し) |
| 既存 host-sourced 運用との混乱 (どっちが効いてるか分からない) | UX | `SessionInfo.credential_source` を UI badge で常時表示 |
| Browser device 紛失時の窓 (rotation 前に攻撃者が使う) | secret 悪用 | docs で「browser 紛失時は credential 自体の rotation を必ず行え」を強調。Tier 2 + WebAuthn PRF で window を縮小 |

## 10. このプランで決めたこと (要約)

1. **Credential の出所を browser に置く path を新設**。host-sourced と併存し、session 単位で Source を選ぶ。
2. **Wire は host channel に frame を 3 種追加するのみ**。Gateway は credential frame の存在も観測しない (no-domain 不変条件を継承)。
3. **Tier 1 (host memory) で MVP**。host disk には書かない (memory-only 不変条件)。
4. **Tier 2 (host 不可視) への移行構造を MVP から開けておく**。`tier` field と Source interface だけ用意。
5. **Web で完結、native client 不要**。Browser secret store は WebCrypto + IndexedDB (WebAuthn PRF 一次)。
6. **配布は per-host opt-in**。auto-broadcast は無く、user gesture を必須。

## 関連メモリ

- [[feedback-gateway-no-domain]] — credential も gateway を経由しない (relay-fallback でも ciphertext frame、frame 種としても gateway channel には出ない)
- [[host-direct-env-inherit]] — env overlay 不変条件は browser-sourced でも維持 (overlay の中身が変わるだけ)
- [[web-active-session-ownership]] — credential 配布履歴も browser-local single source of truth で持つ (host が分散 master を持たない)

## 関連 plan

- [multi-host-gateway.md](./multi-host-gateway.md) — 本 plan の依存元 (host channel / Noise / pairing / identity / nickname store を再利用)
- [client-as-host.md](./client-as-host.md) — client 機の `server` も同じ Source interface で扱える ("This device" host への credential 配布 = localhost への push frame、wire 上は remote と区別なし)
