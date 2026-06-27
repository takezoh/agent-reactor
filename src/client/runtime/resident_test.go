package runtime

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsMissingPaneErrAcceptsSentinel verifies the shared sentinel path:
// errors that wrap ErrPaneMissing (as PtyBackend returns) are recognised as
// missing-pane errors so reconcileWindows evicts the vanished frame instead of
// treating it as transient.
func TestIsMissingPaneErrAcceptsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("runtime: unknown pane %q: %w", "%9", ErrPaneMissing)
	if !isMissingPaneErr(wrapped) {
		t.Fatalf("isMissingPaneErr(wrapped ErrPaneMissing) = false, want true")
	}
	if !errors.Is(wrapped, ErrPaneMissing) {
		t.Fatalf("errors.Is(wrapped, ErrPaneMissing) = false, want true")
	}
}

// TestIsMissingPaneErrIgnoresOther verifies unrelated errors are not classified
// as missing-pane errors.
func TestIsMissingPaneErrIgnoresOther(t *testing.T) {
	if isMissingPaneErr(nil) {
		t.Fatalf("isMissingPaneErr(nil) = true, want false")
	}
	other := errors.New("write: broken pipe")
	if isMissingPaneErr(other) {
		t.Fatalf("isMissingPaneErr(%q) = true, want false", other.Error())
	}
	legacy := errors.New("backend: can't find pane: arc:0.7")
	if isMissingPaneErr(legacy) {
		t.Fatalf("isMissingPaneErr(legacy substring) = true, want false " +
			"after legacy substring fallback removal")
	}
}
