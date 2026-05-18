package runtime

// cloneEnvMap makes a copy of env with extra capacity.
func cloneEnvMap(env map[string]string, extra int) map[string]string {
	if env == nil {
		return make(map[string]string, extra)
	}
	out := make(map[string]string, len(env)+extra)
	for k, v := range env {
		out[k] = v
	}
	return out
}
