package hostexec

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/takezoh/agent-roost/platform/config"
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
	// WorkspaceFolderFor returns the container-side workspace path for a host project path.
	// Used to resolve overlay bind mount targets. When nil, the project path is used as-is.
	WorkspaceFolderFor func(projectPath string) string
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
	if len(cfg.Allow) == 0 && len(cfg.Overlay) == 0 {
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("hostexec: mkdir run dir: %w", err)
	}

	var globalEntries map[string]*entry
	var err error
	if len(cfg.Allow) > 0 {
		globalEntries, err = compileEntries(cfg)
		if err != nil {
			return container.Spec{}, err
		}
	}

	var mounts []string
	var overlayEntries map[string]*entry
	if len(cfg.Overlay) > 0 {
		mounts, overlayEntries, err = b.buildOverlayMounts(projRunDir, projectPath, cfg.Overlay)
		if err != nil {
			return container.Spec{}, err
		}
	}

	allEntries := mergeEntries(globalEntries, overlayEntries)
	if err := b.ensureBroker(projectPath, projRunDir, allEntries); err != nil {
		return container.Spec{}, err
	}

	aliases := extractAliases(cfg)
	if _, err := writeShims(projRunDir, b.cfg.ContainerBinPath, aliases); err != nil {
		return container.Spec{}, fmt.Errorf("hostexec: write shims: %w", err)
	}

	shimsDir := b.cfg.ContainerRunDir + "/" + ShimDirName
	return container.Spec{
		Env:    map[string]string{"PATH": shimsDir + ":$PATH"},
		Mounts: mounts,
	}, nil
}

func mergeEntries(global, overlay map[string]*entry) map[string]*entry {
	if len(overlay) == 0 {
		return global
	}
	out := make(map[string]*entry, len(global)+len(overlay))
	for k, v := range global {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func (b *SpecBuilder) buildOverlayMounts(projRunDir, projectPath string, overlays []config.OverlayEntry) ([]string, map[string]*entry, error) {
	wsDir := projectPath
	if b.cfg.WorkspaceFolderFor != nil {
		wsDir = b.cfg.WorkspaceFolderFor(projectPath)
	}

	overlayDir := filepath.Join(projRunDir, OverlayDirName)
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("hostexec: mkdir overlay dir: %w", err)
	}

	entries := make(map[string]*entry, len(overlays))
	var mounts []string
	for _, ov := range overlays {
		if ov.Target == "" {
			slog.Warn("hostexec: skipping empty overlay target")
			continue
		}
		var dst, hostExecPath string
		if filepath.IsAbs(ov.Target) {
			dst = filepath.Clean(ov.Target)
			hostExecPath = dst
		} else {
			if wsDir == "" {
				slog.Warn("hostexec: workspace folder unknown, skipping relative overlay", "path", ov.Target, "project", projectPath)
				continue
			}
			dst = filepath.Clean(filepath.Join(wsDir, ov.Target))
			hostExecPath = filepath.Clean(filepath.Join(projectPath, ov.Target))
		}
		alias := OverlayAlias(dst)
		if err := writeShim(overlayDir, b.cfg.ContainerBinPath, alias); err != nil {
			return nil, nil, fmt.Errorf("hostexec: write overlay shim %q: %w", ov.Target, err)
		}
		e, err := compileOverlayEntry(filepath.Base(dst), hostExecPath, ov.Allow, ov.Deny)
		if err != nil {
			return nil, nil, err
		}
		entries[alias] = e
		src := filepath.Join(overlayDir, alias)
		mounts = append(mounts, fmt.Sprintf("type=bind,source=%s,target=%s,readonly", src, dst))
	}
	return mounts, entries, nil
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, entries map[string]*entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if br, ok := b.brokers[projectPath]; ok {
		br.storeEntries(entries)
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
		onStop: func() {
			b.mu.Lock()
			delete(b.brokers, projectPath)
			b.mu.Unlock()
		},
	}
	br.storeEntries(entries)
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
