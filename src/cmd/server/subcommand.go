package main

// commandKind names the three top-level dispatch arms recognised by runMain.
// Subcommand mode (`server event`, `server host-exec`, `server mcp-exec`)
// and help mode keep stdout pointed at the terminal; daemon mode routes
// stdout/stderr into the log file.
type commandKind int

const (
	commandKindCLI commandKind = iota
	commandKindDaemon
	commandKindHelp
)

func classifyCommand(args []string) commandKind {
	if len(args) == 0 {
		return commandKindDaemon
	}
	a := args[0]
	if isHelpCommand(a) {
		return commandKindHelp
	}
	if isSubcommandName(a) {
		return commandKindCLI
	}
	if isDaemonFlagToken(a) {
		return commandKindDaemon
	}
	// First arg is a positional non-subcommand: route to CLI so runCommand
	// emits the "unknown command" error rather than firing the daemon.
	return commandKindCLI
}

func isSubcommandName(name string) bool {
	switch name {
	case "event", "host-exec", "mcp-exec":
		return true
	}
	return false
}
