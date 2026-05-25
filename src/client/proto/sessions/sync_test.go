package sessions_test

import (
	"testing"

	"github.com/takezoh/agent-roost/client/proto"
	"github.com/takezoh/agent-roost/client/state"
)

// TestPeerCommandNamesSyncWithState verifies that the proto peer command name
// constants match the state event dispatch keys. If they diverge, commands
// sent by the bridge will not be handled by the daemon.
func TestPeerCommandNamesSyncWithState(t *testing.T) {
	cases := []struct {
		wire  string
		event string
	}{
		{proto.CmdNamePeerSend, state.EventPeerSend},
		{proto.CmdNamePeerList, state.EventPeerList},
		{proto.CmdNamePeerSetSummary, state.EventPeerSetSummary},
		{proto.CmdNamePeerDrainInbox, state.EventPeerDrainInbox},
	}
	for _, c := range cases {
		if c.wire != c.event {
			t.Errorf("proto.CmdName %q != state.Event %q", c.wire, c.event)
		}
	}
}
