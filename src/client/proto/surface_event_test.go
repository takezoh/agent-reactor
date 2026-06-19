package proto

import (
	"os"
	"regexp"
	"testing"
)

// Compile-time assertions: both types must satisfy ServerEvent.
var _ ServerEvent = EvtSurfaceOutput{}
var _ ServerEvent = EvtPromptEvent{}

func TestEvtSurfaceOutputEventName(t *testing.T) {
	t.Parallel()
	got := EvtSurfaceOutput{}.EventName()
	if got != "surface-output" {
		t.Errorf("EventName() = %q; want %q", got, "surface-output")
	}
}

func TestEvtPromptEventEventName(t *testing.T) {
	t.Parallel()
	got := EvtPromptEvent{}.EventName()
	if got != "prompt-event" {
		t.Errorf("EventName() = %q; want %q", got, "prompt-event")
	}
}

// TestEvtSurfaceOutputSequenceGodoc verifies that the Sequence field
// carries the ADR 0010 documentation (subscribe-unit monotonic, reset on
// re-subscribe, no cross-subscribe continuity). This prevents silent
// removal of the design contract from the godoc.
func TestEvtSurfaceOutputSequenceGodoc(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("surface_event.go")
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	re := regexp.MustCompile(`(?s)Sequence.*?subscribe.*?(reset|単調|monotonic)`)
	if !re.MatchString(string(b)) {
		t.Error("surface_event.go: Sequence godoc does not contain ADR 0010 wording (subscribe + reset/単調/monotonic)")
	}
}
