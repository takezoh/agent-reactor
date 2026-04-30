package config

// MergeSandbox returns the effective SandboxConfig for a project by merging
// user-scope and project-scope settings. Merge rules:
//   - project == nil: return user unchanged
//   - Mode (scalar): project wins when non-empty
//   - Devcontainer.EnvScript (scalar): project wins when non-empty
//   - Devcontainer.HostPathMountPrefix (scalar): project wins when non-empty
//   - Devcontainer.ExtraCreateArgs (list): user + project concat
//   - Proxy.Enabled: user wins (proxy is process-wide)
//   - Proxy.SSHAgent.Keys: project replaces when non-empty
//   - Proxy.WinExec.Enabled: user wins
//   - Proxy.WinExec.AllowedExes: project replaces when non-empty
//   - Proxy.WinExec.Resolve: merged; project keys win on collision
func MergeSandbox(user SandboxConfig, project *SandboxConfig) SandboxConfig {
	if project == nil {
		return user
	}
	out := SandboxConfig{
		Mode: user.Mode,
		Devcontainer: DevcontainerConfig{
			ExtraCreateArgs:       appendSlice(user.Devcontainer.ExtraCreateArgs, project.Devcontainer.ExtraCreateArgs),
			EnvScript:             user.Devcontainer.EnvScript,
			AllowProjectEnvScript: user.Devcontainer.AllowProjectEnvScript,
			HostPathMountPrefix:   user.Devcontainer.HostPathMountPrefix,
		},
		Proxy: ProxyConfig{
			Enabled:     user.Proxy.Enabled,
			AWSProfiles: user.Proxy.AWSProfiles,
			GCP:         user.Proxy.GCP,
			SSHAgent:    user.Proxy.SSHAgent,
			WinExec:     mergeWinExec(user.Proxy.WinExec, project.Proxy.WinExec),
		},
	}
	if project.Mode != "" {
		out.Mode = project.Mode
	}
	if project.Devcontainer.EnvScript != "" {
		out.Devcontainer.EnvScript = project.Devcontainer.EnvScript
	}
	if project.Devcontainer.HostPathMountPrefix != "" {
		out.Devcontainer.HostPathMountPrefix = project.Devcontainer.HostPathMountPrefix
	}
	if len(project.Proxy.AWSProfiles) > 0 {
		out.Proxy.AWSProfiles = project.Proxy.AWSProfiles
	}
	if project.Proxy.GCP.ServiceAccount != "" || project.Proxy.GCP.Account != "" || len(project.Proxy.GCP.Projects) > 0 {
		out.Proxy.GCP = project.Proxy.GCP
	}
	if len(project.Proxy.SSHAgent.Keys) > 0 {
		out.Proxy.SSHAgent.Keys = project.Proxy.SSHAgent.Keys
	}
	return out
}

// mergeWinExec merges user and project WinExecConfig.
// Enabled: user wins. AllowedExes: project replaces when non-empty.
// Resolve: merged map; project keys overwrite user keys.
func mergeWinExec(user, project WinExecConfig) WinExecConfig {
	out := WinExecConfig{
		Enabled:     user.Enabled,
		AllowedExes: append([]string(nil), user.AllowedExes...),
		Resolve:     cloneStringMap(user.Resolve),
	}
	if len(project.AllowedExes) > 0 {
		out.AllowedExes = append([]string(nil), project.AllowedExes...)
	}
	for k, v := range project.Resolve {
		if out.Resolve == nil {
			out.Resolve = map[string]string{}
		}
		out.Resolve[k] = v
	}
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func appendSlice(base, extra []string) []string {
	if len(extra) == 0 {
		return append([]string(nil), base...)
	}
	return append(append([]string(nil), base...), extra...)
}
