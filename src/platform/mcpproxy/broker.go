package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/takezoh/agent-roost/platform/procgroup"
)

type serverEntry struct {
	alias   string
	command string
	args    []string
	env     []string // key=value pairs merged over os.Environ
	policy  *Policy
}

type broker struct {
	ctx     context.Context
	sock    string
	ln      net.Listener
	project string
	servers map[string]*serverEntry
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
			slog.Warn("mcpproxy: accept error", "project", b.project, "err", err)
			return
		}
		go b.handleConn(conn.(*net.UnixConn))
	}
}

func (b *broker) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("mcpproxy: panic in handler", "project", b.project, "recover", r)
		}
	}()

	req, fds, err := RecvRequest(conn)
	if err != nil {
		slog.Warn("mcpproxy: recv request failed", "project", b.project, "err", err)
		return
	}

	exitCode := b.dispatch(req, fds)
	resp, _ := json.Marshal(Response{ExitCode: exitCode})
	_, _ = conn.Write(resp)
}

func (b *broker) dispatch(req Request, fds [3]int) int {
	srv, ok := b.servers[req.Alias]
	if !ok {
		slog.Warn("mcpproxy: unknown alias", "project", b.project, "alias", req.Alias)
		stderr := os.NewFile(uintptr(fds[2]), "stderr")
		fmt.Fprintf(stderr, "mcp-exec: unknown alias: %s\n", req.Alias)
		stderr.Close()
		os.NewFile(uintptr(fds[0]), "stdin").Close()
		os.NewFile(uintptr(fds[1]), "stdout").Close()
		return 127
	}
	return b.runMCP(srv, fds)
}

func (b *broker) runMCP(srv *serverEntry, fds [3]int) int {
	containerStdin := os.NewFile(uintptr(fds[0]), "stdin")
	containerStdout := os.NewFile(uintptr(fds[1]), "stdout")
	containerStderr := os.NewFile(uintptr(fds[2]), "stderr")
	defer containerStdin.Close()
	defer containerStdout.Close()
	defer containerStderr.Close()

	stdinW, stdoutR, cmd, err := b.startMCPProcess(srv, containerStderr)
	if err != nil {
		return 1
	}

	out := &syncWriter{w: containerStdout}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer stdinW.Close()
		forwardRequests(containerStdin, stdinW, out, srv.policy, b.project)
	}()
	go func() {
		defer wg.Done()
		defer stdoutR.Close()
		forwardResponses(stdoutR, out, srv.policy)
	}()

	runErr := cmd.Wait()
	wg.Wait()
	return exitCode(b.project, srv.alias, runErr)
}

// startMCPProcess creates pipes, starts the MCP host process, and returns the
// write end of its stdin pipe and the read end of its stdout pipe.
func (b *broker) startMCPProcess(srv *serverEntry, stderr *os.File) (*os.File, *os.File, *exec.Cmd, error) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(stderr, "mcp-exec: pipe: %v\n", err)
		return nil, nil, nil, err
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		stdinR.Close()
		stdinW.Close()
		fmt.Fprintf(stderr, "mcp-exec: pipe: %v\n", err)
		return nil, nil, nil, err
	}
	// procgroup.Command puts the MCP host in its own process group so ctx
	// cancellation SIGKILLs its descendants (npm/node-launched servers) too.
	cmd := procgroup.Command(procgroup.Spec{Ctx: b.ctx, Bin: srv.command, Args: srv.args, Env: srv.env})
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		stdinR.Close()
		stdinW.Close()
		stdoutR.Close()
		stdoutW.Close()
		slog.Warn("mcpproxy: start failed", "project", b.project, "alias", srv.alias, "err", err)
		fmt.Fprintf(stderr, "mcp-exec: start %s: %v\n", srv.alias, err)
		return nil, nil, nil, err
	}
	stdinR.Close()
	stdoutW.Close()
	return stdinW, stdoutR, cmd, nil
}

func exitCode(project, alias string, err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	slog.Warn("mcpproxy: mcp process failed", "project", project, "alias", alias, "err", err)
	return 1
}

// compileServer converts config fields into a serverEntry with pre-resolved env.
func compileServer(alias, command string, args []string, envMap map[string]string, allow, deny []string) (*serverEntry, error) {
	if command == "" {
		return nil, fmt.Errorf("mcpproxy: alias %q: command must not be empty", alias)
	}
	pol, err := CompilePolicy(allow, deny)
	if err != nil {
		return nil, err
	}
	base := os.Environ()
	env := make([]string, 0, len(base)+len(envMap))
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if _, override := envMap[key]; !override {
			env = append(env, kv)
		}
	}
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	return &serverEntry{alias: alias, command: command, args: args, env: env, policy: pol}, nil
}
