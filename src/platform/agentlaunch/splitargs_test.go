package agentlaunch

import (
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single word", "codex", []string{"codex"}, false},
		{"multiple words", "codex -m gpt-4o", []string{"codex", "-m", "gpt-4o"}, false},
		{"single quoted value", `codex -c 'sandbox_mode="danger-full-access"'`, []string{"codex", "-c", `sandbox_mode="danger-full-access"`}, false},
		{"double quoted value", `codex -c "sandbox_mode=full"`, []string{"codex", "-c", "sandbox_mode=full"}, false},
		{"backslash escape in double quote", `codex -c "foo=\"bar\""`, []string{"codex", "-c", `foo="bar"`}, false},
		{"unterminated single quote", "codex 'bad", nil, true},
		{"unterminated double quote", `codex "bad`, nil, true},
		{"extra whitespace", "  codex  -m  gpt-4o  ", []string{"codex", "-m", "gpt-4o"}, false},
		{"tab separator", "codex\t-m\tgpt-4o", []string{"codex", "-m", "gpt-4o"}, false},
		{"adjacent quotes", `foo''bar`, []string{"foobar"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SplitArgs(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SplitArgs(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("SplitArgs(%q)[%d] = %q, want %q", tt.input, i, got[i], w)
				}
			}
		})
	}
}
