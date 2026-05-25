package peers

import (
	"os"

	"github.com/takezoh/agent-roost/client/event"
	"github.com/takezoh/agent-roost/client/proto"
)

// peerClient is the minimal proto.Client surface the MCP handlers need.
// *proto.Client satisfies this interface.
type peerClient interface {
	PeerList(fromFrameID, scope string) ([]proto.PeerPeerInfo, error)
	PeerSend(fromFrameID, toFrameID, text, replyTo string) error
	PeerSetSummary(fromFrameID, summary string) error
	PeerDrainInbox(frameID string) ([]proto.PeerMessage, error)
	Close() error
}

// dialer opens a peerClient to the roost daemon.
type dialer func() (peerClient, error)

// defaultDialer returns a dialer backed by the real daemon socket.
func defaultDialer() dialer {
	return func() (peerClient, error) {
		return dialDaemon()
	}
}

// dialDaemon opens a proto.Client to the roost daemon socket.
// Honors ROOST_SOCKET, falling back to the configured data dir.
func dialDaemon() (*proto.Client, error) {
	socketPath, err := event.ResolveSocketPath()
	if err != nil {
		return nil, err
	}
	return proto.Dial(socketPath)
}

// callerFrameID returns the caller's frame ID from the environment.
func callerFrameID() string {
	return os.Getenv("ROOST_FRAME_ID")
}
