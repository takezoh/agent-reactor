package hostexec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/agent-roost/platform/internal/globutil"
)

// binaryName extracts and validates the binary name from allow patterns.
// All patterns must share the same first non-env-assignment token.
func binaryName(allow []string) (string, error) {
	if len(allow) == 0 {
		return "", fmt.Errorf("allow must not be empty")
	}
	var name string
	for _, pat := range allow {
		fields := skipEnvAssignments(strings.Fields(pat))
		if len(fields) == 0 {
			return "", fmt.Errorf("allow pattern is blank")
		}
		tok := fields[0]
		if name == "" {
			name = tok
		} else if tok != name {
			return "", fmt.Errorf("allow patterns have inconsistent binary names: %q vs %q", name, tok)
		}
	}
	return name, nil
}

func TestShellJoin(t *testing.T) {
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"gh", "pr", "list"}, "gh pr list"},
		{[]string{"gh", "pr", "create", "--title", "hello world"}, "gh pr create --title 'hello world'"},
		{[]string{"gh", "api", "GET", "/repos/owner/repo"}, "gh api GET /repos/owner/repo"},
		{[]string{""}, "''"},
		{[]string{"it's"}, "'it'\\''s'"},
	}
	for _, c := range cases {
		got := shellJoin(c.argv)
		if got != c.want {
			t.Errorf("shellJoin(%v) = %q, want %q", c.argv, got, c.want)
		}
	}
}

func TestCompileGlob(t *testing.T) {
	cases := []struct {
		pattern string
		input   string
		match   bool
	}{
		{"gh pr*", "gh preview", true},
		{"gh pr *", "gh preview", false},
		{"gh pr *", "gh pr create", true},
		{"gh pr *", "gh pr hello world", true},
		{"gh pr *", "gh pr", false}, // * requires at least one char after space
		{"gh api GET /repos/*", "gh api GET /repos/owner/repo", true},
		{"gh api GET /repos/*", "gh api POST /repos/owner/repo", false},
		{"gh * delete*", "gh repo delete", true},
		{"gh * delete*", "gh repo delete --confirm", true},
		{"gh * delete*", "gh repo view", false},
		// * must match newlines so that multi-line commit messages (e.g. -c=...\n...)
		// are covered by a simple "cmd *" pattern.
		{"plastic.exe *", "plastic.exe ci 'foo'\n'-c=line1\nline2'", true},
		{"gh pr *", "gh pr create --body 'multi\nline'", true},
	}
	for _, c := range cases {
		re, err := globutil.CompileGlob(c.pattern)
		if err != nil {
			t.Fatalf("CompileGlob(%q): %v", c.pattern, err)
		}
		got := re.MatchString(c.input)
		if got != c.match {
			t.Errorf("pattern %q against %q: got %v, want %v", c.pattern, c.input, got, c.match)
		}
	}
}

func TestPolicyCheck(t *testing.T) {
	pol, err := CompilePolicy(
		[]string{"gh pr *", "gh issue *", "gh repo view*"},
		[]string{"gh * delete*", "gh repo delete*", "gh auth *"},
	)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		argv    []string
		wantErr bool
	}{
		// allow
		{[]string{"gh", "pr", "list"}, false},
		{[]string{"gh", "pr", "create", "--title", "Add feature"}, false},
		{[]string{"gh", "issue", "list"}, false},
		{[]string{"gh", "repo", "view", "owner/repo"}, false},
		// deny (override allow)
		{[]string{"gh", "repo", "delete", "owner/repo"}, true},
		{[]string{"gh", "auth", "login"}, true},
		// not in allowlist
		{[]string{"gh", "secret", "list"}, true},
		{[]string{"gh", "ssh-key", "list"}, true},
	}
	for _, c := range cases {
		err := pol.Check(c.argv)
		if (err != nil) != c.wantErr {
			t.Errorf("Check(%v) err=%v, wantErr=%v", c.argv, err, c.wantErr)
		}
	}
}

func TestBinaryName(t *testing.T) {
	cases := []struct {
		allow   []string
		want    string
		wantErr bool
	}{
		{[]string{"gh pr *", "gh issue *"}, "gh", false},
		{[]string{"gh pr *"}, "gh", false},
		{[]string{"gh pr *", "aws s3 *"}, "", true}, // inconsistent
		{[]string{}, "", true},                      // empty
		{[]string{""}, "", true},                    // blank pattern
	}
	for _, c := range cases {
		got, err := binaryName(c.allow)
		if (err != nil) != c.wantErr {
			t.Errorf("binaryName(%v) err=%v, wantErr=%v", c.allow, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("binaryName(%v) = %q, want %q", c.allow, got, c.want)
		}
	}
}

func TestPolicyEmptyAllow(t *testing.T) {
	pol, err := CompilePolicy(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.Check([]string{"gh", "pr", "list"}); err == nil {
		t.Error("empty allow should reject all commands")
	}
}

func TestEnvPrefixInPatterns(t *testing.T) {
	pol, err := CompilePolicy(
		[]string{"GH_TOKEN=secret gh pr *", "gh issue *"},
		[]string{"DEBUG=1 gh * delete*"},
	)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		argv    []string
		wantErr bool
	}{
		// env prefix stripped from allow: "GH_TOKEN=secret gh pr *" -> "gh pr *"
		{[]string{"gh", "pr", "list"}, false},
		{[]string{"gh", "pr", "create"}, false},
		// plain allow
		{[]string{"gh", "issue", "list"}, false},
		// env prefix stripped from deny: "DEBUG=1 gh * delete*" -> "gh * delete*"
		{[]string{"gh", "repo", "delete"}, true},
		// not in allowlist
		{[]string{"gh", "secret", "list"}, true},
	}
	for _, c := range cases {
		err := pol.Check(c.argv)
		if (err != nil) != c.wantErr {
			t.Errorf("Check(%v) err=%v, wantErr=%v", c.argv, err, c.wantErr)
		}
	}
}

func TestBinaryNameEnvPrefix(t *testing.T) {
	cases := []struct {
		allow   []string
		want    string
		wantErr bool
	}{
		{[]string{"GH_TOKEN=x gh pr *", "gh issue *"}, "gh", false},
		{[]string{"A=1 B=2 gh pr *"}, "gh", false},          // multiple env assignments
		{[]string{"A=1 gh pr *", "B=2 aws s3 *"}, "", true}, // inconsistent binaries after strip
	}
	for _, c := range cases {
		got, err := binaryName(c.allow)
		if (err != nil) != c.wantErr {
			t.Errorf("binaryName(%v) err=%v, wantErr=%v", c.allow, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("binaryName(%v) = %q, want %q", c.allow, got, c.want)
		}
	}
}

func TestPolicyEmptyAfterEnvStrip(t *testing.T) {
	// Pattern consisting only of env assignments should error.
	_, err := CompilePolicy([]string{"A=1 B=2"}, nil)
	if err == nil {
		t.Error("allow pattern with only env assignments should return error")
	}
	_, err = CompilePolicy([]string{"gh pr *"}, []string{"A=1"})
	if err == nil {
		t.Error("deny pattern with only env assignments should return error")
	}
	// Blank pattern should also error.
	_, err = CompilePolicy([]string{""}, nil)
	if err == nil {
		t.Error("blank allow pattern should return error")
	}
}

func TestValidBinaryName(t *testing.T) {
	valid := []string{"gh", "aws", "kubectl", "2to3", "node.exe", "my-tool", "my_tool"}
	for _, name := range valid {
		if err := validBinaryName(name); err != nil {
			t.Errorf("validBinaryName(%q) unexpected error: %v", name, err)
		}
	}
	invalid := []string{"", "gh;rm", "$(evil)", "a b", "a|b", "a&b", "../traversal"}
	for _, name := range invalid {
		if err := validBinaryName(name); err == nil {
			t.Errorf("validBinaryName(%q) expected error, got nil", name)
		}
	}
}

func TestCompileEntriesInvalidBinaryName(t *testing.T) {
	cfg := config.HostExecConfig{Allow: []string{"$(evil) pr *"}}
	if _, err := compileEntries(cfg); err == nil {
		t.Error("compileEntries with invalid binary name should return error")
	}
}
