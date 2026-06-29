package codex

import (
	"fmt"
	"strings"
)

// Driver-level constants shared by all layers.
const (
	DriverName = "codex"
	SockPrefix = "codex-"
	SockSuffix = ".sock"
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
			i++ // skip resume target; actual locator comes from the launch plan
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

// RemoteAttachArgs returns the argv for the TUI frame that attaches to the
// per-session app-server over its unix domain socket (`codex --remote unix://<sock>`).
// sock is the container-absolute UDS path the app-server binds; the TUI runs in the
// same sandbox, so it connects to that socket directly (no TCP routing bridge).
//
// Thread selection belongs to the app-server/backend. The foreground TUI only
// connects to the remote endpoint; passing `resume <id>` here would route
// through Codex's saved-session CLI path instead of the already-bound remote
// thread.
func RemoteAttachArgs(sock, startDir string) []string {
	args := []string{DriverName}
	args = append(args, "--remote", "unix://"+sock, "--dangerously-bypass-approvals-and-sandbox")
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
