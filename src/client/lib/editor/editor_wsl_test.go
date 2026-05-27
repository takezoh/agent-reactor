package editor

import (
	"testing"
)

func TestWslDistro_set(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "MyDistro")
	if got := wslDistro(); got != "MyDistro" {
		t.Errorf("wslDistro() = %q, want %q", got, "MyDistro")
	}
}

func TestWslDistro_unset(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	if got := wslDistro(); got != "" {
		t.Errorf("wslDistro() = %q, want empty", got)
	}
}

func TestHasRemoteFlag(t *testing.T) {
	cases := []struct {
		parts []string
		want  bool
	}{
		{[]string{"code"}, false},
		{[]string{"code", "--reuse-window"}, false},
		{[]string{"code", "--remote", "wsl+Ubuntu"}, true},
	}
	for _, c := range cases {
		if got := hasRemoteFlag(c.parts); got != c.want {
			t.Errorf("hasRemoteFlag(%v) = %v, want %v", c.parts, got, c.want)
		}
	}
}
