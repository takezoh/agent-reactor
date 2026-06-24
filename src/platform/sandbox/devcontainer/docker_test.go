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

// psFormatFor is the --format string we hand to "docker ps". Drift would silently
// break parsePsLine; pin it so a re-order shows up in review. The mount-hash
// label key is prefix-scoped: default ("") falls back to DefaultNamePrefix,
// and a custom prefix is reflected in the label key so peer daemons under
// different prefixes never observe each other's mount hash.
func TestPsFormat_StableContract(t *testing.T) {
	wantDefault := "{{.ID}}\t{{.State}}\t{{.Label \"reactor-mount-hash\"}}"
	gotDefault := psFormatFor("")
	if gotDefault != wantDefault {
		t.Errorf("psFormatFor(\"\") = %q\nwant                %q\n(parsePsLine assumes this exact column order)", gotDefault, wantDefault)
	}
	if strings.Count(gotDefault, "\t") != 2 {
		t.Errorf("psFormatFor must have exactly 2 tab separators, got %d in %q",
			strings.Count(gotDefault, "\t"), gotDefault)
	}
	wantCustom := "{{.ID}}\t{{.State}}\t{{.Label \"reactor-dev-mount-hash\"}}"
	if got := psFormatFor("reactor-dev"); got != wantCustom {
		t.Errorf("psFormatFor(\"reactor-dev\") = %q\nwant                          %q", got, wantCustom)
	}
}
