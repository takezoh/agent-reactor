package cli

import "github.com/takezoh/agent-roost/client/event"

// resolveSocketPath returns the roost daemon UDS path, preferring the
// ROOST_SOCKET env var when set. Inside a Docker sandbox container the env is
// set to the bind-mounted path (e.g. /tmp/roost.sock) so guest `roost` CLIs
// reach the same host daemon as local invocations.
func resolveSocketPath() (string, error) {
	return event.ResolveSocketPath()
}
