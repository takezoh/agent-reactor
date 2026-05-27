package driver

import (
	"strings"
	"testing"
	"time"
)

func TestClaudeForkCommand(t *testing.T) {
	d, cs, now := newClaude(t)
	_ = now

	tests := []struct {
		name        string
		sessionID   string
		base        string
		wantOK      bool
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "valid session id",
			sessionID:   "abc-123",
			base:        "claude",
			wantOK:      true,
			wantContain: []string{"--resume", "abc-123", "--fork-session"},
		},
		{
			name:      "empty session id",
			sessionID: "",
			base:      "claude",
			wantOK:    false,
		},
		{
			name:      "invalid session id (space)",
			sessionID: "abc 123",
			base:      "claude",
			wantOK:    false,
		},
		{
			name:        "worktree flag stripped",
			sessionID:   "ses-456",
			base:        "claude --worktree feature",
			wantOK:      true,
			wantContain: []string{"--resume", "ses-456", "--fork-session"},
			wantAbsent:  []string{"--worktree"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cs.ClaudeSessionID = tc.sessionID
			cmd, ok := d.ForkCommand(cs, tc.base)
			if ok != tc.wantOK {
				t.Fatalf("ForkCommand ok=%v, want %v (cmd=%q)", ok, tc.wantOK, cmd)
			}
			if !tc.wantOK {
				return
			}
			for _, want := range tc.wantContain {
				if !strings.Contains(cmd, want) {
					t.Errorf("ForkCommand = %q: missing %q", cmd, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(cmd, absent) {
					t.Errorf("ForkCommand = %q: should not contain %q", cmd, absent)
				}
			}
		})
	}
}

func TestClaudeForkChildState(t *testing.T) {
	d, _, now := newClaude(t)

	t.Run("seeds parent id", func(t *testing.T) {
		parent := ClaudeState{ClaudeSessionID: "parent-id-abc"}
		child := d.ForkChildState(parent, now).(ClaudeState)
		if child.ClaudeSessionID != "" {
			t.Errorf("ForkChildState should not copy ClaudeSessionID, got %q", child.ClaudeSessionID)
		}
		if child.ForkParentID != "parent-id-abc" {
			t.Errorf("ForkParentID = %q, want %q", child.ForkParentID, "parent-id-abc")
		}
	})

	t.Run("empty parent id yields no ForkParentID", func(t *testing.T) {
		parent := ClaudeState{ClaudeSessionID: ""}
		child := d.ForkChildState(parent, now).(ClaudeState)
		if child.ForkParentID != "" {
			t.Errorf("ForkParentID = %q, want empty", child.ForkParentID)
		}
	})

	t.Run("wrong parent type yields empty state", func(t *testing.T) {
		child := d.ForkChildState(nil, time.Time{}).(ClaudeState)
		if child.ForkParentID != "" || child.ClaudeSessionID != "" {
			t.Errorf("unexpected non-empty fields in child state from nil parent")
		}
	})
}
