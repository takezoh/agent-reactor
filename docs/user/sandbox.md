# Sandbox Setup

Each agent runs inside a project-scoped devcontainer instead of the host shell, isolating filesystem, network, and credentials per project.

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

## Credential proxy (optional)

The credential proxy brokers short-lived credentials over a per-project Unix socket inside the container, instead of bind-mounting host credential files. Each provider activates only when its own settings are populated — listing nothing means nothing is exposed.

The container needs `curl` available (present in standard base images).

**AWS SSO — multi-profile.** Run `aws sso login` on the host before starting containers. List the profile names that should appear inside the container in the project's `.roost/settings.toml`:

```toml
# <project>/.roost/settings.toml
[sandbox.proxy]
aws_profiles = ["default", "master", "general"]
```

Each name becomes a `[profile <name>]` section in a synthetic `~/.aws/config` inside the container, wired to `credential_process`. Profiles outside the list are not reachable from the container. `~/.aws/sso/cache` is never bind-mounted.

**gcloud — credential isolation.** The OAuth refresh token never enters the container. The container reaches a GCE metadata server emulator running on the host, which calls `gcloud auth application-default print-access-token` (user-account mode) or `gcloud auth print-access-token --impersonate-service-account` (SA mode) on demand. Tokens are always fresh — `gcloud` on the host auto-refreshes via the stored refresh token when needed.

Two modes, selected by the presence of `service_account`. Both require `account` and `active`.

| field | required | description |
|-------|----------|-------------|
| `account` | yes | host gcloud principal whose credentials are used |
| `active` | yes | GCP project ID used as the container's default project |
| `service_account` | SA mode only | SA email to impersonate |
| `projects` | SA mode only | all project IDs available in the container |

**Service-account impersonation (recommended).** Container `gcloud` calls operate as the SA, scoped by its IAM bindings. Project boundaries are enforced.

```toml
# <project>/.roost/settings.toml
[sandbox.proxy.gcp]
account         = "user@example.com"
active          = "proj-prod"
service_account = "sa@proj.iam.gserviceaccount.com"
projects        = ["proj-prod", "proj-staging"]
```

Host prerequisites:

```sh
gcloud auth login
gcloud iam service-accounts add-iam-policy-binding sa@proj.iam.gserviceaccount.com \
  --member="user:user@example.com" \
  --role="roles/iam.serviceAccountTokenCreator"
```

**User-account proxy.** Omits impersonation; the container receives an ADC access token issued on behalf of the user. Refresh-token isolation is preserved, but project boundary enforcement is not. Use when SA setup is not feasible.

```toml
[sandbox.proxy.gcp]
account = "user@example.com"
active  = "my-project"
```

Host prerequisite — ADC credentials must be set up (`gcloud auth login` alone is not sufficient):

```sh
gcloud auth application-default login
```

`gcloud` must be installed in the container image. `gcloud auth login` inside the container fails by design — credentials flow only from the host via the metadata emulator.

**SSH agent — ephemeral keys only.** roost spawns an ephemeral `ssh-agent`, loads only the listed keys, and exposes its socket as `SSH_AUTH_SOCK` inside the container. Direct forwarding of the host `$SSH_AUTH_SOCK` is not supported.

```toml
[sandbox.proxy.ssh_agent]
keys = ["~/.ssh/id_ed25519"]
```

Passphrase-protected keys are skipped (a warning is logged). roost does not mount `~/.ssh` — add `known_hosts` entries via `postCreateCommand` (e.g. `ssh-keyscan github.com >> ~/.ssh/known_hosts`).

**Host-exec broker.** Lets containerized agents invoke host binaries (e.g. `gh`, `aws`, `kubectl`) without receiving credentials or tokens. Commands are filtered by deny/allow glob patterns with `*` as a wildcard. A non-empty `allow` list activates the broker.

```toml
[sandbox.proxy.host_exec]
allow = [
  "gh pr *",
  "gh issue *",
  "gh api GET /repos/*",
]
deny = [
  "gh * delete*",
  "gh auth *",
  "gh secret *",
]
```

Leading `KEY=VALUE` env assignments in patterns are stripped before matching (`"GH_TOKEN=x gh pr *"` ≡ `"gh pr *"`). Deny rules are checked first; unmatched commands are rejected by default.

**Overlay paths.** Use `overlay` to bind-mount shims at specific container paths. Useful when scripts reference binaries via relative paths (e.g. `./bin/plastic.exe`) rather than bare names on `PATH`. Each entry declares a `target` (project-relative or absolute container path) and optional per-overlay `allow`/`deny` patterns.

```toml
[[sandbox.proxy.host_exec.overlay]]
target = "bin/gh"                      # <workspace>/bin/gh
allow  = ["gh pr *", "gh issue *"]
deny   = ["gh auth *"]

[[sandbox.proxy.host_exec.overlay]]
target = "../shared/bin/tool"          # parent dir

[[sandbox.proxy.host_exec.overlay]]
target = "/opt/company/bin/internal"   # absolute path
allow  = ["internal *"]                # allow required; omitting means default-deny
```

Each overlay entry gets a unique broker alias derived from its target path, so two entries sharing a basename (e.g. `bin/gh` and `tools/gh`) remain independent and can carry different `allow`/`deny` rules. An empty `allow` list means default-deny (same as global `host_exec` allow semantics). User-scope and project-scope overlay lists are concatenated; entries with the same `target` are deduplicated with the project entry taking precedence.

**MCP proxy.** Runs MCP servers on the host so credentials (GCP ADC, AWS, etc.) never enter the container. Servers are declared in `~/.roost/settings.toml` or the project's `.roost/settings.toml`:

```toml
[sandbox.proxy.mcp_proxy.servers.observability]
command = "npx"
args    = ["-y", "@example/observability-mcp"]
allow   = ["list_*", "describe_*"]
deny    = ["delete_*"]

[sandbox.proxy.mcp_proxy.servers.observability.env]
GOOGLE_APPLICATION_CREDENTIALS = "~/.config/gcloud/application_default_credentials.json"
```

The map key (`observability`) is the MCP server alias. At container launch roost writes a `.mcp.json` into the project workspace (read-only bind-mount) that routes each configured alias through `roost mcp-exec <alias>`, overriding any project-local `.mcp.json` entry for the same names. No manual `.mcp.json` edits are required.

`allow`/`deny` patterns match the tool name with `*` as wildcard and use deny-first, default-deny semantics. User-scope and project-scope server maps are merged; project entries override user entries on the same alias.

See [Sandbox Backends](../technical/platform/sandbox.md) for the architecture, security model, and lifecycle internals.
