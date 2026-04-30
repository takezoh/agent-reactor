// Package hostexec implements a host-side exec broker: a Unix socket server
// that runs allowlisted host binaries on behalf of container processes,
// forwarding stdio via SCM_RIGHTS file descriptor passing.
package hostexec

import (
	"fmt"
	"regexp"
	"strings"
)

// envAssignRe matches a shell-style env assignment token (KEY=...).
var envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// isEnvAssignment reports whether s is a shell env assignment (KEY=VALUE).
func isEnvAssignment(s string) bool {
	return envAssignRe.MatchString(s)
}

// skipEnvAssignments returns fields with any leading KEY=VALUE tokens removed.
func skipEnvAssignments(fields []string) []string {
	for len(fields) > 0 && isEnvAssignment(fields[0]) {
		fields = fields[1:]
	}
	return fields
}

// trimEnvPrefix strips leading KEY=VALUE tokens from a pattern string so that
// "ENV=x gh pr *" is treated identically to "gh pr *" for matching purposes.
func trimEnvPrefix(pattern string) string {
	fields := skipEnvAssignments(strings.Fields(pattern))
	return strings.Join(fields, " ")
}

// Policy enforces deny-first, allow-second command filtering against a shell
// command string reconstructed from argv. Neither deny nor allow matching
// results in rejection (default-deny).
type Policy struct {
	deny  []*regexp.Regexp
	allow []*regexp.Regexp
}

// CompilePolicy builds a Policy from raw glob pattern lists.
// Patterns use * as a wildcard matching any string including spaces.
// A space before * is literal, so "gh pr *" requires a space before the
// wildcard and does not match "gh preview".
func CompilePolicy(allow, deny []string) (*Policy, error) {
	p := &Policy{}
	for _, pat := range deny {
		stripped := trimEnvPrefix(pat)
		if stripped == "" {
			return nil, fmt.Errorf("deny pattern %q: no command after env assignments", pat)
		}
		re, err := compileGlob(stripped)
		if err != nil {
			return nil, fmt.Errorf("deny pattern %q: %w", pat, err)
		}
		p.deny = append(p.deny, re)
	}
	for _, pat := range allow {
		stripped := trimEnvPrefix(pat)
		if stripped == "" {
			return nil, fmt.Errorf("allow pattern %q: no command after env assignments", pat)
		}
		re, err := compileGlob(stripped)
		if err != nil {
			return nil, fmt.Errorf("allow pattern %q: %w", pat, err)
		}
		p.allow = append(p.allow, re)
	}
	return p, nil
}

// Check returns nil if argv is permitted, or an error describing why it was rejected.
func (p *Policy) Check(argv []string) error {
	s := shellJoin(argv)
	for _, re := range p.deny {
		if re.MatchString(s) {
			return fmt.Errorf("command denied: %s", s)
		}
	}
	for _, re := range p.allow {
		if re.MatchString(s) {
			return nil
		}
	}
	return fmt.Errorf("command not in allowlist: %s", s)
}

// compileGlob converts a glob pattern (only * is special) to a regex.
// * matches any string including spaces; literal characters are escaped.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	parts := strings.Split(pattern, "*")
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.Compile(`\A` + strings.Join(quoted, `.*`) + `\z`)
}

// shellJoin reconstructs a shell command string from argv.
// Tokens containing spaces or shell metacharacters are single-quoted.
// This representation matches Claude Code's Bash permission pattern semantics.
func shellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		if !isShellSafe(r) {
			return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
		}
	}
	return s
}

func isShellSafe(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '@' || r == '='
}
