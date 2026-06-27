// Package appid is the single source of truth for the application's on-disk
// and process identity: binary/command names, directory names, container
// paths, and runtime file names. Callers must reference these constants rather
// than hard-coding the literals, so renaming the app touches exactly one file.
//
// Two tokens span the identity:
//   - Name ("agent-reactor") — the project name, used for directories
//     (~/.agent-reactor, .agent-reactor/, /opt/agent-reactor, ~/.local/lib/agent-reactor).
//   - ClientBin ("arc", Agent Reactor Client) — the client TUI command, used
//     for the runtime files it owns (arc.sock/.pid/.log) and its backend session.
//
// Note: the IPC/runtime env-var contract (ROOST_* names) and the persisted
// JSON key roost_session_id are intentionally NOT defined here — they are a
// separate wire/contract surface kept stable for backward compatibility.
package appid

const (
	// Name is the project/app name, used for on-disk directories.
	Name = "agent-reactor"

	// ClientBin is the client TUI binary and command name — Agent Reactor Client.
	ClientBin = "arc"

	// BridgeBin is the in-container helper binary name (agent-reactor bridge).
	BridgeBin = "reactor-bridge"

	// DotDir is the per-user and per-project dot directory name,
	// e.g. ~/.agent-reactor and <project>/.agent-reactor.
	DotDir = "." + Name

	// LibDirName is the libexec directory name under ~/.local/lib/.
	LibDirName = Name

	// SessionName is the default backend session name for the client.
	SessionName = ClientBin

	// PeersServer is the peers MCP server name registered into agent configs.
	PeersServer = "reactor-peers"

	// Runtime files the client daemon writes into its data directory.
	SocketFileName = ClientBin + ".sock" // arc.sock
	PidFileName    = ClientBin + ".pid"  // arc.pid
	LogFileName    = ClientBin + ".log"  // arc.log

	// Container-side paths for files bind-mounted from the per-project run dir.
	// These are the canonical sources; callers must not hard-code these literals.
	ContainerRunDir           = "/opt/" + Name + "/run"
	ContainerBinaryPath       = ContainerRunDir + "/" + BridgeBin
	ContainerSockFileName     = SocketFileName
	ContainerSockFilePath     = ContainerRunDir + "/" + ContainerSockFileName
	ContainerHostExecSockPath = ContainerRunDir + "/hostexec.sock"
	ContainerMCPSockPath      = ContainerRunDir + "/mcp.sock"
)
