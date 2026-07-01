# Plan — systemd 起動時の 502 レース解消 (sd_notify readiness + proxy 503 map + UI short retry)

- **作成日**: 2026-07-01
- **ブランチ**: `main`
- **ステータス**: draft (未着手 / 実装前)
- **影響範囲**:
  - `src/go.mod` (`github.com/coreos/go-systemd/v22` 追加)
  - `src/platform/lib/systemdnotify/` (新設)
  - `src/cmd/server/gateway.go` (readiness notify 呼び出し)
  - `src/.golangci.yml` (`depguard` の allow list 追加)
  - `src/client/web/host.go` (`ReverseProxy.ErrorHandler` 追加)
  - `src/client/web/src/api/sessions.ts` (`retryOn5xx` helper + `getSessionConfig` 差し替え)
  - `deploy/systemd/agent-reactor-server.service` (`Type=simple` → `Type=notify`)
  - `docs/user/systemd.md` (Type=notify 化の追記)
- **関連 ADR (本 plan の実装で起票)**:
  1. **sd_notify readiness 契約**: `READY=1` を送るタイミング (`net.Listen` 成功直後) の正当性、`NOTIFY_SOCKET` 未設定時の silent no-op fallback、`WATCHDOG` / `RELOADING` は将来スコープとする境界
  2. **reverse proxy transient error mapping**: `httputil.ReverseProxy` の dial 失敗のうち `ECONNREFUSED` / `EHOSTUNREACH` / `*net.DNSError` を **503 + `Retry-After: 1`** にマップし、TLS handshake 失敗 / mid-response 切断は **既存の 502** のまま残すルール
  3. **frontend transient retry policy**: `getSessionConfig` にだけ 5xx retry (最大 3 attempts / 200→400→800ms backoff / total cap 1.4s / `Retry-After` header 尊重) を適用し、他 endpoint への展開は follow-up plan とする境界

## 0. 用語

- **server**: `cmd/server` binary。pty daemon + HTTP/WS gateway を 1 プロセスで持つ backend。systemd 上の unit は `agent-reactor-server.service`。
- **web**: `cmd/web` binary。ブラウザ UI をホストし `/api` `/ws` を server に reverse-proxy する。systemd 上の unit は `agent-reactor-web.service`。
- **readiness**: プロセスが起動しただけでなく外部リクエストを受け付けられる状態。この plan では **server の HTTP listener が bind に成功した瞬間** を readiness と定義する。coordinator boot と DaemonClient dial は listener bind 済の後 in-process で処理可能なので、bind 成功が実質的な "外部受付可能" 状態。
- **race window**: web の `ExecStart` プロセスが起動してから server の listener bind までの時間差。この間の web への request はすべて 502 になる。

## 1. 目的と非目的

### 1.1 目的

1. systemd で `agent-reactor-web.service` を起動した直後にブラウザを開いたとき、`/api/session-config` が **502 で失敗して New Session パレットが空になる状態を排除** する。
2. systemd の依存順序 (`Requires=` / `BindsTo=` / `After=`) が **process fork 順ではなく listener bind 順で cascade する** ようにする。
3. sd_notify が使えない環境 (`NOTIFY_SOCKET` 未設定 = 対話起動 / macOS / Windows) では **既存挙動を壊さず silent no-op** で通す。
4. systemd 契約が効かない環境 (macOS / 対話起動 / オペレータが unit ファイルを override して `Type=simple` に戻したケース) でも、**web の reverse proxy + UI の 2 段で吸収** し、user が reload しなくても New Session パレットが埋まる。

### 1.2 非目的

- server の全 bootstrap 段階を細分化して readiness を段階通知する (`RELOADING=1` / `STOPPING=1` / `STATUS=...`)。今回は `READY=1` の 1 発だけ。
- systemd `Type=notify-reload` / watchdog (`WATCHDOG=1`) の導入。将来別 plan。
- Windows / macOS 用の同等 readiness signal (launchd `KeepAlive` の "successful launch" 等)。プラットフォーム別対応は別 plan。
- reverse proxy の全面書き換え (retry / circuit breaker)。今回は「dial 失敗を 503 に切り分ける」だけ。
- `/api/session-config` 以外の bootstrap fetch retry (`/api/sessions`, `/api/health` 相当) を一斉に見直す。helper は汎用化しつつ、今回の差し替えは `getSessionConfig` のみに絞る。
- orchestrator / claude-app-server の systemd 化。両者は現状 unit を持たないので対象外。

### 1.3 既存制約 (退行禁止)

- `depguard` 境界: `platform/*` は `client/*` / `orchestrator/*` に依存しない (ARCHITECTURE.md)。今回追加する `platform/lib/systemdnotify/` は `cmd/server` からのみ import する形にする。**`coreos/go-systemd/v22/daemon` の allow list を `platform/lib/systemdnotify` パッケージ内に限定** し、他パッケージからの直接 import は禁止する (差し替え可能性を確保)。
- ファイル 500 行 / 関数 80 行 (reducer 例外)。
- Wire 型 / persistence 型は stdlib のみ (ADR-0021)。sd_notify は wire 層ではないので対象外。
- `~/.local/state/agent-reactor/server.log` の startup ログは journald と cat-friendly の 2 系統に流れているため (`gateway.go:logStartup`)、readiness 送信ログを別行で足しても既存の log grep alert を壊さないこと (log tag は `"gateway: readiness notified"` とする)。
- 対話起動 (`./server -addr 127.0.0.1:8443`) が壊れないこと (`NOTIFY_SOCKET` 未設定時は no-op で return nil)。

## 2. 検証済みの根拠 (2026-07-01)

因果は 7 点で確定している (本 plan 作成前の debug session で実証済み)。

1. **Go の `httputil.ReverseProxy` は backend dial 失敗時に確定で 502**  
   `src/client/web/host.go:55-63` と同一 shape の proxy を dead backend に対して走らせて再現:
   ```
   http: proxy error: dial tcp 127.0.0.1:1: connect: connection refused
   status=502  StatusText: Bad Gateway
   ```
   これは `net/http/httputil/reverseproxy.go` の `defaultErrorHandler` の実装。

2. **`web` 側に `ErrorHandler` 上書きなし**: `grep -rn "ErrorHandler" src/client/web/*.go` は 0 hit。

3. **server は sd_notify を一切呼んでいない**: `grep -rn "sd_notify\|SdNotify\|coreos/go-systemd" src/` は 0 hit。依存にすら入っていない。

4. **両 unit が `Type=simple`**:
   ```
   deploy/systemd/agent-reactor-server.service:16:Type=simple
   deploy/systemd/agent-reactor-web.service:21:Type=simple
   ```
   systemd manual: "Type=simple: systemd will consider the unit started immediately after the main service binary has been forked off." **listener bind を待たない**。

5. **listener bind はプロセス fork から遅れる**  
   `cmd/server/main.go:139 → runCommand → runDaemonFn` → coordinator 初期化 (`coordinator.go`) → `startGateway` (`gateway.go:64`) → `net.Listen("tcp", df.addr)` (`gateway.go:80`) の順。config load・logger init・coordinator boot が挟まる。**この間の web からの dial は connection refused**。

6. **web unit の comment 自身が 502 loop を既知としてマーク**  
   `agent-reactor-web.service:16-17`:
   ```
   # BindsTo cascades server stops into us — without it the proxy would
   # 502-loop until systemd restarted server.
   ```
   これは "server 死亡時の cascade" を狙ったコメントで、**起動時 race は未対策**。

7. **502 は UI 側で retry されない**  
   `App.tsx:70-92` の `getSessionConfig().catch(...)` は 401 だけ silent、502 は toast + `sessionConfig=null`。`api/sessions.ts` にも 5xx retry 実装なし → **New Session パレットの projects / commands が空のままユーザに露出する**。

因果チェーン: **systemd `Type=simple` → web が server の listener bind 前に起動 → `ReverseProxy` dial 失敗 → default handler が 502 → `App.tsx` の session-config 取得失敗 → palette が空**。

## 3. 修正方針 (確定)

因果チェーンの 3 段 (systemd / proxy / UI) それぞれに対策を入れる。**A + C + D の 3 段で確定**。

### 3.1 A: systemd に readiness を通知する

**変更点**:
- `agent-reactor-server.service` を `Type=simple` → `Type=notify` に変更。`NotifyAccess=main` を明示。
- `platform/lib/systemdnotify/` を新設し `Ready() error` を 1 個 export。中身は `coreos/go-systemd/v22/daemon.SdNotify(false, daemon.SdNotifyReady)` を薄く wrap しただけ。`NOTIFY_SOCKET` が未設定なら silent nil を返す (`SdNotify` はこのケースで `sent=false, err=nil` を返すので、そのまま透過)。
- `cmd/server/gateway.go` の `startGateway` 内、`net.Listen("tcp", df.addr)` 成功直後 (現在 `logStartup` を呼んでいる位置と同じ) に `systemdnotify.Ready()` を呼ぶ。**失敗しても起動継続** (log Warn だけ)。

**なぜ効くか**: `Type=notify` unit は `READY=1` が届くまで systemd が unit を "activating" 扱いにする。`agent-reactor-web.service` は `Requires=/After=/BindsTo=agent-reactor-server.service` を持つので、readiness (=listener bind 済) が来るまで web の起動を保留する。web が動き出す瞬間には server は必ず listen 済 → connection refused が発生しない。

**効かない場面** (次段 C/D で受ける):
- 対話起動 / macOS / Windows: `NOTIFY_SOCKET` 未設定なので no-op
- オペレータが `Type=simple` に override / 旧 unit を使い続けている環境
- `agent-reactor-server` を単独 restart した瞬間に別 process が web を叩くタイミング (通常は BindsTo で cascade するのでレア)

### 3.2 A のライブラリ選定: `coreos/go-systemd/v22`

CLAUDE.md の Library Selection rule に従い候補比較:

| # | 候補 | trade-offs |
|---|---|---|
| L1 | **`github.com/coreos/go-systemd/v22/daemon`** (採用) | de-facto 標準 (systemd 本体推奨)。CGO 不要、~200KB、Apache-2.0。`SdNotify` / `SdNotifyReady` / `WatchdogEnabled` / `SdNotifyWithFDs` / socket activation helper が同 module に揃っており、将来 WATCHDOG / RELOADING / FDSTORE を扱う際に差し替え PR が不要。呼び出しは 1 行。 |
| L2 | `github.com/okzk/sdnotify` | Minimal wrapper。最終 commit 2020 で maintain 停滞。WATCHDOG helper なし。 |
| L3 | 独自実装 (`net.Dial("unixgram", os.Getenv("NOTIFY_SOCKET"))` + `Write("READY=1\n")`) | 依存 0。protocol は sd_notify(3) manpage で凍結済み。ただし WATCHDOG / RELOADING の追加時に自前実装を拡張することになる。abstract namespace (`@` プリフィクス) の handling も自前。 |

**採用: L1 `coreos/go-systemd/v22/daemon`**。理由:
1. `README.md` の `-license` 集計・依存棚卸しで既に安全なライセンス (Apache-2.0) であること
2. WATCHDOG / socket activation を将来別 plan で導入する見通しがあるため、その時に差し替え PR を一度で済ませたい
3. abstract namespace や error handling を自前で維持するコストを避ける

### 3.3 C: reverse proxy 側で "未起動" と "壊れた" を区別する

**変更点**:
- `src/client/web/host.go` の `backendProxy` に `ErrorHandler` を差し込む。
- **`syscall.ECONNREFUSED` / `syscall.EHOSTUNREACH` / `*net.DNSError`** で失敗した場合 → **`503 Service Unavailable`** + **`Retry-After: 1`** header を返し、response body は `"upstream not ready"` の 1 行。
- **それ以外** (TLS handshake 失敗 / read timeout / mid-response 切断) → **既存の 502 のまま**。TLS 設定ミスは "壊れている" 側でオペレータに気付かせる方向。
- 判別ロジック: `errors.Is(err, syscall.ECONNREFUSED)` / `errors.Is(err, syscall.EHOSTUNREACH)` / `errors.As(err, &dnsErr)` の 3 分岐。

**なぜ効くか**: 現状は "backend 未起動" と "backend 壊れた" のどちらも 502 に潰れており、UI は retry すべきか判別できない。dial-refused だけ 503 + Retry-After に射影すると、"upstream not ready" セマンティクスが UI 層に伝わる。次段 D と組んで初めて意味を持つ。

**log tag**: `"proxy: upstream not ready"` (503 側) と既存の default error log (502 側) を別行に分ける。

### 3.4 D: UI で bootstrap fetch を short retry する

**変更点**:
- `src/client/web/src/api/sessions.ts` に汎用 `retryOn5xx(fetchFn, opts)` helper を追加。
  - 最大 3 attempts / backoff 200ms → 400ms → 800ms (total cap ~1.4s)。
  - response の `Retry-After` header (秒指定 or HTTP-date) があれば **backoff より優先** して尊重。ただし cap は 1.4s。
  - 5xx でリトライ、4xx や成功はそのまま返す。
  - AbortSignal に対応し、caller (`App.tsx` の `cancelled` フラグ相当) 側で cancel 可能。
- 今回は `getSessionConfig` のみ helper 経由に差し替え (`/api/sessions`, `/api/health` などへの展開は follow-up plan)。
- 差し替え後の挙動: 1.4s 以内に成功すれば toast は出さず silent 完了 → user から見て palette は最初から埋まっている。retry しても最後まで 5xx なら既存挙動 (toast + `sessionConfig=null`)。

**なぜ効くか**: A が効かない環境で C の 503 が返っても、UI が短い間隔で retry すれば listener bind (通常 <200ms) に間に合う。C が入っていれば 503 だけ retry、502 (本物の upstream 障害) は retry せず即エラーを出せる。

**なぜ 1.4s cap か**: A が効けば race 窓は listener bind 時間 (通常 <200ms)。C の `Retry-After: 1` に沿うと 1s 前後で 1 回目 retry が成功する見込み。1.4s は "A も C も効かない worst case でも user がパレットを開くまでの体感時間内に収まる" 目安。

## 4. 実装ステップ

### 4.1 依存追加

`src/go.mod` に `github.com/coreos/go-systemd/v22` を追加。`go mod tidy` で `src/go.sum` を更新。

### 4.2 `platform/lib/systemdnotify` パッケージ新設

- **`src/platform/lib/systemdnotify/notify.go`**:
  ```go
  package systemdnotify

  import (
      "fmt"
      "github.com/coreos/go-systemd/v22/daemon"
  )

  // Ready sends READY=1 to systemd via NOTIFY_SOCKET, or returns nil if the
  // socket is not set (non-systemd environment: interactive launch, macOS,
  // Windows). Never fatal — callers should log Warn on error and continue.
  func Ready() error {
      sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
      if err != nil {
          return fmt.Errorf("sd_notify READY=1: %w", err)
      }
      _ = sent // sent=false when NOTIFY_SOCKET is empty (non-systemd) — not an error.
      return nil
  }
  ```
- **`src/platform/lib/systemdnotify/notify_test.go`**:
  - `NOTIFY_SOCKET` 未設定 → `Ready()` が nil (`t.Setenv("NOTIFY_SOCKET", "")`)
  - `NOTIFY_SOCKET` を temp `AF_UNIX unixgram` に向け、round-trip で `"READY=1"` を受信できること (`net.ListenPacket("unixgram", tmpSock)`)
  - `NOTIFY_SOCKET` が存在しない path → error を返すこと

### 4.3 `depguard` allow list 追加

- **`src/.golangci.yml`**:
  - `github.com/coreos/go-systemd/v22/daemon` の import 元を `platform/lib/systemdnotify` パッケージ (と test) のみに制限する rule を追加。
  - `platform/lib/systemdnotify` を `cmd/server` から import 可能に (既に `platform/*` は許可されているはず、diff で確認)。

### 4.4 `cmd/server/gateway.go` 修正

- `startGateway` の `net.Listen("tcp", df.addr)` 成功直後、`logStartup(...)` の呼び出しの直後に:
  ```go
  if err := systemdnotify.Ready(); err != nil {
      slog.Warn("gateway: readiness notify failed", "err", err)
  }
  ```
  を挿入。
- **test hook 化**: `var notifyReady = systemdnotify.Ready` を package var にして、test 側で差し替え可能に。
- **test**: `gateway_test.go` (存在確認要、なければ新設) に "notify path executed once, log Warn on failure" を追加。

### 4.5 `deploy/systemd/agent-reactor-server.service` 修正

```diff
 [Service]
-Type=simple
+Type=notify
+NotifyAccess=main
 StateDirectory=agent-reactor
```

`web` unit は変更なし (`After=/Requires=/BindsTo=` が Type=notify に対しては readiness まで gate する)。

### 4.6 `src/client/web/host.go` に `ErrorHandler` 追加

- `backendProxy` を変更:
  ```go
  return &httputil.ReverseProxy{
      Rewrite:      func(pr *httputil.ProxyRequest) { /* 既存のまま */ },
      ErrorHandler: proxyErrorHandler,
  }
  ```
- `proxyErrorHandler` (同ファイル内 helper):
  - `errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EHOSTUNREACH)` → 503 + `Retry-After: 1` + log Info `"proxy: upstream not ready"`
  - `errors.As(err, &dnsErr)` → 503 (同上)
  - それ以外 → 502 + log Error (既存 default 相当)
- **test**: `host_test.go` に:
  - dead backend (listen closed): 503 + Retry-After 検証
  - 200 backend: 200 スルー
  - TLS 不整合 backend: 502
  - DNS 解決不能 backend: 503

### 4.7 `src/client/web/src/api/sessions.ts` に `retryOn5xx` helper

- **helper 定義**:
  ```ts
  interface RetryOpts {
    maxAttempts?: number;   // default 3
    baseDelayMs?: number;   // default 200
    capMs?: number;         // default 1400
    signal?: AbortSignal;
  }
  async function retryOn5xx<T>(
    fn: () => Promise<T>,
    opts?: RetryOpts,
  ): Promise<T> { /* ... */ }
  ```
- `getSessionConfig` を helper 経由に差し替え。`Retry-After` header (数値秒 or HTTP-date) をパースして backoff に反映、ただし合計 cap 1.4s を超えない。
- **test** (`sessions.test.ts`): 503→200 で silent 成功 / 3 連続 5xx で throw / Retry-After 尊重 / signal abort で早期中断 / 4xx は retry しない、を各分岐で。

### 4.8 `docs/user/systemd.md` 追記

- `Install` セクションに `Type=notify` に変わった旨と、対話起動時は sd_notify が silent no-op になる旨。
- `LAN / external exposure` の drop-in サンプル (`ExecStart=` の上書き) は unit の `Type=notify` を継承するので影響なし、と明記。
- `Verify the cascade` セクションに、`Type=notify` により web の起動が listener bind まで待つようになった旨。

### 4.9 影響ファイル一覧

- 追加: `src/platform/lib/systemdnotify/notify.go`, `src/platform/lib/systemdnotify/notify_test.go`
- 修正: `src/go.mod`, `src/go.sum`, `src/.golangci.yml`, `src/cmd/server/gateway.go`, `src/cmd/server/gateway_test.go` (存在確認要), `src/client/web/host.go`, `src/client/web/host_test.go`, `src/client/web/src/api/sessions.ts`, `src/client/web/src/api/sessions.test.ts`, `deploy/systemd/agent-reactor-server.service`, `docs/user/systemd.md`

## 5. 検証手順

### 5.1 unit test

- `cd src && go test ./platform/lib/systemdnotify/...`
- `cd src && go test ./cmd/server/...`
- `cd src/client/web && npm test -- host`
- `cd src/client/web && npm test -- sessions`

### 5.2 lint / build

- `cd src && make lint` (depguard の rule 追加が正しく反映されているか)
- `make build-server build-web`

### 5.3 手動 e2e (systemd 上で race を再現 & 消滅を確認)

1. `make build-server build-web && make install-systemd`
2. **before の再現** (rollback 検証用):
   - `~/.config/systemd/user/agent-reactor-server.service.d/type-simple.conf` を drop-in で作り `[Service]\nType=\nType=simple\n` として `Type=notify` を打ち消す
   - `systemctl --user daemon-reload && systemctl --user restart agent-reactor-server agent-reactor-web`
   - restart 直後に `curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8080/api/session-config` を叩き、**503** (C が入っているため) or **502** (C を無効化した比較検証) が観測できること
3. **after の確認** (通常経路):
   - drop-in を削除して `daemon-reload` → `restart` → 直後に curl → **200**
   - `systemctl --user show agent-reactor-server -p Type` → `Type=notify` を確認
   - `journalctl --user -u agent-reactor-server -f` に `gateway: readiness notified` 相当の log が出ていること (実装 log tag に合わせる)
4. **D fallback の確認**:
   - `Type=simple` に戻し (A 無効化)、ブラウザで初回起動直後にリロードを 10 回連続で行い、Network タブで 1〜2 回 503 が出ても最終的に 200 → **New Session パレットが空にならない** ことを確認

### 5.4 regression check

- `journalctl --user -u agent-reactor-server -f` に `readiness notify failed` warning が定常出ていないこと
- 対話起動 (`./server -addr 127.0.0.1:8443`) が動作すること (`NOTIFY_SOCKET` 未設定の smoke)
- `systemctl --user stop agent-reactor-server` → `agent-reactor-web` が cascade で停止することを確認 (BindsTo)

## 6. リスク

- **R1 (低)**: `systemdnotify.Ready()` が呼ばれないバグ (`net.Listen` 失敗 path 等) → systemd が `TimeoutStartSec` (default 90s) 経過後に "start operation timed out" で unit 起動失敗。→ test で notify hook が呼ばれることを検証、docs に "notify で hang したら journald 確認" を追記。
- **R2 (低)**: 一部 distro (musl-libc 系, WSL2 等) で `NOTIFY_SOCKET` の abstract namespace ("@" プリフィクス) 処理が違う → `coreos/go-systemd/v22/daemon` は abstract namespace 対応済 (source 確認済み)。自作でないためリスクは吸収される。
- **R3 (低)**: C の 502→503 マップが `handleProtoError` の 502 (daemon RPC 失敗) と混同 → 別レイヤ (reverse proxy vs mux) で衝突しない。log tag を `"proxy: upstream not ready"` vs `"daemon_internal"` で分けて observability を保つ。
- **R4 (中)**: D の retry が実は他 endpoint (`/api/sessions`) にも必要になる可能性 → 汎用 helper として書くので follow-up plan で拾える。今回スコープ絞りは意識的。
- **R5 (低)**: `depguard` の rule 追加ミスで `coreos/go-systemd/v22/daemon` が `cmd/server` から直接 import できてしまう → `.golangci.yml` の rule を PR review でチェック。lint job で自動検知。
- **R6 (低)**: `Type=notify` unit が正しく反映されず旧 `Type=simple` として起動 → `systemctl --user show agent-reactor-server -p Type` を e2e に組み込んで確認 (§5.3-3)。

## 7. 将来の派生 (この plan の対象外)

- **Watchdog (`WATCHDOG=1` + `WatchdogSec=`)**: server が定期的に `WATCHDOG=1` を送り stall 検出。`coreos/go-systemd/v22/daemon.SdWatchdogEnabled()` で timeout を parse できる。別 plan。
- **RELOADING / STOPPING**: `systemctl reload agent-reactor-server` を意味あるものにする。今は reload = restart で足りるため別 plan。
- **socket activation** (`FDSTORE=1`, `LISTEN_FDS`): restart 時の listener 継承。graceful restart 対応の別 plan。
- **launchd / Windows service equivalent**: macOS / Windows での同等 readiness signal。プラットフォーム別 plan。
- **`/api/sessions` 等 bootstrap fetch の 5xx retry 展開**: D の `retryOn5xx` helper を再利用する形で follow-up plan 化。
