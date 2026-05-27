package secretenv

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// resolveTimeout is roost's safety bound on the credproxy resolve subprocess.
// It must exceed the total time credproxy may spend resolving all refs in the
// env-file (N refs × hook_timeout_sec). The actual hook timeout per ref is
// credproxy's concern (hook_timeout_sec in ~/.config/credproxy/config.toml).
const resolveTimeout = 5 * time.Minute

// broker is a per-project unix socket server that gates and resolves secret env-files.
type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	onStop  func()

	// mu guards gate, credproxyBin, and hostPathMountPrefix. These fields may be
	// updated by ensureBroker (under SpecBuilder.mu) while resolve() runs
	// concurrently in connection-handler goroutines, so a broker-level RWMutex is
	// required.
	mu                  sync.RWMutex
	gate                *Gate
	credproxyBin        string
	hostPathMountPrefix string
}

func (b *broker) setConfig(gate *Gate, credproxyBin, hostPathMountPrefix string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gate = gate
	b.credproxyBin = credproxyBin
	b.hostPathMountPrefix = hostPathMountPrefix
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
			slog.Warn("secretenv: accept error", "project", b.project, "err", err)
			return
		}
		go b.handleConn(conn)
	}
}

func (b *broker) handleConn(conn net.Conn) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("secretenv: panic in handler", "project", b.project, "recover", r)
		}
	}()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		slog.Warn("secretenv: decode request", "project", b.project, "err", err)
		writeResponse(conn, Response{Error: "malformed request"})
		return
	}

	resp := b.resolve(req)
	writeResponse(conn, resp)
}

func (b *broker) resolve(req Request) Response {
	b.mu.RLock()
	gate := b.gate
	bin := b.credproxyBin
	mountPrefix := b.hostPathMountPrefix
	b.mu.RUnlock()

	// Reject relative paths: only the container shim (which knows its CWD) can
	// canonicalize relative paths via filepath.Abs. A relative path here means
	// either a shim bug or a direct socket connection attempt.
	if !filepath.IsAbs(req.EnvFilePath) {
		slog.Warn("secretenv: rejected relative path", "project", b.project, "path", req.EnvFilePath)
		return Response{Error: "secretenv: env-file path must be absolute"}
	}

	hostPath := containerToHost(filepath.Clean(req.EnvFilePath), mountPrefix)

	if err := gate.Check(hostPath); err != nil {
		slog.Warn("secretenv: gate denied", "project", b.project, "path", hostPath, "err", err)
		return Response{Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(b.ctx, resolveTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, bin, "resolve", "--env-file", hostPath).Output()
	if err != nil {
		// Output() populates ExitError.Stderr when cmd.Stderr is nil, giving
		// diagnostic detail (hook misconfiguration, auth errors, etc.) that
		// err.Error() alone ("exit status N") does not convey.
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		slog.Warn("secretenv: credproxy resolve failed", "project", b.project, "path", req.EnvFilePath, "err", err, "stderr", stderr)
		return Response{Error: err.Error()}
	}

	var result struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		slog.Warn("secretenv: parse resolve output", "project", b.project, "err", err)
		return Response{Error: "invalid resolve output"}
	}
	return Response{Env: result.Env}
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("secretenv: marshal response", "err", err)
		return
	}
	_, _ = conn.Write(data)
}
