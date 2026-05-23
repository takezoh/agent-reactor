package scheduler

import (
	"testing"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/platform/tracker"
)

func cfg2() wfconfig.Config {
	return wfconfig.Config{
		Tracker: wfconfig.TrackerConfig{
			ActiveStates:   []string{"In Progress", "Todo"},
			TerminalStates: []string{"Done", "Cancelled"},
		},
	}
}

func baseIssue() tracker.Issue {
	return tracker.Issue{
		ID:         "id1",
		Identifier: "P-1",
		Title:      "title",
		State:      "In Progress",
	}
}

func emptySnap() StateSnapshot {
	return StateSnapshot{
		Running: map[string]RunAttempt{},
		Claimed: map[string]struct{}{},
	}
}

// TestEligible_RequiredFields verifies issues missing id/identifier/title/state are excluded.
func TestEligible_RequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*tracker.Issue)
	}{
		{"no ID", func(i *tracker.Issue) { i.ID = "" }},
		{"no Identifier", func(i *tracker.Issue) { i.Identifier = "" }},
		{"no Title", func(i *tracker.Issue) { i.Title = "" }},
		{"no State", func(i *tracker.Issue) { i.State = "" }},
	}
	cfg := cfg2()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			iss := baseIssue()
			tc.mutate(&iss)
			got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg)
			if len(got) != 0 {
				t.Errorf("want 0 eligible, got %d", len(got))
			}
		})
	}
}

// TestEligible_StateFilters verifies active/terminal state filtering.
func TestEligible_StateFilters(t *testing.T) {
	cfg := cfg2()

	t.Run("active state passes", func(t *testing.T) {
		iss := baseIssue()
		iss.State = "In Progress"
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg); len(got) != 1 {
			t.Errorf("want 1, got %d", len(got))
		}
	})
	t.Run("non-active state excluded", func(t *testing.T) {
		iss := baseIssue()
		iss.State = "Backlog"
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg); len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
	t.Run("terminal state excluded", func(t *testing.T) {
		// Done is active but also terminal; terminal takes priority.
		cfg2 := cfg
		cfg2.Tracker.ActiveStates = append(cfg2.Tracker.ActiveStates, "Done")
		iss := baseIssue()
		iss.State = "Done"
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg2); len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
}

// TestEligible_RunningAndClaimed verifies already-running/claimed issues are excluded.
func TestEligible_RunningAndClaimed(t *testing.T) {
	cfg := cfg2()
	iss := baseIssue()

	t.Run("running excluded", func(t *testing.T) {
		snap := emptySnap()
		snap.Running["id1"] = RunAttempt{}
		if got := filterEligible([]tracker.Issue{iss}, snap, cfg); len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
	t.Run("claimed excluded", func(t *testing.T) {
		snap := emptySnap()
		snap.Claimed["id1"] = struct{}{}
		if got := filterEligible([]tracker.Issue{iss}, snap, cfg); len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
}

// TestEligible_RetryAttempts verifies that RetryQueued issues (in retryAttempts) are
// excluded from dispatch — defense-in-depth for SPEC §7.4 (double-dispatch prevention).
func TestEligible_RetryAttempts(t *testing.T) {
	cfg := cfg2()
	iss := baseIssue()

	snap := StateSnapshot{
		Running:       map[string]RunAttempt{},
		Claimed:       map[string]struct{}{},
		RetryAttempts: map[string]RetryEntry{"id1": {IssueID: "id1"}},
	}
	if got := filterEligible([]tracker.Issue{iss}, snap, cfg); len(got) != 0 {
		t.Errorf("want 0 eligible for RetryQueued issue, got %d", len(got))
	}
}

// TestEligible_BlockerRule verifies the Todo+blocker exclusion per SPEC §8.2.
func TestEligible_BlockerRule(t *testing.T) {
	cfg := cfg2()

	t.Run("Todo with non-terminal blocker excluded", func(t *testing.T) {
		iss := baseIssue()
		iss.State = "Todo"
		iss.BlockedBy = []tracker.Blocker{{ID: "b1", Identifier: "P-0", State: "In Progress"}}
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg); len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
	t.Run("Todo with terminal blocker passes", func(t *testing.T) {
		iss := baseIssue()
		iss.State = "Todo"
		iss.BlockedBy = []tracker.Blocker{{ID: "b1", Identifier: "P-0", State: "Done"}}
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg); len(got) != 1 {
			t.Errorf("want 1, got %d", len(got))
		}
	})
	t.Run("non-Todo with non-terminal blocker passes", func(t *testing.T) {
		iss := baseIssue()
		iss.State = "In Progress"
		iss.BlockedBy = []tracker.Blocker{{ID: "b1", Identifier: "P-0", State: "In Progress"}}
		if got := filterEligible([]tracker.Issue{iss}, emptySnap(), cfg); len(got) != 1 {
			t.Errorf("want 1, got %d", len(got))
		}
	})
}

func ptr(v int) *int { return &v }

// TestSortCandidates verifies priority/created_at/identifier ordering per SPEC §8.2.
func TestSortCandidates(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		input []tracker.Issue
		want  []string // expected Identifier order
	}{
		{
			name: "priority ascending, nil last",
			input: []tracker.Issue{
				{ID: "c", Identifier: "P-3", Priority: nil, CreatedAt: base},
				{ID: "b", Identifier: "P-2", Priority: ptr(2), CreatedAt: base},
				{ID: "a", Identifier: "P-1", Priority: ptr(1), CreatedAt: base},
			},
			want: []string{"P-1", "P-2", "P-3"},
		},
		{
			name: "same priority, created_at ascending",
			input: []tracker.Issue{
				{ID: "b", Identifier: "P-2", Priority: ptr(1), CreatedAt: base.Add(time.Hour)},
				{ID: "a", Identifier: "P-1", Priority: ptr(1), CreatedAt: base},
			},
			want: []string{"P-1", "P-2"},
		},
		{
			name: "same priority and created_at, identifier ascending",
			input: []tracker.Issue{
				{ID: "b", Identifier: "P-2", Priority: ptr(1), CreatedAt: base},
				{ID: "a", Identifier: "P-1", Priority: ptr(1), CreatedAt: base},
			},
			want: []string{"P-1", "P-2"},
		},
		{
			name: "multiple nil priorities stable by created_at",
			input: []tracker.Issue{
				{ID: "b", Identifier: "P-2", Priority: nil, CreatedAt: base.Add(time.Hour)},
				{ID: "a", Identifier: "P-1", Priority: nil, CreatedAt: base},
			},
			want: []string{"P-1", "P-2"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortCandidates(tc.input)
			for i, ident := range tc.want {
				if got[i].Identifier != ident {
					t.Errorf("pos %d: want %s, got %s", i, ident, got[i].Identifier)
				}
			}
		})
	}
}
