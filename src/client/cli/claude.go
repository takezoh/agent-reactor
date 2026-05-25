package cli

import "github.com/takezoh/agent-roost/platform/lib/claude"

func init() {
	Register("claude", "Claude Code integration (setup)", claude.Run)
}
