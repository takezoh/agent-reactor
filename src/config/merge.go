package config

// MergeSandbox returns the effective SandboxConfig for a project by merging
// user-scope and project-scope settings. Merge rules:
//   - project == nil: return user unchanged
//   - Mode (scalar): project wins when non-empty
//   - Devcontainer.EnvScript (scalar): project wins when non-empty
//   - Devcontainer.ExtraCreateArgs (list): user + project concat
//   - Proxy.Enabled: user wins (proxy is process-wide)
//   - Proxy.SSHAgent.Keys: project replaces when non-empty
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
		},
		Proxy: ProxyConfig{
			Enabled:     user.Proxy.Enabled,
			AWSProfiles: user.Proxy.AWSProfiles,
			GCP:         user.Proxy.GCP,
			SSHAgent:    user.Proxy.SSHAgent,
		},
	}
	if project.Mode != "" {
		out.Mode = project.Mode
	}
	if project.Devcontainer.EnvScript != "" {
		out.Devcontainer.EnvScript = project.Devcontainer.EnvScript
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

func appendSlice(base, extra []string) []string {
	if len(extra) == 0 {
		return append([]string(nil), base...)
	}
	return append(append([]string(nil), base...), extra...)
}
