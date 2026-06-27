package socketpath_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/socketpath"
)

func TestResolveDaemonSocket_FlagPriority(t *testing.T) {
	t.Setenv("ARC_SOCK", "/from/env")
	got := socketpath.ResolveDaemonSocket("/explicit/sock", "ARC_SOCK", "server.sock")
	if got != "/explicit/sock" {
		t.Errorf("expected /explicit/sock, got %s", got)
	}
}

func TestResolveDaemonSocket_EnvFallback(t *testing.T) {
	t.Setenv("ARC_SOCK", "/from/env")
	got := socketpath.ResolveDaemonSocket("", "ARC_SOCK", "server.sock")
	if got != "/from/env" {
		t.Errorf("expected /from/env, got %s", got)
	}
}

func TestResolveDaemonSocket_HomeFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ARC_SOCK", "") // ensure env path is not taken
	got := socketpath.ResolveDaemonSocket("", "ARC_SOCK", "server.sock")
	want := filepath.Join(tmp, ".agent-reactor", "server.sock")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveDaemonSocket_TmpFallback(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("ARC_SOCK", "")
	got := socketpath.ResolveDaemonSocket("", "ARC_SOCK", "server.sock")
	want := filepath.Join(os.TempDir(), "server.sock")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveDaemonSocket_WhitespaceTrimmed(t *testing.T) {
	t.Setenv("ARC_SOCK", "/from/env")
	got := socketpath.ResolveDaemonSocket("  ", "ARC_SOCK", "server.sock")
	// whitespace-only flag should fall through to env
	if !strings.HasPrefix(got, "/from/env") && got != "/from/env" {
		t.Errorf("expected env path /from/env, got %s", got)
	}
	if got != "/from/env" {
		t.Errorf("expected /from/env, got %s", got)
	}
}
