package driver

import "github.com/takezoh/agent-roost/state"

func (d ClaudeDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	cs, ok := s.(ClaudeState)
	if !ok {
		cs = ClaudeState{}
	}
	launch, err := CommonPrepareCreate(&cs.CommonState, project, command, options, "--worktree")
	return cs, launch, err
}
