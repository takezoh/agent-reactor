# Brokers — Host Mediation and Policy Enforcement

Implementation deep dive of the three brokers that give an in-container agent **limited** access to host privileges, credentials, and MCP servers.

> This document covers the **"how"** (the implementation). The **"why"** — why this shape is a security boundary (long-lived secrets stay on the host; the container only ever sees short-lived tokens and brokered stdio) — is owned by [sandbox.md → Credential Proxy / Host-exec broker / MCP proxy](sandbox.md#credential-proxy). The two cross-link and avoid duplicating content.

| Package | Role | Enforcement |
|---|---|---|
| `platform/credproxy/` | In-process credential server. Bundles providers and issues a per-project bearer token | token ↔ projectID verification |
| `platform/hostexec/` | Runs allowlisted host binaries on behalf of the container (SCM_RIGHTS stdio forwarding) | deny-first allowlist (`Policy.Check`) |
| `platform/mcpproxy/` | Runs MCP servers on the host and relays JSON-RPC stdio | per-tool policy (`Policy.CheckTool`) |
| `platform/secretenv/` | Resolves secret references from an env-file on the host; injects into subprocess env | env-file path allowlist (`Gate.Check`, default-deny) |

## The big picture: credproxy bundles the providers

Both hostexec and mcpproxy implement the external `credproxy/container` `container.Provider` interface and register with the `credproxy.Runner` (`credproxy/credproxy.go:83`, `buildProviders`). This collapses credentials, host-exec, and MCP behind a single per-project Unix-socket server.

```mermaid
flowchart LR
    subgraph container["container (agent process)"]
        AGENT["codex / claude agent"]
        SHIM_GH["shim: gh / aws …<br/>(injected into PATH)"]
        MCPJSON[".mcp.json overlay<br/>(alias → roost shim)"]
    end

    subgraph host["host"]
        subgraph runner["credproxy.Runner (in-process server)"]
            TOK["per-project<br/>bearer token"]
            HEB["hostexec broker"]
            MPB["mcpproxy broker"]
            AWS["awssso / gcloudcli / sshagent<br/>provider"]
        end
        BIN["host binary<br/>(gh, aws, …)"]
        MCPSRV["MCP server<br/>(child process)"]
    end

    AGENT -->|"credential request<br/>(with token)"| TOK
    AGENT --> SHIM_GH -->|"unix socket<br/>+ SCM_RIGHTS"| HEB
    HEB -->|"Policy.Check passes"| BIN
    AGENT --> MCPJSON -->|"unix socket<br/>JSON-RPC"| MPB
    MPB -->|"CheckTool passes"| MCPSRV
    TOK -.->|verify projectID| AWS
```

## credproxy — per-project tokens and provider fan-out

`Runner` (`credproxy.go:36`) holds a `credproxylib.Server` and the provider set. `Start` (`credproxy.go:72`) brings up the server and `buildProviders` registers the awssso / gcloudcli / sshagent providers plus the hostexec and mcpproxy `SpecBuilder`s. The server runs on a child context so `Shutdown` deterministically reaps provider-managed processes such as ssh-agent.

**Token scoping** (`ProjectToken`, `credproxy.go:53`):

- Each project gets a 32-byte (256-bit) token from `crypto/rand` (`generateToken`, `credproxy.go:231`).
- `srv.AddAuthToken(token, projectID)` registers it, where `projectID = container.ProjectRunHash(projectPath)`.
- Repeat requests for the same project reuse the cached token (`tokens map[string]string`).

Container-side requests present this token, and the server matches token → projectID. **This is the boundary that blocks credential leakage across projects.**

## hostexec — proxied execution over SCM_RIGHTS

A container-side shim asks over a Unix socket to "run this binary with this argv"; the broker executes it on the host and returns the exit code. stdio is passed as **the actual file descriptors** via SCM_RIGHTS.

```mermaid
sequenceDiagram
    autonumber
    participant SH as container shim<br/>(/…/gh)
    participant BR as host broker
    participant PO as Policy
    participant BIN as host binary

    SH->>BR: connect (unix socket)
    Note over BR: peerPID(conn) via SO_PEERCRED<br/>(for the audit log)
    SH->>BR: SendRequest(Request{Binary, Args}, fds[3])
    Note over SH,BR: stdin/stdout/stderr fds passed<br/>as SCM_RIGHTS ancillary data
    BR->>BR: loadEntries()[req.Binary]
    alt unknown binary
        BR-->>SH: "unknown binary" → exit 127
    else registered
        BR->>PO: Check(argv)
        Note over PO: deny-first → allow-second<br/>(default-deny)
        alt rejected
            PO-->>BR: error
            BR-->>SH: reason on stderr + non-zero exit
        else allowed
            BR->>BIN: executeRequest (forwarded fds as stdio)
            BIN-->>BR: exit code
            BR-->>SH: Response{ExitCode}
        end
    end
```

**Policy** (`hostexec/policy.go`) evaluates deny → allow and rejects anything matching neither (default-deny, `Check` `policy.go:77`).

- Patterns are globs (`*` matches any string, including spaces). In `"gh pr *"` the space before `*` is **literal**, so it does not match `"gh preview"` (`CompilePolicy` `policy.go:49`).
- argv is reconstructed into a string by `shellJoin` before matching. This is **to align with Claude Code's Bash permission pattern semantics**; tokens that are not `isShellSafe` get single-quoted (`policy.go:95`).
- Leading `KEY=VALUE` env-assignment tokens are stripped by `trimEnvPrefix` before comparison, so `"ENV=x gh pr *"` and `"gh pr *"` are treated identically.

For audit, the caller PID is read via `SO_PEERCRED` (`peerPID`, `broker.go:81`) and logged together with `/proc/<pid>/comm` (`procComm`).

## mcpproxy — JSON-RPC relay and tool gating

MCP servers run as host-side child processes, with newline-delimited JSON-RPC 2.0 relayed bidirectionally between them and the container. `SpecBuilder` (`mcpproxy/provider.go:21`) is also a `container.Provider`.

```mermaid
sequenceDiagram
    autonumber
    participant A as container agent
    participant FR as forwardRequests
    participant PO as Policy
    participant SRV as MCP server (host)
    participant FRES as forwardResponses

    A->>FR: JSON-RPC line
    alt method == "tools/call"
        FR->>PO: CheckTool(params.name)
        alt rejected
            PO-->>FR: error
            FR-->>A: JSON-RPC error (-32601)<br/>(never reaches upstream)
        else allowed
            FR->>SRV: relay line
        end
    else other method
        FR->>SRV: relay unchanged
    end
    SRV-->>FRES: response line
    Note over FRES: tools/list results<br/>filtered by policy
    FRES-->>A: only permitted tools
```

- `forwardRequests` (`relay.go:54`) intercepts `tools/call` and gates it with `policy.CheckTool(name)` (`policy.go:43`, deny-first). A rejected call is answered directly to the container with an error and never forwarded upstream.
- `forwardResponses` (`relay.go:86`) filters `tools/list` results by policy so disallowed tools are never shown to the container.
- `ContainerSpec` (`provider.go:61`) takes the project's `.mcp.json` as a base and generates an overlay pointing each alias at the roost shim (`writeMCPJSON`), then returns two bind mounts (broker socket + `.mcp.json` overlay) plus the `ROOST_MCP_SOCK` env. With no servers configured it returns an empty Spec.

## secretenv — secret reference resolver

`secretenv` resolves opaque references (e.g. `op://vault/item/field`) in an env-file and injects the real values into a single subprocess environment. Invoked ad-hoc within a running session — not at container startup.

```mermaid
sequenceDiagram
    autonumber
    participant SH as container shim<br/>(credproxy on PATH)
    participant RB as roost-bridge secret-run
    participant BR as host broker
    participant GE as Gate
    participant CP as credproxy resolve<br/>(host binary)

    SH->>RB: exec with "run --env-file X -- cmd"
    RB->>BR: connect (unix socket) + Request{env_file_path}
    BR->>GE: Check(path)
    alt denied (not in allowlist)
        GE-->>BR: error
        BR-->>RB: Response{error}
        RB-->>SH: exit 1
    else allowed
        BR->>CP: credproxy resolve --env-file <path>
        CP-->>BR: stdout: {"env":{"K":"V",...}}
        BR-->>RB: Response{env: {K:V,...}}
        RB->>RB: merge into os.Environ()
        RB->>RB: syscall.Exec(cmd, args, merged_env)
    end
```

**Container shim** (`secretenv-shims/credproxy`): a shell script that calls `roost-bridge secret-run "$@"`. It is written to `<projRunDir>/secretenv-shims/` and prepended to container `PATH`, so existing scripts that call `credproxy run` work without modification.

**Broker** (`platform/secretenv/broker.go`): per-project Unix socket server (`<projRunDir>/secretenv.sock`, bind-mounted at `/opt/roost/run/secretenv.sock`). Each connection is handled in its own goroutine. `gate`, `credproxyBin`, and `hostPathMountPrefix` are guarded by a `sync.RWMutex` so concurrent request handling during config reload is race-free.

Before the gate is checked, the broker applies path canonicalization in order:
1. Reject non-absolute paths — the container shim performs `filepath.Abs` (container CWD); a relative path arriving at the broker means a shim bug or a direct socket attempt.
2. `filepath.Clean` — collapse any `.`/`..` sequences.
3. `containerToHost` — strip `HostPathMountPrefix` (boundary-safe: `/mnt` does not strip `/mnternal`). When the prefix is empty (bare-host or no devcontainer) the path is unchanged.

After canonicalization, `gate.Check(hostPath)` runs against the resulting **host absolute path**, and the same `hostPath` is forwarded to `credproxy resolve --env-file`. This is the **gate ≡ open invariant**: the path checked by the gate is byte-identical to the path credproxy opens. The broker must never expand `~`/`$VAR` (that would use the host daemon's env, not the container's, and break the invariant with any post-gate expansion).

**Gate** (`platform/secretenv/gate.go`): `filepath.Match` allowlist, default-deny. `*` matches within a single path segment only — does not cross `/`. Patterns are matched against the **host absolute path** (after `HostPathMountPrefix` is stripped). `allow` values in `settings.toml` should use host paths.

**Resolution**: entirely credproxy's concern. Hook backend (op/mise/vault) and its configuration (`~/.config/credproxy/config.toml`) are never known to roost. roost's role is gate + socket plumbing only. The `credproxy resolve` output contains only env-file-declared secrets; host environment variables are never included.

**Bare-host path**: the real `credproxy run` binary resolves locally with no gate. The shim/broker path is only active inside a devcontainer.

## Related documentation

- The full security model and sandbox lifecycle: [sandbox.md](sandbox.md)
- Where brokers are injected into the container along the launch path: [spawn-and-launch.md](spawn-and-launch.md) (`DevcontainerLauncher`)
- Capability sandboxing as an agent-control guardrail: [guardrails.md → capability](../guardrails.md#3-capability-sandboxing--what-a-running-agent-can-touch)
