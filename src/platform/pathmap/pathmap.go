package pathmap

import "path/filepath"

// Mount is a single bind-mount entry mapping a host absolute path to a
// container absolute path.
type Mount struct {
	Host      string // absolute, no trailing slash
	Container string // absolute, no trailing slash
}

// Mounts is the set of bind mounts for a running container instance.
// ToHost / ToContainer use longest-prefix matching so nested mounts
// (e.g. /workspaces/foo and /workspaces/foo/cache from different hosts)
// resolve correctly.
type Mounts []Mount

// ToHost translates a container-absolute path to a host-absolute path.
// Returns ("", false) when no mount covers the path.
func (ms Mounts) ToHost(container string) (string, bool) {
	container = filepath.Clean(container)
	m, rel, found := longestMatch(ms, container, func(m Mount) string { return m.Container })
	if !found {
		return "", false
	}
	if rel == "." {
		return m.Host, true
	}
	return filepath.Join(m.Host, rel), true
}

// ToContainer translates a host-absolute path to a container-absolute path.
// Returns ("", false) when no mount covers the path.
func (ms Mounts) ToContainer(host string) (string, bool) {
	host = filepath.Clean(host)
	m, rel, found := longestMatch(ms, host, func(m Mount) string { return m.Host })
	if !found {
		return "", false
	}
	if rel == "." {
		return m.Container, true
	}
	return filepath.Join(m.Container, rel), true
}

// longestMatch finds the Mount whose field selected by key is the longest
// prefix of clean. Both fields are filepath.Clean'd on the winning entry.
// Returns ("", false) when no mount covers the path.
func longestMatch(ms Mounts, clean string, key func(Mount) string) (Mount, string, bool) {
	var best Mount
	bestLen := -1
	bestRel := ""
	for _, m := range ms {
		root := filepath.Clean(key(m))
		rel, ok := subpath(clean, root)
		if ok && len(root) > bestLen {
			best = Mount{Host: filepath.Clean(m.Host), Container: filepath.Clean(m.Container)}
			bestLen = len(root)
			bestRel = rel
		}
	}
	if bestLen < 0 {
		return Mount{}, "", false
	}
	return best, bestRel, true
}

// subpath reports whether p is equal to or descends from root, and returns
// the relative path from root to p. Both inputs must already be clean.
// Returns (".", true) on exact match.
func subpath(p, root string) (string, bool) {
	if p == root {
		return ".", true
	}
	prefix := root + "/"
	if len(p) > len(prefix) && p[:len(prefix)] == prefix {
		return p[len(prefix):], true
	}
	return "", false
}
