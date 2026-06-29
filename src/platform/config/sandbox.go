package config

import "fmt"

// SandboxConfig controls how agent processes are isolated.
// mode = "direct" runs agents with no extra sandboxing (default).
// mode = "devcontainer" runs each project via @devcontainers/cli.
// isolation = "project" (default) gives each project its own container.
// isolation = "shared" mounts all configured project roots and paths into one shared container.
type SandboxConfig struct {
	Mode         string             `toml:"mode"`
	Isolation    string             `toml:"isolation"`
	Devcontainer DevcontainerConfig `toml:"devcontainer"`
	Proxy        ProxyConfig        `toml:"proxy"`
}

// IsSandboxed reports whether the sandbox mode is an active isolation backend.
// Both "" and "direct" mean no sandboxing.
func (s SandboxConfig) IsSandboxed() bool {
	return s.Mode != "" && s.Mode != "direct"
}

// Validate rejects unknown sandbox modes/isolation values and deprecated proxy config at startup.
func (s SandboxConfig) Validate() error {
	switch s.Mode {
	case "", "direct", "devcontainer":
	default:
		return fmt.Errorf("sandbox.mode=%q is unknown; valid values: direct, devcontainer", s.Mode)
	}
	switch s.Isolation {
	case "", "project", "shared":
	default:
		return fmt.Errorf("sandbox.isolation=%q is unknown; valid values: project, shared", s.Isolation)
	}
	return s.Proxy.GCP.Validate()
}

// DevcontainerConfig holds settings for the devcontainer sandbox mode.
type DevcontainerConfig struct {
	// Path, when non-empty, is the devcontainer.json directory to use instead of
	// auto-discovery (<project>/.devcontainer → ~/.devcontainer). ~ is expanded.
	// At user scope this is the shared container's devcontainer directory.
	// At project scope this overrides the project's devcontainer path and implies
	// project-level isolation even when the user has set isolation=shared.
	Path string `toml:"path"`

	// NamePrefix is the docker container name & label prefix used by this daemon.
	// Empty falls back to the default "reactor" prefix. Daemons running side-by-side
	// against the same docker host (e.g. a primary daemon and a dev gateway
	// started via scripts/run-dev.sh) MUST configure a distinct prefix here; the
	// prefix scopes both ContainerName ("<prefix>-shared" / "<prefix>-<projectHash>")
	// AND the docker ps --filter label keys, so peer daemons become invisible to
	// each other and the mount-hash recreate path no longer rm's foreign containers.
	NamePrefix string `toml:"name_prefix"`

	// ExtraCreateArgs are appended verbatim to "docker create".
	ExtraCreateArgs []string `toml:"extra_create_args"`

	// EnvScript is a path to a script that prints KEY=VALUE lines (dotenv format)
	// to stdout. It receives the project path as its first argument.
	EnvScript string `toml:"env_script"`

	// AllowProjectEnvScript lists project paths whose project-scope env_script
	// is permitted to run.
	AllowProjectEnvScript []string `toml:"allow_project_env_script"`

	// HostPathMountPrefix, when non-empty, makes the auto-mounted project workspace
	// appear at "<prefix><host-path>" inside the container instead of the default
	// "<host-path>" (host-mirroring). Has no effect if devcontainer.json explicitly
	// sets workspaceFolder or workspaceMount. Must be an absolute path or empty.
	HostPathMountPrefix string `toml:"host_path_mount_prefix"`
}

// ProxyConfig holds the in-process credential proxy provider settings.
type ProxyConfig struct {
	AWSProfiles []string        `toml:"aws_profiles"`
	GCP         GCPConfig       `toml:"gcp"`
	SSHAgent    SSHAgentConfig  `toml:"ssh_agent"`
	HostExec    HostExecConfig  `toml:"host_exec"`
	MCPProxy    MCPProxyConfig  `toml:"mcp_proxy"`
	SecretEnv   SecretEnvConfig `toml:"secret_env"`
}

// SecretEnvConfig configures the host-gated secret reference resolver.
// When configured, a per-project Unix socket broker gates container requests
// by env-file path and delegates resolution to the host `credproxy resolve`
// binary. The hook backend (op/mise/vault) is configured entirely in
// credproxy's own config (~/.config/credproxy/config.toml).
//
// This is an intentional exception to the "long-lived secrets stay on host"
// invariant: the resolved value enters the subprocess env for its lifetime only.
//
// Bare-host users run the real `credproxy run` binary (no gate, no broker).
// Container users run the client-provided `credproxy` shim, which brokers to
// this host-side resolver.
type SecretEnvConfig struct {
	// Allow is the list of env-file path patterns the container may request.
	// Uses filepath.Match glob syntax; default-deny when empty.
	// '*' matches within a single path segment only — it does NOT cross '/'.
	// Example: "/workspace/*.env" matches "/workspace/dev.env" but NOT "/workspace/sub/dev.env".
	Allow []string `toml:"allow"`
}

// MCPProxyConfig lists MCP servers to run on the host with stdio proxied into the container.
type MCPProxyConfig struct {
	Servers map[string]MCPProxyServer `toml:"servers"`
}

// MCPProxyServer defines one MCP server to proxy.
type MCPProxyServer struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env"`
	Allow   []string          `toml:"allow"`
	Deny    []string          `toml:"deny"`
}

// HostExecConfig configures host binaries exposed to containers via the host-exec broker.
type HostExecConfig struct {
	Allow   []string       `toml:"allow"`
	Deny    []string       `toml:"deny"`
	Overlay []OverlayEntry `toml:"overlay"`
}

// OverlayEntry bind-mounts a host-exec shim at a specific container path.
type OverlayEntry struct {
	Target string   `toml:"target"`
	Allow  []string `toml:"allow"`
	Deny   []string `toml:"deny"`
}

// SSHAgentConfig controls SSH agent injection into containers.
type SSHAgentConfig struct {
	Keys []string `toml:"keys"`
}

// GCPConfig holds per-project gcloud CLI credential settings.
type GCPConfig struct {
	ServiceAccount    string   `toml:"service_account"`
	Account           string   `toml:"account"`
	Active            string   `toml:"active"`
	Projects          []string `toml:"projects"`
	EnableUserAccount bool     `toml:"enable_user_account"`
}

// Validate returns an error if the config uses the removed enable_user_account field.
func (g GCPConfig) Validate() error {
	if g.EnableUserAccount {
		return fmt.Errorf("sandbox.proxy.gcp: enable_user_account has been removed; delete it and use account + active instead")
	}
	return nil
}

// ProjectsConfig holds project root directories and explicit project paths.
type ProjectsConfig struct {
	ProjectRoots []string `toml:"project_roots"`
	ProjectPaths []string `toml:"project_paths"`
}

// ListProjects returns all project directories.
func (p *ProjectsConfig) ListProjects() []string {
	return listProjectsFrom(p.ProjectRoots, p.ProjectPaths)
}
