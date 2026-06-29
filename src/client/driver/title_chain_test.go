package driver

import "testing"

// TestCardTitleChain locks the Card Title/Subtitle priority chain implemented
// by resolveCardTitleSubtitle and applied across every driver's view().
//
//	Title    = aiTitle → summary → ""  (web fills "New Session" placeholder)
//	Subtitle = summary → lastPrompt → ""
//
// LastPrompt is never a Title candidate. UI layers dedup an exact-match
// Subtitle against Title; the helper itself keeps Subtitle populated so
// non-rendering consumers keep their label source.
func TestCardTitleChain(t *testing.T) {
	tests := []struct {
		name                         string
		aiTitle, summary, lastPrompt string
		wantTitle, wantSubtitle      string
	}{
		{"AI title wins, subtitle gets summary", "ai", "sum", "prompt", "ai", "sum"},
		{"AI title wins, subtitle falls to last prompt", "ai", "", "prompt", "ai", "prompt"},
		{"AI title wins, subtitle empty when no other content", "ai", "", "", "ai", ""},
		{"summary promotes; subtitle stays Summary so downstream consumers see it", "", "sum", "prompt", "sum", "sum"},
		{"summary promotes; subtitle still Summary even without last prompt", "", "sum", "", "sum", "sum"},
		{"last prompt alone stays in subtitle, never title", "", "", "prompt", "", "prompt"},
		{"all empty → web client owns New Session placeholder", "", "", "", "", ""},
		// Legacy multi-line summary: not a Title candidate (Title slot has no
		// per-line splitter, partial promotion would create dedup misses in
		// the Web subtitle renderer). Title stays empty so the Web
		// "New Session" placeholder takes over; Subtitle keeps the full
		// multi-line value for the legacy multi-line render path.
		{"multi-line summary: not a title candidate, full string stays in subtitle", "", "line1\nline2", "", "", "line1\nline2"},
		{"multi-line summary with AI title: title from AI, subtitle keeps full multi-line", "ai", "line1\nline2", "", "ai", "line1\nline2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, subtitle := resolveCardTitleSubtitle(tc.aiTitle, tc.summary, tc.lastPrompt)
			if title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", title, tc.wantTitle)
			}
			if subtitle != tc.wantSubtitle {
				t.Errorf("Subtitle = %q, want %q", subtitle, tc.wantSubtitle)
			}
		})
	}
}

// TestClaudeViewWiresTitleChain spot-checks that the Claude driver's view()
// actually routes through resolveCardTitleSubtitle (regression guard for the
// 5 driver wirings — one driver-level test stands in for the family, the
// per-case matrix lives on resolveCardTitleSubtitle directly).
func TestClaudeViewWiresTitleChain(t *testing.T) {
	d, cs, _ := newClaude(t)
	cs.Summary = "the summary"
	cs.LastPrompt = "the prompt"
	v := d.view(cs)
	if v.Card.Title != "the summary" {
		t.Errorf("Title = %q, want the summary (Summary should promote when AI title empty)", v.Card.Title)
	}
	// Subtitle stays = Summary even though Title now equals it; UI layer
	// (SessionList.subtitleText) hides the duplicate row.
	if v.Card.Subtitle != "the summary" {
		t.Errorf("Subtitle = %q, want the summary (data layer keeps it; UI dedups)", v.Card.Subtitle)
	}
}
