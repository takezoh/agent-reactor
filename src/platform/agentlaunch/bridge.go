package agentlaunch

import (
	"fmt"

	"github.com/takezoh/credproxy/container"
)

// codex stream bridge constants — same protocol values as client/driver.CodexApp*
// (copied here so platform/ does not depend on client/).
const (
	streamSockName     = "codex.sock"
	streamLoopbackPort = 8282
)

// ContainerBridgeSpec returns the credproxy BridgeSpec that runs sockbridge
// inside the project devcontainer. Appended to postCreate so the bridge is
// available before any frame connects.
func ContainerBridgeSpec(containerRunDir string) container.BridgeSpec {
	return container.BridgeSpec{
		ListenAddr:          fmt.Sprintf("127.0.0.1:%d", streamLoopbackPort),
		ContainerSocketPath: containerRunDir + "/" + streamSockName,
	}
}
