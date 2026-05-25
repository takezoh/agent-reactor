package cli

import "github.com/takezoh/agent-roost/client/lib/peers"

func init() {
	Register("peers-mcp", "roost-peers MCP server (stdio)", peers.Run)
}
