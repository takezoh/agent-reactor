package codex

import (
	"fmt"
	"strings"
)

// Driver-level constants shared by all layers.
const (
	DriverName   = "codex"
	SockPrefix   = "codex-"
	SockSuffix   = ".sock"
	LoopbackPort = 8282
)

// CommandConfig is the parsed form of a codex launch command string.
type CommandConfig struct {
	ServerBin  string
	ServerArgs []string
	Model      string
}

// ParseCommand parses a pre-tokenized codex argv into a CommandConfig.
// The caller is responsible for tokenizing the command string (e.g. via
// agentlaunch.SplitArgs) before passing it here.
func ParseCommand(argv []string) (CommandConfig, error) {
	if len(argv) == 0 || argv[0] != DriverName {
		return CommandConfig{}, fmt.Errorf("codex: unsupported command %q", strings.Join(argv, " "))
	}
	cfg := CommandConfig{ServerBin: DriverName}
	for i := 1; i < len(argv); i++ {
		arg := argv[i]
		switch arg {
		case "resume":
			i++ // skip thread ID; resume target comes from plan.Stream.ResumeThreadID
		case "-m", "--model":
			if i+1 < len(argv) {
				cfg.Model = argv[i+1]
				i++
			}
		case "-c", "--config", "--enable", "--disable":
			if i+1 < len(argv) {
				cfg.ServerArgs = append(cfg.ServerArgs, arg, argv[i+1])
				i++
			}
		}
	}
	return cfg, nil
}

// AppServerListenArgs returns the argv for `codex app-server --listen unix://<sock>`.
// extra is passed through verbatim (e.g. ["-c", "key=val"] config overrides).
// When sandboxExternal is true, -c sandbox_mode="danger-full-access" is appended.
func AppServerListenArgs(serverBin, sock string, extra []string, sandboxExternal bool) []string {
	args := []string{serverBin, "app-server", "--listen", "unix://" + sock}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// AppServerStdioArgs returns the argv for `codex app-server` (stdio transport, no --listen).
func AppServerStdioArgs(extra []string, sandboxExternal bool) []string {
	args := []string{DriverName, "app-server"}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// RemoteAttachArgs returns the argv for the TUI pane that attaches to the app-server
// via the routing sockbridge listener at ws://127.0.0.1:<bridgePort>/<sessionID>.
//
// Cold start (threadID == ""): `codex --remote ...` so the TUI creates the thread.
// Warm start uses `codex resume <id> --remote ...`.
func RemoteAttachArgs(bridgePort int, sessionID, threadID, startDir string) []string {
	remote := fmt.Sprintf("ws://127.0.0.1:%d/%s", bridgePort, sessionID)
	args := []string{DriverName}
	if threadID != "" {
		args = append(args, "resume", threadID)
	}
	args = append(args, "--remote", remote, "--dangerously-bypass-approvals-and-sandbox")
	if startDir != "" {
		args = append(args, "-C", startDir)
	}
	return args
}

// ShellJoinArgv single-quote-escapes each element and joins with spaces,
// producing a string safe for embedding inside a shell command (e.g. docker exec bash -lc '...').
func ShellJoinArgv(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(parts, " ")
}
