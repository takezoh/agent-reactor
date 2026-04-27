# Sandbox Backends

## Purpose

Sandbox backends isolate agent processes per project — each project directory runs inside its own container with scoped filesystem, network, and capability restrictions.

The state layer knows only `LaunchPlan.Project` (the project path); it has no awareness of which backend is active. Backend selection and command wrapping live in the runtime layer; container lifecycle lives in the `sandbox/` package.

Image build uses `@devcontainers/cli` (`devcontainer build`); container lifecycle (create / start / exec / remove) uses docker directly.

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
| Image resolution | `ResolveImage` checks `roost-proj-<hash>` then `roost-user` via `docker image inspect` at session start | No fallback to arbitrary images; explicit `roost build` is required. Build-time and run-time scope selection are cleanly separated |

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

### Image Build

Images are built explicitly with `roost build`, not on-demand at session start:

```sh
roost build <project-path>   # builds roost-proj-<hash>:latest for a specific project
roost build --user           # builds roost-user:latest from ~/.devcontainer/
```

`roost build` calls `devcontainer build --config <materializeDir>/devcontainer.json --image-name <image>`.

A project that has no `.devcontainer/devcontainer.json` uses the user-scope image instead.

### Image Resolution

At session start, `ResolveImage` selects the image for a project:

1. Check `roost-proj-<hash>:latest` via `docker image inspect`. If it exists, use it with the project's own materialize dir.
2. Check `roost-user:latest`. If it exists, use it with `~/.roost/user/devcontainer/`.
3. Neither found → return an error directing the user to run `roost build`.

### Container Identity

One long-lived container per project idles between frame activations. Frames join via `docker exec -it` rather than spawning a new container per frame.

- Container name: `roost-<sha256[:6] of project path>`
- Labels: `roost-managed=1`, `roost-project=<abs-path>`

Multiple projects sharing `roost-user:latest` each get their own container; image sharing happens at build time only.

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
| `mounts` | Extra bind-mounts passed as `--mount` / `-v` to `docker create` |
| `containerEnv` | Environment variables injected via `-e` |
| `containerUser` | User for `docker create -u` |
| `remoteUser` | User for `docker exec -u` (takes precedence over `containerUser`) |
| `workspaceFolder` | Container-side workspace path (default: `/workspaces/<basename>`) |
| `workspaceMount` | Replaces the default workspace bind-mount |
| `runArgs` | Extra args appended to `docker create` |
| `postCreateCommand` | Command (string or array) run once after the container is created |

Variable substitution in string values: `${localWorkspaceFolder}`, `${localWorkspaceFolderBasename}`, `${containerWorkspaceFolder}`, `${localEnv:VAR}`.

All other keys (e.g. `features`, `customizations`) are ignored.

### Crash Recovery

`PruneOrphans` runs at daemon startup. It lists containers with label `roost-managed=1` and removes any whose `roost-project` label value is absent from sessions.json.

## Host Mounts

Bind-mounts into containers are declared in devcontainer.json `mounts`:

```json
{
  "mounts": [
    "type=bind,source=${localEnv:HOME}/.claude.json,target=/home/vscode/.claude.json,consistency=cached"
  ]
}
```

`sandbox/` does not have a global host-mounts config. Tool-specific paths belong in project or user devcontainer.json, keeping the sandbox layer tool-agnostic.

## Credential Proxy

When `[sandbox.proxy] enabled = true`, roost starts an in-process HTTP forward proxy backed by the `credproxy` library. The proxy listens on an ephemeral loopback port (`127.0.0.1:0`) and is reached from containers via `host.docker.internal`. Its lifetime is tied to the roost process — no external daemon is needed.

### AWS SSO Credentials (multi-profile)

The proxy generates a synthetic `~/.aws/config` inside each container. Every profile entry uses `credential_process` to call back to the roost proxy via a small helper script (`/opt/roost/aws-creds`). Both the config and the script are bind-mounted read-only; no credentials are stored inside the container.

**Proxy route:** `/aws-credentials/<profile>` — returns `credential_process`-format JSON (`Version:1`, `AccessKeyId`, `SecretAccessKey`, `SessionToken`, `Expiration`).

Two container env vars carry the proxy coordinates:

| Container env var | Value |
|---|---|
| `ROOST_AWS_TOKEN` | Ephemeral bearer token (never written to disk) |
| `ROOST_PROXY_PORT` | TCP port of the in-process proxy |

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

Bind-mounting `~/.config/gcloud` exposes the OAuth refresh token. Instead, roost can generate a synthetic `CLOUDSDK_CONFIG` directory and refresh a short-lived access token on the host, so containers receive only the access token (≤1h TTL).

**Per-project configuration** — in the project's `.roost/settings.toml`:

```toml
[sandbox.proxy.gcp]
account  = "user@example.com"            # from `gcloud auth list`
projects = ["proj-prod", "proj-staging"] # GCP project IDs; first entry is the active default
```

When `account` and `projects` are set, roost:

- Calls `gcloud auth print-access-token --account=<account>` on the host every 50 minutes and writes the result to `<dataDir>/gcp/<hash>/access-token`.
- Generates a synthetic `CLOUDSDK_CONFIG` directory with one `configurations/config_<projectId>` per listed project. Each configuration sets `auth/access_token_file` to `/opt/roost/gcp-token`.
- Bind-mounts both files read-only into the container.
- Injects `CLOUDSDK_CONFIG=/opt/roost/gcloud-config` into the container environment.

Inside the container:

```sh
gcloud config list                               # shows active project (first listed)
gcloud --configuration=proj-staging projects list  # switch to another project
gcloud --project=proj-staging storage ls          # also works
```

`~/.config/gcloud` is never bind-mounted. `gcloud auth login` inside the container will fail (read-only mount) — authenticate on the host before starting containers.

The first token refresh is synchronous at container start. If `gcloud auth print-access-token` fails (not logged in), a warning is logged and the container's `gcloud` calls will receive 401 until the host re-authenticates.

**Container image requirement:** `gcloud` must be installed in the image.

### SSH Agent Forwarding

When `ssh_agent.forward = true`, roost forwards SSH credentials into the container so tools like `git` can authenticate over SSH without exposing private keys.

```toml
[sandbox.proxy.ssh_agent]
forward = true          # forward host $SSH_AUTH_SOCK directly
# keys = ["~/.ssh/id_ed25519"]  # alternatively, spawn ephemeral agent with specific keys only
```

The container receives `SSH_AUTH_SOCK` pointing to the forwarded socket. No key material is written inside the container.
