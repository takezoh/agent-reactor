package proto

import "testing"

// Compile-time assertions: all four types satisfy the Command interface.
var _ Command = CmdSurfaceSubscribe{}
var _ Command = CmdSurfaceUnsubscribe{}
var _ Command = CmdSurfaceResize{}
var _ Command = CmdSurfaceWriteRaw{}

func TestCmdSurfaceCommandNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cmd  Command
		want string
	}{
		{CmdSurfaceSubscribe{SessionID: "s1"}, CmdNameSurfaceSubscribe},
		{CmdSurfaceUnsubscribe{SessionID: "s1"}, CmdNameSurfaceUnsubscribe},
		{CmdSurfaceResize{SessionID: "s1", Cols: 80, Rows: 24}, CmdNameSurfaceResize},
		{CmdSurfaceWriteRaw{SessionID: "s1", Data: []byte("hello")}, CmdNameSurfaceWriteRaw},
	}

	for _, tc := range cases {
		got := tc.cmd.CommandName()
		if got != tc.want {
			t.Errorf("CommandName() = %q, want %q", got, tc.want)
		}
	}
}
