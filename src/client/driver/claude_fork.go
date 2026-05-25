package driver

import (
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/lib/claude/cli"
)

// ForkCommand returns the CLI invocation for forking the current Claude
// conversation into a new independent branch. It requires a known
// ClaudeSessionID; returns ok=false when one is not yet available.
//
// The command is built by resuming the existing conversation
// (--resume <id>) and requesting a new conversation ID (--fork-session).
// --worktree is stripped by cli.ResumeCommand since the forked session
// inherits the original StartDir and must not create a new worktree.
func (d ClaudeDriver) ForkCommand(s state.DriverState, baseCommand string) (string, bool) {
	cs, ok := s.(ClaudeState)
	if !ok || cs.ClaudeSessionID == "" || !isAlphanumHyphen(cs.ClaudeSessionID) {
		return "", false
	}
	return cli.ResumeCommand(baseCommand, cs.ClaudeSessionID) + " --fork-session", true
}
