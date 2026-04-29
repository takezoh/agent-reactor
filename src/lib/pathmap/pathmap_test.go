package pathmap_test

import (
	"testing"

	"github.com/takezoh/agent-roost/lib/pathmap"
)

func TestToHost(t *testing.T) {
	ms := pathmap.Mounts{
		{Host: "/home/u/myapp", Container: "/workspaces/myapp"},
	}

	cases := []struct {
		name      string
		container string
		wantHost  string
		wantOK    bool
	}{
		{"exact root", "/workspaces/myapp", "/home/u/myapp", true},
		{"sub dir", "/workspaces/myapp/backend/api", "/home/u/myapp/backend/api", true},
		{"trailing slash input", "/workspaces/myapp/", "/home/u/myapp", true},
		{"no mount", "/var/log/x", "", false},
		{"dotdot escape", "/workspaces/myapp/../etc", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ms.ToHost(tc.container)
			if ok != tc.wantOK || got != tc.wantHost {
				t.Errorf("ToHost(%q) = (%q, %v), want (%q, %v)", tc.container, got, ok, tc.wantHost, tc.wantOK)
			}
		})
	}
}

func TestToContainer(t *testing.T) {
	ms := pathmap.Mounts{
		{Host: "/home/u/myapp", Container: "/workspaces/myapp"},
	}

	cases := []struct {
		name          string
		host          string
		wantContainer string
		wantOK        bool
	}{
		{"exact root", "/home/u/myapp", "/workspaces/myapp", true},
		{"sub dir", "/home/u/myapp/src/main.go", "/workspaces/myapp/src/main.go", true},
		{"no mount", "/home/u/other/file", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ms.ToContainer(tc.host)
			if ok != tc.wantOK || got != tc.wantContainer {
				t.Errorf("ToContainer(%q) = (%q, %v), want (%q, %v)", tc.host, got, ok, tc.wantContainer, tc.wantOK)
			}
		})
	}
}

func TestLongestPrefixMatch(t *testing.T) {
	// /workspaces/foo is mounted from /home/u/foo
	// /workspaces/foo/cache is mounted from /data/cache (nested mount)
	ms := pathmap.Mounts{
		{Host: "/home/u/foo", Container: "/workspaces/foo"},
		{Host: "/data/cache", Container: "/workspaces/foo/cache"},
	}

	cases := []struct {
		container string
		wantHost  string
		wantOK    bool
	}{
		{"/workspaces/foo/src/main.go", "/home/u/foo/src/main.go", true},
		{"/workspaces/foo/cache/x.log", "/data/cache/x.log", true},
		{"/workspaces/foo/cache", "/data/cache", true},
		{"/workspaces/foo", "/home/u/foo", true},
	}
	for _, tc := range cases {
		t.Run(tc.container, func(t *testing.T) {
			got, ok := ms.ToHost(tc.container)
			if ok != tc.wantOK || got != tc.wantHost {
				t.Errorf("ToHost(%q) = (%q, %v), want (%q, %v)", tc.container, got, ok, tc.wantHost, tc.wantOK)
			}
		})
	}
}

func TestEmptyMountsReturnsNotFound(t *testing.T) {
	var ms pathmap.Mounts
	if _, ok := ms.ToHost("/workspaces/foo"); ok {
		t.Error("empty Mounts.ToHost should return false")
	}
	if _, ok := ms.ToContainer("/home/u/foo"); ok {
		t.Error("empty Mounts.ToContainer should return false")
	}
}
