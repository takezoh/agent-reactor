package event

import (
	"os"
	"testing"
)

func TestRunMissingType(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Errorf("expected error when type missing")
	}
}

func TestRunNoSocketIsNoop(t *testing.T) {
	os.Unsetenv("ROOST_SOCKET")
	if err := Run([]string{"PreToolUse"}); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestResolveSocketPathFromEnv(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "/tmp/custom.sock")
	got, err := ResolveSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/custom.sock" {
		t.Errorf("got %q", got)
	}
}

func TestResolveSocketPathFromConfig(t *testing.T) {
	os.Unsetenv("ROOST_SOCKET")
	got, err := ResolveSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Errorf("empty path")
	}
}
