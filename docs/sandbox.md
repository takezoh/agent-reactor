# Sandbox Backends

## Purpose

Sandbox backends isolate agent processes per project — each project directory runs inside its own container with scoped filesystem, network, and capability restrictions.

The state layer knows only `LaunchPlan.Project` (the project path); it has no awareness of which backend is active. Backend selection and command wrapping live in the runtime layer; container lifecycle lives in the `sandbox/` package.

Roost does not build images. The image name is declared by the user in `devcontainer.json` (`image:` or `build.name`). Container lifecycle (create / start / exec / remove) uses docker directly.

## Layer Responsibilities

| Layer | Sandbox role |
|---|---|
| `state/` | Holds `LaunchPlan.Project`. Backend-agnostic |
| `runtime/` | `AgentLauncher` wraps `LaunchPlan` into `WrappedLaunch{Command, Cleanup}`. `SandboxDispatcher` resolves which launcher (direct / devcontainer) to use per project |
| `sandbox/` | `Manager[I any]` interface + backend implementations. Owns container lifecycle only; does not import driver / lib / runtime / tui |

`sandbox/` is tool-agnostic. It does not contain knowledge of any specific tool (e.g. Claude). Tool-specific host paths are declared by the user in `~/.roost/settings.toml`; they are never hardcoded in Go source.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Backend abstraction | `sandbox.Manager[I any]` + typed `Instance[I]` | Eliminates `any` and forced type-asserts. Backend-specific state (e.g. `*devcontainer.ContainerState`) is carried as the type parameter |
| Instance scope | Per-project; frames share via ref-count | Multiple frames in the same project share one container. Image is shared across projects at build time but containers remain per-project. `AcquireFrame` / `ReleaseFrame` manage the ref-count; `DestroyInstance` is called when the count reaches zero |
| Config resolution | User scope + project scope merged by `config.SandboxResolver` with mtime-based caching | Default direct mode; individual repos opt into devcontainer without daemon restart |
| Lifecycle and effect | detach → `EffDetachClient` only (container survives); shutdown → `EffReleaseFrameSandboxes` then `EffKillSession` (container destroyed); SIGINT (ctx cancel) → same as detach | Container lifetime decisions are expressed as state-layer effects, ordered in the event loop rather than a defer stack |
| Crash recovery | `PruneOrphans` at daemon startup enumerates containers labelled `roost-managed=1` and kills any whose project is not in sessions.json | Covers SIGKILL / panic paths where defer and effects never run. sessions.json is the ground truth |
| Image resolution | `LoadSpec` reads `image:` (top-level) then `build.name` from devcontainer.json | Roost does not build images. The user builds externally and declares the image name in devcontainer.json |

## Frame Lifecycle Interaction

**New frame**
`AgentLauncher.WrapLaunch` → `EnsureInstance` (singleflight-serialized per project) → `AcquireFrame` → the resulting `WrappedLaunch` is embedded in `EffSpawnTmuxWindow`

**Warm start**
`AdoptFrame` reclaims the still-running container and increments the ref-count for each restored frame

**Frame exit / shutdown**
`Cleanup` callback → `ReleaseFrame` → if count reaches zero → `DestroyInstance`

**Daemon startup**
`PruneOrphans` kills containers outside the known project set

## Devcontainer Backend

### Image Preparation

Roost does not build images. Build them outside roost by any means (e.g. `devcontainer build`, `docker build`) and declare the resulting tag in the project's `.devcontainer/devcontainer.json`:

```jsonc
// Option A — use a pre-existing image directly
{ "image": "myproject:dev" }

// Option B — Dockerfile-based project: set build.name (roost extension) as the tag
{
  "build": {
    "dockerfile": "Dockerfile",
    "name": "myproject:dev"  // roost reads this as the image to use
  }
}
```

Then build and tag the image yourself before starting a session:

```sh
devcontainer build --workspace-folder . --image-name myproject:dev
# or
docker build -t myproject:dev .
```

### Image Resolution

At session start, `LoadSpec` reads the image name from `.devcontainer/devcontainer.json`:

1. If `image:` is set → use it.
2. Else if `build.name` is set → use it.
3. Neither found → error: `devcontainer.json: image or build.name is required`.

The image must already exist locally (or be pullable by docker on first `docker create`).

### Container Identity

One long-lived container per project idles between frame activations. Frames join via `docker exec -it` rather than spawning a new container per frame.

- Container name: `roost-<sha256[:6] of project path>`
- Labels: `roost-managed=1`, `roost-project=<abs-path>`

Multiple projects can share the same image name (declare the same `image:` or `build.name`). Each project still gets its own container.

### Workspace Mount

The project directory is bind-mounted into the container:

```
<project-path>  →  /workspaces/<project-basename>
```

Override with `workspaceMount` in devcontainer.json. Additional mounts are declared in the `mounts` array of devcontainer.json. The working directory is set via `docker create -w` — no `WORKDIR` is needed in the Dockerfile.

### devcontainer.json Support

Keys parsed from devcontainer.json:

| Key | Effect |
|---|---|
| `image` | Image name to use (standard devcontainer.json field) |
| `build.name` | Image name to use when `build:` is present (roost extension; `--image-name` equivalent) |
| `mounts` | Extra bind-mounts passed as `--mount` / `-v` to `docker create` |
| `containerEnv` | Environment variables injected via `-e` |
| `containerUser` | User for `docker create -u` |
| `remoteUser` | User for `docker exec -u` (takes precedence over `containerUser`) |
| `workspaceFolder` | Container-side workspace path (default: `/workspaces/<basename>`) |
| `workspaceMount` | Replaces the default workspace bind-mount |
| `runArgs` | Extra args appended to `docker create` |
| `postCreateCommand` | Command (string or array) run once after the container is created |
| `preExecCommand` | Shell string (roost extension) run inside the container before each `docker exec` launch, with cwd already set to the exec workdir. Default: `mise trust 2>/dev/null \|\| true`. |

Variable substitution in string values: `${localWorkspaceFolder}`, `${localWorkspaceFolderBasename}`, `${containerWorkspaceFolder}`, `${localEnv:VAR}`.

All other keys (e.g. `features`, `customizations`) are ignored.

### Crash Recovery

`PruneOrphans` runs at daemon startup. It lists containers with label `roost-managed=1` and removes any whose `roost-project` label value is absent from sessions.json.

## Container IPC Endpoint

Each sandboxed project gets a dedicated Unix socket at `<dataDir>/run/<project-hash>/roost.sock` on the host. It is bind-mounted read-write into the container at `/opt/roost/run/roost.sock` (via the per-project run dir mount that already exists for credential helper files). The container agent reads `ROOST_SOCKET` (set to `/opt/roost/run/roost.sock`) to locate it.

**API surface**: only `hook-event` is implemented. Commands such as `event`, `surface.send_text`, `peer.send`, `shutdown`, and all others are structurally absent — no handler is registered, so they receive a protocol error without touching state.

**Authentication**: at frame spawn time, a 32-byte `crypto/rand` token is generated and injected into the container via `ROOST_SOCKET_TOKEN`. Every `hook-event` message carries the token; server-side Lookup resolves it to the owning frame ID. No client-supplied frame ID is trusted.

**Warm-start recovery**: tokens are persisted to `<dataDir>/warm/<frameID>.json` (atomic write, `0o600`). On daemon warm restart (containers survive, daemon replaces), `RecoverSandboxFrames` reads `warm/*.json` and re-registers each token for live frames so hook events continue to work immediately. The `warm/` directory is never bind-mounted into containers — a container process cannot read other frames' tokens.

**Cold start**: `warm/` is wiped before `LoadSnapshot` runs, ensuring stale tokens from a previous run do not survive a session-destroying restart.

## Host Mounts

Bind-mounts into containers are declared in devcontainer.json `mounts`:

```json
{
  "mounts": [
    "type=bind,source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,consistency=cached"
  ]
}
```

`sandbox/` does not have a global host-mounts config for arbitrary paths. Tool-specific paths belong in project or user devcontainer.json, keeping the sandbox layer tool-agnostic.

### Workspace mount target

roost automatically bind-mounts the project directory into the container. By default the container-side path mirrors the host path exactly (e.g. host `/home/u/proj` → container `/home/u/proj`), so editor path resolution and CLI commands work without translation.

If you need the workspace under a different prefix inside the container, set `host_path_mount_prefix` in `~/.roost/settings.toml`:

```toml
[sandbox.devcontainer]
host_path_mount_prefix = "/mnt"
# host /home/u/proj → container /mnt/home/u/proj
```

This setting is ignored when devcontainer.json explicitly specifies `workspaceFolder` or `workspaceMount`, which always take priority.

## Credential Proxy

When `[sandbox.proxy] enabled = true`, roost starts an in-process HTTP proxy backed by the `credproxy` library. The proxy listens on a Unix socket at `<dataDir>/run/credproxy.sock` on the host and is bind-mounted per-project into the container at `/opt/roost/run/credproxy.sock`. Its lifetime is tied to the roost process — no external daemon is needed.

### AWS SSO Credentials (multi-profile)

The proxy generates a synthetic `~/.aws/config` inside each container. Every profile entry uses `credential_process` to call back to the roost proxy via a small helper script (`/opt/roost/run/aws-creds.sh`). The config, the script, and the proxy socket are available under `/opt/roost/run`; no credentials are stored inside the container.

**Proxy route:** `/aws-credentials/<profile>` — returns `credential_process`-format JSON (`Version:1`, `AccessKeyId`, `SecretAccessKey`, `SessionToken`, `Expiration`).

Two container env vars carry the proxy coordinates:

| Container env var | Value |
|---|---|
| `ROOST_AWS_TOKEN` | Ephemeral bearer token (never written to disk) |
| `ROOST_PROXY_SOCK` | In-container Unix socket path (`/opt/roost/run/credproxy.sock`) |

**Per-project profile configuration** — in the project's `.roost/settings.toml`:

```toml
# <project-root>/.roost/settings.toml
[sandbox.proxy]
aws_profiles = ["default", "master", "general"]
```

Each name in the array appears as a `[profile <name>]` section in the synthetic `~/.aws/config`. Including `"default"` adds a `[default]` section so `aws` commands without `--profile` also work. Profiles not listed are not reachable from the container. If `aws_profiles` is absent or empty, no synthetic config is mounted and AWS proxy is a no-op for that project.

Enable the proxy in the global `~/.roost/settings.toml`:

```toml
[sandbox.proxy]
enabled = true
```

The provider calls `aws configure export-credentials --format process --profile <name>` on the host, then falls back to reading `~/.aws/sso/cache/*.json`. Run `aws sso login` on the host before starting containers. `~/.aws/sso/cache` is never bind-mounted — containers obtain short-lived credentials through the proxy only.

**Container image requirement:** `curl` must be available (present in standard base images; document explicitly for minimal images).

### Claude Code (Subscription)

Claude Code uses OAuth subscription credentials stored in `~/.claude/.credentials.json`. Container-side auth state is determined by the presence of this file; environment variables alone are not sufficient for the interactive UI to show a logged-in state.

Declare the bind-mount in devcontainer.json to expose the credential store:

```json
{
  "mounts": [
    "type=bind,source=${localEnv:HOME}/.claude,target=/home/vscode/.claude,consistency=cached",
    "type=bind,source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,consistency=cached"
  ]
}
```

This exposes the OAuth refresh token to the container. Accept this trade-off or restrict write access to specific subdirs if the threat model requires tighter isolation.

### gcloud CLI

Bind-mounting `~/.config/gcloud` exposes the OAuth refresh token. Instead, roost impersonates a service account on the host and passes only the short-lived SA-scoped access token (≤1h TTL) into the container. The container's `gcloud` calls are limited to what the SA's IAM bindings permit — it cannot act on projects outside those bindings.

**Per-project configuration** — in the project's `.roost/settings.toml`:

```toml
[sandbox.proxy.gcp]
service_account = "sa@proj.iam.gserviceaccount.com"   # required — SA to impersonate
projects        = ["proj-prod", "proj-staging"]        # required — GCP project IDs; first entry is the active default
account         = "user@example.com"                   # optional — host gcloud principal (defaults to current gcloud auth)
```

`service_account` and `projects` must both be set. Configuring `account` alone (without `service_account`) is rejected because it would produce a full-scope user token with no project restriction.

**Host prerequisites:**
- Run `gcloud auth login` (or use a service account key) on the host so gcloud can obtain tokens.
- Grant the host principal `roles/iam.serviceAccountTokenCreator` on the SA:
  ```sh
  gcloud iam service-accounts add-iam-policy-binding sa@proj.iam.gserviceaccount.com \
    --member="user:user@example.com" \
    --role="roles/iam.serviceAccountTokenCreator"
  ```

When configured, roost:

- Calls `gcloud auth print-access-token --account=<account> --impersonate-service-account=<sa>` every 50 minutes and writes the result to `<dataDir>/gcp/<hash>/access-token`.
- Generates a synthetic `CLOUDSDK_CONFIG` directory with one `configurations/config_<projectId>` per listed project. Each configuration sets `auth/access_token_file` so gcloud reads the token file on every invocation.
- Injects `CLOUDSDK_CONFIG=/opt/roost/run/gcloud-config` into the container environment.

Inside the container:

```sh
gcloud config list                                  # shows SA as active account, first project as active
gcloud --configuration=proj-staging projects list   # switch to another project
gcloud --project=proj-staging storage ls            # also works
```

`~/.config/gcloud` is never bind-mounted. `gcloud auth login` inside the container will fail — authenticate on the host before starting containers.

The first token refresh is synchronous at container start. If the impersonation call fails, a warning is logged and the container's `gcloud` calls will receive 401 until the host re-authenticates.

**Container image requirement:** `gcloud` must be installed in the image.

### SSH Agent (Ephemeral Keys)

Roost spawns an ephemeral `ssh-agent`, loads only the listed key files, and exposes the socket as `SSH_AUTH_SOCK` inside the container. The container can sign but never sees private key material. Direct forwarding of the host `$SSH_AUTH_SOCK` is not supported — it would expose all keys the host agent holds to container processes.

```toml
[sandbox.proxy.ssh_agent]
keys = ["~/.ssh/id_ed25519"]
```

The container receives `SSH_AUTH_SOCK` pointing to the ephemeral agent socket. No key material is written inside the container.

`SSH_AUTH_SOCK` is injected at both container-creation time (`docker create -e`) and at each frame launch (`docker exec -e`). The per-exec injection means that updating `keys` in the config takes effect on the next launch without recreating the container.

**Passphrase-protected keys** are not supported — `ssh-add` runs non-interactively, so passphrase prompts are skipped and a warning is logged.

**`known_hosts`**: roost does not mount `~/.ssh` into the container. If `ssh -T git@github.com` fails with a host-key verification error, add the host key inside the container image (e.g. via `postCreateCommand: "ssh-keyscan github.com >> ~/.ssh/known_hosts"`).

**Container reuse**: the `/opt/roost/run` directory is bind-mounted at container-creation time. If an existing container lacks this mount (created with an older roost version), remove it with `docker rm -f roost-$(echo -n <project-path> | sha256sum | head -c 12)` and relaunch.
