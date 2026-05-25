package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/takezoh/agent-roost/client/runtime"
	"github.com/takezoh/agent-roost/platform/mcpproxy"
)

func init() {
	Register("mcp-exec", "relay stdio to a host MCP server via the mcpproxy broker", runMCPExec)
}

func runMCPExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mcp-exec <alias>")
	}

	conn, err := net.Dial("unix", runtime.ContainerMCPSockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-exec: broker unavailable (%v)\n", err)
		os.Exit(127)
	}
	uc := conn.(*net.UnixConn)

	req := mcpproxy.Request{Alias: args[0]}
	fds := [3]int{int(os.Stdin.Fd()), int(os.Stdout.Fd()), int(os.Stderr.Fd())}
	if err := mcpproxy.SendRequest(uc, req, fds); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "mcp-exec: %v\n", err)
		os.Exit(127)
	}

	var resp mcpproxy.Response
	if err := json.NewDecoder(uc).Decode(&resp); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "mcp-exec: read response: %v\n", err)
		os.Exit(127)
	}

	conn.Close()
	os.Exit(resp.ExitCode)
	return nil //nolint:govet // unreachable after os.Exit
}
