package devcontainer

import (
	"strings"
	"testing"
)

func TestParsePsLine(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantID    string
		wantState string
		wantHash  string
		wantErr   bool
	}{
		{
			name:      "full row with mount-hash label",
			line:      "abc123\trunning\tdeadbeef0011",
			wantID:    "abc123",
			wantState: "running",
			wantHash:  "deadbeef0011",
		},
		{
			name:      "row without mount-hash (project-mode container)",
			line:      "xyz789\texited\t",
			wantID:    "xyz789",
			wantState: "exited",
			wantHash:  "",
		},
		{
			name:      "two-column row (older docker output without label column)",
			line:      "id1\tcreated",
			wantID:    "id1",
			wantState: "created",
			wantHash:  "",
		},
		{
			name:    "single column is malformed",
			line:    "id-only",
			wantErr: true,
		},
		{
			name:    "empty line is malformed",
			line:    "",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePsLine(tc.line)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.ID != tc.wantID || got.State != tc.wantState || got.MountHash != tc.wantHash {
				t.Errorf("got %+v, want ID=%q State=%q MountHash=%q",
					got, tc.wantID, tc.wantState, tc.wantHash)
			}
		})
	}
}

// psFormat is the --format string we hand to "docker ps". Drift would silently
// break parsePsLine; pin it so a re-order shows up in review.
func TestPsFormat_StableContract(t *testing.T) {
	want := "{{.ID}}\t{{.State}}\t{{.Label \"roost-mount-hash\"}}"
	if psFormat != want {
		t.Errorf("psFormat = %q\nwant      %q\n(parsePsLine assumes this exact column order)", psFormat, want)
	}
	// Sanity: it should produce three tab-separated fields.
	if strings.Count(psFormat, "\t") != 2 {
		t.Errorf("psFormat must have exactly 2 tab separators, got %d in %q",
			strings.Count(psFormat, "\t"), psFormat)
	}
}
