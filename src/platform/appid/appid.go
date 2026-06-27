// Package appid is the single source of truth for the application's on-disk
// and process identity: binary/command names, directory names, container
// paths, and runtime file names. Callers must reference these constants rather
// than hard-coding the literals, so renaming the app touches exactly one file.
//
// Two tokens span the identity:
//   - Name ("agent-reactor") — the project name, used for directories
//     (~/.agent-reactor, .agent-reactor/, /opt/agent-reactor, ~/.local/lib/agent-reactor).
//   - ClientBin ("server") — the backend binary name, used for the runtime
//     files it owns (server.sock/.pid/.log).
//
// Note: the IPC/runtime env-var contract (ROOST_* names) and the persisted
// JSON key roost_session_id are intentionally NOT defined here — they are a
// separate wire/contract surface kept stable for backward compatibility.
package appid

const (
	// Name is the project/app name, used for on-disk directories.
	Name = "agent-reactor"

	// ClientBin is the backend daemon binary and command name.
	// (The historical "arc" TUI command was removed in phase F-E; the
	// identifier is kept to minimise caller churn, but its literal now
	// matches the cmd/server binary.)
	ClientBin = "server"

	// BridgeBin is the in-container helper binary name (agent-reactor bridge).
	BridgeBin = "reactor-bridge"

	// DotDir is the per-user and per-project dot directory name,
	// e.g. ~/.agent-reactor and <project>/.agent-reactor.
	DotDir = "." + Name

	// LibDirName is the libexec directory name under ~/.local/lib/.
	LibDirName = Name

	// Runtime files the daemon writes into its data directory.
	SocketFileName = ClientBin + ".sock" // server.sock
	PidFileName    = ClientBin + ".pid"  // server.pid
	LogFileName    = ClientBin + ".log"  // server.log

	// Container-side paths for files bind-mounted from the per-project run dir.
	// These are the canonical sources; callers must not hard-code these literals.
	ContainerRunDir           = "/opt/" + Name + "/run"
	ContainerBinaryPath       = ContainerRunDir + "/" + BridgeBin
	ContainerSockFileName     = SocketFileName
	ContainerSockFilePath     = ContainerRunDir + "/" + ContainerSockFileName
	ContainerHostExecSockPath = ContainerRunDir + "/hostexec.sock"
	ContainerMCPSockPath      = ContainerRunDir + "/mcp.sock"
)
