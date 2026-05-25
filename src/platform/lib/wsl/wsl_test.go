package wsl

import "testing"

func TestIsWSLEnv(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	if !IsWSL() {
		t.Errorf("IsWSL with env set = false")
	}
}
