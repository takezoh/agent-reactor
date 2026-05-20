// Package credproxy provides an in-process credential proxy server that
// fans out to per-provider SpecBuilders (AWS SSO, GCP, SSH agent, hostexec,
// MCP proxy). Container paths are accepted as parameters so this package
// does not depend on platform/agentlaunch.
package credproxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/agent-roost/platform/hostexec"
	"github.com/takezoh/agent-roost/platform/mcpproxy"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
	"github.com/takezoh/credproxy/providers/awssso"
	"github.com/takezoh/credproxy/providers/gcloudcli"
	"github.com/takezoh/credproxy/providers/sshagent"
)

// Paths holds the container-side paths that providers need. Callers supply
// these from platform/agentlaunch constants so credproxy stays independent.
type Paths struct {
	RunDir  string
	BinPath string
	MCPSock string
}

// Runner holds an in-process credential proxy server and provider SpecBuilders.
type Runner struct {
	srv       *credproxylib.Server
	providers []container.Provider

	mu     sync.Mutex
	tokens map[string]string // projectPath → bearer token
}

// ProjectToken returns the bearer token for projectPath, generating and
// registering a new one if none exists. Safe for concurrent use.
func (r *Runner) ProjectToken(projectPath string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[projectPath]; ok {
		return t, nil
	}
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("credproxy: generate token for %s: %w", projectPath, err)
	}
	projectID := container.ProjectRunHash(projectPath)
	r.srv.AddAuthToken(token, projectID)
	r.tokens[projectPath] = token
	return token, nil
}

// Start starts an in-process credential proxy and registers all built-in
// providers. resolveSandbox provides per-project SandboxConfig; paths carries
// the container-side paths the providers need.
func Start(ctx context.Context, dataDir string, resolveSandbox func(string) config.SandboxConfig, paths Paths) (*Runner, error) {
	runBase := dataDir + "/run"
	if err := os.MkdirAll(runBase, 0o700); err != nil {
		return nil, fmt.Errorf("credproxy: create run dir: %w", err)
	}
	sockPath := filepath.Join(runBase, "credproxy.sock")

	runner := &Runner{tokens: make(map[string]string)}
	providers := buildProviders(ctx, runBase, sockPath, resolveSandbox, runner.ProjectToken, paths)

	var routes []credproxylib.Route
	for _, p := range providers {
		routes = append(routes, p.Routes()...)
	}

	srv, err := credproxylib.New(credproxylib.ServerConfig{
		ListenUnix: sockPath,
		Routes:     routes,
	})
	if err != nil {
		return nil, fmt.Errorf("credproxy: create server: %w", err)
	}

	for _, p := range providers {
		if r, ok := p.(container.PeriodicRegistrar); ok {
			r.RegisterPeriodic(srv)
		}
	}
	for _, p := range providers {
		if err := p.Init(); err != nil {
			return nil, fmt.Errorf("credproxy: provider %s init: %w", p.Name(), err)
		}
	}

	runner.srv = srv
	runner.providers = providers

	go func() {
		_ = srv.Run(ctx)
		_ = os.Remove(sockPath)
	}()

	return runner, nil
}

func buildProviders(
	ctx context.Context,
	runBase, sockPath string,
	resolveSandbox func(string) config.SandboxConfig,
	tokenFor func(string) (string, error),
	paths Paths,
) []container.Provider {
	cred := buildCredProviders(ctx, runBase, sockPath, resolveSandbox, tokenFor, paths)
	tool := buildToolProviders(ctx, runBase, resolveSandbox, paths)
	return append(cred, tool...)
}

func buildCredProviders(
	ctx context.Context,
	runBase, sockPath string,
	resolveSandbox func(string) config.SandboxConfig,
	tokenFor func(string) (string, error),
	paths Paths,
) []container.Provider {
	aws := awssso.NewSpecBuilder(
		awssso.Config{
			HostRunBase:       runBase,
			HostSockPath:      sockPath,
			ContainerRunDir:   paths.RunDir,
			ContainerSockPath: paths.RunDir + "/credproxy.sock",
		},
		func(p string) []string { return resolveSandbox(p).Proxy.AWSProfiles },
		tokenFor,
	)
	gcp := gcloudcli.NewSpecBuilder(
		ctx,
		gcloudcli.Config{RunBase: runBase, ContainerRunDir: paths.RunDir},
		func(p string) gcloudcli.GCPConfig {
			g := resolveSandbox(p).Proxy.GCP
			return gcloudcli.GCPConfig{Account: g.Account, ServiceAccount: g.ServiceAccount, Active: g.Active, Projects: g.Projects}
		},
	)
	ssh := sshagent.NewSpecBuilder(
		ctx,
		sshagent.Config{RunBase: runBase, ContainerRunDir: paths.RunDir},
		func(p string) []string { return resolveSandbox(p).Proxy.SSHAgent.Keys },
	)
	return []container.Provider{aws, gcp, ssh}
}

func buildToolProviders(
	ctx context.Context,
	runBase string,
	resolveSandbox func(string) config.SandboxConfig,
	paths Paths,
) []container.Provider {
	wsFolderFor := func(p string) string {
		dc := resolveSandbox(p).Devcontainer
		if dc.HostPathMountPrefix == "" {
			return p
		}
		return dc.HostPathMountPrefix + p
	}
	he := hostexec.NewSpecBuilder(ctx,
		hostexec.Config{RunBase: runBase, ContainerRunDir: paths.RunDir, ContainerBinPath: paths.BinPath, WorkspaceFolderFor: wsFolderFor},
		func(p string) config.HostExecConfig { return resolveSandbox(p).Proxy.HostExec },
	)
	mcp := mcpproxy.NewSpecBuilder(ctx,
		mcpproxy.Config{RunBase: runBase, ContainerSockPath: paths.MCPSock, ContainerBinPath: paths.BinPath, WorkspaceFolderFor: wsFolderFor},
		func(p string) config.MCPProxyConfig { return resolveSandbox(p).Proxy.MCPProxy },
	)
	return []container.Provider{he, mcp}
}

// ContainerSpec fans out to all providers and merges their Env, Mounts, and BridgeSpecs.
// Provider errors are logged as warnings and do not abort the other providers.
func (r *Runner) ContainerSpec(ctx context.Context, projectPath string) (container.Spec, error) {
	out := container.Spec{Env: map[string]string{}}
	for _, p := range r.providers {
		s, err := p.ContainerSpec(ctx, projectPath)
		if err != nil {
			slog.Warn("credproxy: provider failed", "provider", p.Name(), "project", projectPath, "err", err)
			continue
		}
		for k, v := range s.Env {
			out.Env[k] = v
		}
		out.Mounts = append(out.Mounts, s.Mounts...)
		out.BridgeSpecs = append(out.BridgeSpecs, s.BridgeSpecs...)
	}
	return out, nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
