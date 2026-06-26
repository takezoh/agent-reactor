package driver

import (
	"errors"
	"strings"
	"testing"

	"github.com/takezoh/agent-reactor/client/state"
)

func TestApplySummaryJobResultClampsOversized(t *testing.T) {
	long := strings.Repeat("x", summaryDisplayCap+10)
	got, inFlight, ok := applySummaryJobResult("", true, state.DEvJobResult{
		Result: SummaryCommandResult{Summary: long},
	})
	if !ok {
		t.Fatal("expected ok=true for SummaryCommandResult")
	}
	if inFlight {
		t.Error("expected inFlight=false after job completion")
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected clamped summary to end with …, got %q", got)
	}
	if r := []rune(got); len(r) != summaryDisplayCap+1 {
		t.Errorf("expected %d code points (incl. trailing …), got %d (%q)",
			summaryDisplayCap+1, len(r), got)
	}
}

func TestApplySummaryJobResultPreservesShort(t *testing.T) {
	short := "fix auth"
	got, _, ok := applySummaryJobResult("", true, state.DEvJobResult{
		Result: SummaryCommandResult{Summary: short},
	})
	if !ok || got != short {
		t.Errorf("expected short summary unchanged, got ok=%v val=%q", ok, got)
	}
}

func TestApplySummaryJobResultKeepsPrevOnError(t *testing.T) {
	prev := "previous summary"
	got, inFlight, ok := applySummaryJobResult(prev, true, state.DEvJobResult{
		Result: SummaryCommandResult{Summary: "ignored"},
		Err:    errors.New("boom"),
	})
	if !ok {
		t.Fatal("expected ok=true even on err")
	}
	if inFlight {
		t.Error("expected inFlight=false after error too")
	}
	if got != prev {
		t.Errorf("expected previous summary preserved on err, got %q", got)
	}
}

func TestApplySummaryJobResultIgnoresUnknownResult(t *testing.T) {
	got, inFlight, ok := applySummaryJobResult("prev", true, state.DEvJobResult{
		Result: nil,
	})
	if ok {
		t.Error("expected ok=false when Result is not SummaryCommandResult")
	}
	if !inFlight {
		t.Error("expected inFlight unchanged when Result type mismatches")
	}
	if got != "prev" {
		t.Errorf("expected summary unchanged when Result type mismatches, got %q", got)
	}
}
