package codex

import (
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    CommandConfig
		wantErr bool
	}{
		{
			name:    "bare codex",
			command: "codex",
			want:    CommandConfig{ServerBin: "codex"},
		},
		{
			name:    "model flag",
			command: "codex -m gpt-4o",
			want:    CommandConfig{ServerBin: "codex", Model: "gpt-4o"},
		},
		{
			name:    "resume skips thread id",
			command: "codex resume abc-123",
			want:    CommandConfig{ServerBin: "codex"},
		},
		{
			name:    "config flag",
			command: "codex -c key=val",
			want:    CommandConfig{ServerBin: "codex", ServerArgs: []string{"-c", "key=val"}},
		},
		{
			name:    "unsupported command",
			command: "claude",
			wantErr: true,
		},
		{
			name:    "empty command",
			command: "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCommand(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.ServerBin != tt.want.ServerBin || got.Model != tt.want.Model {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if len(got.ServerArgs) != len(tt.want.ServerArgs) {
				t.Errorf("ServerArgs = %v, want %v", got.ServerArgs, tt.want.ServerArgs)
			}
		})
	}
}

func TestAppServerListenArgs(t *testing.T) {
	tests := []struct {
		name            string
		serverBin       string
		sock            string
		extra           []string
		sandboxExternal bool
		wantContains    []string
		wantAbsent      []string
	}{
		{
			name:         "basic no sandbox",
			serverBin:    "codex",
			sock:         "/tmp/codex-abc.sock",
			extra:        nil,
			wantContains: []string{"codex", "app-server", "--listen", "unix:///tmp/codex-abc.sock"},
			wantAbsent:   []string{"sandbox_mode"},
		},
		{
			name:            "with sandbox",
			serverBin:       "codex",
			sock:            "/run/codex-xyz.sock",
			sandboxExternal: true,
			wantContains:    []string{"-c", `sandbox_mode="danger-full-access"`},
		},
		{
			name:         "extra args passed through",
			serverBin:    "codex",
			sock:         "/s.sock",
			extra:        []string{"-c", "model=gpt-4o"},
			wantContains: []string{"-c", "model=gpt-4o"},
		},
		{
			name:            "sandbox_mode element not split",
			serverBin:       "codex",
			sock:            "/s.sock",
			sandboxExternal: true,
			// Verify the sandbox_mode value is a single element (no splitting on =)
			wantContains: []string{`sandbox_mode="danger-full-access"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AppServerListenArgs(tt.serverBin, tt.sock, tt.extra, tt.sandboxExternal)
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("AppServerListenArgs(...) = %v, missing element %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				for _, g := range got {
					if strings.Contains(g, absent) {
						t.Errorf("AppServerListenArgs(...) = %v, unexpected element containing %q", got, absent)
					}
				}
			}
		})
	}
}

func TestRemoteAttachArgs(t *testing.T) {
	tests := []struct {
		name         string
		bridgePort   int
		sessionID    string
		threadID     string
		startDir     string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "cold start no thread",
			bridgePort:   8282,
			sessionID:    "sess1",
			wantContains: []string{"codex", "--remote", "ws://127.0.0.1:8282/sess1"},
			wantAbsent:   []string{"resume"},
		},
		{
			name:         "warm start with thread",
			bridgePort:   8282,
			sessionID:    "sess2",
			threadID:     "thread-abc",
			wantContains: []string{"resume", "thread-abc", "--remote"},
		},
		{
			name:         "with startDir",
			bridgePort:   8282,
			sessionID:    "sess3",
			startDir:     "/workspace/foo",
			wantContains: []string{"-C", "/workspace/foo"},
		},
		{
			name:         "no startDir omits -C",
			bridgePort:   8282,
			sessionID:    "sess4",
			wantAbsent:   []string{"-C"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoteAttachArgs(tt.bridgePort, tt.sessionID, tt.threadID, tt.startDir)
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("RemoteAttachArgs(...) = %v, missing element %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				for _, g := range got {
					if g == absent {
						t.Errorf("RemoteAttachArgs(...) = %v, unexpected element %q", got, absent)
					}
				}
			}
		})
	}
}

func TestShellJoinArgv(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"simple", []string{"echo", "hello"}, "'echo' 'hello'"},
		{"single quote in value", []string{"echo", "it's"}, `'echo' 'it'\''s'`},
		{"sandbox_mode flag preserved", []string{"-c", `sandbox_mode="danger-full-access"`}, `'-c' 'sandbox_mode="danger-full-access"'`},
		{"empty", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShellJoinArgv(tt.args); got != tt.want {
				t.Errorf("ShellJoinArgv(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
