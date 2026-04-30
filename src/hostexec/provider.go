package hostexec

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

// Config holds path configuration for the hostexec SpecBuilder.
type Config struct {
	// RunBase is the parent of per-project run directories on the host.
	RunBase string
	// ContainerRunDir is the mount target inside the container (e.g. /opt/roost/run).
	ContainerRunDir string
	// ContainerBinPath is the roost binary path inside the container.
	ContainerBinPath string
}

// SpecBuilder implements container.Provider for host-exec proxying.
// It starts a per-project Unix socket broker that runs allowlisted host binaries
// on behalf of container processes using SCM_RIGHTS stdio forwarding.
type SpecBuilder struct {
	ctx    context.Context
	cfg    Config
	cfgFor func(projectPath string) config.HostExecConfig

	mu      sync.Mutex
	brokers map[string]*broker
}

// NewSpecBuilder creates a SpecBuilder.
// cfgFor returns the HostExecConfig for a given project path.
// ctx cancellation shuts down all brokers.
func NewSpecBuilder(ctx context.Context, cfg Config, cfgFor func(string) config.HostExecConfig) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		cfg:     cfg,
		cfgFor:  cfgFor,
		brokers: make(map[string]*broker),
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "hostexec" }

// Init creates RunBase.
func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.cfg.RunBase, 0o700)
}

// Routes returns nil; hostexec uses sockets, not HTTP routes.
func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project broker, writes shims, and
// injects the PATH entry for the shims directory.
// Returns an empty Spec when no HostExecConfig entries are configured for projectPath.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string) (container.Spec, error) {
	cfg := b.cfgFor(projectPath)
	if len(cfg.Allow) == 0 {
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("hostexec: mkdir run dir: %w", err)
	}

	if err := b.ensureBroker(projectPath, projRunDir, cfg); err != nil {
		return container.Spec{}, err
	}

	aliases := extractAliases(cfg)
	if _, err := writeShims(projRunDir, b.cfg.ContainerBinPath, aliases); err != nil {
		return container.Spec{}, fmt.Errorf("hostexec: write shims: %w", err)
	}

	shimsDir := b.cfg.ContainerRunDir + "/" + ShimDirName
	return container.Spec{
		Env: map[string]string{"PATH": shimsDir + ":$PATH"},
	}, nil
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, cfg config.HostExecConfig) error {
	entries, err := compileEntries(cfg)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.brokers[projectPath]; ok {
		return nil
	}

	sockPath := filepath.Join(projRunDir, "hostexec.sock")
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("hostexec: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:     b.ctx,
		sock:    sockPath,
		ln:      ln,
		project: projectPath,
		entries: entries,
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

func extractAliases(cfg config.HostExecConfig) []string {
	seen := map[string]struct{}{}
	for _, pat := range cfg.Allow {
		if fields := skipEnvAssignments(strings.Fields(pat)); len(fields) > 0 {
			seen[fields[0]] = struct{}{}
		}
	}
	aliases := make([]string, 0, len(seen))
	for name := range seen {
		aliases = append(aliases, name)
	}
	return aliases
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
