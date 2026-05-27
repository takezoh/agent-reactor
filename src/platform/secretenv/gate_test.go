package secretenv

import (
	"testing"
)

func TestContainerToHost(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{"no prefix (bare host)", "/home/u/proj/x.env", "", "/home/u/proj/x.env"},
		{"under prefix", "/mnt/home/u/proj/x.env", "/mnt", "/home/u/proj/x.env"},
		{"exact prefix", "/mnt", "/mnt", "/"},
		{"not under prefix (boundary safe: /mnternal)", "/mnternal/x.env", "/mnt", "/mnternal/x.env"},
		{"not under prefix (different path)", "/etc/x.env", "/mnt", "/etc/x.env"},
		{"nested prefix", "/devcontainer/workspace/proj/x.env", "/devcontainer/workspace", "/proj/x.env"},
		{"trailing slash in prefix", "/mnt/data/x.env", "/mnt/", "/data/x.env"},
		{"double trailing slash in prefix", "/mnt/data/x.env", "/mnt//", "/data/x.env"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := containerToHost(tc.path, tc.prefix)
			if got != tc.want {
				t.Errorf("containerToHost(%q, %q) = %q; want %q", tc.path, tc.prefix, got, tc.want)
			}
		})
	}
}

func TestGate_allow(t *testing.T) {
	g := NewGate([]string{"/home/user/project/*.env", "/etc/secrets/*"})

	if err := g.Check("/home/user/project/dev.env"); err != nil {
		t.Errorf("expected allow, got %v", err)
	}
	if err := g.Check("/etc/secrets/prod"); err != nil {
		t.Errorf("expected allow, got %v", err)
	}
}

func TestGate_deny(t *testing.T) {
	g := NewGate([]string{"/home/user/project/*.env"})

	if err := g.Check("/etc/passwd"); err == nil {
		t.Error("expected deny, got nil")
	}
	if err := g.Check("/home/user/other/dev.env"); err == nil {
		t.Error("expected deny, got nil")
	}
}

func TestGate_emptyAllowlist(t *testing.T) {
	g := NewGate(nil)
	if err := g.Check("/any/path.env"); err == nil {
		t.Error("expected deny on empty allowlist")
	}
}
