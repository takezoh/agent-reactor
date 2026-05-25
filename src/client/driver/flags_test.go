package driver

import "testing"

func TestHasFlagToken(t *testing.T) {
	cases := []struct {
		command string
		flag    string
		want    bool
	}{
		{"claude --foo", "--foo", true},
		{"claude --foo=bar", "--foo", true},
		{"claude --foobar", "--foo", false},
		{"claude", "--foo", false},
		{"", "--foo", false},
	}
	for _, tc := range cases {
		got := hasFlagToken(tc.command, tc.flag)
		if got != tc.want {
			t.Errorf("hasFlagToken(%q, %q) = %v, want %v", tc.command, tc.flag, got, tc.want)
		}
	}
}

func TestStripFlagToken(t *testing.T) {
	cases := []struct {
		command string
		flag    string
		want    string
	}{
		// basic removal
		{"claude --enable-auto-mode", "--enable-auto-mode", "claude"},
		// multiple occurrences removed
		{"claude --enable-auto-mode --foo --enable-auto-mode", "--enable-auto-mode", "claude --foo"},
		// = form is NOT removed
		{"claude --enable-auto-mode=true", "--enable-auto-mode", "claude --enable-auto-mode=true"},
		// flag not present: no-op
		{"claude --foo", "--enable-auto-mode", "claude --foo"},
		// empty command
		{"", "--enable-auto-mode", ""},
		// flag in the middle
		{"claude --enable-auto-mode --worktree", "--enable-auto-mode", "claude --worktree"},
	}
	for _, tc := range cases {
		got := stripFlagToken(tc.command, tc.flag)
		if got != tc.want {
			t.Errorf("stripFlagToken(%q, %q) = %q, want %q", tc.command, tc.flag, got, tc.want)
		}
	}
}
