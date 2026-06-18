# Web client⇄server (`server` + `web`)

The tmux-free system is split into two processes:

- **`server` (backend)** — a headless host: it runs agent sessions over pty (on
  the host, or later in a devcontainer) and exposes them over a REST + WebSocket
  API. Sessions are **host-owned**: they keep running when a client disconnects,
  and **several clients can attach to and share one session** (one operator's
  `claude-code` can be driven from another's browser). It serves no UI.
- **`web` (web-client host)** — serves the browser UI (xterm.js) and
  reverse-proxies `/api` and `/ws` to the backend, so the browser talks only to
  this one origin. Future native clients connect to the backend directly instead.

Architecture: [remote-client design](../technical/remote-client/design.md).

## Build & run

The fastest path for local dev launches both processes together:

```sh
make run-dev
#   → backend  : http://127.0.0.1:8443
#     web      : http://127.0.0.1:8080
#     Open →   http://127.0.0.1:8080/#token=<generated>
#   Ctrl-C stops both. (scripts/run-dev.sh; override BACKEND_ADDR/WEB_ADDR/TOKEN.)
```

Or run them separately:

```sh
make build-server build-web           # → ./server  ./web

# Backend (clients connect here). TLS self-signed by default; token generated if unset:
./server -addr :8443
#   → "agent-reactor backend on https://:8443  token=<generated>"

# Web-client host, pointed at the backend:
./web -addr :8080 -server https://127.0.0.1:8443
```

Open `http://<web-host>:8080/#token=<token>` in a browser. The token goes in the
URL **fragment** (`#…`), not the query string, so it is never sent to a server,
logged, or leaked via `Referer`. The static page holds no secrets and loads
without auth; authority is on the data plane: the page uses the token as an
`Authorization: Bearer` header for the REST API (proxied to the backend) and
exchanges it for a short-lived, single-use ticket to open each WebSocket
(browsers cannot set headers on a WebSocket). For an SSH-only host, forward the
web port (`ssh -L 8080:localhost:8080 <host>`).

> The `web` process proxies to an **http** backend (local dev: `server -insecure`)
> or a **real-certificate https** backend; it cannot verify a self-signed https
> backend. `make run-dev` runs both over plain HTTP on localhost.

## Using it

- **Create** — type a command (e.g. `bash`) and an optional project directory,
  then **+ create**. The session starts in a pty and you attach automatically.
- **Attach / switch** — click a session in the left list. Each attach starts from
  a screen snapshot (reattach-safe), then streams live; multiple tabs — and
  multiple clients — can attach to and share the same session.
- **Detach-safe** — closing the browser/tab detaches only; the session keeps
  running on the backend and can be re-attached later.
- **Interact** — type in the terminal; it resizes with the window.
- **Server-side events** — OSC 9/777 notifications, OSC 133 prompt markers, and
  window titles are captured on the backend and shown in the events panel (so a
  notification reaches the operating client, not the server host).
- **Stop** — the ✕ next to a session terminates it.

## Flags

`server` (backend) and `web` (web-client host) share the same transport flags:

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `server`: `:8443`, `web`: `:8080` | Listen address |
| `-server` | `web` only: `http://127.0.0.1:8443` | Backend base URL to proxy `/api` and `/ws` to |
| `-token` | `server` only: generated | Bearer token (REST: `Authorization: Bearer …`; WebSocket: single-use ticket; never accepted as a query parameter) |
| `-tls-cert` / `-tls-key` | — | TLS certificate/key; self-signed if omitted |
| `-insecure` | false | Serve plain HTTP (local dev only) |
