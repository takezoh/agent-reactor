package runtime

import (
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/shellalias"
)

func TestBuildSpawnCommand(t *testing.T) {
	// The bare shell command explicitly execs the user's passwd login shell
	// instead of returning "" (which would defer to a multiplexer default-shell).
	got := buildSpawnCommand("shell", nil)
	if got == "" {
		t.Fatal("shell spawn must not be empty")
	}
	if !strings.HasPrefix(got, "exec ") || !strings.HasSuffix(got, " -l") ||
		!strings.Contains(got, shellalias.LoginShellCommand) {
		t.Errorf("shell spawn = %q, want exec <login shell> -l", got)
	}

	// Non-shell commands are exec'd directly.
	if got := buildSpawnCommand("claude --model sonnet", nil); got != "exec claude --model sonnet" {
		t.Errorf("non-shell spawn = %q, want %q", got, "exec claude --model sonnet")
	}
}
