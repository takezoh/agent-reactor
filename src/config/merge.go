package config

// MergeSandbox returns the effective SandboxConfig for a project by merging
// user-scope and project-scope settings. Merge rules:
//   - project == nil: return user unchanged
//   - Mode (scalar): project wins when non-empty
//   - Devcontainer.EnvScript (scalar): project wins when non-empty
//   - Devcontainer.HostPathMountPrefix (scalar): project wins when non-empty
//   - Devcontainer.ExtraCreateArgs (list): user + project concat
//   - Proxy.SSHAgent.Keys: project replaces when non-empty
//   - Proxy.HostExec.Allow/Deny: user + project concat
//   - Proxy.HostExec.Overlay: user + project concat, duplicates removed
//   - Proxy.MCPProxy.Servers: user + project concat; project server overrides user on same alias
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
			AWSProfiles: user.Proxy.AWSProfiles,
			GCP:         user.Proxy.GCP,
			SSHAgent:    user.Proxy.SSHAgent,
			HostExec: HostExecConfig{
				Allow:   appendSlice(user.Proxy.HostExec.Allow, project.Proxy.HostExec.Allow),
				Deny:    appendSlice(user.Proxy.HostExec.Deny, project.Proxy.HostExec.Deny),
				Overlay: mergeOverlays(user.Proxy.HostExec.Overlay, project.Proxy.HostExec.Overlay),
			},
			MCPProxy: MCPProxyConfig{
				Servers: mergeMCPServerMap(user.Proxy.MCPProxy.Servers, project.Proxy.MCPProxy.Servers),
			},
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

// mergeMCPServerMap merges user and project server maps.
// Project entries override user entries on the same alias; others are kept.
func mergeMCPServerMap(user, project map[string]MCPProxyServer) map[string]MCPProxyServer {
	if len(user) == 0 && len(project) == 0 {
		return nil
	}
	out := make(map[string]MCPProxyServer, len(user)+len(project))
	for alias, s := range user {
		out[alias] = s
	}
	for alias, s := range project {
		out[alias] = s
	}
	return out
}

func appendSlice(base, extra []string) []string {
	if len(extra) == 0 {
		return append([]string(nil), base...)
	}
	return append(append([]string(nil), base...), extra...)
}

// mergeOverlays concatenates user and project overlay entries, deduplicating by Target.
// Project entries take precedence over user entries with the same Target.
func mergeOverlays(user, project []OverlayEntry) []OverlayEntry {
	seen := make(map[string]struct{}, len(user)+len(project))
	out := make([]OverlayEntry, 0, len(user)+len(project))
	for _, e := range project {
		if _, ok := seen[e.Target]; !ok {
			seen[e.Target] = struct{}{}
			out = append(out, e)
		}
	}
	for _, e := range user {
		if _, ok := seen[e.Target]; !ok {
			seen[e.Target] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}
