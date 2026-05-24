package stream

import (
	"fmt"
	"strings"

	"github.com/takezoh/agent-roost/client/driver"
	"github.com/takezoh/agent-roost/client/state"
)

// Re-exported from driver/ so callers need not import both packages.
const (
	DriverName   = driver.CodexDriverName
	SockPrefix   = driver.CodexAppServerSockPrefix
	SockSuffix   = driver.CodexAppServerSockSuffix
	LoopbackPort = driver.CodexAppServerLoopbackPort
	// RunDirName is the subdirectory under the daemon data dir that holds
	// per-session codex app-server UDS files: <dataDir>/run/<RunDirName>/.
	RunDirName = driver.CodexDriverName
)

// CommandConfig is the parsed form of a codex launch command string.
type CommandConfig struct {
	ServerBin  string
	ServerArgs []string
	Model      string
}

// ParseCommand parses a codex launch command string into a CommandConfig.
func ParseCommand(command string) (CommandConfig, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 || fields[0] != DriverName {
		return CommandConfig{}, fmt.Errorf("stream backend: unsupported command %q", command)
	}
	cfg := CommandConfig{ServerBin: DriverName}
	for i := 1; i < len(fields); i++ {
		arg := fields[i]
		switch arg {
		case "resume":
			// Skip thread ID; resume target comes from plan.Stream.ResumeThreadID.
			i++
		case "-m", "--model":
			if i+1 < len(fields) {
				cfg.Model = fields[i+1]
				i++
			}
		case "-c", "--config", "--enable", "--disable":
			if i+1 < len(fields) {
				cfg.ServerArgs = append(cfg.ServerArgs, arg, fields[i+1])
				i++
			}
		}
	}
	return cfg, nil
}

func buildServerArgs(extra []string, sandboxExternal bool, sockPath string) []string {
	args := []string{"app-server", "--listen", "unix://" + sockPath}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// BuildRemoteCommand assembles the pane command that attaches the codex TUI
// to the session's app-server via the routing sockbridge listener.
//
// The bridge listens on a fixed port and routes connections by URL path:
// ws://127.0.0.1:<port>/<sessionID>. The bridge rewrites the path to "/"
// before forwarding to the per-session UDS, so the app-server always sees "/".
//
// Cold start (threadID == ""): `codex --remote ...` so the TUI creates the
// thread. Warm start uses `codex resume <id> --remote ...`.
func BuildRemoteCommand(bridgePort int, sessionID state.SessionID, threadID, startDir string) string {
	remote := fmt.Sprintf("ws://127.0.0.1:%d/%s", bridgePort, string(sessionID))
	args := []string{DriverName}
	if threadID != "" {
		args = append(args, "resume", threadID)
	}
	args = append(args, "--remote", remote, "--dangerously-bypass-approvals-and-sandbox")
	if startDir != "" {
		args = append(args, "-C", startDir)
	}
	return strings.Join(args, " ")
}

// shellJoinArgv single-quote-escapes each element and joins with spaces.
func shellJoinArgv(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(parts, " ")
}

// prefixWriter is an io.Writer that captures up to max bytes into dst.
type prefixWriter struct {
	dst *strings.Builder
	max int
}

func newPrefixWriter(dst *strings.Builder, max int) *prefixWriter {
	return &prefixWriter{dst: dst, max: max}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	if p.dst.Len() < p.max {
		room := p.max - p.dst.Len()
		if room > len(b) {
			room = len(b)
		}
		p.dst.Write(b[:room])
	}
	return len(b), nil
}
