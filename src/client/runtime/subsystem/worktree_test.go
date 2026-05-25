package subsystem

import "testing"

func TestIsManagedWorktreePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/some/proj/.roost/worktrees/foo", true},
		{"/some/proj/.roost/worktrees/foo/", true},
		{"/some/proj/.roost/sessions/foo", false},
		{"/some/proj/worktrees/foo", false},
		{"/just/a/path", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsManagedWorktreePath(c.path); got != c.want {
			t.Errorf("IsManagedWorktreePath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestGenerateWorktreeNames(t *testing.T) {
	names := GenerateWorktreeNames(3)
	if len(names) != 3 {
		t.Fatalf("len = %d", len(names))
	}
	for _, n := range names {
		if n == "" {
			t.Errorf("empty name in %v", names)
		}
	}
}

func TestGenerateWorktreeNamesZero(t *testing.T) {
	names := GenerateWorktreeNames(0)
	if len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}
