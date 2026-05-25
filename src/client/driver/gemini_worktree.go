package driver

import "github.com/takezoh/agent-roost/client/state"

func (d GeminiDriver) PrepareCreate(s state.DriverState, _ state.SessionID, project, command string, options state.LaunchOptions) (state.DriverState, state.CreateLaunch, error) {
	gs, ok := s.(GeminiState)
	if !ok {
		gs = GeminiState{}
	}
	launch, err := CommonPrepareCreate(&gs.CommonState, project, command, options, "--worktree", "--workspace")
	return gs, launch, err
}
