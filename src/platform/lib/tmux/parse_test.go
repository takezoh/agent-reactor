package tmux

import (
	"reflect"
	"testing"
)

func TestParseRoostWindows(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []RoostWindow
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single",
			in:   "@5\troost-1\n",
			want: []RoostWindow{{WindowID: "@5", ID: "roost-1"}},
		},
		{
			name: "multiple with blanks and untagged",
			in:   "@5\troost-1\n@7\t\n\n@9\troost-2\n",
			want: []RoostWindow{
				{WindowID: "@5", ID: "roost-1"},
				{WindowID: "@9", ID: "roost-2"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseRoostWindows(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("my-session")
	if c.SessionName != "my-session" {
		t.Errorf("SessionName = %q", c.SessionName)
	}
	if c.defaultTimeout == 0 {
		t.Errorf("defaultTimeout zero")
	}
}
