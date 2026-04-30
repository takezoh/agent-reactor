package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/hostexec"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
	"github.com/takezoh/credproxy/providers/awssso"
	"github.com/takezoh/credproxy/providers/gcloudcli"
	"github.com/takezoh/credproxy/providers/sshagent"
)

// CredProxyRunner holds an in-process credential proxy server and a set of
// provider-specific SpecBuilders. Each provider encapsulates all knowledge of
// its credential system; this runner fans out ContainerSpec calls and merges results.
type CredProxyRunner struct {
	srv       *credproxylib.Server
	providers []container.Provider

	mu     sync.Mutex
	tokens map[string]string // projectPath → bearer token
}

// ProjectToken returns the bearer token for projectPath, generating and registering
// a new one if none exists yet. Safe for concurrent use.
func (r *CredProxyRunner) ProjectToken(projectPath string) (string, error) {
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

// StartCredProxy starts an in-process credential proxy and registers all built-in
// providers. resolveSandbox provides per-project SandboxConfig; it is called
// lazily inside each provider's ContainerSpec.
func StartCredProxy(ctx context.Context, dataDir string, resolveSandbox func(string) config.SandboxConfig) (*CredProxyRunner, error) {
	runBase := dataDir + "/run"
	sockPath := filepath.Join(runBase, "credproxy.sock")

	runner := &CredProxyRunner{tokens: make(map[string]string)}
	providers := buildProviders(ctx, dataDir, runBase, sockPath, resolveSandbox, runner.ProjectToken)

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
	dataDir, runBase, sockPath string,
	resolveSandbox func(string) config.SandboxConfig,
	tokenFor func(string) (string, error),
) []container.Provider {
	awsSpec := awssso.NewSpecBuilder(
		awssso.Config{
			HostRunBase:       runBase,
			HostSockPath:      sockPath,
			ContainerRunDir:   ContainerRunDir,
			ContainerSockPath: ContainerRunDir + "/credproxy.sock",
		},
		func(p string) []string { return resolveSandbox(p).Proxy.AWSProfiles },
		tokenFor,
	)
	gcpSpec := gcloudcli.NewSpecBuilder(
		ctx,
		gcloudcli.Config{GCPDir: dataDir + "/gcp", RunBase: runBase, ContainerRunDir: ContainerRunDir},
		func(p string) gcloudcli.GCPConfig {
			g := resolveSandbox(p).Proxy.GCP
			return gcloudcli.GCPConfig{Account: g.Account, ServiceAccount: g.ServiceAccount, Projects: g.Projects}
		},
	)
	sshSpec := sshagent.NewSpecBuilder(
		ctx,
		sshagent.Config{RunBase: runBase, ContainerRunDir: ContainerRunDir},
		func(p string) []string { return resolveSandbox(p).Proxy.SSHAgent.Keys },
	)
	hostExecSpec := hostexec.NewSpecBuilder(
		ctx,
		hostexec.Config{
			RunBase:          runBase,
			ContainerRunDir:  ContainerRunDir,
			ContainerBinPath: ContainerBinaryPath,
		},
		func(p string) config.HostExecConfig { return resolveSandbox(p).Proxy.HostExec },
	)
	return []container.Provider{awsSpec, gcpSpec, sshSpec, hostExecSpec}
}

// ContainerSpec fans out to all providers and merges their Env and Mounts.
// Provider errors are logged as warnings and do not abort the other providers.
func (r *CredProxyRunner) ContainerSpec(ctx context.Context, projectPath string) (container.Spec, error) {
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
