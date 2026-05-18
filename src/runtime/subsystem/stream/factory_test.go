package stream

import (
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestFactoryMakeIDDistinguishesSandboxMode(t *testing.T) {
	f := &Factory{}
	autoID := f.makeID("/repo", state.SandboxOverrideAuto)
	hostID := f.makeID("/repo", state.SandboxOverrideHost)
	if autoID == hostID {
		t.Fatalf("auto and host IDs collided: %q", autoID)
	}
	if want := state.SubsystemID("stream:auto:/repo"); autoID != want {
		t.Errorf("autoID = %q, want %q", autoID, want)
	}
	if want := state.SubsystemID("stream:host:/repo"); hostID != want {
		t.Errorf("hostID = %q, want %q", hostID, want)
	}
}

func TestFactoryMakeIDEscapesColons(t *testing.T) {
	f := &Factory{}
	id := f.makeID("/repo:weird", state.SandboxOverrideAuto)
	if want := state.SubsystemID("stream:auto:/repo_weird"); id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}
