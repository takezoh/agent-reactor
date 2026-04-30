# Agent Roost

**"Command your agent fleet with zero friction."**

Agent Roost is your mission control for orchestrating multiple AI agents and maximizing developer creativity. Stop wasting time managing terminal tabs or polling for progress, and transform your workflow into a seamless commanding experience.

### Value & Experience

- **Deploy Agents with Minimal Steps**: Break free from "launch rituals" like directory hopping, environment activation, or long command strings. Just pick a project and hit a key. Send your agents into the field with the absolute minimum of friction.
- **Turn Wait Time into Free Time**: See at a glance whether an agent is working, waiting for your input, or pending tool approval. Visualize the status of your entire fleet across all projects without ever having to wander through multiple terminals.
- **Zero-Friction Context Switching**: Jump into any session the instant intervention is needed. With high-speed previews just by moving your cursor, you can oversee dozens of concurrent tasks without breaking your cognitive flow.
- **An Unshakeable Foundation for Agents**: Built on the rock-solid architecture of tmux, your agents' thoughts never stop even if you close the UI or lose your connection. Roost provides the most stable "ground" for autonomous agents to keep running until the job is done.

## Layout

```text
┌───────────────────┬────────┐
│                   │SESSIONS│
│  Pane 0: MAIN     │ ▼ projA│
│  (always focused) │  #1 ● │
│                   │  #2 ◆ │
├───────────────────┤ ▼ projB│
│  Pane 1: LOG      │  #1 ○ │
└───────────────────┴────────┘
```

## Requirements

- Go 1.26+
- tmux 3.2+

## Installation

```bash
make install
```

Installs to `~/.local/bin/roost`.

## Usage

```bash
roost
```

Creates a tmux session (or attaches to an existing one) and launches with a 3-pane layout.

### Sandbox

roost can run each agent inside a project-specific devcontainer instead of the host shell.

**Requirements:** Place a `devcontainer.json` in `<project>/.devcontainer/` and declare the image name using `image:` (pre-existing image) or `build.name` (roost extension for Dockerfile-based projects).

**Build images before first use** (roost does not build images):

```bash
# Dockerfile-based: build and set build.name in devcontainer.json
devcontainer build --workspace-folder . --image-name myproject:dev

# Or use docker directly:
docker build -t myproject:dev .
```

At session start, roost reads the image name from devcontainer.json (`image:` takes precedence over `build.name`).

Enable devcontainer mode in `~/.roost/settings.toml`:

```toml
[sandbox]
mode = "devcontainer"
```

**Restrict container egress (optional):** roost forwards `extra_create_args` to every `docker create`, so you can attach containers to a custom bridge whose egress you control with iptables.

```toml
[sandbox.devcontainer]
extra_create_args = ["--network", "roost-egress"]
```

Set up the bridge once on the host:

```sh
docker network create --opt com.docker.network.bridge.name=roost-egress roost-egress
sudo iptables -I DOCKER-USER -i roost-egress -j DROP
sudo iptables -I DOCKER-USER -i roost-egress -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
sudo iptables -I DOCKER-USER -i roost-egress -p udp --dport 53 -j ACCEPT
# ...allow specific destination IPs as needed
```

iptables operates on IPs, not hostnames; CDN-fronted services require maintaining IP ranges out-of-band. `--network=none` is not recommended — it blocks the model API the agent needs.

**Resource limits (optional):** use `runArgs` in `.devcontainer/devcontainer.json` to cap resource usage per project:

```jsonc
{
  "runArgs": ["--pids-limit", "512", "--memory", "4g", "--cpus", "2.0"]
}
```

**Workspace file ownership:** set `containerUser` and `remoteUser` to a non-root user to avoid root-owned files in the workspace. roost passes `containerUser` to `docker create -u` and `remoteUser` to `docker exec -u`.

```jsonc
{
  "containerUser": "ubuntu",
  "remoteUser": "ubuntu"
}
```

**Workspace mount target (optional):** by default roost mirrors the host path inside the container (host `/home/u/proj` → container `/home/u/proj`). Override the prefix when the host path cannot exist in the container:

```toml
[sandbox.devcontainer]
host_path_mount_prefix = "/mnt"   # → container /mnt/home/u/proj
```

Ignored when devcontainer.json sets `workspaceFolder` or `workspaceMount`.

**Pre-exec hook (optional):** `preExecCommand` (roost extension) in devcontainer.json runs inside the container before each `docker exec` launch, with cwd already set to the exec workdir. Default: `mise trust 2>/dev/null || true`.

**Tool credential bind-mounts:** declare interactive-auth credential paths in devcontainer.json `mounts`. Example for Claude Code subscription auth:

```jsonc
{
  "mounts": [
    "type=bind,source=${localEnv:HOME}/.claude,target=/home/vscode/.claude,consistency=cached",
    "type=bind,source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,consistency=cached"
  ]
}
```

#### Credential proxy (optional)

Enable to broker short-lived credentials over a per-project Unix socket inside the container, instead of bind-mounting host credential files:

```toml
# ~/.roost/settings.toml
[sandbox.proxy]
enabled = true
```

The container needs `curl` available (present in standard base images).

**AWS SSO — multi-profile.** Run `aws sso login` on the host before starting containers. List the profile names that should appear inside the container in the project's `.roost/settings.toml`:

```toml
# <project>/.roost/settings.toml
[sandbox.proxy]
aws_profiles = ["default", "master", "general"]
```

Each name becomes a `[profile <name>]` section in a synthetic `~/.aws/config` inside the container, wired to `credential_process`. Profiles outside the list are not reachable from the container. `~/.aws/sso/cache` is never bind-mounted.

**gcloud — service-account impersonation.** Container `gcloud` calls operate as the impersonated SA, scoped by the SA's IAM bindings. The OAuth refresh token never enters the container.

```toml
# <project>/.roost/settings.toml
[sandbox.proxy.gcp]
service_account = "sa@proj.iam.gserviceaccount.com"   # required
projects        = ["proj-prod", "proj-staging"]        # required; first entry is the active default
account         = "user@example.com"                   # optional — defaults to current host gcloud principal
```

Configuring `account` without `service_account` is rejected (would yield a full-scope user token).

Host prerequisites:

```sh
gcloud auth login                                                                  # or use a SA key
gcloud iam service-accounts add-iam-policy-binding sa@proj.iam.gserviceaccount.com \
  --member="user:user@example.com" \
  --role="roles/iam.serviceAccountTokenCreator"
```

`gcloud` must be installed in the container image. `gcloud auth login` inside the container fails by design.

**SSH agent — ephemeral keys only.** roost spawns an ephemeral `ssh-agent`, loads only the listed keys, and exposes its socket as `SSH_AUTH_SOCK` inside the container. Direct forwarding of the host `$SSH_AUTH_SOCK` is not supported.

```toml
[sandbox.proxy.ssh_agent]
keys = ["~/.ssh/id_ed25519"]
```

Passphrase-protected keys are skipped (a warning is logged). roost does not mount `~/.ssh` — add `known_hosts` entries via `postCreateCommand` (e.g. `ssh-keyscan github.com >> ~/.ssh/known_hosts`).

**GitHub — `GH_TOKEN`.** When the proxy is enabled, the host's `gh auth token` value is injected into the container as `GH_TOKEN` so the `gh` CLI works without bind-mounting `~/.config/gh`.

**WSL2 Windows exe broker (WSL2 only).** Lets containerized agents invoke Windows-side executables (`*.exe`, `*.ps1`) through a host-side broker. A non-empty `allowed_exes` activates the broker; empty / absent disables it. Ignored on non-WSL2 hosts.

```toml
[sandbox.proxy.win_exec]
allowed_exes  = ["powershell.exe", "code.exe"]   # basenames the container may invoke
[sandbox.proxy.win_exec.resolve]
"notify.ps1"  = "C:\\Tools\\notify.ps1"          # optional; unlisted names use Windows PATH
```

See [Sandbox Backends](docs/sandbox.md) for the architecture, security model, and lifecycle internals.

### Hook Setup

Register roost hooks so agent status (● ◆ ◇ ○ ■) updates in real time. Run once per agent type:

```bash
roost claude setup    # registers hooks in ~/.claude/settings.json
roost codex setup     # registers hooks in ~/.codex/
roost gemini setup    # registers hooks in ~/.gemini/settings.json
```

Hooks are idempotent — re-running adds only missing entries and never overwrites existing config.

Without hooks, roost still launches sessions but status detection degrades to polling (idle detection only).

### Key Bindings

**Prefix bindings** work regardless of which pane is focused (pane navigation, detach, palette open).
**SESSIONS pane bindings** are active only when the SESSIONS pane is focused (session operations).

#### Prefix Keys

Default prefix: `Ctrl+b` (same as tmux default). Configurable via `[tmux] prefix`.

| Key | Action |
|------|-----------|
| `prefix Space` | Toggle MAIN ↔ SESSIONS pane |
| `prefix Escape` | Preview project |
| `prefix z` | Zoom MAIN pane |
| `prefix d` | Detach (tmux stays alive; re-run `roost` to resume) |
| `prefix q` | Quit all (tmux session is destroyed) |
| `prefix p` | Command palette |
| `prefix C-p` | Push driver palette (overlay a new agent onto the current session) |

#### Command Palette (`prefix p`)

Displayed as a popup. Filter tools by typing, press Enter to execute.

```text
> new█
▸ new-session       Create session
  create-project    Create new project dir and start session
```

| Tool | Description |
|--------|------|
| `new-session` | Create session |
| `create-project` | Create new project dir and start session |
| `stop-session` | Stop session |
| `detach` | Detach (keep session) |
| `shutdown` | Shutdown (discard sessions) |

#### SESSIONS Pane Bindings

| Key | Action |
|------|-----------|
| `j`/`k` or `↑`/`↓` | Select session (previews in MAIN pane) |
| `Enter` | Switch to selected session → return to MAIN |
| `n` | Quick launch (default command) |
| `N` | Launch with command selection |
| `d` | Stop session |
| `Tab` | Collapse/expand project |
| `1`-`5` | Toggle status filter (Running / Waiting / Idle / Stopped / Pending) |
| `0` | Reset filter |
| `[` / `]` | Cycle workspace (shown only when 2+ workspaces exist) |
| `` ` `` | Reset workspace filter to All |

### Session States

| Display | State |
|------|------|
| `●` green | Running (producing output) |
| `◆` yellow | Waiting (awaiting input) |
| `◇` yellow | Pending approval (awaiting tool execution permission) |
| `○` gray | Idle (no output for 30+ seconds) |
| `■` red | Stopped |

## Configuration

```toml
# ~/.roost/settings.toml

# data_dir = ""                 # Override config/data directory (default: ~/.roost)

[log]
level = "info"                  # "debug" | "info" | "warn" | "error"

[tmux]
session_name = "roost"
prefix = "C-b"                  # Prefix key
pane_ratio_horizontal = 75      # Main pane width % (1-99)
pane_ratio_vertical = 75        # Main pane height % (1-99)

[monitor]
poll_interval_ms = 1000         # Background polling interval
fast_poll_interval_ms = 100     # Polling interval while TUI is active
idle_threshold_sec = 30         # Seconds of silence before "Idle" (○)

[session]
auto_name = true                # Auto-generate session names
default_command = "shell"       # Command run by `n` (quick launch)
commands = [                    # Commands available via `N`
  "claude",
  "codex",
  "gemini",
  "shell",
]
push_commands = [               # Commands available via push-driver palette
  "shell",
  "git diff",
  "git diff --staged",
]

[projects]
project_roots = ["~/projects"]       # Subdirs of each root become projects
project_paths = ["~/myproject"] # Explicit project paths

[sandbox]
mode = "devcontainer"               # "direct" (default) | "devcontainer"

[sandbox.devcontainer]
# extra_create_args = []            # appended to "docker create"
# env_script = ""                   # script that prints KEY=VALUE lines for a project
# host_path_mount_prefix = ""       # prefix for the auto-mounted workspace inside the container
                                    # empty (default): mirror host path as-is (e.g. /home/u/proj → /home/u/proj)
                                    # "/mnt": prepend prefix (e.g. /home/u/proj → /mnt/home/u/proj)
                                    # ignored when devcontainer.json sets workspaceFolder or workspaceMount

# [sandbox.proxy]                   # credential proxy — see "Sandbox > Credential proxy" above
# enabled = true

[driver]
# summarize_command = "claude -p --model=haiku --no-session-persistence --setting-sources user"
# summarize_command = "codex exec --ephemeral --model gpt-4o-mini -"
# summarize_command = "gemini -p '' -m gemini-2.5-flash-lite"

[drivers.claude]
show_thinking = false           # Show extended thinking blocks in MAIN pane

# Desktop notifications — empty rules = disabled.
# Each rule AND-combines driver / command / project / kind.
# Empty string or "*" on any axis means "match any".
# Driver / command / project are glob patterns (path.Match);
# project supports "~" expansion.

[[notifications.rules]]
# claude requests tool approval under ~/projects/prjA
driver  = "claude"
project = "~/projects/prjA"
kind    = "pending_approval"

[[notifications.rules]]
# any agent finishes its turn
kind = "done"
```

Works with default values even without a config file.

### Per-Project Configuration

Each project directory can have its own `.roost/settings.toml`:

```toml
# <project-dir>/.roost/settings.toml

[workspace]
name = "work"    # Group this project under a named workspace
```

The workspace switcher chip bar appears in the SESSIONS pane automatically when
two or more distinct workspaces exist, and is hidden for single-workspace setups.
Projects without a settings file fall back to the `default` workspace.
