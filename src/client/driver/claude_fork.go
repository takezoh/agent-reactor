package driver

import (
	"time"

	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/lib/claude/cli"
)

// ForkCommand returns the CLI invocation for forking the current Claude
// conversation into a new independent branch. It requires a known
// ClaudeSessionID; returns ok=false when one is not yet available.
func (d ClaudeDriver) ForkCommand(s state.DriverState, baseCommand string) (string, bool) {
	cs, ok := s.(ClaudeState)
	if !ok || cs.ClaudeSessionID == "" || !isAlphanumHyphen(cs.ClaudeSessionID) {
		return "", false
	}
	return cli.ForkCommand(baseCommand, cs.ClaudeSessionID), true
}

// ForkChildState creates the initial driver state for a forked session.
// It records the parent's ClaudeSessionID as ForkParentID so the child
// can reject the parent id when it arrives in the first hook after
// `--fork-session` launch, preventing identity poisoning.
func (d ClaudeDriver) ForkChildState(parent state.DriverState, now time.Time) state.DriverState {
	cs := d.NewState(now).(ClaudeState)
	if parentCS, ok := parent.(ClaudeState); ok && parentCS.ClaudeSessionID != "" {
		cs.ForkParentID = parentCS.ClaudeSessionID
	}
	return cs
}
