# Web client⇄server (`server` + `web`)

The runtime stack is two processes:

- **`server`** — the single-process backend. One Go binary that owns both
  the pty session daemon (typed proto over a Unix socket, `-data-dir`
  rooted) and the HTTP/WS gateway that translates browser REST/WebSocket
  traffic into in-process daemon calls. Every session is a pty managed in
  the same process (host or devcontainer launch); several browser tabs can
  attach to and share one session. Sessions outlive any single browser
  client.
- **`web` (web-client host)** — serves the browser UI (React + xterm.js) and
  reverse-proxies `/api` and `/ws` to `server`, so the browser talks only to
  this one origin. Future native clients connect to `server` directly.

Wire-level detail and the full REST + WebSocket vocabulary live in
[server gateway internals](../technical/web-gateway.md).

## Build & run

The fastest path for local dev launches the **entire** stack — an isolated
`server` backend plus the web host — together:

```sh
make run-dev
#   → backend : http://127.0.0.1:8443  (sock: $ROOT/.run-dev/server/server.sock)
#     web     : http://127.0.0.1:8080
#     Open  →  http://127.0.0.1:8080/
#   Ctrl-C stops both processes (the scratch dir is preserved by default;
#   set CLEAN_DATA_DIR=1 to wipe it).
```

The scratch data dir means `make run-dev` never collides with the user's
production `server` backend (`~/.agent-reactor/`); the two can run side by
side. Overrides: `BACKEND_ADDR`, `WEB_ADDR`, `SERVER_DATA_DIR` (custom
isolated dir), `CLEAN_DATA_DIR=1` (wipe the scratch dir on exit).

Or run the backend + web host standalone:

```sh
make build-server build-web      # → ./server ./web

# The server binary boots the daemon and a co-resident HTTP/WS gateway in
# one process under -data-dir. SIGTERM tears down both. Pick any writable
# path; the example uses an XDG-style cache dir so the command works for
# non-root users out of the box.
./server -addr :8443 -data-dir "$HOME/.cache/agent-reactor-web"
#   → "agent-reactor backend on https://:8443  sock=…/server.sock"

# Web-client host, pointed at the backend:
./web -addr :8080 -server https://127.0.0.1:8443
```

The backend always owns its socket — there is no "attach mode" because the
gateway is in-process. The socket path is derived from `-data-dir`
(`<data-dir>/server.sock`); two backends with distinct `-data-dir` values
run independently, which is what `make run-dev` relies on to stay isolated
from a user-scope `~/.agent-reactor/` install.

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

## Terminal font

The web terminal font is set from `~/.agent-reactor/settings.toml`. Both keys are
optional; leaving them unset keeps the xterm.js built-in monospace default.

```toml
[terminal]
font_family = "HackGen Console NF"   # any CSS font-family value
font_size   = 14                     # px; omit or 0 keeps the default
```

The gateway re-reads `settings.toml` on every `GET /api/session-config`, so a
change takes effect on the next page load — no rebuild or restart needed. The
font must be installed on the machine running the **browser** (it is applied as a
local font, not downloaded); if it is missing, xterm falls back to its default.

## Flags

`server` (backend) and `web` (web-client host) share the same transport flags:

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `server`: `:8443`, `web`: `:8080` | Listen address |
| `-server` | `web` only: `http://127.0.0.1:8443` | Backend base URL to proxy `/api` and `/ws` to |
| `-token` | `server` only: generated | Bearer token (REST: `Authorization: Bearer …`; WebSocket: single-use ticket; never accepted as a query parameter) |
| `-tls-cert` / `-tls-key` | — | TLS certificate/key; self-signed if omitted |
| `-insecure` | false | Serve plain HTTP (local dev only) |
