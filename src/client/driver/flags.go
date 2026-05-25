package driver

import "strings"

// hasFlagToken returns true when command contains the exact flag as a token
// (or with a = suffix like "--flag=value"). Assumes alias expansion has
// already been applied by the caller.
func hasFlagToken(command, flag string) bool {
	for _, p := range strings.Fields(command) {
		if p == flag || strings.HasPrefix(p, flag+"=") {
			return true
		}
	}
	return false
}

// stripFlagToken removes exact flag tokens; "--flag=value" form is left intact
// (asymmetric with hasFlagToken by design — targets boolean-only flags).
func stripFlagToken(command, flag string) string {
	parts := strings.Fields(command)
	out := parts[:0]
	for _, p := range parts {
		if p == flag {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}
