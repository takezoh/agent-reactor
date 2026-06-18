# Web client⇄server (`server`)

`server` is the tmux-free, web-based client⇄server: it manages agent sessions
over pty on the host (or, later, in a devcontainer) and serves a browser client
that creates, attaches to, and stops them over WebSocket. A client may run on the
same or a different host. Architecture: [remote-client design](../technical/remote-client/design.md).

## Build & run

```sh
make build-server                     # → ./server

# TLS by default (self-signed); bearer token generated and printed if unset:
./server -addr :8443
#   → "agent-reactor server on https://:8443  token=<generated>"

# Local dev without TLS:
./server -insecure -addr 127.0.0.1:8443 -token mytoken

# Bring your own certificate:
./server -tls-cert cert.pem -tls-key key.pem -addr :8443 -token mytoken
```

Open `https://<host>:8443/#token=<token>` in a browser. The token goes in the
URL **fragment** (`#…`), not the query string, so it is never sent to the
server, logged, or leaked via `Referer`. The static page itself holds no secrets
and loads without auth; authority is on the data plane: the page uses the token
as an `Authorization: Bearer` header for the REST API and exchanges it for a
short-lived, single-use ticket to open each WebSocket (browsers cannot set
headers on a WebSocket). For cross-host access, point the browser at the
server's address; for an SSH-only host, forward the port (`ssh -L`).

## Using it

- **Create** — type a command (e.g. `bash`) and an optional project directory,
  then **+ create**. The session starts in a pty and you attach automatically.
- **Attach / switch** — click a session in the left list. Each attach starts from
  a screen snapshot (reattach-safe), then streams live; multiple tabs can attach
  to the same session.
- **Interact** — type in the terminal; it resizes with the window.
- **Server-side events** — OSC 9/777 notifications, OSC 133 prompt markers, and
  window titles are captured on the server and shown in the events panel (so a
  notification reaches the operating client, not the server host).
- **Stop** — the ✕ next to a session terminates it.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `:8443` | Listen address |
| `-token` | generated | Bearer token (REST: `Authorization: Bearer …`; WebSocket: single-use ticket; never accepted as a query parameter) |
| `-tls-cert` / `-tls-key` | — | TLS certificate/key; self-signed if omitted |
| `-insecure` | false | Serve plain HTTP (local dev only) |
