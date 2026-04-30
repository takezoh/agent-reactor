package hostexec

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
)

// Request is the control message sent by the container-side client.
type Request struct {
	Binary string   `json:"binary"` // container alias name (e.g. "gh")
	Args   []string `json:"args"`
	Cwd    string   `json:"cwd"`
}

// Response is the control message returned by the broker after the child exits.
type Response struct {
	ExitCode int `json:"exit_code"`
}

// SendRequest writes req alongside fds (stdin/stdout/stderr) in a single WriteMsgUnix call.
// The fds are passed via SCM_RIGHTS ancillary data.
func SendRequest(conn *net.UnixConn, req Request, fds [3]int) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("hostexec: marshal request: %w", err)
	}
	rights := syscall.UnixRights(fds[0], fds[1], fds[2])
	_, _, err = conn.WriteMsgUnix(data, rights, nil)
	if err != nil {
		return fmt.Errorf("hostexec: send request: %w", err)
	}
	return nil
}

// RecvRequest reads a request and its three stdio fds from conn.
// fds are parsed before JSON so they can be closed on any subsequent error.
func RecvRequest(conn *net.UnixConn) (Request, [3]int, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, 128)
	n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("hostexec: recv request: %w", err)
	}

	// Parse fds from SCM_RIGHTS before touching the JSON payload.
	// After ReadMsgUnix succeeds, the kernel has already allocated the fds in
	// this process's fd table; any error below must close them to avoid leaks.
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("hostexec: parse cmsg: %w", err)
	}
	if len(scms) == 0 {
		return Request{}, [3]int{}, fmt.Errorf("hostexec: no fds in request")
	}
	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return Request{}, [3]int{}, fmt.Errorf("hostexec: parse unix rights: %w", err)
	}
	closeFDs := func(ff []int) {
		for _, fd := range ff {
			_ = syscall.Close(fd)
		}
	}
	if len(fds) < 3 {
		closeFDs(fds)
		return Request{}, [3]int{}, fmt.Errorf("hostexec: expected 3 fds, got %d", len(fds))
	}
	// Close any unexpected extra fds beyond the three we need.
	closeFDs(fds[3:])

	var req Request
	if err := json.Unmarshal(buf[:n], &req); err != nil {
		closeFDs(fds[:3])
		return Request{}, [3]int{}, fmt.Errorf("hostexec: unmarshal request: %w", err)
	}
	return req, [3]int{fds[0], fds[1], fds[2]}, nil
}
