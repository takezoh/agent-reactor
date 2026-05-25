package wfconfig

import (
	"os"
	"regexp"

	platformcfg "github.com/takezoh/agent-roost/platform/config"
)

// varPattern matches exactly "$NAME" (anchored, uppercase/underscore).
var varPattern = regexp.MustCompile(`^\$([A-Z_][A-Z0-9_]*)$`)

// expandVar resolves "$VARNAME" to its env value; returns s unchanged otherwise.
func expandVar(s string) string {
	m := varPattern.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	return os.Getenv(m[1])
}

// expandPath applies "~" expansion then "$VAR" resolution (path fields only).
func expandPath(s string) string {
	s = platformcfg.ExpandPath(s)
	return expandVar(s)
}

// expandAPIKey resolves "$VAR" for api_key (no tilde expansion).
func expandAPIKey(s string) string {
	return expandVar(s)
}
