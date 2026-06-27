package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/takezoh/agent-reactor/client/runtime"
	"github.com/takezoh/agent-reactor/platform/hostexec"
)

// runHostExec dials the in-container hostexec broker UDS, forwards the
// requested binary + argv with the calling stdio fds attached, then exits
// with the broker's reported status. The broker round-trip is the only path
// a sandboxed agent has to reach a host binary it does not have access to in
// its container image.
func runHostExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: host-exec <binary> [args...]")
	}

	conn, err := net.Dial("unix", runtime.ContainerHostExecSockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "host-exec: broker unavailable (%v)\n", err)
		os.Exit(127)
	}
	uc := conn.(*net.UnixConn)

	cwd, _ := os.Getwd()
	req := hostexec.Request{
		Binary: args[0],
		Args:   args[1:],
		Cwd:    cwd,
	}
	fds := [3]int{int(os.Stdin.Fd()), int(os.Stdout.Fd()), int(os.Stderr.Fd())}
	if err := hostexec.SendRequest(uc, req, fds); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "host-exec: %v\n", err)
		os.Exit(127)
	}

	var resp hostexec.Response
	if err := json.NewDecoder(uc).Decode(&resp); err != nil {
		conn.Close()
		fmt.Fprintf(os.Stderr, "host-exec: read response: %v\n", err)
		os.Exit(127)
	}

	conn.Close()
	os.Exit(resp.ExitCode)
	return nil
}
