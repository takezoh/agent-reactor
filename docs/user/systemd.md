# Run as a systemd service (production)

This guide brings the two-process production stack (`server`, `web`) up as
per-user systemd units. The dev launcher (`scripts/run-dev.sh`) remains the
right tool for ad-hoc work — the systemd path is for hosts that should
restart on crash, autostart on boot, and persist logs through `journald`.

> A user-launched `server` backend's data directory (`~/.agent-reactor/`)
> and the service's (`~/.local/state/agent-reactor/`) are independent — both
> can run in parallel without interfering.

## Architecture (gateway + web host)

```
server   ── owns sessions, sockets, on-disk state (-data-dir),
            and serves the HTTP/WS gateway in the same process
   ▲
   └── web    ── browser UI + reverse proxy to server
```

Restarting `web` does not lose sessions — sessions live in `server`.
Restarting `server` drops every attached browser tab and recreates sessions
from disk on the next boot.

## Prerequisites

- Linux with systemd (Ubuntu 24.04 verified).
- Docker reachable from the invoking user — either via membership in the
  `docker` group, or via a rootless docker socket at
  `$XDG_RUNTIME_DIR/docker.sock` (auto-detected).
- A user-writable `~/.local/bin/` and `~/.config/systemd/user/`.

## Install

```sh
# 1) build the production binaries + libexec helpers
make build-server build-web

# 2) install binaries (renamed to service vocabulary) and unit files
make install-systemd
#   → ~/.local/bin/agent-reactor-server
#   → ~/.local/bin/agent-reactor-web
#   → ~/.local/lib/agent-reactor/{reactor-bridge,notify.ps1}
#   → ~/.config/systemd/user/agent-reactor-{server,web}.service

# 3) make services survive logout (boot-time autostart)
loginctl enable-linger $USER

# 4) start the stack (cascades down to server)
systemctl --user daemon-reload
systemctl --user enable --now agent-reactor-web.service
```

The cascade is by `Requires=` / `BindsTo=`: enabling `web` pulls in
`server` (the daemon + gateway). There is no need to enable the lower unit
separately.

## Connect from a browser

```sh
# from your workstation
ssh -L 8080:127.0.0.1:8080 prod-host

# on prod-host (once)
cat ~/.local/state/agent-reactor/server.token

# in the browser
http://127.0.0.1:8080/#token=<paste>
```

The bearer token is generated on first boot and persisted to
`~/.local/state/agent-reactor/server.token` (mode 0600). Restarting `server`
re-reads the same file, so bookmarked URLs survive a unit reload. Rotate by
deleting the file and restarting `agent-reactor-server`.

## Logs

```sh
journalctl --user -u agent-reactor-server -f
journalctl --user -u agent-reactor-web    -f
```

The backend's slog output is appended to
`~/.local/state/agent-reactor/server.log` (rotated per startup), and its
session socket is at `~/.local/state/agent-reactor/server.sock`.

## Verify the cascade

```sh
# stopping server cascades down: web stops within a few seconds.
systemctl --user stop agent-reactor-server
systemctl --user status agent-reactor-web     # → inactive

# restart web alone — server stays active, sessions survive.
systemctl --user start agent-reactor-server
systemctl --user restart agent-reactor-web
# (existing browser tab keeps its session list; web reconnect is transparent)
```

## LAN / external exposure

The shipped units bind `127.0.0.1` and pass `-insecure` — appropriate for
single-user hosts where access is always via SSH tunnel. The browser only
talks to `agent-reactor-web`; `agent-reactor-server` is an internal backend
the web unit reverse-proxies to. **For LAN access, override `-web` only and
keep `-server` on loopback.**

### Bind `-web` to 0.0.0.0 (plain HTTP — loopback-grade trust required)

Create a drop-in (`systemctl --user edit agent-reactor-web` opens an editor;
or write the file directly):

```sh
mkdir -p ~/.config/systemd/user/agent-reactor-web.service.d
cat > ~/.config/systemd/user/agent-reactor-web.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=%h/.local/bin/agent-reactor-web -addr 0.0.0.0:8080 -insecure -server http://127.0.0.1:8443
EOF
systemctl --user daemon-reload
systemctl --user restart agent-reactor-web
ss -tlnp | grep 8080   # expect *:8080
```

The empty `ExecStart=` line on its own is required — systemd refuses to
append a second `ExecStart=` to a `Type=simple` unit, so without the reset
the drop-in is silently ignored and the unit keeps the shipped
`127.0.0.1:8080` binding. `systemctl --user show agent-reactor-web -p
ExecStart -p DropInPaths` is the fastest way to confirm an override is in
effect.

Plain HTTP on `0.0.0.0` means the bearer token (`server.token`) crosses the
LAN in cleartext. Acceptable only on a trusted segment; otherwise add TLS
(below) or front with a reverse proxy.

### TLS direct on `-web`

1. Drop a certificate pair somewhere readable by your user (e.g.
   `~/.config/agent-reactor/tls/{fullchain.pem,privkey.pem}`).
2. Extend the drop-in:
   ```ini
   [Service]
   ExecStart=
   ExecStart=%h/.local/bin/agent-reactor-web \
     -addr 0.0.0.0:8443 \
     -tls-cert %h/.config/agent-reactor/tls/fullchain.pem \
     -tls-key  %h/.config/agent-reactor/tls/privkey.pem \
     -server http://127.0.0.1:8443
   ```
   (drop `-insecure`; `-tls-cert` / `-tls-key` / `-addr` are CLI flags, not
   env-aware. A future `web.env` hook is reserved by `EnvironmentFile=-` but
   has no env-aware knobs today.)
3. `daemon-reload` + `restart agent-reactor-web`.

### Reverse proxy in front (recommended for real production)

Keep `-web` on `127.0.0.1:8080 -insecure` and front it with nginx / caddy +
Let's Encrypt on the host. The proxy terminates TLS and forwards to
loopback; no drop-in needed.

### Firewall

Binding to `0.0.0.0` is not enough on hosts with a firewall. Check + open:

```sh
sudo ufw status                       # if active:
sudo ufw allow 8080/tcp               # (or 8443 for TLS)
sudo iptables -nL INPUT | grep -E '8080|policy'
```

Cloud VMs additionally need the provider's security-group / VPC firewall
open on the same port.

## Uninstall

```sh
systemctl --user disable --now agent-reactor-web.service
systemctl --user stop agent-reactor-server.service
rm ~/.config/systemd/user/agent-reactor-{server,web}.service
rm ~/.local/bin/agent-reactor-{server,web}
rm -rf ~/.local/lib/agent-reactor
# Optionally also drop persistent state (sessions, token, logs):
rm -rf ~/.local/state/agent-reactor
loginctl disable-linger $USER  # if you no longer want any user service to autostart
```
