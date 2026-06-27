package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/takezoh/agent-reactor/client/lib/editor"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/features"
)

var editorLaunch = editor.Launch

// PaletteScope controls which tool set DefaultRegistry registers.
type PaletteScope int

const (
	// ScopeStandard registers project-independent tools (new-session,
	// create-project, stop-session, detach, shutdown).
	ScopeStandard PaletteScope = iota
	// ScopeProject registers project-dependent tools (new-session,
	// push-driver, fork-session) for operating within an active session.
	ScopeProject
)

// PaletteContext holds per-invocation state for gating tool visibility.
// Evaluated fresh each time the palette opens, unlike static feature flags.
type PaletteContext struct {
	Scope PaletteScope
	// ActiveOccupant=="frame" and an active session exists.
	MainHasDriverFrame bool
	// Root driver of the active session supports fork (e.g. claude).
	MainHasForkableDriver bool
	// PushCommands is the list from settings.toml [session].push_commands.
	// Each entry becomes a separate palette item when MainHasDriverFrame is true.
	PushCommands []string
	// HasActiveProject is true when an active session's project path is known.
	// Gates open-editor so the tool is hidden when no project is resolvable.
	HasActiveProject bool
}

// DefaultRegistry returns the built-in palette tool set.
// feats gates optional tools behind runtime feature flags (currently unused
// after the peers feature removal, but kept on the signature so callers do
// not need to be updated atomically with every flag churn).
// pctx gates per-invocation context-sensitive tools; zero value omits them.
func DefaultRegistry(feats features.Set, pctx ...PaletteContext) *Registry {
	_ = feats
	r := NewRegistry()
	var pc PaletteContext
	if len(pctx) > 0 {
		pc = pctx[0]
	}
	registerNewSession(r)
	if pc.Scope == ScopeStandard {
		registerStandardTools(r)
	}
	if pc.Scope == ScopeProject {
		registerProjectTools(r, pc)
	}
	if pc.HasActiveProject {
		registerOpenInEditor(r)
	}
	return r
}

func registerNewSession(r *Registry) {
	r.Register(Tool{
		Name:        "new-session",
		Description: "Create session",
		Params: []Param{
			{Name: "project", Options: func(ctx *ToolContext) []string { return ctx.Config.Projects }},
			{Name: "command", Options: func(ctx *ToolContext) []string { return ctx.Config.Commands }},
		},
		Run: func(ctx *ToolContext, args map[string]string) (*ToolInvocation, error) {
			opts := state.LaunchOptions{
				Worktree: state.WorktreeOption{Enabled: args["worktree"] == "on"},
			}
			sandbox := state.SandboxOverrideAuto
			if args["sandbox"] == "direct" {
				sandbox = state.SandboxOverrideHost
			}
			_, err := ctx.Client.CreateSession(args["project"], args["command"], sandbox, opts)
			return nil, err
		},
	})
}

func registerStandardTools(r *Registry) {
	r.Register(Tool{
		Name:        "create-project",
		Description: "Create new project dir and start session",
		Params: []Param{
			{Name: "root", Options: func(ctx *ToolContext) []string { return ctx.Config.ProjectRoots }},
			{Name: "name", Options: func(ctx *ToolContext) []string { return nil }},
		},
		Run: runCreateProject,
	})
	r.Register(Tool{
		Name:        "stop-session",
		Description: "Stop session",
		Params: []Param{
			{Name: "session_id", Options: func(ctx *ToolContext) []string { return nil }},
		},
		Run: func(ctx *ToolContext, args map[string]string) (*ToolInvocation, error) {
			return nil, ctx.Client.StopSession(args["session_id"])
		},
	})
	r.Register(Tool{
		Name:        "shutdown",
		Description: "Shutdown (discard sessions)",
		Run: func(ctx *ToolContext, args map[string]string) (*ToolInvocation, error) {
			return nil, ctx.Client.Shutdown()
		},
	})
}

func registerProjectTools(r *Registry, pc PaletteContext) {
	if pc.MainHasDriverFrame {
		for _, cmd := range pc.PushCommands {
			r.Register(Tool{
				Name:        "command: " + cmd,
				Description: "Push " + cmd + " onto active session",
				Run: func(ctx *ToolContext, _ map[string]string) (*ToolInvocation, error) {
					_, activeID, _, err := ctx.Client.ListSessions()
					if err != nil || activeID == "" {
						return nil, fmt.Errorf("no active session")
					}
					return nil, ctx.Client.PushDriver(activeID, cmd, nil)
				},
			})
		}
	}
	if pc.MainHasForkableDriver {
		r.Register(Tool{
			Name:        "fork-session",
			Description: "Fork active session (new branch)",
			Run: func(ctx *ToolContext, _ map[string]string) (*ToolInvocation, error) {
				_, activeID, _, err := ctx.Client.ListSessions()
				if err != nil || activeID == "" {
					return nil, fmt.Errorf("no active session")
				}
				_, err = ctx.Client.ForkSession(activeID)
				return nil, err
			},
		})
	}
}

func runCreateProject(ctx *ToolContext, args map[string]string) (*ToolInvocation, error) {
	path, err := makeProjectDir(ctx.Config.ProjectRoots, args["root"], args["name"])
	if err != nil {
		return nil, err
	}
	return &ToolInvocation{
		Name: "new-session",
		Args: map[string]string{"project": path},
	}, nil
}

// makeProjectDir creates a new project directory `name` under `root`.
// `root` must be one of the configured project_roots — palette
// free-form input fallback (when ProjectRoots is empty) must not be
// allowed to create directories at arbitrary paths. The name is
// validated to forbid path separators (`/`, `\`) and leading dots.
func makeProjectDir(roots []string, root, name string) (string, error) {
	if !slices.Contains(roots, root) {
		return "", fmt.Errorf("root must be one of configured project_roots")
	}
	if name == "" {
		return "", fmt.Errorf("name required")
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid project name: %q", name)
	}
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func registerOpenInEditor(r *Registry) {
	r.Register(Tool{
		Name:        "open-editor",
		Description: "Open project in editor",
		Run: func(ctx *ToolContext, _ map[string]string) (*ToolInvocation, error) {
			path := ctx.Config.ActiveProject
			if path == "" {
				return nil, fmt.Errorf("no active project")
			}
			target := editor.ResolveTarget(path, ctx.Config.Editor.Extensions)
			cmd := ctx.Config.Editor.Command
			if cmd == "" {
				cmd = "code"
			}
			return nil, editorLaunch(cmd, target)
		},
	})
}
