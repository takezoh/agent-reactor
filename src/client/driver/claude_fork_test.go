package driver

import (
	"strings"
	"testing"
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
