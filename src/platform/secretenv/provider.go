package secretenv

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

const (
	shimDirName = "secretenv-shims"
	shimName    = "credproxy"
)

// Config holds path configuration for the SpecBuilder.
type Config struct {
	RunBase          string
	ContainerRunDir  string
	ContainerBinPath string
	// HostPathMountPrefixFor returns the HostPathMountPrefix for a given project
	// path. Used to translate container-absolute env-file paths to host-absolute
	// paths before gating. Nil means no translation (bare-host or no devcontainer).
	HostPathMountPrefixFor func(projectPath string) string
}

// SpecBuilder implements container.Provider for secret env-file resolution.
// It starts a per-project Unix socket broker that gates requests by env-file
// path allowlist and delegates resolution to the host `credproxy resolve` binary.
type SpecBuilder struct {
	ctx    context.Context
	cfg    Config
	cfgFor func(projectPath string) config.SecretEnvConfig

	mu      sync.Mutex
	brokers map[string]*broker
}

// NewSpecBuilder creates a SpecBuilder. cfgFor returns SecretEnvConfig per project.
func NewSpecBuilder(ctx context.Context, cfg Config, cfgFor func(string) config.SecretEnvConfig) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		cfg:     cfg,
		cfgFor:  cfgFor,
		brokers: make(map[string]*broker),
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "secretenv" }

func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.cfg.RunBase, 0o700)
}

func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project broker, writes the credproxy shim,
// and injects the shims directory into PATH.
// Returns an empty Spec when no allow patterns are configured for the project,
// or when the host credproxy binary cannot be found.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string) (container.Spec, error) {
	cfg := b.cfgFor(projectPath)
	if len(cfg.Allow) == 0 {
		return container.Spec{}, nil
	}

	credproxyBin, err := exec.LookPath("credproxy")
	if err != nil {
		slog.Warn("secretenv: credproxy not found on host PATH; feature inactive", "project", projectPath, "err", err)
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("secretenv: mkdir run dir: %w", err)
	}

	hostPrefix := ""
	if b.cfg.HostPathMountPrefixFor != nil {
		hostPrefix = b.cfg.HostPathMountPrefixFor(projectPath)
	}

	if err := b.ensureBroker(projectPath, projRunDir, cfg.Allow, credproxyBin, hostPrefix); err != nil {
		return container.Spec{}, err
	}

	shimDir := filepath.Join(projRunDir, shimDirName)
	if err := writeShim(shimDir, b.cfg.ContainerBinPath); err != nil {
		return container.Spec{}, fmt.Errorf("secretenv: write shim: %w", err)
	}

	containerShimDir := b.cfg.ContainerRunDir + "/" + shimDirName
	sockMount := filepath.Join(projRunDir, ContainerSockName)
	containerSock := b.cfg.ContainerRunDir + "/" + ContainerSockName
	return container.Spec{
		Env:    map[string]string{"PATH": containerShimDir + ":$PATH"},
		Mounts: []string{fmt.Sprintf("type=bind,source=%s,target=%s", sockMount, containerSock)},
	}, nil
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, allow []string, credproxyBin, hostPathMountPrefix string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if br, ok := b.brokers[projectPath]; ok {
		// Update gate, binary, and mount prefix on config change. setConfig uses
		// broker's own RWMutex so concurrent resolve() calls see a consistent update.
		br.setConfig(NewGate(allow), credproxyBin, hostPathMountPrefix)
		return nil
	}

	sockPath := filepath.Join(projRunDir, ContainerSockName)
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("secretenv: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:                 b.ctx,
		sock:                sockPath,
		ln:                  ln,
		project:             projectPath,
		gate:                NewGate(allow),
		credproxyBin:        credproxyBin,
		hostPathMountPrefix: hostPathMountPrefix,
		onStop: func() {
			b.mu.Lock()
			delete(b.brokers, projectPath)
			b.mu.Unlock()
		},
	}
	b.brokers[projectPath] = br
	go br.serve()
	return nil
}

func writeShim(shimDir, containerBinPath string) error {
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", shimDir, err)
	}
	path := filepath.Join(shimDir, shimName)
	content := fmt.Sprintf("#!/bin/sh\nexec %s secret-run \"$@\"\n", containerBinPath)
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		if info, serr := os.Stat(path); serr == nil && info.Mode().Perm() == 0o755 {
			return nil
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return os.Chmod(path, 0o755)
}

func (b *SpecBuilder) watchShutdown(ctx context.Context) {
	<-ctx.Done()
	b.mu.Lock()
	listeners := make([]net.Listener, 0, len(b.brokers))
	for _, br := range b.brokers {
		listeners = append(listeners, br.ln)
	}
	b.mu.Unlock()
	for _, ln := range listeners {
		ln.Close()
	}
}
