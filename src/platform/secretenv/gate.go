package secretenv

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Gate enforces an allowlist of env-file path patterns.
// Uses filepath.Match glob syntax. Default-deny when Allow is empty.
//
// Pattern notes:
//   - '*' matches any sequence of non-separator characters within ONE path
//     segment. "/workspace/*.env" matches "/workspace/dev.env" but NOT
//     "/workspace/sub/dev.env".
//   - '**' is NOT treated as a recursive wildcard; it is equivalent to two
//     adjacent '*' within the same segment. To allow an entire directory tree,
//     list each level explicitly or use the path prefix pattern "/dir/*".
//   - Patterns are matched against HOST-absolute paths (after container→host
//     translation via HostPathMountPrefix). The broker performs Abs+Clean+
//     containerToHost before calling Check; callers must pass the resulting
//     host absolute path.
type Gate struct {
	allow []string
}

// containerToHost strips HostPathMountPrefix from a container-absolute path to
// produce the corresponding host-absolute path. When prefix is empty (bare-host
// or no devcontainer) the path is returned unchanged.
//
// The strip is boundary-safe: prefix "/mnt" only matches paths that start with
// "/mnt/" (or equal "/mnt"), so "/mnternal/x" is never incorrectly stripped.
func containerToHost(path, prefix string) string {
	// Normalize: strip any trailing slash from prefix so "/mnt/" behaves the
	// same as "/mnt". This makes the function robust to user misconfiguration.
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return path
	}
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		return path[len(prefix):]
	}
	return path
}

// NewGate builds a Gate from a list of filepath.Match glob patterns.
func NewGate(allow []string) *Gate {
	patterns := make([]string, len(allow))
	copy(patterns, allow)
	return &Gate{allow: patterns}
}

// Check returns nil if path matches at least one allow pattern, or an error.
func (g *Gate) Check(path string) error {
	for _, pat := range g.allow {
		ok, err := filepath.Match(pat, path)
		if err != nil {
			return fmt.Errorf("secretenv gate: invalid pattern %q: %w", pat, err)
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("secretenv gate: %q is not in the allowlist", path)
}
