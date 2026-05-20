package agentlaunch

import "context"

// DirectDispatcher is the no-op Dispatcher: it passes the plan through unchanged.
// SockPath, when non-empty, is injected as ROOST_SOCKET so hook subprocesses
// can reach the daemon without relying on baked-in or fallback paths.
type DirectDispatcher struct {
	SockPath string
}

func (d DirectDispatcher) Wrap(_ context.Context, _ string, plan LaunchPlan) (WrappedLaunch, error) {
	merged := stripContainerOnlyEnv(plan.Env)
	if d.SockPath != "" {
		merged = cloneAndSet(merged, "ROOST_SOCKET", d.SockPath)
	}
	return WrappedLaunch{
		Command:  plan.Command,
		StartDir: plan.StartDir,
		Env:      merged,
	}, nil
}

func (DirectDispatcher) AdoptFrame(_ context.Context, _, _ string) (func(context.Context) error, []Mount, error) {
	return nil, nil, nil
}

func (DirectDispatcher) EnsureProject(_ context.Context, _ string) error { return nil }

func (DirectDispatcher) IsContainer(_ string) bool { return false }

func (DirectDispatcher) BeginColdStart() {}
func (DirectDispatcher) EndColdStart()   {}

// stripContainerOnlyEnv returns a copy of env without ROOST_SOCKET_TOKEN.
func stripContainerOnlyEnv(env map[string]string) map[string]string {
	out := cloneEnvMap(env, 0)
	delete(out, "ROOST_SOCKET_TOKEN")
	return out
}

func cloneAndSet(env map[string]string, key, value string) map[string]string {
	out := cloneEnvMap(env, 1)
	out[key] = value
	return out
}

func cloneEnvMap(src map[string]string, extra int) map[string]string {
	out := make(map[string]string, len(src)+extra)
	for k, v := range src {
		out[k] = v
	}
	return out
}
