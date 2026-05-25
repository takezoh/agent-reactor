package config

import "path/filepath"

// ResolveDockerHost returns the DOCKER_HOST value to set, or "" to leave the
// daemon's environment untouched. Pure: callers inject env values and a stat
// callback so this is testable without touching the filesystem.
//
// Shared by the roost coordinator and the orchestrator (both launch
// devcontainers), so it lives in platform/ rather than either binary's layer.
func ResolveDockerHost(envDockerHost, xdgRuntimeDir string, socketExists func(string) bool) string {
	if envDockerHost != "" {
		return ""
	}
	if xdgRuntimeDir == "" {
		return ""
	}
	sock := filepath.Join(xdgRuntimeDir, "docker.sock")
	if !socketExists(sock) {
		return ""
	}
	return "unix://" + sock
}
