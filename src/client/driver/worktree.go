package driver

import (
	"strings"

	"github.com/takezoh/agent-roost/client/state"
)

type worktreeRequest struct {
	Enabled bool
	Name    string
}

func parseWorktreeFlags(command string, flags ...string) (worktreeRequest, string) {
	parts := strings.Fields(command)
	out := make([]string, 0, len(parts))
	var req worktreeRequest
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		matched := false
		for _, flag := range flags {
			switch {
			case p == flag:
				req.Enabled = true
				matched = true
				if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "-") {
					req.Name = parts[i+1]
					i++
				}
			case strings.HasPrefix(p, flag+"="):
				req.Enabled = true
				req.Name = strings.TrimPrefix(p, flag+"=")
				matched = true
			}
			if matched {
				break
			}
		}
		if !matched {
			out = append(out, p)
		}
	}
	return req, strings.Join(out, " ")
}

func resolveWorktreeRequest(command string, options state.LaunchOptions, flags ...string) (worktreeRequest, string) {
	req, stripped := parseWorktreeFlags(command, flags...)
	if options.Worktree.Enabled {
		req.Enabled = true
	}
	return req, strings.TrimSpace(stripped)
}

func appendFlag(command, flag string, enabled bool) string {
	command = strings.TrimSpace(command)
	if !enabled || command == "" {
		return command
	}
	return strings.TrimSpace(command + " " + flag)
}

// CommonPrepareCreate strips worktree flags from command and sets
// LaunchOptions.Worktree.Enabled. The subsystem resolves the actual
// worktree directory during BindFrame; drivers only signal intent here.
func CommonPrepareCreate(c *CommonState, project, command string, options state.LaunchOptions, flags ...string) (state.CreateLaunch, error) {
	req, stripped := resolveWorktreeRequest(command, options, flags...)
	return state.CreateLaunch{
		Command:  strings.TrimSpace(stripped),
		StartDir: project,
		Options:  state.LaunchOptions{Worktree: state.WorktreeOption{Enabled: req.Enabled}},
	}, nil
}
