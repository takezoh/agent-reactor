package runtime

import "github.com/takezoh/agent-roost/client/state"

// TestState exposes the runtime's state for test assertions. Must
// only be called from tests — production code accesses state
// exclusively through the event loop.
func (r *Runtime) TestState() state.State {
	return r.state
}
