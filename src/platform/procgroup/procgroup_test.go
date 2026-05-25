package procgroup

import (
	"context"
	"testing"
	"time"
)

func TestCommandDefaults(t *testing.T) {
	cmd := Command(Spec{Ctx: context.Background(), Bin: "echo", Args: []string{"hi"}})
	if cmd.WaitDelay != DefaultWaitDelay {
		t.Errorf("WaitDelay = %v, want default %v", cmd.WaitDelay, DefaultWaitDelay)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != "hi" {
		t.Errorf("Args = %v, want [echo hi]", cmd.Args)
	}
}

func TestCommandOverrides(t *testing.T) {
	cmd := Command(Spec{
		Ctx:       context.Background(),
		Bin:       "echo",
		Dir:       "/tmp",
		Env:       []string{"FOO=bar"},
		WaitDelay: 2 * time.Second,
	})
	if cmd.WaitDelay != 2*time.Second {
		t.Errorf("WaitDelay = %v, want 2s", cmd.WaitDelay)
	}
	if cmd.Dir != "/tmp" {
		t.Errorf("Dir = %q, want /tmp", cmd.Dir)
	}
	if len(cmd.Env) != 1 || cmd.Env[0] != "FOO=bar" {
		t.Errorf("Env = %v, want [FOO=bar]", cmd.Env)
	}
}

func TestCommandNilCtxDoesNotPanic(t *testing.T) {
	cmd := Command(Spec{Bin: "echo"})
	if cmd == nil {
		t.Fatal("Command returned nil")
	}
}

func TestNewBootNonceUnique(t *testing.T) {
	a, b := NewBootNonce(), NewBootNonce()
	if a == "" || b == "" {
		t.Fatal("NewBootNonce returned empty")
	}
	if a == b {
		t.Errorf("NewBootNonce not unique: %q", a)
	}
}

func TestNilTrackerIsNoop(t *testing.T) {
	var tr *Tracker
	tr.Track(123)   // must not panic
	tr.Untrack(123) // must not panic
	tr.Prune()      // must not panic
	(&Tracker{}).Prune()
}
