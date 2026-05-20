package runtime

import (
	"testing"
)

// TestDevcontainerLauncherFor_DirectLauncher verifies that devcontainerLauncherFor
// returns nil for a non-sandbox launcher (DirectLauncher).
func TestDevcontainerLauncherFor_DirectLauncher(t *testing.T) {
	l := launcher(Config{})
	if got := devcontainerLauncherFor(l); got != nil {
		t.Errorf("DirectLauncher: got %v, want nil", got)
	}
}

// TestDevcontainerLauncherFor_NilDispatcher verifies nil is returned for an
// adapter wrapping a dispatcher that has no devcontainer backend.
func TestDevcontainerLauncherFor_AdapterNilDevcontainer(t *testing.T) {
	l := NewDispatcherAdapter(nil)
	if got := devcontainerLauncherFor(l); got != nil {
		t.Errorf("adapter with nil: got %v, want nil", got)
	}
}
