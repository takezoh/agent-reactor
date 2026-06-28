package runtime

import (
	"errors"
	"fmt"
	"testing"
)

// TestIsMissingFrameErrAcceptsSentinel verifies the shared sentinel path:
// errors that wrap ErrFrameMissing (as PtyBackend returns) are recognised as
// missing-frame errors so reconcileWindows evicts the vanished frame instead of
// treating it as transient.
func TestIsMissingFrameErrAcceptsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("runtime: unknown frame %q: %w", "f9", ErrFrameMissing)
	if !isMissingFrameErr(wrapped) {
		t.Fatalf("isMissingFrameErr(wrapped ErrFrameMissing) = false, want true")
	}
	if !errors.Is(wrapped, ErrFrameMissing) {
		t.Fatalf("errors.Is(wrapped, ErrFrameMissing) = false, want true")
	}
}

// TestIsMissingFrameErrIgnoresOther verifies unrelated errors are not classified
// as missing-frame errors.
func TestIsMissingFrameErrIgnoresOther(t *testing.T) {
	if isMissingFrameErr(nil) {
		t.Fatalf("isMissingFrameErr(nil) = true, want false")
	}
	other := errors.New("write: broken pipe")
	if isMissingFrameErr(other) {
		t.Fatalf("isMissingFrameErr(%q) = true, want false", other.Error())
	}
	legacy := errors.New("backend: can't find pane: arc:0.7")
	if isMissingFrameErr(legacy) {
		t.Fatalf("isMissingFrameErr(legacy substring) = true, want false " +
			"after legacy substring fallback removal")
	}
}
