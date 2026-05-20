package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/credproxy/container"
	credproxylib "github.com/takezoh/credproxy/credproxy"
)

// SpecBuilder implements container.Provider for MCP server proxying.
// It starts a per-project Unix socket broker that runs allowlisted MCP servers
// on the host, relaying JSON-RPC stdio with tool-level policy enforcement.
type SpecBuilder struct {
	ctx    context.Context
	cfg    Config
	cfgFor func(projectPath string) config.MCPProxyConfig

	mu      sync.Mutex
	brokers map[string]*broker
}

// Config holds path configuration for the SpecBuilder.
type Config struct {
	RunBase            string              // parent of per-project run directories on the host
	ContainerSockPath  string              // mcp.sock path inside the container
	ContainerBinPath   string              // roost binary path inside the container
	WorkspaceFolderFor func(string) string // returns container-side workspace path for a project
}

// NewSpecBuilder creates a SpecBuilder.
func NewSpecBuilder(ctx context.Context, cfg Config, cfgFor func(string) config.MCPProxyConfig) *SpecBuilder {
	b := &SpecBuilder{
		ctx:     ctx,
		cfg:     cfg,
		cfgFor:  cfgFor,
		brokers: make(map[string]*broker),
	}
	go b.watchShutdown(ctx)
	return b
}

func (b *SpecBuilder) Name() string { return "mcpproxy" }

func (b *SpecBuilder) Init() error {
	return os.MkdirAll(b.cfg.RunBase, 0o700)
}

func (b *SpecBuilder) Routes() []credproxylib.Route { return nil }

// ContainerSpec starts (or reuses) the per-project MCP broker, generates a
// .mcp.json shim file, and returns mounts for both the broker socket and the
// .mcp.json overlay. Returns an empty Spec when no servers are configured.
func (b *SpecBuilder) ContainerSpec(_ context.Context, projectPath string) (container.Spec, error) {
	cfg := b.cfgFor(projectPath)
	if len(cfg.Servers) == 0 {
		return container.Spec{}, nil
	}

	projRunDir := filepath.Join(b.cfg.RunBase, container.ProjectRunHash(projectPath))
	if err := os.MkdirAll(projRunDir, 0o700); err != nil {
		return container.Spec{}, fmt.Errorf("mcpproxy: mkdir run dir: %w", err)
	}

	if err := b.ensureBroker(projectPath, projRunDir, cfg); err != nil {
		return container.Spec{}, err
	}

	mcpJSONHostPath := filepath.Join(projRunDir, "mcp.json")
	if err := writeMCPJSON(mcpJSONHostPath, projectPath+"/.mcp.json", cfg.Servers, b.cfg.ContainerBinPath); err != nil {
		return container.Spec{}, fmt.Errorf("mcpproxy: write mcp.json: %w", err)
	}

	sockHostPath := filepath.Join(projRunDir, filepath.Base(b.cfg.ContainerSockPath))

	wsDir := projectPath
	if b.cfg.WorkspaceFolderFor != nil {
		wsDir = b.cfg.WorkspaceFolderFor(projectPath)
	}

	return container.Spec{
		Env: map[string]string{"ROOST_MCP_SOCK": b.cfg.ContainerSockPath},
		Mounts: []string{
			fmt.Sprintf("type=bind,source=%s,target=%s", sockHostPath, b.cfg.ContainerSockPath),
			fmt.Sprintf("type=bind,source=%s,target=%s,readonly", mcpJSONHostPath, wsDir+"/.mcp.json"),
		},
	}, nil
}

// writeMCPJSON writes a merged .mcp.json to path.
// It reads projectMCPJSON (the project's .mcp.json) as a base, then overlays
// shim entries for each alias so the broker aliases shadow any direct entries.
// Entries not in servers pass through unchanged.
// Skips the write when the file already contains identical content.
func writeMCPJSON(path, projectMCPJSON string, servers map[string]config.MCPProxyServer, containerBin string) error {
	// Start with the project's existing mcpServers entries (arbitrary JSON preserved).
	merged := make(map[string]json.RawMessage)
	if raw, err := os.ReadFile(projectMCPJSON); err == nil {
		var doc struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if json.Unmarshal(raw, &doc) == nil {
			for k, v := range doc.MCPServers {
				merged[k] = v
			}
		}
	}

	// Override broker-managed aliases with shim entries.
	type mcpEntry struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	for alias := range servers {
		shim, err := json.Marshal(mcpEntry{Type: "stdio", Command: containerBin, Args: []string{"mcp-exec", alias}})
		if err != nil {
			return err
		}
		merged[alias] = shim
	}

	data, err := json.MarshalIndent(map[string]any{"mcpServers": merged}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o600)
}

func (b *SpecBuilder) ensureBroker(projectPath, projRunDir string, cfg config.MCPProxyConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.brokers[projectPath]; ok {
		return nil
	}

	servers, err := compileServers(cfg)
	if err != nil {
		return err
	}

	sockPath := filepath.Join(projRunDir, filepath.Base(b.cfg.ContainerSockPath))
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("mcpproxy: listen %s: %w", sockPath, err)
	}

	br := &broker{
		ctx:     b.ctx,
		sock:    sockPath,
		ln:      ln,
		project: projectPath,
		servers: servers,
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

func compileServers(cfg config.MCPProxyConfig) (map[string]*serverEntry, error) {
	m := make(map[string]*serverEntry, len(cfg.Servers))
	for alias, s := range cfg.Servers {
		e, err := compileServer(alias, s.Command, s.Args, s.Env, s.Allow, s.Deny)
		if err != nil {
			return nil, err
		}
		m[alias] = e
	}
	return m, nil
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
