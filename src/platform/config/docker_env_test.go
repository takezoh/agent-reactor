package config

import "testing"

func TestResolveDockerHost(t *testing.T) {
	exists := func(path string) bool { return true }
	absent := func(path string) bool { return false }

	cases := []struct {
		name          string
		envDockerHost string
		xdgRuntimeDir string
		socketExists  func(string) bool
		want          string
	}{
		{
			name:          "DOCKER_HOST already set",
			envDockerHost: "tcp://remote:2376",
			xdgRuntimeDir: "/run/user/1000",
			socketExists:  exists,
			want:          "",
		},
		{
			name:          "XDG_RUNTIME_DIR set and socket exists",
			envDockerHost: "",
			xdgRuntimeDir: "/run/user/1000",
			socketExists:  exists,
			want:          "unix:///run/user/1000/docker.sock",
		},
		{
			name:          "XDG_RUNTIME_DIR set but socket absent",
			envDockerHost: "",
			xdgRuntimeDir: "/run/user/1000",
			socketExists:  absent,
			want:          "",
		},
		{
			name:          "both empty",
			envDockerHost: "",
			xdgRuntimeDir: "",
			socketExists:  exists,
			want:          "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveDockerHost(tc.envDockerHost, tc.xdgRuntimeDir, tc.socketExists)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
