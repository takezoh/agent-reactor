package stream

import (
	"strings"
	"testing"
)

func TestParseCommandBasic(t *testing.T) {
	cfg, err := ParseCommand(DriverName)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerBin != DriverName {
		t.Errorf("ServerBin = %q", cfg.ServerBin)
	}
}

func TestParseCommandFlags(t *testing.T) {
	cfg, err := ParseCommand(DriverName + " -m gpt-4 -c key=val")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if len(cfg.ServerArgs) != 2 || cfg.ServerArgs[0] != "-c" || cfg.ServerArgs[1] != "key=val" {
		t.Errorf("ServerArgs = %v", cfg.ServerArgs)
	}
}

func TestParseCommandResumeSkipsID(t *testing.T) {
	cfg, err := ParseCommand(DriverName + " resume some-id --enable foo")
	if err != nil {
		t.Fatal(err)
	}
	// "resume" consumes the next arg; --enable foo should still be captured
	if len(cfg.ServerArgs) != 2 || cfg.ServerArgs[0] != "--enable" {
		t.Errorf("ServerArgs = %v", cfg.ServerArgs)
	}
}

func TestParseCommandWrongDriver(t *testing.T) {
	if _, err := ParseCommand("notcodex"); err == nil {
		t.Error("expected error")
	}
	if _, err := ParseCommand(""); err == nil {
		t.Error("expected error on empty")
	}
}

func TestBuildServerArgs(t *testing.T) {
	args := buildServerArgs(nil, false, "/tmp/x.sock")
	if args[0] != "app-server" {
		t.Errorf("got %v", args)
	}
	args = buildServerArgs([]string{"-c", "k=v"}, true, "/tmp/x.sock")
	found := false
	for _, a := range args {
		if strings.Contains(a, "danger-full-access") {
			found = true
		}
	}
	if !found {
		t.Errorf("sandboxExternal flag missing: %v", args)
	}
}

func TestBuildRemoteCommandCold(t *testing.T) {
	cmd := BuildRemoteCommand(8080, "sess1", "", "/work")
	if !strings.Contains(cmd, "--remote ws://127.0.0.1:8080/sess1") {
		t.Errorf("remote missing: %s", cmd)
	}
	if strings.Contains(cmd, "resume") {
		t.Errorf("cold start should not have resume: %s", cmd)
	}
	if !strings.Contains(cmd, "-C /work") {
		t.Errorf("startDir missing: %s", cmd)
	}
}

func TestBuildRemoteCommandWarm(t *testing.T) {
	cmd := BuildRemoteCommand(8080, "sess1", "tid", "")
	if !strings.Contains(cmd, "resume tid") {
		t.Errorf("warm start should have resume: %s", cmd)
	}
}

func TestShellJoinArgv(t *testing.T) {
	got := shellJoinArgv([]string{"a b", "c'd"})
	if got != "'a b' 'c'\\''d'" {
		t.Errorf("got %q", got)
	}
}

func TestPrefixWriter(t *testing.T) {
	var sb strings.Builder
	w := newPrefixWriter(&sb, 5)
	n, err := w.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 {
		t.Errorf("n = %d, expected to count all input", n)
	}
	if sb.String() != "hello" {
		t.Errorf("dst = %q", sb.String())
	}
	// further writes should be ignored
	w.Write([]byte("xx"))
	if sb.String() != "hello" {
		t.Errorf("dst = %q", sb.String())
	}
}
