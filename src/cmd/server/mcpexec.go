package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/takezoh/agent-reactor/client/runtime"
	"github.com/takezoh/agent-reactor/platform/mcpproxy"
)

// runMCPExec dials the in-container mcpproxy broker UDS, asks the broker to
// relay the calling stdio to a registered host MCP server identified by alias,
// then exits with the broker's reported status.
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
	return nil
}
