# Plan — Multi-host + Gateway 構成への移行

- **作成日**: 2026-06-27
- **ブランチ**: `main`
- **ステータス**: draft (未着手 / 設計レビュー段階)
- **影響範囲**: 新規 `gateway/` layer + `cmd/gateway/` binary、`platform/transport/` 新設、`client/` の対外面分離、`web/` の host 概念 + nickname store 追加、wire shape 拡張
- **関連 ADR (将来起こす)**: control tunnel transport / data plane channel (WebRTC + LAN + opaque relay) / E2E noise layer / pairing flow / host ACL distribution と authoritative host authz / host selection UX / **gateway no-domain 原則 (relay/tunnel/authorizer のみ、display name や capability を持たない)**

## 0. 用語

- **Host**: 1 物理 PC ≒ 1 server プロセス (旧 `cmd/server`)。pty 実行と sandbox lifecycle の権限境界。
- **Gateway**: 公開網に置く新規 binary。**relay / tunnel / authorizer の 3 責務のみ**を担い、ドメイン (session / frame / driver / agent / capability / ACL 中身 / host display name) を **持たないし解釈しない**。具体的責務:
  - **Tunnel**: host との control tunnel を張り heartbeat と signaling を中継する
  - **Relay**: WebRTC signaling と (P2P/LAN 不能時の) host channel ciphertext の素通し relay
  - **Authorizer**: user identity (passkey 等) の認証と、user 署名済 op の opaque な配送

  Gateway が持って良い情報は **host pubkey fingerprint (relay routing 用) / control tunnel online 状態 / relay byte counter / user identity credential** のみ。host display name や capability snapshot は **client-local (browser localStorage)** か **host channel 経由** で取得する。
- **Web client**: ブラウザ側 SPA (`src/client/web`)。接続できた host ごとに独立した host channel を張り、それぞれの session list を per-host section で表示する。
- **Frame**: web UI のタブ単位 (`proto.FrameInfo`)。1 session 配下に 0..N。
- **Session**: pty session (`proto.SessionInfo`)。host に紐づく。ID 空間は host-local 一意、global identity は `(host_id, session_id)` の composite。
- **Control tunnel**: host が gateway に張る outbound 接続 (gRPC bidi)。heartbeat / signaling / ACL push / (fallback 時) opaque relay の搬送のみ。**session bytes はこの tunnel に流れない**。
- **Host channel**: browser ↔ host の data plane 接続 (WebRTC / LAN / opaque relay のいずれか)。Noise で wrap され、host の session list / view-update / pty bytes が流れる。
- **E2E**: end-to-end。client ↔ host 間で gateway も読めない暗号路。
- **Pairing**: host を gateway / client に初回登録する手順。

## 1. 目的とスコープ

### 1.1 目的

1. host を**複数物理 PC** に配置し、web client が**接続できた host の session を per-host section でまとめて操作**できる ("current host" や host 切替の概念は client に持たない、merged list 型も持たない、接続不能 host は placeholder で正直に出す)。
2. host を**公開網に直接晒さない** (inbound port 不要)。攻撃面を gateway 1 点に集約。
3. **gateway 運用者を信頼しない** E2E (gateway が侵害されても pty 平文も session 件数も漏れない)。
4. **LAN 限定の直結**を残し、gateway 不在 / オフライン環境でも動かせる。
5. **WebRTC P2P を MVP からの一次データプレーン** に据える (browser ↔ host 直結、gateway は signaling のみ)。relay fallback は P2P/LAN いずれも不能な host channel に対してのみ。
6. **Gateway はドメイン情報を一切持たない** (relay/tunnel/authorizer の 3 責務のみ)。host display name や capability snapshot は browser localStorage か host channel から取る。これにより gateway 侵害時の info leak が pubkey fingerprint と relay traffic pattern のみに縮小される。

### 1.2 非目的 (この plan の対象外)

- セッション live-migration (pty state を別 host に移す)。pty は host-pin。
- gateway の HA / multi-region active-active。MVP は single instance + standby。
- orchestrator binary の gateway 経由化。orchestrator は当面 host 同居 (`platform/` 共有のみ)。
- credential / secret の cross-host 同期。host ごと独立。
- web client native app (PWA / Tauri)。ブラウザ前提を維持。

### 1.3 既存制約 (退行禁止)

- `depguard` import 境界 (ARCHITECTURE.md): `platform/* → client/*, orchestrator/* 不可`、`client/* → orchestrator/* 不可`、`orchestrator/* → client/* 不可`。新 `gateway/` は同じ規約に組み込む。
- ファイル 500 行 / 関数 80 行 (reducer 例外)。
- wire 型 / persistence 型は stdlib のみ (ADR-0021)。
- `activeSessionID` は client 単独管理 ([[web-active-session-ownership]])。host scoping でも維持。
- sandbox release は frame teardown 経路で統一済 (d1e3a8c4)。分散構成で同経路を破らない。
- `pty_backend.envSlice` は daemon の `os.Environ` を base にした overlay ([[host-direct-env-inherit]])。host 内不変条件として維持。

## 2. ドメインモデル変更 (最重要)

分散化で最初に効くのは **モデルに "Host" を一級として加える**こと。後段の transport / authn / UX はここに従属する。

### 2.1 概念モデル

```
WebClient ──(多対多)── Host ──(1対多)── Session ──(1対多)── Frame
                       │
                       ├─ Sandbox (host-local)
                       ├─ Driver registry (host-local)
                       └─ Credential proxy (host-local)
```

不変条件:

1. **Session は単一 Host に永続束縛**。Session migration は禁止 (pty buffer は host-local リソース)。
2. **Frame は親 Session の Host を継承**。frame 単位で host を持たない。
3. **Sandbox / Driver / Credential は host-local**。cross-host 参照を新設しない (security + 認可粒度)。
4. **WORKFLOW.md / 設定** は host-local。中央集約は別 plan。

### 2.2 P2P-first 前提 (data structure の根本)

**データ構造は browser ↔ host が直接 P2P で通信することを前提に設計する**。Gateway は relay / tunnel / authorizer の 3 責務に徹し、**ドメイン (session / frame / driver / agent / capability / ACL 中身 / host display name) を持たないし解釈しない** ([[feedback-gateway-no-domain]])。これは Phase 5 の最適化ではなく、wire shape と routing の前提条件。

帰結 (以後の節はすべてこの前提に従う):

1. **Session ID 空間は host-local 一意**。global identity は **composite `(host_id, session_id)`**。Frame も同様に `(host_id, session_id, frame_id)`。host を跨いで一意である必要はない (ULID の 80bit randomness で実用上の衝突もない)。
2. **Browser は host ごとに独立した data channel** (WebRTC P2P を一次、LAN 直結が次、gateway opaque relay が fallback) を張る。session list / view-update / output frame はその channel 上だけを流れる。
3. **Gateway hello と Host hello は別フレーム種**。
   - Gateway hello: host directory (pubkey fp + signaling hints + online のみ) + signaling 中継。**display name / capability / session はここに載らない**。
   - Host hello: その host の session 一覧 + capability。browser が**それぞれの host channel で受領**する。
4. **Session list は host ごと独立**。browser state は `connections[host_id].sessions` の per-host 構造のみ持つ。**"merged sessions" 型は wire / state / UI のどこにも作らない**。UI は接続できた host の session を表示するだけで、全 host の完全性は保証しない (接続不能 host は placeholder で正直に出す)。
5. **ACL の最終権威は host**。gateway は user 署名済 op を opaque blob で forward するだけで内容を parse しない。host が user 署名を verify して accept する。**gateway が侵害されても未知 client は host に入れない**。
6. **Audit at gateway は connection metadata のみ** (pubkey fp + byte counter)。session 中身も session 件数も display name も capability も gateway は知り得ない。
7. **Cross-host operation は wire 上に存在しない**。"全 host の session を kill" のような atomic op は提供しない。browser が各 host channel に独立して投げる。
8. **Host display name (nickname) は browser localStorage のみ**。pubkey_fp → nickname の map を browser が持つ。gateway / host / mDNS TXT のどれにも nickname は載らない。

### 2.3 wire 型差分

frame の出所は **Gateway** か **Host** のどちらか。browser はそれぞれの channel で別のフレーム種を受け取り、**混ぜない**。

#### Go (`src/client/proto/response.go`)

```go
type SessionInfo struct {
    ID                 string           `json:"id"`           // host-local 一意
    HostID             string           `json:"host_id"`      // NEW: host が常に明示 (defensive redundancy + browser routing key)
    Project            string           `json:"project"`
    // ... 既存 fields ...
}

type FrameInfo struct {
    ID          string `json:"id"`             // host-local
    HostID      string `json:"host_id"`        // NEW: 同上
    Command     string `json:"command"`
    SubsystemID string `json:"subsystem_id,omitempty"`
    TargetID    string `json:"target_id,omitempty"`
}

// NEW: Gateway → browser の directory 配布用
// Gateway no-domain 原則 (§0): display_name や capability は載せない。
// host を識別する最小情報 (pubkey fingerprint + signaling hints + online) のみ。
type HostDirectoryEntry struct {
    ID                string         `json:"id"`                    // host_id: gateway 発行の opaque routing key (例 "h_7k3m")
    PubkeyFingerprint string         `json:"pubkey_fingerprint"`    // TOFU 用 (browser nickname store の key にもなる)
    Online            bool           `json:"online"`
    LastSeenAt        string         `json:"last_seen_at"`
    Signaling         SignalingHints `json:"signaling"`             // ICE / LAN / relay 可否
}

type SignalingHints struct {
    IceServers     []IceServerConfig `json:"ice_servers"`
    LanEndpoints   []LanEndpoint     `json:"lan_endpoints,omitempty"`   // pairing 時 host が報告した LAN IP/port (gateway は routing 用にのみ使う、内容を解釈しない)
    RelayAvailable bool              `json:"relay_available"`            // P2P 不能時の opaque relay 可否
}

// HostInfo (UI で扱う表示用 view-model) は別途:
// directory + browser localStorage の nickname map + 現在のセッション connection 状態を browser 側で合成して作る。
// wire 型としては HostDirectoryEntry のみが gateway frame に乗る。
// display name / capability / session count はここに**乗らない** (gateway no-domain 原則)。
```

#### TypeScript wire frames

**Gateway channel** (browser ↔ gateway control plane):

```ts
type GatewayHelloFrame = {
  k: "gw_hello";
  client_id: string;
  hosts: HostDirectoryEntry[];
  default_host_id?: string;        // New Session の preference (browser→gateway→browser で echo)
};

type HostStatusChangedFrame = {
  k: "host_status";
  host_id: string;
  online: boolean;
  signaling?: SignalingHints;      // ICE refresh
};

// WebRTC SDP/ICE relay。payload は Noise で包んだ後 base64
type SignalingFrame = {
  k: "signal";
  host_id: string;
  payload_b64: string;
};

type GatewayFrame =
  | GatewayHelloFrame
  | HostStatusChangedFrame
  | SignalingFrame
  | RespOKFrame
  | RespErrFrame;
```

**Host channel** (browser ↔ host data plane, P2P / LAN / opaque relay):

```ts
type HostHelloFrame = {
  k: "host_hello";
  host_id: string;
  capabilities: string[];           // ["driver.codex", "agent.claude-sonnet-4-6", "sandbox.devcontainer", ...]
  sessions: SessionInfo[];          // この host の session 全量 (空配列も valid)
  // active_session の hint は出さない — active は browser 単独管理
};

type HostViewUpdateFrame = {
  k: "host_view_update";
  host_id: string;
  sessions?: SessionInfo[];         // 全量 snapshot (ADR-0023 と同じ broadcast-shape 規約)
  removed_session_ids?: string[];   // host-local id
};

type OutputFrame = [number, "o", string, string];   // [timeSec, "o", b64, session_id]。session_id は host-local

// 既存 ControlFrame / TranscriptTailFrame / EventLogTailFrame / NotificationFrame / RespOK / RespErr は
// すべて per-host channel に乗る。session_id は host-local として扱う。

type HostFrame =
  | HostHelloFrame
  | HostViewUpdateFrame
  | OutputFrame
  | ControlFrame
  | TranscriptTailFrame
  | EventLogTailFrame
  | NotificationFrame
  | RespOKFrame
  | RespErrFrame;
```

#### 既存 `HelloFrame` / `ViewUpdateFrame` の扱い

- 既存 `HelloFrame` (sessions + activeSessionID 同梱) は **gateway channel に乗らない**。Phase 1 (single-host fixed) では `HostHelloFrame` をそのまま既存 `HelloFrame` の入れ替えに使い、wire compat を 1 段で切る (ADR-0021 hand-written wire 規約に従う)。
- `activeSessionID` は **どの frame にも乗らない**。browser 単独管理 ([[web-active-session-ownership]]) を P2P-first でも維持。

### 2.4 ID 空間: host-local + composite global

- **Session ID は host-local 一意**。host を跨いで一意である必要はない (ULID 80bit randomness で実用上の衝突は無視可能)。
- **Global identity は composite `(host_id, session_id)`**。browser / orchestrator / audit はすべて composite で扱う。
- **Frame ID も host-local**: `(host_id, session_id, frame_id)` で global。
- wire 上の `SessionInfo.id` 自体は host-local のまま (既存 codec resource を維持)。browser は受信 channel の host_id と組合せる。`SessionInfo.host_id` の field 値は host が常に同じ値を入れる **defensive redundancy** (mismatched 受信は protocol error として `RespErr` で拒否)。
- gateway 内 routing は `(client_id, host_id)` まで。**session_id レベルの routing table は持たない** (P2P-first のため不要)。

### 2.5 Web client は host 非依存 (host 切替の概念を持たない)

この plan で**最重要の UX 不変条件**:

- **session list は host ごとに独立**して保持し、UI は接続できた host のものを section として並べる。**"merged" / "unified list" の概念を wire / state / UI のどこにも置かない**。"current host" / "active host" / "host 切替" のような **client 側 mode も存在しない**。完全性を装わない (接続不能 host は placeholder で正直に出す)。
- `activeSessionID` は client 単独管理を維持 ([[web-active-session-ownership]])。host_id は session を引いた結果として自然に導出される (state に "current host" を持ち上げない)。
- frame 切替で host_id が変わっても UI 側に特別処理は要らない。各 frame は自分の host channel に向かうだけで、上位 state に "切替" は出ない。
- host offline の session も section 内に残し、UI 上は overlay で reconnecting badge を出す。**自動で active から外さない** (ユーザの明示操作のみ)。
- "host を選ぶ" 操作は **New Session 時の 1 phase のみ**。それ以外で host を選択させる UI は出さない (header の host badge は read-only metadata 表示で、interactive な切替操作ではない)。
- LAN 直結 / WebRTC P2P / opaque relay の reachability 切替 (§9) は同一 host channel 内の transport 差分であって、client 側の mode 変化ではない。

## 3. アーキテクチャ全体図

Gateway は **relay / tunnel / authorizer の 3 責務のみ**、ドメイン情報を持たない (§0)。**data plane は browser ↔ host 直結** (WebRTC P2P を一次、LAN 直結が次、gateway opaque relay が最後の fallback)。

```
                     ┌──────────────────────────┐
                     │        GATEWAY            │
                     │  (RELAY / TUNNEL /        │
                     │   AUTHORIZER ONLY)        │
                     │                            │
                     │  - control tunnel (gRPC)   │   ← tunnel
                     │  - WebRTC signaling        │   ← relay
                     │  - opaque relay fallback   │   ← relay
                     │  - WebAuthn/passkey login  │   ← authorizer
                     │  - user-signed op forward  │   ← authorizer
                     │  - audit (pubkey + byte    │
                     │    counter のみ)            │
                     │                            │
                     │  持たない: display name,    │
                     │   capability, session/     │
                     │   frame meta, ACL 中身      │
                     └──┬──────┬────────┬─────────┘
   signaling /          │      │        │      / signaling
   heartbeat (gRPC)     │      │        │
                        ▼      ▼        ▼
                     ┌────┐ ┌────┐  ┌────┐
                     │ H1 │ │ H2 │  │ H3 │   (server processes, per PC)
                     └─┬──┘ └─┬──┘  └─┬──┘
                       │      │       │
        ═══════════════╪══════╪═══════╪══════════════ data plane
                       │      │       │     (WebRTC P2P / LAN / opaque relay)
                       ▼      ▼       ▼
                     ┌────────────────────┐
                     │      Browser        │
                     │                      │
                     │  N parallel host     │
                     │  channels (1/host)   │
                     │  + 1 gateway channel │
                     │                      │
                     │  接続できた host を   │
                     │  per-host section で │
                     │  並べる (merge せず) │
                     └──────────────────────┘

LAN 直結時:   Browser ──── HOST (gateway を経由しない、mDNS 発見 + 同 E2E 鍵)
```

データ経路の優先順位 (host channel ごと独立に動的選択):

1. **LAN 直結** (mDNS 発見 + pubkey TOFU、gateway 不要)
2. **WebRTC P2P** (gateway は signaling のみ。確立後 data は gateway を経由しない)
3. **Gateway opaque relay** (LAN/P2P いずれも不能時。gateway は ciphertext を素通す。session の存在/件数を知らない)

すべての経路で上位に **同じ Noise セッションを再利用**するため、経路切替はアプリ層から透過。経路は **host channel 単位**で選択され、host ごとに違う経路を同時に使う (例: H1 は LAN 直結、H2 は P2P、H3 は relay fallback)。

## 4. コンポーネント定義

### 4.1 Gateway (新設) — `cmd/gateway/` + `gateway/`

#### 4.1.1 責務 (relay / tunnel / authorizer のみ、ドメインフリー)

Gateway は **agent-reactor のドメイン (session / frame / driver / agent / capability / host display name / ACL 中身) を持たないし解釈しない**。下記すべては relay / tunnel / authorizer のいずれかの責務に分類される。

- **[Tunnel] Control tunnel terminator**: host からの outbound gRPC bidi stream を保持。用途は heartbeat + signaling 中継 + user 署名済 ACL op の opaque な push + (optional) opaque relay fallback。**session_id ベースの routing は持たない** (P2P-first のため session は host ↔ browser 直結)。
- **[Tunnel] Host directory**: 登録済 host の **pubkey fingerprint + signaling hints (ICE / LAN endpoint) + online 状態** のみを保持し browser に push。**display name や capability snapshot は持たない**。display name は pairing 時に browser localStorage に保存される (gateway を経由しない、§4.3.2)。capability は host channel 経由で host が直接 browser に出す (Q17 案 A 確定)。
- **[Relay] Browser control edge**: WSS endpoint。authorizer 認証 → directory 配布 + signaling 中継。data plane は経由しない。
- **[Relay] WebRTC signaling**: SDP offer/answer/ICE candidate の opaque 中継。Noise 暗号文として扱い解釈しない。
- **[Relay] Opaque relay fallback**: P2P / LAN いずれも不能な host channel についてのみ、browser ↔ host 暗号文 stream を素通し relay。**gateway は session の存在も件数も認識しない**、byte counter のみ。
- **[Relay] ACL op forwarder**: client 追加/削除の operation を **opaque な user 署名付き blob** として該当 host に push。gateway は **op 内容を parse しない**。最終的な authz は host が自分の allowlist で user 署名を verify して決定する。
- **[Authorizer] User identity authn**: WebAuthn/passkey で user を認証し、scoped assertion token を発行。token は host への authority ではなく assertion (Q14)。
- **[Authorizer] Pairing service**: host / client 登録の short-lived token 発行 + user 署名集約。pairing 時に **display name 等のドメイン情報は受け取らない** (host CLI が pairing 完了後に browser へ pubkey を提示、browser が user に nickname を尋ねて localStorage に保存)。
- **[Tunnel] Audit log (pubkey + metadata only)**: 接続元 client pubkey、対象 host pubkey fingerprint、connection open/close、relay bytes 集計。**display name / session id / capability は記録しない** (gateway は元から知らない)。
- **[Tunnel] Health**: host heartbeat 集計、metrics export (control plane / relay 観点のみ、session 単位 metric は host 側で expose、§6.5)。

#### 4.1.2 内部構成

```
gateway/
  edge/
    browser.go       # WSS handler (tunnel: directory 配布 / signaling)
    host.go          # gRPC server (tunnel: heartbeat / signaling / acl op forward)
    signaling.go     # WebRTC SDP / ICE 中継 (opaque)
  registry/
    host.go          # in-memory + persisted host directory (pubkey fp + signaling hints のみ、display name や capability は持たない)
    client.go        # client identities / passkey credentials
    pairing.go       # join-token 発行 / redemption / user-signed op の opaque な保管
  authz/
    forward.go       # user 署名済 op (opaque blob) を host に forward (内容を parse しない)
    token.go         # short-lived assertion (host が verify)
  relay_fallback/
    opaque.go        # P2P/LAN 不能 host channel の暗号文 stream を素通し relay
    bandwidth.go     # per-(client, host) rate limit + byte counter
  audit/
    log.go           # connection metadata only (pubkey fp + bytes、session id / display name / capability は持たない)
  metrics/
    prometheus.go    # /metrics
cmd/gateway/
  main.go            # config load, edge bind, signal handling
```

#### 4.1.3 状態の永続化

- **必要**: host registry (**pubkey fingerprint + signaling hints のみ**、display name は持たない)、client identities (passkey credential)、pairing tokens、user 署名済 ACL op の opaque blob 履歴 (replay 配布用)、audit log (pubkey fp + byte counter)。
- **不要**: session state (host が持つ)、pty buffer (E2E のため不可)、host display name (browser localStorage)、capability snapshot (host channel 経由)。
- **採用**: sqlite (single-file、stdlib-only `modernc.org/sqlite` 検討。CGO 回避前提だと選択肢限定)。代替: bbolt (純 Go、KV)。decision は ADR で。
- **HA はスコープ外**: MVP は single-instance。設定/audit は外部 backup スクリプトで。

#### 4.1.4 設定

- `~/.agent-reactor/gateway.yaml` or `--config` (既存 dotfile 規約に乗る)。
- TLS cert は ACME (Let's Encrypt) 自動 / 手動 cert 両対応。dev は self-signed。
- Bind: browser edge は `:443`、host edge は `:8443` (gRPC) を既定。

### 4.2 Server (host) の変更 — `client/` + `cmd/server/`

#### 4.2.1 削除/縮退する責務

- 既存の **browser-facing HTTP/WS endpoint** は **WebRTC DataChannel endpoint + LAN-direct WSS** に置換 (gateway tunnel ではなく browser と直結)。
- LAN-direct WSS listen は MVP 時点から残す (gateway 不在環境のため)。default on、`--no-lan-listen` で停止可能。

#### 4.2.2 新規責務

- **Gateway control client** (`client/gateway/`): gRPC bidi outbound。heartbeat / signaling 受信 / opaque な user 署名済 op の受信。**session bytes も capability snapshot もこの channel に流さない** (gateway はドメインを持たない)。
- **WebRTC peer endpoint** (`client/webrtc/`): browser からの incoming offer を gateway 経由で受け、PeerConnection 確立 → DataChannel で session API を serve。
- **LAN-direct endpoint** (`client/lanlisten/`): WSS direct (self-signed cert + pubkey pinning)。WebRTC と同じ session API を serve。
- **E2E endpoint** (`client/e2e/`): Noise responder。transport (WebRTC / LAN / fallback relay) 非依存。確立した鍵で host channel 全体を暗号化。
- **Per-browser session server** (`client/hostchan/`): 1 browser connection ≒ 1 host channel として `HostHello` / `HostViewUpdate` / `OutputFrame` を serve。state は既存 `client/state/` を流用、view-update broadcast に **host_id を常に付与**。**capability snapshot もここの `HostHelloFrame` で配布する** (gateway は capability を持たない、Q17 案 A 確定)。
- **Host identity** (`client/identity/`): Ed25519 鍵生成、pubkey export、pairing 応答。
- **Client allowlist (authz authoritative)** (`client/authz/`): 自 host に接続を許す client pubkey 集合。gateway が opaque に forward する user 署名済 op を host が自分で verify して取り込む。最終決定は host 側 (signed-by-user op のみ accept、unsigned や gateway 単独の push は reject)。
- **mDNS advertiser** (LAN 直結用): `_agent-reactor._tcp.local.` を advertise。鍵 fingerprint を TXT に乗せる (display name は TXT に出さない、それを引き当てる nickname は browser 側にある)。

#### 4.2.3 既存責務 (維持)

- pty session lifecycle、driver registry、sandbox 管理、credential proxy。
- frame teardown → sandbox release の経路 (d1e3a8c4)。
- env overlay invariant ([[host-direct-env-inherit]])。

### 4.3 Web client の変更 — `src/client/web/`

#### 4.3.1 状態スライス追加

```ts
// src/client/web/src/state/hostsSlice.ts (新設)
type HostsState = {
  directory: HostDirectoryEntry[];                // gateway hello から (+ HostStatusChanged で patch)。display name や capability はここに**入らない**
  nicknames: Record<string, string>;              // pubkey_fingerprint → user-defined nickname (localStorage 永続、gateway は持たない)
  connections: Record<string, HostConnection>;    // host_id → 接続状態
  defaultHostID: string | null;                   // New Session Phase 1 用 preference (localStorage 永続)
};

type HostConnection = {
  hostID: string;
  transport: "p2p" | "lan" | "relay";
  status: "connecting" | "connected" | "disconnected";
  sessions: SessionInfo[];                        // host channel から受領 (HostHello + HostViewUpdate)
  capabilities: string[];                         // host channel `HostHelloFrame.capabilities` から受領 (gateway 経由ではない)
  lastError?: string;
};

// host を識別する key は pubkey_fingerprint (TOFU 後不変)。host_id は gateway 発行の opaque routing key で
// pubkey rotation 時のみ別物になり得る。nicknames は pubkey_fingerprint を key にすることで rotation 耐性を持つ。
```

- `activeSessionID` slice は変更なし (client 単独管理を維持)。host_id は `SessionInfo.host_id` を読むだけで導出。
- **Session list は per-host のまま**。UI は `connections[host_id].sessions` を host ごとに iterate して section 描画する。**store にも selector にも "merged sessions" を作らない** (single source of truth は per-host で完結、flatten 表示は presentation 層の自由)。
- **Nickname store は localStorage で完結**。pairing 完了時に `nicknames[pubkey_fingerprint] = userInput` を保存。display は `nicknames[fp] ?? pubkey_fingerprint.slice(0, 8)` を fallback。gateway はこの情報を**一切持たない / 配布しない**。複数 device 間の nickname 同期は MVP では未対応 (将来 P2P-first な sync は別 plan)。
- New Session palette で**選択中の host は palette phase store にローカル保持** (ADR-0036 / 0056 系の slice composition と整合)。global state に "selected host" を持ち上げない — host 選択は palette 内のフォーム状態であって app mode ではない。
- `directory` は **GatewayHelloFrame** で初期化、`HostStatusChangedFrame` で更新。`connections` は host channel ごとに独立 reducer で managed。
- WebRTC PeerConnection / LAN WSS / relay-fallback の transport 切替は `transport` 表示のみ反映、`sessions` には影響しない (Noise セッション継続)。

#### 4.3.2 UI 変更点

1. **Header**: active session の host を **read-only badge** で表示 (`<nickname> · via relay` / `<nickname> · LAN` 等)。`<nickname>` は browser localStorage `nicknames[pubkey_fp]` を引き当てる、未設定は pubkey fp prefix で fallback。host offline 時は色変化。**登録 host が 1 つの環境では非表示** (visual noise 削減)。クリック時は palette `Inspect host` で詳細 (pubkey fp 全文、reachability、connection 統計) を開く (情報閲覧、host 切替ではない)。
2. **Session list (drawer / list)**: 接続できた host ごとに **section** で並べる (詳細は §10.2)。接続失敗 / disconnect の host は section header のみ表示 + `Not reachable` placeholder。完全性は装わない。host 1 つの環境では section header を省略 (visual noise 削減)。
3. **Command palette `New Session`**:
   - **HostSelectPhase** を新設 (ToolSelectPhase の前段)。
   - 選択 host で利用可能な driver/agent のみを後続 phase に列挙 (capability filter は host channel 確立後に reduce、§12.2)。
   - default host があれば auto-skip (`Enter` 単発で確定)。
   - Host offline は disabled visible + skip-navigation (palette 規約 ADR-0050)。
4. **Reconnect indicator**: 個別 session が host disconnect 中なら terminal pane 上に半透明 overlay (`Host <nickname> is offline — reconnecting…` + spinner)。nickname 未設定なら pubkey fp prefix で fallback。
5. **Default host setting**: 設定ペイン (palette `Settings > Default host`)。
6. **TOFU nickname 入力**: 初回 host 接続時に inline で nickname を尋ねる palette flow。skip 時は pubkey fp prefix が以後の表示に使われる (空欄許容)。
7. **Rename host**: palette `Rename host` で nickname を後から更新 (localStorage のみ、gateway/host に通知なし)。

#### 4.3.3 振る舞いの不変条件

- frame teardown 時の sandbox release 期待は変えない (host 側で処理)。
- session が `host disconnect` 中でも frame は残す。再接続時に view-update で復元。
- frame teardown / new session を host offline 時に発行した場合は 503 (palette ADR-0046 相当の 409 と類似の reason code) で明示拒否。silent fail させない。

## 5. トランスポート層

control plane (gateway 経由) と data plane (host 直結) を **明確に分離**。data plane は host channel ごと独立に transport を選ぶ。

| 役割 | 区間 | プロトコル | 暗号 | 多重化 |
|---|---|---|---|---|
| **Control plane** | Browser ↔ Gateway | WSS (HTTP/1.1 upgrade) | TLS 1.3 | 1 client = 1 WS、frame は `gw_hello`/`host_status`/`signal`/`resp` のみ |
| **Control plane** | Host ↔ Gateway | gRPC bidi (HTTP/2) | mTLS | host あたり 1 tunnel、heartbeat + signaling + acl push + (optional) fallback relay |
| **Data plane (primary)** | Browser ↔ Host (P2P) | WebRTC DataChannel (DTLS-SCTP) | DTLS + 上位 Noise | 1 PeerConnection / host、session ごと DataChannel (or 1 channel + 上位 mux) |
| **Data plane (LAN)** | Browser ↔ Host (LAN) | WSS direct | TLS (self-signed) + 上位 Noise | 1 WSS / host |
| **Data plane (fallback)** | Browser ↔ Gateway ↔ Host | gRPC stream を opaque relay | DTLS / TLS + 上位 Noise (gateway は素通し) | 1 stream / host channel |

#### 重要原則

1. **すべての data plane 経路で Noise を transport の上に被せる**。経路切替 (P2P → relay 等) で host channel の Noise セッションを切らない。
2. **Browser ↔ Host data は WebRTC が一次**。WS direct は LAN 限定、gateway relay は最後の fallback。
3. **Gateway は data plane を semantic に解釈しない**。relay fallback でも payload は Noise ciphertext として素通す。
4. **HostChannel の多重化粒度**: 1 PeerConnection = 1 host。session 単位の stream 分割は WebRTC DataChannel の複数開設で行う (DataChannel は cheap)。代替: 1 DataChannel + 上位 yamux mux (decision は ADR)。
5. **HoL 設計**: data plane は WebRTC = SCTP unordered stream を許容。WSS direct (LAN) のみ HoL あり、session 数が少ない LAN 用途では許容。
6. **WebTransport (HTTP/3)** は将来の WS direct 置換候補。MVP では使わない。

#### control plane proto スケッチ

```proto
// platform/wire/proto/control.proto (新設)
// Gateway no-domain 原則 (§0): control plane proto は session / capability / display name を持たない。
service GatewayControl {
  // Host が outbound で呼ぶ。Heartbeat + signaling + user 署名済 op forward + (optional) opaque fallback relay。
  // session bytes も capability snapshot もここには流れない。
  rpc Connect(stream HostControlMsg) returns (stream GatewayControlMsg);
}

message HostControlHello {        // control plane の host registration (data plane の HostHelloFrame と別名で衝突回避)
  string host_id = 1;
  bytes  signature = 2;
  // capabilities は載せない (gateway no-domain、Q17 案 A 確定)。capability は host channel `HostHelloFrame` で browser へ直接送る。
  repeated LanEndpoint lan_endpoints = 3;     // signaling hints として gateway が routing 用にのみ使う
  string version = 4;
}

message SignalingRelay {        // SDP / ICE / fallback relay 開始通知の総称
  bytes  client_pubkey_fp = 1;   // gateway 側 audit 用 (中身は知らない、display name は付与しない)
  string sig_session_id = 2;     // signaling 単位の id (session_id ではない)
  bytes  payload = 3;            // opaque (browser ↔ host の Noise ciphertext or WebRTC SDP)
}

message UserSignedOp {            // ACL 等の op を gateway が opaque に forward する汎用 envelope
  string op_kind = 1;             // "acl_add" / "acl_remove" / "host_revoke" など (envelope ラベル、gateway は dispatch 用にしか使わない)
  bytes  signed_payload = 2;      // user identity key で署名済 blob (gateway は parse しない、host が verify)
  string user_id = 3;             // gateway authorizer が認証した user (host が user_id と signed_payload の署名者を突合)
}
// data plane stream は WebRTC DataChannel に直接乗るため proto には現れない
// host display name / capability は proto に**載らない** (gateway no-domain)
```

### Noise + transport の分離

- **Noise pattern**: `XK` (client が host pubkey を事前に知っている前提) を基本、初回 pairing 中のみ `IK` か `XX` を使い分け。Decision は ADR で。
- **Rekey 周期**: 1 GiB or 1 時間 (どちらか先到達)。
- **Forward secrecy**: Noise の ephemeral key で担保。長期鍵漏洩でも過去 session は復号できない。

## 6. セキュリティ設計

### 6.1 脅威モデル (STRIDE 簡略)

| 脅威 | 対策 |
|---|---|
| Gateway 運用者の悪意 / 侵害 | E2E (Noise) で pty 平文を関与不能に。**ドメインフリー設計により leak 面積は pubkey fingerprint + relay traffic pattern のみ** (display name / capability / session 件数 / driver 一覧は gateway に存在しない) |
| 中間網盗聴 | TLS 1.3 + 上位 Noise (二重) |
| Host への不正接続 | mTLS (control plane) + Noise XK (data plane) + **host 側 client pubkey allowlist が authz authoritative** (gateway は user 署名済 op を opaque に forward するのみ、host が中身を verify して accept) |
| Client 盗用 (PC 紛失) | client device cert を gateway から即時 revoke、host 側にも user 署名済 remove op を opaque forward |
| Replay | Noise nonce + gateway 側 assertion token TTL |
| 横展開 (1 host 経由で他 host) | host 間に信頼関係なし。client ↔ host 1:1 (session 単位) |
| DoS (gateway 帯域) | client / host ごと rate limit、opaque relay の per-(client, host) 帯域配分 |
| Side channel (pty 中身を audit log で漏らす) | 設計上 audit log に中身を入れない (E2E で技術的にも不可) |
| LAN 直結時の adversary | mDNS 偽装防止に pubkey pinning (TOFU)、cleartext 経路を持たない |
| Display name / nickname の leak | Browser localStorage のみに保持、gateway に送らない。device 共有 (PC 紛失) のみがリスク面 — gateway 侵害では漏れない |

### 6.2 認証チェーン

#### 6.2.1 Host → Gateway

1. host 起動時に Ed25519 鍵をローカル生成 (`~/.agent-reactor/host.key`、0600)。
2. 初回 pairing: gateway が発行した join-token (`AR-PAIR-XXXXX-XXXXX`、TTL 10 分) を host 側 CLI に投入 → host が pubkey を gateway に登録。
3. 以降は **mTLS** で host cert ↔ gateway cert 双方向検証 + 起動時 `HostHello` で nonce 署名。

#### 6.2.2 Client → Gateway (control plane のみ)

1. browser は **passkey / WebAuthn** で gateway にログイン。OAuth/OIDC への置換可能性は ADR で。
2. gateway が **scoped assertion token** (短命、5–15 分) を発行。token は `(client_id, allowed_host_ids, scope, exp)` を含み、**user identity key で署名**される。
3. WSS 接続時に Authorization header で送る (gateway 側で session 発行検証 + 後段の host への ACL push に流用)。
4. **token は host への authorization ではなく assertion**: host channel 確立時に host が独立に検証し、自分の allowlist と突合する (Q14)。gateway 単独では新規 client を accept させられない。

#### 6.2.3 Client → Host (E2E)

1. directory 経由で client が host の pubkey fingerprint を取得 (`HostDirectoryEntry.pubkey_fingerprint`、初回は TOFU で UI 確認)。
2. transport (LAN / WebRTC / opaque relay) 確立 → 上位で Noise XK handshake (gateway は relay 時も中身を読まない)。
3. 確立後、**host channel 全体が 1 Noise セッション**。session ごとに鍵を分けない (rekey 周期は §5 末尾)。pty bytes は Noise framing 内に `session_id` (host-local) で分けて流す。

### 6.3 ペアリングフロー

#### 6.3.1 Host pairing (CLI)

```sh
# 1. gateway 側で token 発行 (admin web or CLI)。display-name は受け取らない (gateway no-domain 原則)。
gateway pair host --ttl 10m
> AR-PAIR-7K3M-Q92X   # 出力 (gateway は token と pubkey 受領予定のみを記憶)

# 2. host 側で投入
server pair --gateway https://gw.example.com --token AR-PAIR-7K3M-Q92X
> Generated host key (fingerprint: 9f:3a:...)
> Registered with gateway (pubkey only).
> Tunnel established.
```

#### 6.3.2 Client pairing (browser)

- gateway URL を初回入力 → passkey 登録 (WebAuthn) → admin 承認 (任意の許可ポリシー)。
- Client device 単位 (browser profile + WebAuthn credential)。
- 初回 host 接続時 (TOFU 後) に **browser が nickname 入力 UI を出す** → localStorage `nicknames[pubkey_fp]` に保存。gateway には送らない。

#### 6.3.3 Host TOFU (Client 側)

- Client が初めて見る host の pubkey は palette で fingerprint 表示 + ユーザ確認 + nickname 入力。承認後 localStorage に pin。
- 鍵が変わったら明示拒否 + 警告 (SSH の `Host key verification failed` 相当)。nickname は pubkey_fp に紐づくため、新 pubkey は別 nickname として扱う。

### 6.4 失効

- **Host revoke**: gateway 側 admin から pubkey fingerprint を blocklist → control tunnel に `GOAWAY` → 既存 LAN/P2P/relay の data plane は host 側で **user 署名済 revoke op の opaque forward を受領した時点で切断** (Q18)。gateway は op の中身を解釈しない、ただ forward する。LAN 直結中など gateway 不経由の経路には別配送経路 (admin LAN CLI 等) が必要。
- **Client revoke**: user identity key で署名した remove op を host 群に opaque forward → host が allowlist から削除し既存 host channel を切断。並行して gateway 側 WebAuthn credential を削除 (新規 assertion token 発行を止める)。
- **Host 側 panic kill**: ローカル CLI `server panic` で control tunnel を切断 + 全 host channel 切断 + 自分の鍵を destroy。盗難時用 (gateway が侵害されていても host だけで完結)。
- **Browser-local nickname**: device 紛失時は browser localStorage を消すだけで nickname / default host preference が消える。gateway / host への通知不要 (元から知らない情報)。

### 6.5 監査・観測

- **Gateway** (ドメインフリー観測、relay/tunnel/authorizer の観点のみ):
  - `audit.log` に `(timestamp, client_pubkey_fp, host_pubkey_fp, event, bytes_relay_in, bytes_relay_out)`。`event ∈ {control_open, control_close, signaling_relay, data_relay_open, data_relay_close, authz_signin, op_forward}`。**session_id / display_name / capability / driver は記録しない** (gateway は元から知らない)。
  - `op_forward` event は **op 種別 (例: "acl_add") のみ**記録、user 署名済 payload の中身は opaque blob として保管 (admin 用 replay)、ただし parse 解釈しない。
  - pty 中身は当然含めない (Noise で読めない)。
- **Host** (per-host channel ごとに session レベルを観測): 既存 logger (`platform/logger`) に `host_id` を一貫付与。session-level event は host audit のみに置く。display name は host も知らない (browser-local) ため pubkey fp で識別する。
- **Metrics (Prometheus)**: `active_control_tunnels` (gateway↔host)、`active_host_channels` (browser↔host、host 集計、gateway は host channel 単位のみ)、`e2e_handshake_errors`、`p2p_establish_success_rate`、`opaque_relay_bytes_total`。**`active_sessions_per_host` は host 側で expose**、gateway 側 metrics には出さない。**`distinct_drivers_per_host` のような capability 由来 metrics も gateway には出さない**。

## 7. データプレーン詳細 (P2P-first)

### 7.1 Control plane の確立 (gateway 経由、heartbeat 維持)

1. Host 起動 → gRPC `Connect()` を gateway に dial (TLS 1.3 + mTLS)。
2. 最初の message: `HostControlHello{host_id, signature(nonce), lan_endpoints, version}` (これは control plane proto。data plane の `HostHelloFrame` とは別物)。**capabilities は含めない** (gateway no-domain 原則、host channel 経由でのみ送る、Q17 案 A 確定)。
3. gateway は host directory を update (pubkey_fp + signaling hints + online のみ) → 接続中の browser に `HostStatusChanged{online:true}` を push。
4. 以降は heartbeat + signaling + user 署名済 op の opaque forward のみが control tunnel を流れる。**session bytes も capability snapshot もここに流さない**。

### 7.2 Browser ↔ Host channel の確立 (data plane)

順序:

1. Browser が gateway WSS にログイン → `GatewayHelloFrame` で directory を受領。
2. Browser は online host ごとに **並行に host channel 確立を試みる** (lazy ではなく eager — N hosts 並列接続):
   - **a. LAN 直結**: directory の `lan_endpoints` を 200ms tap (RTT + pubkey fingerprint 検証)。成功すれば即採用。
   - **b. WebRTC P2P**: ICE candidates 交換 (signaling は gateway 経由 = `SignalingFrame`)。成功すれば採用。
   - **c. Opaque relay fallback**: 上記いずれも 5 秒で確立しなければ gateway opaque relay を要請。
3. transport 確立後、上位で **Noise XK handshake** (host pubkey 既知 = directory で配布済)。
4. Noise 確立後、host が `HostHelloFrame{host_id, capabilities, sessions}` を送る → browser が `connections[host_id]` を populate。
5. 以降 `HostViewUpdateFrame` / `OutputFrame` / `RespOK`/`RespErr` を流す。

### 7.3 多重化粒度

- **1 host channel = 1 PeerConnection** (or LAN WSS、or relay-stream)。
- session ごとの多重化は **WebRTC DataChannel を session 数だけ開設** が一次案。DataChannel は cheap (確立 RTT 1〜2)。
- 代替案: 1 DataChannel + 上位 yamux mux。判断は ADR (Open Q5)。
- Output frame の `[time,"o",b64,session_id]` shape は維持 (session_id は host-local)。

### 7.4 Reconnect / resume

- **Per-host 独立**。host A の切断は host A 接続だけに影響、B/C は無関係に流れ続ける。
- 各 host channel は exponential backoff + jitter で再接続。最初は前回成功した transport (P2P/LAN/relay) を試し、ダメなら次に降格。
- Host 復帰後、既存 session を `HostHello.sessions` で**全量再送** (ADR-0023 broadcast-shape を per-host で踏襲)。Browser は `connections[host_id].sessions` を置換。
- Terminal scrollback (ADR-0066) は browser 側 buffer。host 再接続後の `OutputFrame` 連続性は既存 ADR-0025 (transcript REST backfill → WS tail) パターンを per-host channel で再利用 (詳細は §15 Phase 3 で詰める)。

## 8. P2P / LAN / Relay の経路選択

### 8.1 経路の役割 (再掲)

P2P は **MVP からの一次データプレーン**。Phase 2 ではなく wire shape の前提。

| 経路 | 一次条件 | gateway 関与 | 暗号 |
|---|---|---|---|
| LAN 直結 | 同一 LAN + LAN endpoint 到達可 | 不要 | TLS self-signed + Noise |
| WebRTC P2P | ICE 確立成功 | signaling のみ | DTLS + Noise |
| Opaque relay | 上記いずれも不能 | 素通し relay | TLS + Noise (gateway は ciphertext のみ) |

### 8.2 シグナリング

- Browser が PeerConnection を作成 → offer SDP を Noise で wrap → `SignalingFrame.payload_b64` で gateway 経由送付。
- Host が answer SDP + ICE candidates を返す (同様に Noise wrap)。
- gateway は SDP/ICE を opaque に relay。SDP の中身を知らない (鍵共有していない)。

### 8.3 切替セマンティクス

- ICE 失敗 / LAN 不達は host channel 単位で発生。**他 host channel に影響しない**。
- 同一 host channel 内での経路降格 (P2P → relay) は Noise セッションを維持したまま transport を差し替え。Browser store の `connections[host_id].transport` だけ変わる。`sessions` には影響しない。
- 手動 `Force relay` / `Force P2P` は palette の `Inspect host` に置く (デバッグ用)。

### 8.4 セキュリティ

- DTLS / TLS の証明書は ephemeral。**長期信頼は上位 Noise セッションが提供**。
- transport 切替時に Noise session を維持 (新 transport に Noise state を bind し直すだけ)。
- Opaque relay 経由でも gateway は Noise ciphertext のみ取り扱う。session 件数も知らない (relay 単位 = host channel 単位)。

## 9. LAN 直結

### 9.1 発見

- Host が `_agent-reactor._tcp.local.` を mDNS advertise。TXT に `fp=<sha256-pubkey-prefix>` と `port=8081` のみ。**display name は TXT に出さない** (nickname は browser localStorage、gateway/host 双方が知らない、§4.3.2 / §6.3)。
- Browser は LAN 上にいる場合 (gateway 接続失敗 or ユーザが明示選択時) mDNS proxy 経由で発見。**mDNS は browser から直接叩けないため**、LAN 用の小さい discovery helper が必要 (Decision 必要 — §16)。
- 発見した host の表示名は browser localStorage の `nicknames[fp]` を引き当てる。未知 fp は TOFU フローで nickname 入力。

### 9.2 接続

- WSS direct (self-signed cert)。
- Cert 検証は **pubkey fingerprint pinning** (TOFU)。CA 信頼は使わない。
- 上位 Noise セッションは gateway 経由と同じ鍵材料で確立。**ペアリング情報を gateway 経由で済ませておけば LAN 直結はそのまま動く**。

### 9.3 切替 UX

- 同じ host channel が LAN 直結 / WebRTC P2P / opaque relay のいずれで成立しているかは **同一 host エントリ内の reachability badge** で示す。複数項目には**しない**。
- 自動選択順: LAN > WebRTC P2P > opaque relay。手動 override は palette `Inspect host` で。

## 10. Host を UI に露出させる場所

### 10.1 New Session

- Palette 起動 → `New Session` を選択 → **Phase 1: HostSelect** → Phase 2: ToolSelect → Phase 3: ParamSelect (既存 ADR-0050 と整合)。
- default host があれば Phase 1 を `Enter` で skip。
- Phase 2 は host channel 確立後 capability で filter (gateway no-domain のため事前 filter は不可、§12.2 案 A 確定)。未接続 host を選んだ場合は最初 unfiltered → 確立後に reduce。
- Offline host は disabled visible + skip-navigation。reason: `Offline since 14:32` (last_seen_at は directory に乗る gateway 情報、相対表示は browser 側でフォーマット)。
- Host 表示は `nicknames[pubkey_fp] ?? pubkey_fp.slice(0, 8)` で、初回 TOFU 時のみ inline で nickname 入力欄を出す。

### 10.2 Header / Session list

- **Header**: active session の host を read-only badge で表示 (`<nickname> · via <reachability>`)。`<nickname>` は browser localStorage の `nicknames[pubkey_fp]` を引き当てた値。display only — クリックで palette `Inspect host` を開けるが、host を切替えるための UI ではない。**登録 host が 1 つの環境では非表示**。
- **Session list**: 接続できた host ごとに section を出す (`<nickname>` header + その host の session card)。**接続失敗 / disconnect の host は section header のみ表示 + `Not reachable` placeholder**。展開しても session card は出さない。完全性は保証しない — UI はその時点で reachable な事実を出す。
- **Section 順序**: default は last-connected の new 順。host が少ない (例: < 5) ときは全展開、多いときは default 折り畳み (last-connected のみ展開) — 閾値は user setting で override。
- **Card 共通装飾**: section 内 session card は ADR-0033 系の `border_title_secondary` を踏襲。host 名は section header に出ているので card 内の host badge は省略 (二重表示を避ける)。
- **Nickname 編集**: palette `Rename host` で nickname を後から変更可能 (localStorage 更新)。gateway / host への通知は発生しない。

### 10.3 Default host

- **New Session の Phase 1 で auto-select 候補にする default 値** (それ以上の意味は持たない — "current host" ではない)。
- localStorage 永続 (pubkey_fp で記録)。複数 device で同期しない (MVP)。
- 設定: palette `Settings > Default host for New Session` → host listbox (nickname 表示)。
- default host を変えても**既存 session の見え方は変わらない** (session list は常に接続できた host の section で構成される)。

### 10.4 Reconnect 表示

- Session terminal の上に半透明 overlay (`Host <nickname> is offline — reconnecting…` + spinner)。nickname 未設定なら pubkey_fp prefix で fallback。
- ADR-0080 (status indicator exempt from reduced-motion) と同じ guard 規約に従う。

## 11. セッションライフサイクル (分散環境)

### 11.1 状態遷移

| State | 意味 |
|---|---|
| `pending` | host で session 作成中 |
| `running` | pty 稼働 |
| `idle` / `waiting` | 既存と同じ |
| `host-disconnected` | NEW: host channel (browser↔host data plane) 切断中。session 自体は host が保持 |
| `stopped` | 既存と同じ |
| `host-evicted` | NEW: host が revoke された / 再構築された等で session を放棄 |

### 11.2 Host 切断時の振る舞い

- **Per-host channel 独立**: host A の data plane 切断は **他 host channel に影響しない**。host A の session のみ `host-disconnected` 表示、host B/C のセッションは通常通り流れ続ける。
- **Gateway 側**: control tunnel の heartbeat lost を検知 → directory の `online:false` を全 browser に push (`HostStatusChangedFrame`)。session 状態は知らない (P2P-first のため)。
- **Browser 側**: 該当 host channel の reducer が `connections[host_id].status="disconnected"` に遷移 → その host が持つ session 全てを overlay 表示。terminal frame は破棄しない。input は queue せず drop (silent fail 禁止 → `RespErrFrame{code:"host_offline"}` でユーザに通知)。
- **Host 復帰**: browser が host channel 再確立 → host が `HostHelloFrame.sessions` で全量再送 → browser reducer が `connections[host_id].sessions` を置換。pty buffer は host-local なので session が残っていれば即復元。

### 11.3 Frame teardown / Sandbox release

- 既存経路 (d1e3a8c4) を host 内で維持。
- **client 切断は host が host channel の TCP/DataChannel 切断で直接検知**する (gateway 経由ではない、P2P-first のため)。検知しても **暗黙の teardown を送らない**: session は host が保持し続け、ユーザが明示的に teardown した時のみ sandbox release。
- 「browser 閉じたら片付ける」を望むときは別 plan (user setting で opt-in)。silent teardown はデフォルト禁止 (誤切断による作業消失の危険)。

### 11.4 Host eviction

- Admin が host を gateway directory から削除した時の段階遷移 (Q18 参照):
  1. gateway control tunnel に revoke 通知 → control plane 切断。
  2. user 署名済 client allowlist 全削除 op を `UserSignedOp` envelope に詰めて gateway 経由で host に opaque forward (LAN 直結 admin / 別 device など補助経路を冗長化) → host が自分で signed_payload を verify、allowlist を空にして既存 host channel を切断。
  3. browser 側は `connections[host_id]` が消失 → session を `host-evicted` に遷移、UI は read-only (terminal scrollback 閲覧のみ) に降格、新 input 拒否。Browser localStorage の nickname は admin op では消えない (device 側の判断)。
- **注意**: 2 が反映されるまでの間、既に確立済の LAN/P2P host channel は使い続けられる可能性がある (gateway directory 削除 ≠ 全 data plane 即停止)。Gateway は no-domain 原則により data plane を kill する authority も中身を見る能力も持たないため、即時切断は host への直接通達 (LAN 直結 admin / 物理 panic kill) が必要。これは設計選択 (P2P-first を取った帰結) として明示する。

## 12. Host capability 公開

### 12.1 Capability の粒度

- `driver.<name>` (例 `driver.codex`, `driver.claude`)
- `agent.<name>` (`agent.claude-sonnet-4-6` 等、host 上で sign-in 済か別 token 投入済を意味する)
- `sandbox.<kind>` (`sandbox.devcontainer`, `sandbox.host-direct`)
- `os.<platform>` (`os.linux/amd64`)
- `feature.<name>` (`feature.lan-direct`, `feature.p2p`)

### 12.2 公開タイミング (Decided — gateway no-domain 原則)

capability の出所は **host channel の `HostHelloFrame.capabilities` のみが authoritative**。Gateway は capability snapshot を持たない (no-domain 原則)。Q17 は **案 A 確定** で closed。

- capability は `HostHelloFrame` (data plane) でのみ受領する。
- New Session palette は host が未接続なら capability filter なしで全 driver を出し、host channel 確立後に reduce する。Initial render は最初に開いた host 候補の subset、後で update。
- 動的変化 (driver 認識 / agent sign-in) は **host channel 上の `HostViewUpdateFrame` (extended) で delta** を流す。gateway はこの delta を見ない。

### 12.3 ACL との関係

- ACL の**最終権威は host** (§4.1.1 / Q14): host が自分の `client_pubkey → scope` map を持ち、user 署名済 op だけを accept。
- gateway は ACL の **opaque forwarder**: user 署名済 add/remove op を opaque blob (`UserSignedOp`) として control tunnel で host に forward + 永続化と replay 配布を担当。**gateway は op の中身を parse しない**。
- `scope` 値: `interactive` (default) / `read-only` (input 拒否、output 閲覧のみ) / `admin` (host 自身の設定変更可) / `driver-allowlist:[...]` (利用可能 driver を制限)。Decision は ADR で。scope の解釈は host のみ。
- **capability filter ≠ ACL**: capability は host が「物理的に」できることの列挙、ACL は「許可された」操作の制限。両方を palette UI で重ねて適用する。両者とも gateway は知らない。

## 13. 既存アーキとの統合

### 13.1 ディレクトリ追加

```
src/
  cmd/gateway/main.go                   # NEW binary entry
  gateway/                              # NEW layer (relay / tunnel / authorizer only, no domain)
    edge/ registry/ authz/ relay_fallback/ audit/ metrics/
                                        # registry は pubkey fp + signaling hints のみ
                                        # authz は user signed op の opaque forward + token issuance
  platform/
    transport/                          # NEW: control proto + noise + grpc helpers
      control/ noise/ grpc/
    identity/                           # NEW: Ed25519 keys, fingerprints, pairing tokens
    wire/proto/                         # NEW: control.proto (host↔gateway control)
  client/
    gateway/                            # NEW: host 側 gateway control client (旧 client/tunnel/)
    webrtc/                             # NEW: host 側 WebRTC peer endpoint
    lanlisten/                          # NEW: LAN-direct WSS endpoint
    hostchan/                           # NEW: per-browser session server (HostHello/HostViewUpdate を serve、capability も配布)
    e2e/                                # NEW: Noise responder (transport 非依存)
    identity/                           # NEW: host key store
    authz/                              # NEW: client allowlist (authz authoritative、user 署名 verify を host が実施)
    discovery/                          # NEW: mDNS advertiser (TXT には pubkey fp + port のみ、display name は持たない)
    runtime/                            # 既存 (変更最小限、view-update に host_id 付与)
  client/web/src/
    state/hostsSlice.ts                 # NEW: HostsState (directory + nicknames + connections)
    storage/nicknameStore.ts            # NEW: pubkey_fp → nickname の localStorage adapter
```

### 13.2 depguard 拡張

`src/.golangci.yml` に追加:

```yaml
gateway:
  files: ["**/gateway/**"]
  allow: ["std", "platform/..."]    # gateway は platform のみ参照
  deny: ["client/...", "orchestrator/..."]

platform.transport:
  files: ["**/platform/transport/**"]
  allow: ["std"]                    # transport は完全に純粋 (Noise + gRPC helper)
```

`client/gateway/`, `client/webrtc/`, `client/lanlisten/`, `client/e2e/`, `client/hostchan/` は `platform/transport/` に依存。orchestrator から `client/` への参照禁止は維持。

### 13.3 Binary 数

3 → 4 (server / orchestrator / claude-app-server / gateway)。`make build-all` に gateway を追加。`reactor-bridge` は変更なし。

### 13.4 Makefile target

```makefile
build-gateway: ## Build → ./gateway
	cd src && go build -o ../gateway ./cmd/gateway
```

### 13.5 テスト方針

- Gateway 単体: in-memory control tunnel mock を `platform/transport/control/fake/` に置き、relay / tunnel / authorizer の各責務を `tier-1` で覆う。**gateway no-domain 不変条件の regression guard**: session_id / display_name / capability / driver list / agent name が gateway audit log と registry にいかなる経路でも入らないことを test で観察。`UserSignedOp` が gateway 側で parse されないこと (opaque blob 維持) も test で固定。
- 統合: server ↔ gateway ↔ web を 1 process で起動する e2e harness (`src/cmd/gateway/e2e_test.go`)。Tier-2。
- Noise handshake: 既知 test vector (Noise Explorer 出力) で固定回帰。
- 既存 [[host-direct-env-inherit]] / sandbox release 不変条件は host 側の既存テストで担保 (新規 transport で介入しない)。

## 14. Wire 型変更一覧 (実装時の差分カタログ)

P2P-first の前提で **Gateway channel と Host channel を別フレーム種** に分離する。既存 `HelloFrame` / `ViewUpdateFrame` は **削除して置換** (混在は wire-shape の semantics を曖昧にする)。

### 14.1 Channel 別フレーム一覧

| Channel | フレーム | 出所 | 目的 | 備考 |
|---|---|---|---|---|
| Gateway | `GatewayHelloFrame` (`k:"gw_hello"`) | gateway | client_id + host directory + default_host_id | session を含まない |
| Gateway | `HostStatusChangedFrame` (`k:"host_status"`) | gateway | host online / signaling hints の delta | |
| Gateway | `SignalingFrame` (`k:"signal"`) | gateway | WebRTC SDP / ICE の opaque 中継 | payload は Noise ciphertext |
| Gateway | `RespOKFrame` / `RespErrFrame` | gateway | gateway 側 op の応答 | code 拡張 (下記) |
| Host | `HostHelloFrame` (`k:"host_hello"`) | host | host_id + capabilities + sessions[] | per-host channel 確立直後 |
| Host | `HostViewUpdateFrame` (`k:"host_view_update"`) | host | per-host session 全量 broadcast (ADR-0023 規約) | |
| Host | `OutputFrame` (`[time,"o",b64,session_id]`) | host | 既存 shape のまま、session_id は host-local | |
| Host | `ControlFrame` / `TranscriptTailFrame` / `EventLogTailFrame` / `NotificationFrame` | host | 既存と同じ | session_id を host-local 解釈 |
| Host | `RespOKFrame` / `RespErrFrame` | host | host op の応答 | code 拡張 (下記) |

### 14.2 型レベル差分

| 型 | 変更 | 互換性 |
|---|---|---|
| `SessionInfo` | `host_id: string` (required) を追加 | wire shape が変わる → version bump |
| `FrameInfo` | `host_id: string` (required) を追加 | 同上 |
| (NEW) `HostDirectoryEntry` | `{ id, pubkey_fingerprint, online, last_seen_at, signaling }` (gateway no-domain: display_name と capabilities は持たない) | — |
| (NEW) browser-local `nicknames: Record<pubkey_fp, string>` | localStorage 永続、wire には乗らない | — |
| (NEW) `SignalingHints` | `{ ice_servers[], lan_endpoints[]?, relay_available }` | — |
| (REMOVED) 既存 `HelloFrame` | `GatewayHelloFrame` + `HostHelloFrame` に分離 | 旧 frame は撤去 |
| (REMOVED) 既存 `ViewUpdateFrame` | `HostViewUpdateFrame` に置換 | 旧 frame は撤去 |
| `RespErrFrame.code` 拡張 | `"host_offline"`, `"host_evicted"`, `"e2e_pinning_failed"`, `"host_id_mismatch"` (defensive redundancy 違反), `"unknown_host"`, `"signaling_timeout"` | — |
| (NEW) `ClientToHost` op | `Subscribe(session_id) / SendInput(session_id, b64) / Resize(session_id, cols, rows) / Detach(session_id)` 等は **host channel に直接送信** (host-local session_id)。gateway を経由しない | wire op の宛先が変わる |

### 14.3 同期方針

- `web/src/wire/server.ts` と `client/proto/response.go` の同期は ADR-0021 (hand-written) を維持。
- codec test (`web/src/wire/codec.test.ts`) と Go fixtures を **gateway frame / host frame 別 fixture file** に分けて維持。channel 跨ぎでの誤適用 (gateway channel に host frame が混入する等) を unit test で防ぐ。
- defensive redundancy (`SessionInfo.host_id` と受信 channel の host_id 不一致) は codec layer で reject → `RespErr{code:"host_id_mismatch"}`。

## 15. 段階導入計画

**wire shape は最初から P2P-first を確定**させる (channel 分離、composite ID、authoritative host ACL)。transport の実装順は別 — relay-fallback だけでも MVP は shippable で、P2P / LAN は後追いできる。phase の "wire 不変条件" は段階ごとに 0 段だけ変える方針。

### Phase 0 — 設計合意 (this plan)

- 本ドキュメント + 必要 ADR 起こし (channel 分離 / E2E / pairing / host ACL 配布 / WebRTC mux 粒度)。
- wire shape の確定 (§14)。

### Phase 1 — Host 概念導入 (single-host fixed)

- wire 型に `host_id` 追加 (固定値 `"local"`)。
- 既存 `HelloFrame` → `HostHelloFrame` に **置換** (gateway 不在のため `GatewayHelloFrame` はスキップ、host 直結のみ)。
- `SessionInfo.host_id` / `FrameInfo.host_id` を web UI に伝播 (display しないが内部で routing key 化)。
- depguard に `gateway/` 空 layer + 規約だけ追加。

### Phase 2 — Gateway MVP (relay/tunnel/authorizer 確立 + opaque relay-only data plane)

- `cmd/gateway` + `gateway/` の最小実装。pairing CLI (display name を受けない、pubkey 登録のみ)、host directory (pubkey fp + signaling hints のみ)、user 署名済 op の opaque forwarder、opaque relay-fallback。
- Host ↔ gateway gRPC control tunnel (mTLS のみ、E2E は次 phase)。
- Browser ↔ gateway WSS で `GatewayHelloFrame` + `SignalingFrame` (signaling 配線は通すが peer 確立は phase 4)。
- Browser ↔ Host data plane: **opaque relay fallback** 経路のみ実装。WebRTC は配線せず、すべて gateway 経由で素通し relay。
- Browser 側 **nickname store (localStorage)** と TOFU nickname 入力 UI を同 Phase で実装 (display name を gateway に持たせない原則のため、最初から必要)。
- multi-host UI: directory 表示 (nickname or pubkey fp prefix) / palette HostSelectPhase / **session list は per-host section 表示** (merged 型は作らない)。
- LAN 直結 endpoint は host 側で起動するが MVP UI は使わない (Phase 3 で接続)。

> 不変条件: この phase で wire shape は確定。Phase 3 以降は transport を差し替えても wire は変えない。gateway no-domain 不変条件 (display name / capability / session id を gateway に流さない) もここで test 固定。

### Phase 3 — LAN 直結

- Host mDNS advertiser + LAN-direct WSS endpoint (Phase 2 で実装済を web UI 側で接続)。
- Browser 側 LAN endpoint 到達試行 (gateway directory の `lan_endpoints` を tap)。
- 経路自動選択 (LAN > opaque relay)。

### Phase 4 — E2E Noise

- `platform/transport/noise` 実装 (Noise XK)。
- Host channel (LAN / relay 両方) を Noise で wrap。
- gateway audit log を metadata-only に確定 (E2E によって sessions に触れないことを test で観察)。
- Host pubkey TOFU UI (palette `Inspect host` で fingerprint 表示)。

### Phase 5 — WebRTC P2P

- `client/webrtc/` 実装 (pion/webrtc)。
- Browser 側 PeerConnection + DataChannel。signaling は既存 `SignalingFrame` を使用。
- 経路自動選択 (LAN > P2P > opaque relay)。
- Phase 4 の Noise セッションを transport 切替時に bind し直し (sessions は維持)。

### Phase 6 — 観測 / 運用

- Prometheus metrics、Gateway admin UI、bandwidth quota / fair-share、host pubkey rotation。

## 16. Open Questions

| # | 問い | 影響 | 候補 |
|---|---|---|---|
| Q1 | E2E に Noise XK / IK / XX のどれを base にするか | handshake RTT、pubkey 配布タイミング | XK (pubkey 事前共有前提)、初回のみ XX。Decision → ADR |
| Q2 | Gateway 永続化に sqlite / bbolt のどちらを採るか | CGO 制約、運用バックアップ | bbolt (純 Go) を一次案、複雑クエリ要なら modernc.org/sqlite |
| Q3 | Browser からの mDNS 発見手段 | LAN 直結の UX 成立可否 | (a) host 側に静的 IP 入力、(b) gateway が登録時に LAN IP も保存 → browser はそれを試す、(c) PWA / extension 経由。**(b) が現実解**。要検証 |
| Q4 | Client → Gateway authn を passkey / OAuth どちらに寄せるか | 運用、devices 同期 | MVP: passkey。OIDC は組織導入時のオプション |
| Q5 | WebRTC mux 粒度: session ごと DataChannel vs 1 DataChannel + 上位 yamux | 接続コスト、HoL、実装複雑度 | session/DataChannel を一次案 (cheap、simple)。session 数 20+ 想定で yamux への移行を ADR |
| Q6 | host が落ちた session の保持期間 | リソース、UX | host 側 24h pty buffer 維持 → eviction、gateway 側は metadata のみ永続 |
| Q7 | gateway HA を将来どう入れるか | 設計余地 | session token を JWT 化して stateless 化、registry を外部 KV |
| Q8 | sandbox release の暗黙 trigger (browser 閉じ) を opt-in にする粒度 | 誤切断による作業消失 vs リソース | default off、user setting で `auto-release after N min disconnected` |
| Q9 | mTLS cert のローテーション | 運用負荷 | host cert を Ed25519 self-signed + 鍵自体は不変、cert の有効期限を長く取り fingerprint 検証主体 |
| Q10 | Host section の default 展開ポリシー (多 host 時) | スクロール量、識別容易性 | host < 5 は全展開、5+ は default 折り畳み (last-connected のみ展開) を一次案。閾値・既定の override は user setting |
| Q11 | LAN 直結時の WS port を host ごと固定にするか動的にするか | port 衝突、firewall | 既定 8081、占有時自動加算。mDNS TXT で広告 |
| Q12 | section 内 session の sort key | 表示順の自然さ | 一次案: `created_at desc` (host 内のみ)。secondary は project。host 横断 sort は不要 (UI に merged list 概念がないため) |
| Q13 | Browser が起動時に全 host 並列接続を試みるか lazy か | 起動コスト vs section visible 時の事前接続 | eager (online host 全てに並列接続) を一次案。host > N (例 10) なら recent N に絞り、残りは section 展開で lazy 接続。接続できない host は `Not reachable` 表示のまま放置 (完全性を装わない) |
| Q14 | Host 側 client allowlist の merge 方針 (gateway push の user 署名検証粒度) | gateway 侵害時の防御 | user identity key (passkey 派生 or 独立) で署名された add/remove op のみ host が accept。gateway 単独では新 client を追加できない |
| Q15 | Opaque relay-fallback で gateway が見る最小単位 | 監査・課金粒度 | host channel 単位 (= browser↔host 1 接続)。session 件数は不可視。byte counter は per host channel |
| Q16 | Phase 2 で WebRTC を skip して relay-only で出すリスク | wire shape との整合 | wire shape は変えない (channel 分離維持)。relay 配線のみで MVP 検証可能。実装は §15 phase 順 |
| Q17 | ~~Host capability snapshot を gateway directory に乗せるか~~ | — | **Closed (Decision: 案 A 確定)**: gateway no-domain 原則 (§0) により capability は gateway に乗せない。palette は host channel 確立後に reduce する。応答性問題が破綻したら別途 plan を起こす (gateway に焼くのは選択肢から除外) |
| Q18 | Host eviction 後の data plane 強制切断手段 (§11.4) | 即時失効の保証 | 一次案: user 署名済 revoke op を gateway が opaque forward + LAN 直結 admin 経路 + 物理 panic kill の冗長配送。gateway 単独では LAN/P2P を切れない設計を許容 |
| Q19 | Browser-local nickname の複数 device 同期 | UX 一貫性 | MVP 未対応 (device 単位)。将来は P2P-first な同期 (user identity key + Noise) を別 plan で。gateway 経由同期は **gateway no-domain 違反のため不採用** |

## 17. リスクと緩和

| リスク | 影響 | 緩和 |
|---|---|---|
| Noise + gRPC + WebRTC を MVP に同梱 → 工期肥大 | リリース遅延 | Phase 分割 (§15)。MVP は E2E なし mTLS のみで動かす選択肢を残す |
| Browser mDNS 制約で LAN 直結が破綻 | LAN 利用者の体験劣化 | gateway 登録時に LAN IP を保存する案 (Q3) を Phase 4 までに検証 |
| gateway 単一障害点 | 全体停止 | LAN 直結が常に動く設計 (gateway 不要モード)。HA は Phase 7 以降 |
| pairing token フィッシング | host が偽 gateway に登録される | host pair CLI で gateway URL を明示確認 + admin が host 登録時に承認 |
| WebRTC NAT 越え失敗時の opaque relay 帯域コスト | 運用費 | gateway 同居の opaque relay で開始、必要なら独立 TURN-server 化を後追い |
| 既存 sandbox release invariant の破損 | コンテナリーク | host channel 切断は teardown を意味しない原則を §11.3 に明文化、e2e test で防衛 |
| Gateway no-domain 原則の意図せぬ違反 (将来の機能追加で gateway に capability や display name を載せたくなる誘惑) | gateway 侵害時の info leak 面積が拡大、ドメイン進化と gateway 改修が結合 | Phase 2 で **regression guard test** を入れる (gateway audit log / registry / control proto に session_id / display_name / capability が現れないことを test で固定)。**新機能追加時のレビュー観点として §0 / §4.1.1 / [[feedback-gateway-no-domain]] を必ず参照** |
| Browser-local nickname 喪失 | device 紛失時に全 host 名が `9f3a...` 表示になる | nickname は localStorage backup script で個別保存可能。複数 device 同期は MVP 未対応 (Q19) |
| First-time TOFU 時の nickname 入力疲労 (host が多いとき) | 初期体験劣化 | pubkey fp prefix を default nickname 候補として提示。空 nickname は許容して fp prefix を render する |

## 18. このプランで決めたこと (要約)

1. **データ構造は P2P-first**。wire shape は browser ↔ host が直接通信する前提で組む。Gateway channel と Host channel は別フレーム種、ID 空間は host-local + composite global、**session は host ごと独立、"merged" 型を作らない**。
2. **Host は新 1 級ドメインオブジェクト**。session/frame に `host_id` を導入し、wire を含めて全体に伝播させる。**Web client は host 切替 mode を持たず、接続できた host の session を per-host section で表示するだけ** — host は session に付随する属性であって app mode でもなく、全 host の完全性も装わない (接続不能 host は `Not reachable` placeholder)。
3. **Gateway は relay / tunnel / authorizer の 3 責務のみ、ドメイン情報を持たない / 解釈しない** ([[feedback-gateway-no-domain]])。session / frame / driver / agent / capability / ACL 中身 / host display name は **gateway に存在しない**。Gateway が持つのは pubkey fingerprint / signaling hints / online 状態 / relay byte counter / user identity credential のみ。
4. **Data plane は browser ↔ host 直結**: WebRTC P2P (一次) / LAN 直結 / opaque relay fallback。Noise は transport の上に被せ、経路切替で session を切らない。
5. **ACL の最終権威は host**。gateway が侵害されても未知 client は host に入れない。gateway は user 署名済 op を opaque blob として forward するのみ、内容を parse しない。host が user 署名を verify して accept。
6. **Host-initiated outbound** で host を公開網に晒さない (control plane も data plane も)。
7. **LAN 直結は MVP からのスコープ**。gateway 不在でも動く設計を維持。
8. **Display name / nickname は browser localStorage 完結**。gateway / host / mDNS TXT のどこにも nickname を流さない。
9. **段階導入**: Phase 1 host 概念 → Phase 2 gateway MVP (wire shape 確定 + relay-only data plane + nickname store) → Phase 3 LAN → Phase 4 E2E Noise → Phase 5 WebRTC P2P → Phase 6 観測。**wire shape は Phase 2 で確定し、以降の transport 入替で wire は変えない**。

## 関連メモリ

- [[feedback-gateway-no-domain]] — gateway は relay/tunnel/authorizer 3 責務、ドメイン情報を持たない (本 plan の中核原則)
- [[web-active-session-ownership]] — host scoping 後も client 単独管理を維持
- [[web-gateway-isolation]] — 新 gateway とは別概念だが、enqueueInternal の wedge と同種の routing 罠を分散構成でも避ける
- [[host-direct-env-inherit]] — host 内 env overlay 不変条件は分散構成でも host-local に閉じる
- [[feedback-no-arc-as-service-unit]] — gateway を独立 daemon 化する設計判断と矛盾しないか確認 (gateway は "公開網に出る役割" として独立する理由があるので OK、単に process 分けではない)
