package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/client/config"
)

func TestParseDaemonArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    daemonFlagSet
		wantErr bool
	}{
		{name: "empty", args: nil, want: daemonFlagSet{addr: ":8443"}},
		{name: "data-dir space", args: []string{"-data-dir", "/var/lib/x"}, want: daemonFlagSet{addr: ":8443", dataDir: "/var/lib/x"}},
		{name: "data-dir equals", args: []string{"-data-dir=/var/lib/x"}, want: daemonFlagSet{addr: ":8443", dataDir: "/var/lib/x"}},
		{name: "double dash", args: []string{"--data-dir", "/var/lib/x"}, want: daemonFlagSet{addr: ":8443", dataDir: "/var/lib/x"}},
		{name: "addr override", args: []string{"-addr", "127.0.0.1:9090"}, want: daemonFlagSet{addr: "127.0.0.1:9090"}},
		{name: "noauth+insecure", args: []string{"-insecure", "-no-auth"}, want: daemonFlagSet{addr: ":8443", insecure: true, noAuth: true}},
		{name: "token", args: []string{"-token", "abcdef"}, want: daemonFlagSet{addr: ":8443", token: "abcdef"}},
		{name: "token-file", args: []string{"-token-file", "/run/tok"}, want: daemonFlagSet{addr: ":8443", tokenFile: "/run/tok"}},
		{name: "tls", args: []string{"-tls-cert", "/c", "-tls-key", "/k"}, want: daemonFlagSet{addr: ":8443", certFile: "/c", keyFile: "/k"}},
		{name: "unknown flag", args: []string{"-unknown"}, wantErr: true},
		{name: "positional rejected", args: []string{"-data-dir", "/x", "extra"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDaemonArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *got != tc.want {
				t.Errorf("parsed = %+v, want %+v", *got, tc.want)
			}
		})
	}
}

// TestRunMainInjectsDataDirIntoConfig verifies that `-data-dir` reaches the
// config BEFORE logger init: the captured DataDir at the moment runDaemon
// runs must match the flag-specified path, not the bootstrap config's value.
func TestRunMainInjectsDataDirIntoConfig(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	bootstrapDir := t.TempDir()
	flagDir := t.TempDir()

	loadBootstrapConfig = func() (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.DataDir = bootstrapDir
		return cfg, nil
	}

	var capturedDir string
	initLoggerWithDataDir = func(level, dir string) error {
		capturedDir = dir
		return nil
	}
	runDaemonFn = func(*daemonFlagSet) error { return nil }

	var stdout, stderr bytes.Buffer
	code := runMain([]string{"-data-dir", flagDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if capturedDir != flagDir {
		t.Errorf("logger got dir %q, want %q (flag must win over bootstrap config)",
			capturedDir, flagDir)
	}
}

// TestRunMainExportsDataDirToEnv verifies that -data-dir is exported to
// ROOST_DATA_DIR so a downstream fresh loadConfig() (runDaemon does this)
// resolves the SAME path the flag specified. Without the setenv hop, the
// runtime would log to the flag dir but place its socket/sessions/pid under
// the bootstrap dir — the systemd cascade would 502-loop.
func TestRunMainExportsDataDirToEnv(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	flagDir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		return config.DefaultConfig(), nil
	}
	initLoggerWithDataDir = func(level, dir string) error { return nil }
	runDaemonFn = func(*daemonFlagSet) error { return nil }

	var stdout, stderr bytes.Buffer
	if code := runMain([]string{"-data-dir", flagDir}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if got := os.Getenv("ROOST_DATA_DIR"); got != flagDir {
		t.Errorf("ROOST_DATA_DIR = %q, want %q (flag must be exported so a "+
			"fresh ResolveDataDir() in runDaemon returns the same path)",
			got, flagDir)
	}
}

// TestRunMainFlagWinsOverStaleEnv verifies that a pre-existing ROOST_DATA_DIR
// in the process env (e.g. inherited from the operator's shell) does not
// silently override an explicit -data-dir flag. systemd `systemctl --user
// start` inherits the user's env, so the unit's ExecStart=… -data-dir would
// otherwise be a no-op for any developer with ROOST_DATA_DIR in their rc.
func TestRunMainFlagWinsOverStaleEnv(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	t.Setenv("ROOST_DATA_DIR", "/should/be/overridden")
	flagDir := t.TempDir()
	loadBootstrapConfig = func() (*config.Config, error) {
		return config.DefaultConfig(), nil
	}
	var capturedDir string
	initLoggerWithDataDir = func(level, dir string) error {
		capturedDir = dir
		return nil
	}
	runDaemonFn = func(*daemonFlagSet) error { return nil }

	var stdout, stderr bytes.Buffer
	if code := runMain([]string{"-data-dir", flagDir}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if capturedDir != flagDir {
		t.Errorf("logger got %q, want %q (flag must beat stale env)", capturedDir, flagDir)
	}
	if got := os.Getenv("ROOST_DATA_DIR"); got != flagDir {
		t.Errorf("ROOST_DATA_DIR = %q, want %q", got, flagDir)
	}
}

func TestRunMainBadDaemonFlagExitsTwo(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	loadBootstrapConfig = func() (*config.Config, error) {
		return config.DefaultConfig(), nil
	}
	parseDaemonArgsFn = func(args []string) (*daemonFlagSet, error) {
		return nil, errors.New("bad flag")
	}
	called := false
	runDaemonFn = func(*daemonFlagSet) error { called = true; return nil }

	var stdout, stderr bytes.Buffer
	code := runMain([]string{"-data-dir", "/x"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if called {
		t.Fatal("runDaemonFn must not be called when daemon flag parse fails")
	}
	if !strings.Contains(stderr.String(), "bad flag") {
		t.Errorf("stderr = %q, want it to contain 'bad flag'", stderr.String())
	}
}

// TestRunMainBadDaemonFlagSurfacesWhenConfigNil guards against silently
// bypassing flag validation when the bootstrap config load fails — that
// would let `-data-dir bogus-positional-arg` slip past the early-exit and
// produce a confusing config-error trace from the coordinator instead of
// the precise flag-parse diagnostic.
func TestRunMainBadDaemonFlagSurfacesWhenConfigNil(t *testing.T) {
	restore := stubMainDeps(t)
	defer restore()

	loadBootstrapConfig = func() (*config.Config, error) {
		return nil, errors.New("settings.toml broken")
	}
	parseCalled := false
	parseDaemonArgsFn = func(args []string) (*daemonFlagSet, error) {
		parseCalled = true
		return nil, errors.New("unexpected positional")
	}
	runDaemonFn = func(*daemonFlagSet) error { return nil }
	initLoggerWithDataDir = func(level, dir string) error { return nil }

	var stdout, stderr bytes.Buffer
	code := runMain([]string{"-data-dir", "/x", "stray"}, &stdout, &stderr)
	if !parseCalled {
		t.Fatal("parseDaemonArgsFn must run even when bootstrap config load failed")
	}
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unexpected positional") {
		t.Errorf("stderr = %q, want flag parse diagnostic", stderr.String())
	}
}
