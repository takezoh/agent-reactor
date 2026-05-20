package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands a leading "~" to the user home directory.
func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}

// listProjectsFrom enumerates project directories from roots + explicit paths.
// Factored out so both platform/config (ProjectsConfig.ListProjects) and
// client/config (Config.ListProjects) can use it without importing each other.
func listProjectsFrom(roots, paths []string) []string {
	var projects []string
	for _, root := range roots {
		root = ExpandPath(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				projects = append(projects, filepath.Join(root, e.Name()))
			}
		}
	}
	for _, p := range paths {
		p = ExpandPath(p)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			projects = append(projects, p)
		}
	}
	return projects
}
