package hostexec

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
)

type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	entries atomic.Pointer[map[string]*entry]
	onStop  func()
}

func (b *broker) storeEntries(m map[string]*entry) { b.entries.Store(&m) }
func (b *broker) loadEntries() map[string]*entry   { return *b.entries.Load() }

func (b *broker) serve() {
	defer b.ln.Close()
	defer func() { _ = os.Remove(b.sock) }()
	defer b.onStop()
	for {
		conn, err := b.ln.Accept()
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			slog.Warn("hostexec: accept error", "project", b.project, "err", err)
			return
		}
		go b.handleConn(conn.(*net.UnixConn))
	}
}

func (b *broker) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("hostexec: panic in handler", "project", b.project, "recover", r)
		}
	}()

	callerPID := peerPID(conn)
	req, fds, err := RecvRequest(conn)
	if err != nil {
		slog.Warn("hostexec: recv request failed", "project", b.project, "err", err)
		return
	}

	exitCode := b.dispatch(req, fds, callerPID)

	resp, _ := json.Marshal(Response{ExitCode: exitCode})
	_, _ = conn.Write(resp)
}

func (b *broker) dispatch(req Request, fds [3]int, callerPID int) int {
	e, ok := b.loadEntries()[req.Binary]
	if !ok {
		slog.Warn("hostexec: unknown binary", "project", b.project, "binary", req.Binary, "caller_pid", callerPID, "caller", procComm(callerPID))
		stderr := os.NewFile(uintptr(fds[2]), "stderr")
		fmt.Fprintf(stderr, "host-exec: unknown binary: %s\n", req.Binary)
		stderr.Close()
		os.NewFile(uintptr(fds[0]), "stdin").Close()
		os.NewFile(uintptr(fds[1]), "stdout").Close()
		return 127
	}
	return executeRequest(b.ctx, e, b.project, req, fds, callerPID)
}

// peerPID returns the PID of the process on the other end of conn via SO_PEERCRED.
// Returns 0 on any error.
func peerPID(conn *net.UnixConn) int {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0
	}
	var pid int
	_ = raw.Control(func(fd uintptr) {
		cred, cerr := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if cerr == nil {
			pid = int(cred.Pid)
		}
	})
	return pid
}

// procComm returns the comm name (process name, up to 15 chars) of pid via /proc.
// Returns "" on any error.
func procComm(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
