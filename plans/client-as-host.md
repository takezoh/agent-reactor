# Plan — Client device を host として扱う (hostexec on local device)

- **作成日**: 2026-06-30
- **ブランチ**: `main`
- **ステータス**: draft (依存: [multi-host-gateway.md](./multi-host-gateway.md) Phase 3 LAN 直結 が確立していること)
- **影響範囲**: docs (セットアップ手順) / web UI (nickname default + "This device" 表示) / `client/lanlisten/` に loopback-only bind mode + 同一マシン判定 / `cmd/server/` のフラグ追加。**新規 binary なし / native client なし / 大規模 transport 追加なし**。
- **関連 ADR (将来起こす)**: (a) local host bootstrapping (auto-launch / manual / systemd-user) / (b) loopback-only listen + same-machine auth (uid check / unix socket option) / (c) "This device" UX 規約 (section header / nickname default / 1 host 時の visual noise 抑制との両立)

## 0. 用語

- **Local host**: client device 自身で動く `server` プロセス (browser と同じ PC)。
- **Remote host**: 既存。別 PC 上の `server` プロセス。
- **統一原則**: 両者は [multi-host-gateway.md](./multi-host-gateway.md) の `Host` 概念で**同一**に扱う。wire / state / session model に区別を作らず、UX 表示 (section header の "This device" バッジ等) だけが差分。

## 1. 目的とスコープ

### 1.1 目的

1. **client device 上で pty を回せるようにする** (gateway / remote host 不在のスタンドアロン用途、開発機 1 台での完結利用、出先のオフライン作業)。
2. multi-host plan の延長で実現する: **既存 `server` binary を localhost に向けて起動するだけ**。新 binary も新 transport も新 wire frame も導入しない。
3. **native client 化 (PWA / Tauri) を回避**。browser は browser のまま。
4. browser-only 端末 (mobile / tablet / 共用 kiosk) では「local hostexec 不可」という制約を**明示**し、UX で正直に表現する (接続不能 host を `Not reachable` placeholder で出す既存規約 §10.2 をそのまま流用)。

### 1.2 非目的

- **Browser だけで pty を spawn**: 原理的に不可能 (browser sandbox は child process / pty を扱えない)。WebContainers / WASI も native binary (codex / claude-code / shell) を回せないため解にならない。
- **PWA / Tauri / Electron 化**: [multi-host-gateway.md §1.2](./multi-host-gateway.md) の非目的をそのまま継承。
- **mobile / tablet 等で `server` を起動できない device 向けの local hostexec**: これは technical constraint であり本 plan の解決対象ではない (remote host or gateway 経由 host を使う運用に倒す)。
- **`server` binary の自動起動 / installer**: MVP は手動起動 + docs。将来の systemd-user / launchd / Windows サービス 化は別 plan。

### 1.3 既存制約 (退行禁止)

- [multi-host-gateway.md](./multi-host-gateway.md) の **host 概念 / wire shape / nickname store / pairing / identity / TOFU** をすべて再利用。
- `depguard` / 500 行 / 80 行 / wire stdlib のみ。
- [[host-direct-env-inherit]]: local host も同じ host 不変条件に従う。
- [[feedback-gateway-no-domain]]: local host を gateway 経由で扱う必要はない (LAN direct で済む)、gateway directory に local host を載せるのも任意 (mDNS で発見 / pubkey TOFU で完結する)。

## 2. アーキテクチャ

### 2.1 全体像

```
Client device (1 PC)
┌──────────────────────────────────────────────────────────┐
│                                                            │
│   Browser SPA  ──── LAN direct (loopback) ──────►  server  │
│   (web client)        host channel (Noise)          (local │
│                                                       host)│
│                                                            │
└──────────────────────────────────────────────────────────┘
                              │
                              │ (任意) gateway は不要 — gateway 経由でも動くが
                              │ standalone 用途では gateway を起動する必要すらない
                              ▼
                        (Internet / gateway は optional)
```

- Browser は `127.0.0.1` (or `[::1]`) の `server` を **LAN direct 経路**で叩く。Gateway directory に登録しなくても discover できる (§3.1)。
- Gateway を併用する場合 (remote host も使いたい構成) は local host を 1 つの host directory entry として登録するだけ。Web UI は local も remote も同じ section list で並べる。
- Local host の `server` も既存の identity / pairing / Noise を**そのまま使う** (鍵を生成、TOFU、Noise XK)。「同一マシンなんだから検証なし」とはしない (中間 process の悪意 / 別 user / 別 browser profile からのアクセスを区別する必要があるため)。

### 2.2 不変条件

1. **Local host も "1 つの Host"**。wire / state / session model に local-specific な field を作らない。区別が必要な場面は **UI のみ** (section header の "This device" 表示)。
2. **Loopback-only bind**: local host は default で `127.0.0.1:<port>` にのみ listen。同一 LAN の他マシンから到達不可能 (それは remote host の用途であって local host の用途ではない)。LAN direct も併用したい case は `--listen 0.0.0.0:<port>` を user が明示的に opt-in する。
3. **同一マシン判定は信用しない**: 同一マシンの別 user / 別 browser profile / 別 process からのアクセスも 1 つの "client" として扱い、pairing と TOFU を要求する。「localhost だから無条件信頼」は採らない。
4. **Browser のみの端末では存在しない**: `server` が起動していない device では local host は単に discover されない (host directory にも mDNS にも出ない)。UI 上で "local host is missing" のような誘導は出さない (制約の正直な表現)。
5. **Gateway 不要モードを維持**: local host だけで完結する standalone 用途では、gateway を起動する必要も登録する必要もない。[multi-host-gateway.md §1.1 目的 (4)](./multi-host-gateway.md) の「LAN 限定 / gateway 不在で動かせる」をこの構成で具体化する。

## 3. 実装ポイント

### 3.1 Discovery: browser から local host を見つける

選択肢:

| 方式 | 利点 | 欠点 | 採否 |
|---|---|---|---|
| **(a) `localhost` 固定エントリ** (browser が常に `127.0.0.1:<default-port>` を試す) | 0-configuration、setup 手順最小 | port 衝突時に動かない、複数 local host instance を扱えない | **採用** (MVP の primary path) |
| (b) mDNS (`_agent-reactor._tcp.local.`) | 複数 instance / 任意 port に対応、remote LAN host と同じ機構 | browser から mDNS を直接叩けない (Q3 と同じ制約) | 補助 (gateway 登録 + LAN endpoint 経由) |
| (c) Gateway directory 登録 | gateway 併用構成ではそのまま機能 | gateway を起動しないと使えない (standalone 用途で破綻) | gateway 併用時のみ |

MVP は **(a) + (c)**: browser が起動時に `127.0.0.1` の default port (例 8081) を tap し、応答があれば local host として `connections` に追加。Gateway を併用している場合は (c) が並行に効く (gateway directory に local host pubkey fp が出る、両経路で同じ host pubkey fp に到達するため重複登録しない)。

### 3.2 Loopback-only listen + same-machine 認証

```
server --listen 127.0.0.1:8081 --bind loopback    # MVP default
server --listen unix:/tmp/agent-reactor.sock --bind unix-socket-uid <uid>   # 強化案 (Q3 で検討)
server --listen 0.0.0.0:8081 --bind lan           # 既存 remote host 用 (LAN direct)
```

- `--bind loopback`: TCP socket を loopback interface にのみ bind。他 LAN マシンからの connect を OS レベルで遮断。
- `--bind unix-socket-uid`: unix domain socket + peercred で connect 元 uid を check。同一 user のみ accept。最も強い isolation だが browser → unix socket の橋渡しを別途必要とする (WebSocket は unix socket を直接叩けないため、localhost http proxy + Authorization header か、本格的には別 channel が必要)。MVP は採用せず ADR で評価。

### 3.3 Pairing と TOFU

- 初回 browser 接続時、local host pubkey を browser に提示 → TOFU 承認 + nickname 入力 (default `This device`)。
- pairing token は不要 (gateway 経由でないため): browser は LAN direct で host pubkey を直接受け取り、user が `Trust this device` を palette で押下することで pin。
- pubkey rotation 後は別 host 扱い (既存 TOFU 規約と同一)。

### 3.4 UX 差分 ([multi-host-gateway.md §10](./multi-host-gateway.md) との整合)

- **Nickname default**: local host (= browser と同じ origin) は初回 TOFU 時の nickname 入力欄を `This device` で pre-fill (空欄 enter で確定)。User は変更可能。
- **Section header**: local host section は default で展開 (多 host 時の折り畳みポリシー Q10 でも常に展開対象)。順序は last-connected を maintain しつつ、tie-break で local host を上位に。
- **Header badge**: 1 host 環境では section header / nickname badge を非表示にする既存規約 (§10.2) と整合: **local host のみの構成では section header も header badge も省略**。"This device" であることが明白なので二重表示しない。
- **`Inspect host`**: local host の場合は `via loopback` 表示 (P2P / LAN / relay の reachability badge に並ぶ追加 variant)。
- **Browser-only 端末での扱い**: local host が discover できない端末では、host directory に local host が出ない / palette `New Session` の HostSelectPhase で local host が listing されない。"local host を起動してください" のような誘導は MVP では出さない (将来の installer plan で検討)。

### 3.5 セットアップ手順 (docs)

```sh
# 1. server を local で起動 (loopback-only)
server --listen 127.0.0.1:8081 --bind loopback

# 2. browser で UI を開く (gateway 不在モード)
#    → 127.0.0.1:8081 へ LAN direct 接続が tap される
#    → 初回は TOFU prompt + nickname 入力 (default "This device")

# 3. 以後は通常通り New Session 等を行う
```

- gateway 併用構成では `server --gateway https://gw.example.com ...` を追加。
- launchd / systemd-user 用の unit file は **本 plan では提供しない** (将来別 plan)。

## 4. セキュリティ

### 4.1 脅威モデル ([multi-host-gateway.md §6.1](./multi-host-gateway.md) を継承)

| 脅威 | 対策 |
|---|---|
| 同一マシンの別 user による access | loopback bind + pairing/TOFU (default)。さらに強化したい場合は unix socket + uid check (Q3、ADR で検討) |
| 同一 user の別 browser profile / 別 app による access | pairing token (LAN direct でも `Trust this device` gesture を要求)、TOFU pubkey pin |
| Localhost 偽装 (別 process が同 port を hijack) | pubkey TOFU で防止。再起動後に pubkey が変わっていたら拒否 (multi-host-gateway.md §6.3.3 と同じ規約) |
| Browser device 紛失 | local host 自体は遠隔から到達不可能 (loopback bind)。物理アクセスがある攻撃者は OS のフルアクセスを持つので host 単位での防御は意味が薄い (脅威モデル外) |
| 同一マシンの他 app が同 port を専有 | port 衝突: `--listen` で別 port 指定 → docs に明示 |
| 別マシンから接続される | loopback bind により OS レベルで拒否。`--bind lan` を明示しない限り起こらない |

### 4.2 既存 host とのセキュリティ等価性

- Local host も **既存 host と完全に同じ Noise / TOFU / pairing 経路**を通る。"localhost だから検証 skip" は採らない (§2.2 不変条件 3)。
- これにより、local host を gateway 経由で remote から触る構成 ("自宅 PC を local host として起動、外出先 browser から gateway 経由で繋ぐ") も追加実装なしで成立する (= remote host と区別がないため)。

## 5. 段階導入

### Phase 0 — 設計合意 (本 plan)

- ADR: local host bootstrapping (auto vs manual)、loopback-only bind の規約、`This device` UX 規約。

### Phase 1 — Loopback listen mode + docs

- `cmd/server` に `--bind loopback|lan|unix-socket-uid` フラグ追加。default は `loopback` ではなく **既存挙動を変えない** (既存 install への影響を避ける)、新規 user 向け quick-start docs で `--bind loopback` を案内。
- Browser 側で `127.0.0.1:<default-port>` tap を起動時に試みる (応答なければ無視)。
- セットアップ docs を追加。

### Phase 2 — `This device` UX

- Nickname default、section header の "This device" 表示、1 host 時の visual noise 抑制との整合、`via loopback` reachability badge。
- 多 host 環境での local host section 順序 (last-connected ベース + tie-break で上位)。

### Phase 3 — Hardening (任意)

- Unix socket + peercred 認証 mode (`--bind unix-socket-uid`)。Browser からの接続経路を ADR で決定 (localhost http proxy 経由など)。
- Browser-only 端末での "local host が存在しない" 状態をどう UX で伝えるか (現状: 何も表示しない、これで十分なら追加実装なし)。

### Phase 4 — Auto-launch (将来、別 plan)

- systemd-user / launchd / Windows サービス unit。Installer / `agent-reactor up` のような CLI bootstrap。本 plan の外。

## 6. Open Questions

| # | 問い | 影響 | 候補 |
|---|---|---|---|
| L1 | Local host の default discovery port | port 衝突 / setup 摩擦 | 8081 を default、衝突時は browser が 8082, 8083 を順次 tap。docs で明示 |
| L2 | Unix socket + uid check を MVP で入れるか | 同一マシン他 user 防御の強度 | MVP は loopback + TOFU、unix socket は Phase 3 (browser → unix socket 橋渡しの実装コストが高いため) |
| L3 | Browser が local host を自動 tap するかどうか (default on/off) | 起動時のネットワーク noise / プライバシー | default on (loopback への tap はリスクが低い)、user setting で off 可能 |
| L4 | 多 host 環境での local host section 表示優先度 | UI 視認性 | last-connected を maintain しつつ、tie-break で local host を最上位 |
| L5 | Local host を gateway directory にも登録するか (gateway 併用時) | UI 重複 / discovery 経路の冗長性 | 登録する (gateway 経由でも到達可能にしておく)、ただし browser は同 pubkey fp で deduplication |
| L6 | Browser-only 端末で "local host が無い" ことを UI で示唆するか | UX vs noise | MVP は示唆しない (host directory に出ないだけ、誘導は将来の installer plan で) |
| L7 | Auto-launch (systemd-user 等) を本 plan に含めるか | scope 肥大 | 別 plan に切り出す (本 plan は manual launch + docs のみ)。Quick-start docs では `nohup` / `screen` を案内 |

## 7. リスクと緩和

| リスク | 影響 | 緩和 |
|---|---|---|
| Port 衝突で起動できない | setup 摩擦 | `--listen` で代替 port 指定、docs に明示。Browser は 8081 → 8082 → 8083 の順で tap |
| Loopback bind を user が忘れて `0.0.0.0` で起動 | 同一 LAN マシンへの意図せぬ exposure | docs で `--bind loopback` を default 推奨、`server --help` の例も loopback を first |
| Pairing/TOFU を skip させる誘惑 (「同じマシンなのに面倒」) | 同一マシンの別 user / app による access | §2.2 不変条件 3 を ADR で明文化、code review で skip 提案を reject |
| Browser-only 端末での混乱 ("自分の PC で動かない") | UX | docs に「local hostexec には `server` 起動が必要」を明記、tablet/mobile では remote host 運用を推奨 |
| `server` を多重起動する user | port 衝突 + identity 重複 | server 側で pidfile + lock で多重起動を検知して exit |

## 8. このプランで決めたこと (要約)

1. **Client 機での hostexec は既存 `server` binary をそのまま使う**: localhost に 1 つ起動するだけで、browser は LAN direct 経由で接続。
2. **Native client / PWA / Tauri は導入しない**。browser は browser のまま。multi-host-gateway.md §1.2 の非目的を維持。
3. **Local host も "1 つの Host"**: wire / state / session model に区別を作らず、UX 上の "This device" 表示だけ差分。
4. **Loopback-only bind を default で推奨**。同一マシンの別 user / app からも pairing/TOFU を要求 (localhost 無条件信頼は採らない)。
5. **Browser-only 端末では local hostexec は不可能**: technical constraint を正直に表現 (host directory に出ない、誘導もしない)。
6. **Gateway 不在モードを具体化**: multi-host-gateway.md の "LAN 限定でも動く" 目的をこの構成が体現する。

## 関連メモリ

- [[feedback-gateway-no-domain]] — local host も gateway を経由する必要がない (LAN direct loopback で済む) ことで原則が自然に維持される
- [[host-direct-env-inherit]] — local host も同じ env overlay 不変条件に従う (実装差分なし)
- [[web-active-session-ownership]] — local host の session も client 単独管理を維持
- [[feedback-no-arc-as-service-unit]] — `server` を local で常駐させる場面が増えるが、これは「公開網に出る役割を独立 process にする」原則に反しない (local host は閉じた network しか触らない)

## 関連 plan

- [multi-host-gateway.md](./multi-host-gateway.md) — 本 plan の依存元 (Host 概念 / host channel / LAN direct / identity / nickname store / TOFU をすべて再利用)
- [client-credential-injection.md](./client-credential-injection.md) — local host への credential 配布も remote と同じ wire frame で扱える ("This device" への push は wire 上 remote host と同形)
