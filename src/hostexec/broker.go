package hostexec

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
)

type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	entries map[string]*entry // keyed by container alias
	onStop  func()
}

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

	req, fds, err := RecvRequest(conn)
	if err != nil {
		slog.Warn("hostexec: recv request failed", "project", b.project, "err", err)
		return
	}

	exitCode := b.dispatch(req, fds)

	resp, _ := json.Marshal(Response{ExitCode: exitCode})
	_, _ = conn.Write(resp)
}

func (b *broker) dispatch(req Request, fds [3]int) int {
	e, ok := b.entries[req.Binary]
	if !ok {
		slog.Warn("hostexec: unknown binary", "project", b.project, "binary", req.Binary)
		stderr := os.NewFile(uintptr(fds[2]), "stderr")
		fmt.Fprintf(stderr, "host-exec: unknown binary: %s\n", req.Binary)
		stderr.Close()
		os.NewFile(uintptr(fds[0]), "stdin").Close()
		os.NewFile(uintptr(fds[1]), "stdout").Close()
		return 127
	}
	return executeRequest(b.ctx, e, b.project, req, fds)
}
