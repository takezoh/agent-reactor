package state

func init() {
	RegisterEvent[struct{}](EventShutdown, reduceShutdown)
}

// reduceShutdown persists state, acknowledges the caller, and releases sandbox
// resources. Daemon termination itself is driven by the signal handler in the
// process entrypoint — the reducer no longer asks a backend to tear down a TUI
// session, so the effect list ends after EffReleaseFrameSandboxes.
func reduceShutdown(s State, connID ConnID, reqID string, _ struct{}) (State, []Effect) {
	return s, []Effect{
		EffPersistSnapshot{},
		EffSendResponseSync{ConnID: connID, ReqID: reqID, Body: nil},
		EffReleaseFrameSandboxes{},
	}
}
