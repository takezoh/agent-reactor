package agentlaunch

import (
	"fmt"

	"github.com/takezoh/agent-roost/platform/lib/codex"
)

// ContainerStreamBridgeCmd returns the postCreate shell command that starts the
// per-session routing sockbridge inside the devcontainer. It listens on the fixed
// loopback port and routes each WebSocket connection to the per-session unix
// socket at containerRunDir/codex-<sessionID>.sock.
//
// The command runs the "sockbridge" subcommand of roost-bridge (which is already
// bind-mounted into the container at ContainerBinaryPath) in the background.
func ContainerStreamBridgeCmd(containerRunDir string) string {
	return fmt.Sprintf(
		"%s sockbridge -listen 127.0.0.1:%d -route-dir %s -route-prefix %s -route-suffix %s &",
		ContainerBinaryPath, codex.LoopbackPort, containerRunDir, codex.SockPrefix, codex.SockSuffix,
	)
}
