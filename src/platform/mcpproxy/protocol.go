package mcpproxy

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
)

// Request is the control message sent by the container-side mcp-exec client.
type Request struct {
	Alias string `json:"alias"`
}

// Response is returned by the broker after the MCP host process exits.
type Response struct {
	ExitCode int `json:"exit_code"`
}

// SendRequest writes req alongside fds (stdin/stdout/stderr) in a single
// WriteMsgUnix call using SCM_RIGHTS ancillary data.
func SendRequest(conn *net.UnixConn, req Request, fds [3]int) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("mcpproxy: marshal request: %w", err)
	}
	rights := syscall.UnixRights(fds[0], fds[1], fds[2])
	_, _, err = conn.WriteMsgUnix(data, rights, nil)
	if err != nil {
		return fmt.Errorf("mcpproxy: send request: %w", err)
	}
	return nil
}

// RecvRequest reads a request and its three stdio fds from conn.
func RecvRequest(conn *net.UnixConn) (Request, [3]int, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 128)
	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: recv request: %w", err)
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: parse cmsg: %w", err)
	}
	if len(scms) == 0 {
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: no fds in request")
	}
	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: parse unix rights: %w", err)
	}
	closeFDs := func(ff []int) {
		for _, fd := range ff {
			_ = syscall.Close(fd)
		}
	}
	if len(fds) < 3 {
		closeFDs(fds)
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: expected 3 fds, got %d", len(fds))
	}
	closeFDs(fds[3:])

	var req Request
	if err := json.Unmarshal(buf[:n], &req); err != nil {
		closeFDs(fds[:3])
		return Request{}, [3]int{}, fmt.Errorf("mcpproxy: unmarshal request: %w", err)
	}
	return req, [3]int{fds[0], fds[1], fds[2]}, nil
}
