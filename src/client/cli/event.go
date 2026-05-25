package cli

import "github.com/takezoh/agent-roost/client/event"

func init() {
	Register("event", "Send an event to the daemon", event.Run)
}
