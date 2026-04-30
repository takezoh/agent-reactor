package winexec

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	credproxy "github.com/takezoh/agent-roost/auth/credproxy"
	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/lib/wsl"
	credproxylib "github.com/takezoh/credproxy/pkg/credproxy"
)

// SpecBuilder implements credproxy.Provider for the WSL2 Windows exe broker.
// On non-WSL2 hosts, ContainerSpec always returns an empty Spec.
type SpecBuilder struct {
	ctx     context.Context
	runBase string // parent of per-project run dirs

	mu      sync.Mutex
	brokers map[string]*broker // projectPath → broker
}

// NewSpecBuilder creates a SpecBuilder. ctx cancellation shuts down all brokers.
func NewSpecBuilder(ctx context.Context, runBase string) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		runBase: runBase,
		brokers: map[string]*broker{},
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "winexec" }

// Init creates runBase. Idempotent.
func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.runBase, 0o700)
}

// Routes returns nil; this provider uses sockets, not HTTP routes.
func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project broker, writes shims, and
// returns an empty Spec — the shims appear in the container automatically via
// the run-dir bind mount. Returns an empty Spec on non-WSL2 hosts.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string, sb config.SandboxConfig) (credproxy.Spec, error) {
	if !wsl.IsWSL() {
		return credproxy.Spec{}, nil
	}
	cfg := sb.Proxy.WinExec
	if len(cfg.AllowedExes) == 0 {
		return credproxy.Spec{}, nil
	}

	projRunDir := filepath.Join(b.runBase, credproxy.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return credproxy.Spec{}, fmt.Errorf("winexec: mkdir run dir: %w", err)
	}

	if err := b.ensureBroker(projectPath, projRunDir, cfg); err != nil {
		return credproxy.Spec{}, err
	}

	if _, err := writeShims(projRunDir, cfg.AllowedExes); err != nil {
		return credproxy.Spec{}, fmt.Errorf("winexec: write shims: %w", err)
	}

	return credproxy.Spec{}, nil
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, cfg config.WinExecConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.brokers[projectPath]; ok {
		existing.cfg.Store(&cfg)
		return nil
	}

	sockPath := filepath.Join(projRunDir, "winexec.sock")
	_ = os.Remove(sockPath) // remove stale socket from prior run

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("winexec: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:     b.ctx,
		sock:    sockPath,
		ln:      ln,
		project: projectPath,
		onStop: func() {
			b.mu.Lock()
			delete(b.brokers, projectPath)
			b.mu.Unlock()
		},
	}
	br.cfg.Store(&cfg)
	b.brokers[projectPath] = br
	go br.serve()
	slog.Info("winexec: broker started", "project", projectPath, "sock", sockPath)
	return nil
}

func (b *SpecBuilder) watchShutdown(ctx context.Context) {
	<-ctx.Done()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, br := range b.brokers {
		br.ln.Close()
	}
}
