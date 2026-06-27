package event

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/platform/appid"
)

func TestRunMissingType(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Errorf("expected error when type missing")
	}
}

func TestRunNoSocketIsNoop(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	if err := Run([]string{"PreToolUse"}); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	if err := Run([]string{"-bogus", "PreToolUse"}); err == nil {
		t.Errorf("expected error for unknown flag")
	}
}

func TestResolveSocketPathFromEnv(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "/tmp/custom.sock")
	got, err := ResolveSocketPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/custom.sock" {
		t.Errorf("got %q, want /tmp/custom.sock", got)
	}
}

func TestResolveSocketPathFromConfig(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	got, err := ResolveSocketPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Errorf("empty path")
	}
	if !strings.HasSuffix(got, appid.SocketFileName) {
		t.Errorf("got %q, want suffix %q", got, appid.SocketFileName)
	}
}

// TestResolveSocketPath_DataDirOverride asserts that when ROOST_SOCKET is
// unset, an explicit dataDirOverride argument is used to derive the socket
// path. This is the phase F-D mechanism that lets a hook command installed
// outside any server-spawned context (e.g. Claude/Gemini settings.json) target
// a specific daemon by its data dir.
func TestResolveSocketPath_DataDirOverride(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	got, err := ResolveSocketPath("/foo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/foo", appid.SocketFileName)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveSocketPath_RoostSocketWins asserts that ROOST_SOCKET still wins
// over an explicit dataDirOverride. Container sandboxes set ROOST_SOCKET to
// the bind-mounted path; that pinning must not be overridden by a stray
// -data-dir flag carried into the container environment.
func TestResolveSocketPath_RoostSocketWins(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "/env/wins.sock")
	got, err := ResolveSocketPath("/foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/env/wins.sock" {
		t.Errorf("got %q, want /env/wins.sock (ROOST_SOCKET must beat override)", got)
	}
}

// TestResolveSocketPath_MultipleDataDirs spot-checks the phase F-D scenario:
// two daemons launched with different -data-dir values must resolve to
// disjoint socket paths. The check is socket-path level (we don't dial
// daemons here) and exercises the core routing primitive.
func TestResolveSocketPath_MultipleDataDirs(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	gotA, err := ResolveSocketPath("/a")
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := ResolveSocketPath("/b")
	if err != nil {
		t.Fatal(err)
	}
	wantA := filepath.Join("/a", appid.SocketFileName)
	wantB := filepath.Join("/b", appid.SocketFileName)
	if gotA != wantA {
		t.Errorf("daemon A: got %q, want %q", gotA, wantA)
	}
	if gotB != wantB {
		t.Errorf("daemon B: got %q, want %q", gotB, wantB)
	}
	if gotA == gotB {
		t.Errorf("daemons collapsed to same socket %q", gotA)
	}
}

// TestRunWithDataDirNoDaemon verifies the phase F-D arg ordering: the setup
// scripts emit `event <type> -data-dir <dir>` (flag AFTER positional), and
// the manual parser must accept that form. With no daemon listening at the
// override path the send fails internally and is logged as a warning, but
// Run still returns nil — that is the documented contract for hook delivery.
func TestRunWithDataDirNoDaemon(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	dir := t.TempDir()
	if err := Run([]string{"PreToolUse", "-data-dir", dir}); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

// TestRunDataDirEqualsForm covers the -data-dir=VALUE form. The scripts use
// the space-separated form, but the equals form is a standard convention
// hand-typed invocations may rely on.
func TestRunDataDirEqualsForm(t *testing.T) {
	t.Setenv("ROOST_SOCKET", "")
	dir := t.TempDir()
	if err := Run([]string{"PreToolUse", "-data-dir=" + dir}); err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

// TestParseEventArgs exercises the arg-stripper directly so regressions in
// the manual parser surface without depending on Run's noop branch.
func TestParseEventArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantType string
		wantDir  string
		wantErr  bool
	}{
		{"positional only", []string{"PreToolUse"}, "PreToolUse", "", false},
		{"flag before positional", []string{"-data-dir", "/x", "PreToolUse"}, "PreToolUse", "/x", false},
		{"flag after positional", []string{"PreToolUse", "-data-dir", "/x"}, "PreToolUse", "/x", false},
		{"double dash flag", []string{"PreToolUse", "--data-dir", "/x"}, "PreToolUse", "/x", false},
		{"equals form", []string{"PreToolUse", "-data-dir=/x"}, "PreToolUse", "/x", false},
		{"missing value", []string{"PreToolUse", "-data-dir"}, "", "", true},
		{"empty args", nil, "", "", true},
		{"unknown flag", []string{"-bogus", "PreToolUse"}, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotType, gotDir, err := parseEventArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if gotType != tc.wantType {
				t.Errorf("type=%q want %q", gotType, tc.wantType)
			}
			if gotDir != tc.wantDir {
				t.Errorf("dir=%q want %q", gotDir, tc.wantDir)
			}
		})
	}
}
