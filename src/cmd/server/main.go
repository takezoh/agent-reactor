// Command server is the agent-reactor backend: one binary, one resident
// process, that owns both the pty session daemon (typed proto IPC over a
// Unix socket under -data-dir) and the HTTP/WS gateway translating browser
// REST/WS traffic into in-process daemon calls.
//
// Runtime files under -data-dir (default ~/.agent-reactor) follow the
// `server.*` naming defined in platform/appid: server.sock, server.pid,
// server.log.
//
// Modes
//
//  1. Daemon mode (default; no subcommand): boots the coordinator (event
//     loop, host + container IPC listeners, persistence) AND a co-resident
//     gateway goroutine that dials the same socket and listens on -addr.
//     Coordinator and gateway share one ctx; SIGINT/SIGTERM cancels both.
//
//  2. Subcommand mode (one-shot, exits when done):
//     - `server event <type>` — hook event sender. The Claude and Gemini
//     hooks the daemon registers at startup (via client/lib/agenthook)
//     invoke this to forward state-change/Stop/PreToolUse/... events to a
//     running backend over its host socket.
//     - `server host-exec <bin> [args ...]` — in-container shim that
//     routes an allowlisted host binary invocation through the
//     per-project host-exec broker via SCM_RIGHTS stdio forwarding.
//     - `server mcp-exec <alias>` — in-container shim that relays MCP
//     stdio to a host-side MCP server through the per-project
//     mcp-proxy broker.
//     - `server help` / `server -h` prints usage.
//
// Mixing a coordinator flag (e.g. -data-dir) with a subcommand on the same
// command line is rejected by Go's flag package because subcommands consume
// args[0:] without inheriting the daemon flag set.
//
// # Panic recovery
//
// Panics are contained at three boundaries so one bad request cannot tear
// down the backend:
//   - runMain installs a defer-recover that converts a panic on the main
//     goroutine into a logged error + non-zero exit instead of a process
//     abort.
//   - fd 2 is dup'd to the slog target so a panic on a non-main goroutine —
//     which bypasses runMain's recover() — still lands in server.log
//     instead of vanishing onto a possibly-detached terminal.
//   - The gateway serve goroutine in cmd/server/gateway.go and the
//     coordinator goroutine in cmd/server/coordinator.go each install their
//     own recover() that logs the stack and cancels the shared ctx so the
//     other side shuts down cleanly instead of being left half-running.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/event"
	"github.com/takezoh/agent-reactor/client/procio"
	"github.com/takezoh/agent-reactor/platform/appid"
	"github.com/takezoh/agent-reactor/platform/logger"
)

var (
	loadBootstrapConfig   = config.Load
	initLoggerWithDataDir = logger.InitWithDataDir
	closeLogger           = logger.Close
	redirectStderr        = logger.RedirectStderr
	parseDaemonArgsFn     = parseDaemonArgs
	runDaemonFn           = runDaemon
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) (code int) {
	kind := classifyCommand(args)
	cfg, cfgErr := loadBootstrapConfig()

	// Resolve the daemon flag set once at the top so EVERY downstream call to
	// config.ResolveDataDir() returns the flag-specified path. We export
	// ROOST_DATA_DIR (the highest-precedence branch inside ResolveDataDir) so
	// the flag wins over a stale shell env (systemd --user inherits the user's
	// env, so `export ROOST_DATA_DIR=…` in a developer's rc would otherwise
	// silently override the unit's explicit ExecStart= -data-dir).
	//
	// Parse runs regardless of cfgErr / cfg==nil so a malformed settings.toml
	// never hides a bad daemon flag from the operator.
	var dFlags *daemonFlagSet
	if kind == commandKindDaemon {
		parsed, err := parseDaemonArgsFn(args)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", appid.ClientBin, err)
			return 2
		}
		dFlags = parsed
		if parsed.dataDir != "" {
			_ = os.Setenv("ROOST_DATA_DIR", parsed.dataDir)
			if cfg != nil {
				cfg.DataDir = parsed.dataDir
			}
		}
	}

	loggerReady, loggerErr := initMainLogger(cfg, kind == commandKindDaemon)
	if loggerReady {
		defer closeLogger()
	}
	defer func() {
		if rec := recover(); rec != nil {
			err := fmt.Errorf("panic: %v", rec)
			if loggerReady {
				slog.Error("panic recovered", "err", err)
			}
			code = finishMain(kind, err, loggerReady, loggerErr, stdout, stderr)
		}
	}()

	if loggerErr != nil {
		return finishMain(kind, loggerErr, false, loggerErr, stdout, stderr)
	}
	switch kind {
	case commandKindCLI, commandKindHelp:
		procio.UseTerminal()
	case commandKindDaemon:
		procio.UseLogFile(logger.LogFile())
		// Dup fd 2 to the log file so goroutine panics (which bypass the
		// main-goroutine recover() and write the stack trace straight to
		// stderr) land in the log instead of vanishing onto a terminal that
		// may or may not still be there.
		redirectStderr()
	default:
	}
	if cfgErr != nil {
		slog.Error("config load failed during logger bootstrap", "err", cfgErr)
	}

	err := runCommand(args, dFlags, stdout)
	if err != nil {
		slog.Error("main failed", "err", err)
	}
	return finishMain(kind, err, true, nil, stdout, stderr)
}

func finishMain(kind commandKind, err error, loggerReady bool, loggerErr error, stdout, stderr io.Writer) int {
	if kind == commandKindDaemon {
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", appid.ClientBin, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: exited\n", appid.ClientBin)
		return 0
	}
	if !loggerReady && loggerErr != nil {
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func initMainLogger(cfg *config.Config, rotate bool) (bool, error) {
	level := "info"
	dataDir := ""
	if cfg != nil {
		level = cfg.Log.Level
		dataDir = cfg.ResolveDataDir()
	}
	if rotate {
		logger.Rotate(dataDir)
	}
	if err := initLoggerWithDataDir(level, dataDir); err != nil {
		return false, err
	}
	return true, nil
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func runCommand(args []string, df *daemonFlagSet, stdout io.Writer) error {
	if len(args) == 0 || isDaemonFlagToken(args[0]) {
		return runDaemonFn(df)
	}
	if isHelpCommand(args[0]) {
		printUsage(stdout)
		return nil
	}
	switch args[0] {
	case "event":
		return event.Run(args[1:])
	case "host-exec":
		return runHostExec(args[1:])
	case "mcp-exec":
		return runMCPExec(args[1:])
	}
	return fmt.Errorf("unknown command: %s (run `%s help` for usage)", args[0], appid.ClientBin)
}

// isDaemonFlagToken reports whether arg looks like a flag for the daemon flag
// set. We treat any leading-`-` token (other than help) as belonging to the
// daemon: the flag parser will reject unknown flags loudly, so a typo never
// silently falls through to "unknown subcommand".
func isDaemonFlagToken(arg string) bool {
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		return false
	}
	return !isHelpCommand(arg)
}

func isHelpCommand(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "%s - agent-reactor backend (daemon + HTTP/WS gateway)\n", appid.ClientBin)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s                              Run the daemon and HTTP/WS gateway\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -data-dir <path>             Run with custom data dir (socket / sessions / pid)\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -addr <host:port>            Gateway listen address (default :8443)\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -token <bearer>              Set the bearer token (generated if omitted)\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -token-file <path>           Persisted-token file (generated on first boot)\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -tls-cert <path> -tls-key <path>   Use the supplied TLS material\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -insecure                    Serve plain HTTP (local dev only)\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s -no-auth                     Disable bearer + WS-ticket auth (loopback only)\n", appid.ClientBin)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintf(w, "  %s event <type>                 Send an event to a running daemon\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s host-exec <binary> [args...] Run a host binary via the hostexec broker\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s mcp-exec <alias>             Relay stdio to a host MCP server\n", appid.ClientBin)
	fmt.Fprintf(w, "  %s help                         Show this help message\n", appid.ClientBin)
}
