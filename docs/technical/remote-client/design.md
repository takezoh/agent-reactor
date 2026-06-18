# Remote client-server architecture (tmux 全廃 + pty multiplexer + Web client)

> Status: **In progress**. Locks the decisions taken in the planning thread and
> defines the target architecture for splitting `arc` into a standalone server
> and a remote (Web-first) client. The tmux-free web client⇄server is implemented
> under `src/` (`platform/termvt`, `server/*`, `client/web`, `cmd/server`); see
> §9 for status. Legacy tmux `arc` removal and pure-core reuse are the remaining
> work.

## 1. Goal

Split `arc` into a **standalone server process** and a **remote client** that can
connect to a server on the **same or a different host**. `orchestrator` is *already*
a standalone, TUI-less, network-reachable server (FC/IS loop + observability HTTP),
so it is the reference shape — the real work is re-architecting `arc`.

Locked decisions (planning thread):

| Decision | Choice |
|---|---|
| `arc` client interactivity | **Full interactive** (stream the agent terminal, not just cards/control) |
| Multiplexer | **Abolish tmux entirely**; build our own pty + multiplexer server-side |
| Client language | **Free** — first client is **Web (xterm.js)**; native (Go/Rust) optional later |
| Server language | **Go** (stays in the existing module/layers) |

## 2. Why this is tractable

`arc` already separates daemon ↔ TUI over a typed IPC (`proto`, NDJSON over
`net.Conn`), and the runtime already hides tmux behind a **DI seam** —
`TmuxBackend` role interfaces (`PaneLifecycle`, `PaneIO`, `WindowLayout`,
`PaneInspect`, `SessionEnv`, `TmuxControl`) in `client/runtime/backends.go`.
The reducer only emits `Effect`s; the backend performs the I/O. tmux is therefore
the **presentation + process-supervision** layer, **not** the state/data layer.

Evidence it is replaceable:

- The **Codex** path already bypasses tmux for its data plane (app-server over
  UDS + WebSocket; the pane just attaches with `codex --remote`).
- OSC handling is already server-side today: `tapManager` feeds `tmux pipe-pane`
  output into the `client/driver/vt` emulator, which fires callbacks
  (OSC 9/99/777, titles, OSC 133 prompt phases) → `EvPaneOsc`/`EvPanePrompt`.

So: **replace the `TmuxBackend` implementation with a `PtyBackend`** (own pty +
multiplexer), keep the pure core (`state/Reduce`, `Driver`) untouched, and add a
network transport + Web gateway.

## 3. Build-vs-buy boundary

There is **no embeddable Go terminal-multiplexer library** (gotty/ttyd/wetty are
single-pty apps; zellij is a Rust app). So:

- **Buy (off-the-shelf primitives):** pty, server-side screen buffer / VT, wire
  transport, browser renderer.
- **Build (maps onto existing seams):** session multiplexing, detach/reattach
  persistence, process supervision — i.e. the `PtyBackend` + the existing
  runtime/reducer.

## 4. Recommended stack

```
Server (Go, tmux-free):
  pty supervision : github.com/creack/pty          (v1.1.24, MIT, Linux/macOS + resize)
  screen state    : github.com/charmbracelet/x/vt  (grid + scrollback + Render() snapshot
                                                     + Resize + Damage + OSC handlers)
  multiplexing    : built in-house (TmuxBackend → PtyBackend, on the runtime/reducer)
  wire            : existing proto (NDJSON); output frames = asciicast v2 [t,"o",data];
                    control = structured events; transport = unix (local) /
                    tcp+tls+token (remote) / WebSocket (web gateway)

Client:
  first  : Web + xterm.js (+ thin attach bridge)   — zero-install, broadest reach
  later  : Go TUI (bubbletea + x/vt) — lightweight terminal-in-terminal
  later  : native GPU (Rust alacritty_terminal / termwiz) — only if desktop-grade
           throughput/latency is required
```

### Library selection justification (per AGENTS.md “Library Selection”)

- **pty — `creack/pty` vs `aymanbagabas/go-pty` vs `Kodecable/crosspty`.** The
  server runs on Linux (docker). `creack/pty` is the de-facto, smallest, most
  battle-tested (MIT, v1.1.24 / 2024-10), with `Setsize`/`InheritSize` resize
  support. `go-pty` (v0.2.3) only adds Windows ConPTY (builds *on* creack) — adopt
  it only if a Go server on Windows is ever required. **Chosen: creack/pty.**
- **screen buffer — `charmbracelet/x/vt` vs `hinshun/vt10x` vs `rcarmo/go-te`.**
  Reattach needs a server-held grid + scrollback + a `Render()` snapshot + resize.
  `x/vt` provides exactly that (`Scrollback` default 10k, `Render()`, `Resize`,
  `Damage` diffs, `RegisterOscHandler`, `SafeEmulator`) **and** is in the charm
  ecosystem already vendored by the client (`bubbletea`/`lipgloss`/`x/term`).
  Risk: pre-1.0 (untagged) → isolate behind an internal wrapper. `vt10x` is older
  and lacks the damage/snapshot ergonomics; `go-te` is new/unproven.
  **Chosen: charmbracelet/x/vt (wrapped).**
- **client renderer — `xterm.js`.** De-facto web terminal (powers VS Code),
  has an attach addon, WebGL renderer. Matches “client language is free”.
- **wire — asciicast v2.** NDJSON `[time, code, data]`, incremental, live-stream
  friendly (<100 ms), interruption-resilient — and it co-exists with arc’s
  existing NDJSON `proto`.

> Wire-format/persistence types must stay stdlib-only (AGENTS.md). `crypto/tls`
> is stdlib, so TLS is fine; the asciicast framing is stdlib `encoding/json`.

## 5. OSC handling: server-side, two-path “tee”

The server feeds every pty byte into the VT emulator **and** forwards the raw
stream to the client. OSC splits into two classes:

| Class | Examples | Server action | To client |
|---|---|---|---|
| **Semantic / control** | OSC 133 (prompt/exit), OSC 9/777 (notify), OSC 0/1/2 (title), OSC 7 (cwd) | **Terminate server-side** → drive driver state (status, tags, run-state) | **Structured proto event** (not raw bytes) |
| **Rendering** | SGR, OSC 4/10/11/12 (palette), sixel/OSC 1337 (images) | Pass through unparsed | Raw bytes (asciicast frame) — client terminal renders |

Wins unique to server-side termination on a **remote** setup:

1. **Notification destination is correct** — OSC 9/777 become a proto event
   delivered to the *operating client* instead of a toast on the server host.
2. **Hook-less run-state** — OSC 133 lets the server track command/exit
   boundaries → idle/running/waiting without per-agent hooks.
3. **Policy** — terminate OSC 52 (clipboard exfil), sanitize title injection,
   rate-limit notifications.

Open design point: for OSC the client terminal must **not** see (OSC 52, or OSC 9
re-routed to a proto event), strip it from the passthrough stream and re-emit as
a control event. v1 keeps **raw passthrough + sniff (tee)**; a future
server-rendered (cell-diff, mosh/zellij style) mode terminates all OSC.

This generalizes the existing `EvPaneOsc`/`EvPanePrompt` path; `x/vt`’s
`RegisterOscHandler(cmd, func([]byte) bool)` (data = `"<cmd>;<text>"`) and
`Callbacks{Title,Bell,…}` are the hooks.

## 6. Transport & auth

| Path | Transport | Auth |
|---|---|---|
| Local, same host | Unix socket (existing) | SO_PEERCRED (existing) |
| Remote | TCP + **TLS** | **bearer token** (mTLS optional) |
| Web | WebSocket (gateway bridges proto ↔ ws) | bearer token via `Authorization` header (REST); short-lived single-use ticket (WS); default origin check (anti-CSWSH) |

`proto.Client`/`DialConn` already accept any `net.Conn`, so TLS is a drop-in
dialer; the server `StartIPC` grows a `StartIPCNet(tcp, tlsConfig, Authenticator)`
sibling and the auth check abstracts from `checkPeerCred` to an `Authenticator`
interface (peercred | bearer-token). SSH tunnels remain a zero-config fallback
(wrap the TCP listener) — but are *not* the primary path because they couple auth
to OS accounts, can’t express per-session/read-only authz, and don’t provide
attach/multiplexing semantics (those stay app-level regardless).

The Web gateway follows the **Zellij web-client pattern**: a thin layer that
reuses the client protocol as a translation bridge between the browser’s
WebSocket and the server’s IPC.

## 7. Phased plan

0. **Transport abstraction** — `Authenticator` seam + `StartIPCNet` (TCP+TLS+token);
   `proto` TLS dialer. Yields remote *control + observability* (no interactivity).
1. **Observation completeness** — replace TUI’s direct local-file reads
   (`arc.log`, transcripts) with `FileRelay`-over-wire so a remote client needs no
   disk access.
2. **Full pty interactivity (the core)** — `PtyBackend` (creack/pty + x/vt),
   pane-stream subsystem, asciicast frames + input/resize control, **Web gateway +
   xterm.js client**, server-side OSC tee. Replace `swap-pane` rendering.
3. **tmux removal** — delete the tmux backend; local == remote (transport only
   differs); client-side layout composition replaces the tmux 3-pane control screen.
4. **(optional) orchestrator control API / unified client** — add control
   endpoints (pause/resume/cancel/requeue) + TLS/token; optionally let `arc`’s
   client speak to the orchestrator too.

## 8. Risks

- **VT fidelity** — `x/vt` must hold up vs tmux’s mature handling (copy-mode,
  truecolor, terminfo edge cases). Mitigate: raw passthrough in v1 (the client’s
  real terminal does emulation); the server grid is only for snapshot/scrollback.
- **`x/vt` pre-1.0** — wrap behind an internal interface (DI seam) to absorb API
  churn.
- **Reattach atomicity** — snapshot (`Render()`) + subscribe must be atomic vs
  live writes; serialize under the session’s single-writer lock.
- **Backpressure** — slow clients must not corrupt the byte stream; production
  needs bounded buffering with disconnect-on-overflow, not frame-dropping.
- **Process supervision** — re-implement tmux’s `remain-on-exit`/liveness as pty
  EOF/exit → events (maps onto existing `EvPaneDied` semantics).

## 9. Implementation status

The tmux-free web client⇄server now lives in `src/` (the original
`playground/webterm/` PoC has been ported and removed):

- `platform/termvt/` — pty (`creack/pty`) → server-side VT (`charmbracelet/x/vt`)
  tee + multi-session Manager; emits typed events (OSC 9/133/title captured as
  Control), reattach snapshot via `Render()`, resize, multi-subscriber fan-out.
- `server/web/` — WebSocket↔termvt attach (asciicast v2 output + control frames),
  REST `/api/sessions`, static-client mux. Auth: bearer token via `Authorization`
  header (never a query param); WebSocket connections use a short-lived,
  single-use ticket minted over that API and the default same-origin check
  (anti-CSWSH). All responses carry `Referrer-Policy: no-referrer` and a strict
  CSP (`script-src 'self'`; xterm.js is vendored, not CDN-loaded).
- `server/session/` — session lifecycle over `termvt.Manager`; launch wrapping via
  `agentlaunch.Dispatcher` (Direct now; `SandboxDispatcher`/devcontainer drop-in).
- `client/web/` — embedded `xterm.js` single-page client.
- `cmd/server/` — composition + TLS (self-signed default) + token + graceful stop.

Run: `make build-server && ./server -insecure -token <tok> -addr :8443`, then open
`/#token=<tok>` (fragment, not query). Go tests cover termvt, the session service,
the REST mux + header auth, the single-use ticket store, the security headers, and
the http→ws→pty attach path including ticket gating and cross-origin rejection;
`go test -race` green.

**Remaining (large, incremental):** reuse the pure core (`state.Reduce`/`Driver`)
for status detection / view / persistence per §“C”, and remove the legacy tmux
`arc` (`cmd/arc`, `client/runtime` tmux, `client/tui`) so `grep -ri tmux src/` is 0.
