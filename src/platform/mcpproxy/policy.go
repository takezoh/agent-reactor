// Package mcpproxy implements a host-side MCP server broker: a Unix socket
// server that runs allowlisted MCP server processes on behalf of container
// agents, relaying JSON-RPC stdio while enforcing tool-level policy.
package mcpproxy

import (
	"fmt"
	"regexp"

	"github.com/takezoh/agent-roost/platform/internal/globutil"
)

// Policy enforces deny-first, allow-second tool filtering.
// Patterns match the tool name directly (e.g. "list_*", "delete_bucket").
// Deny is checked before allow; neither matching is default-deny.
type Policy struct {
	deny  []*regexp.Regexp
	allow []*regexp.Regexp
}

// CompilePolicy builds a Policy from allow/deny pattern lists.
// Patterns match tool names with * as wildcard.
func CompilePolicy(allow, deny []string) (*Policy, error) {
	p := &Policy{}
	for _, pat := range deny {
		re, err := globutil.CompileGlob(pat)
		if err != nil {
			return nil, fmt.Errorf("mcpproxy: deny pattern %q: %w", pat, err)
		}
		p.deny = append(p.deny, re)
	}
	for _, pat := range allow {
		re, err := globutil.CompileGlob(pat)
		if err != nil {
			return nil, fmt.Errorf("mcpproxy: allow pattern %q: %w", pat, err)
		}
		p.allow = append(p.allow, re)
	}
	return p, nil
}

// CheckTool returns nil if the tool is permitted, or an error if denied/unlisted.
func (p *Policy) CheckTool(toolName string) error {
	for _, re := range p.deny {
		if re.MatchString(toolName) {
			return fmt.Errorf("tool denied: %s", toolName)
		}
	}
	for _, re := range p.allow {
		if re.MatchString(toolName) {
			return nil
		}
	}
	return fmt.Errorf("tool not in allowlist: %s", toolName)
}

// FilterTools returns the subset of tool names permitted by the policy.
func (p *Policy) FilterTools(names []string) []string {
	out := names[:0:0]
	for _, n := range names {
		if p.CheckTool(n) == nil {
			out = append(out, n)
		}
	}
	return out
}
