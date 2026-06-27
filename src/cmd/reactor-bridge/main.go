// reactor-bridge is the thin client binary deployed inside devcontainers.
// It handles the container-side roles that need to reach the host client daemon:
//
//	event <type>          – agent hook (forwards CmdEvent / CmdHookEvent to daemon)
//	host-exec <bin>       – PATH shim target (proxies stdio to host via SCM_RIGHTS)
//	mcp-exec <alias>      – MCP proxy client (relays stdio to host MCP server via SCM_RIGHTS)
//	secret-run run ...    – secret env-file resolver shim (impersonates "credproxy run")
//
// Agent integration setup (Claude / Codex / Gemini hook registration) is
// owned by repo-root scripts/setup-{claude,codex,gemini}.sh as of phase F-C;
// the in-binary `setup <agent>` subcommand has been removed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/takezoh/agent-reactor/client/event"
	"github.com/takezoh/agent-reactor/platform/appid"
	"github.com/takezoh/agent-reactor/platform/hostexec"
	"github.com/takezoh/agent-reactor/platform/mcpproxy"
	"github.com/takezoh/agent-reactor/platform/secretenv"
)

// hostExecSockPath is the Unix socket for the host-exec broker inside the container.
const hostExecSockPath = appid.ContainerHostExecSockPath

// mcpSockPath is the Unix socket for the MCP proxy broker inside the container.
const mcpSockPath = appid.ContainerMCPSockPath

// secretEnvSockPath is the Unix socket for the secretenv broker inside the container.
// Uses secretenv.ContainerSockName so it stays in sync with provider.go's mount target.
const secretEnvSockPath = appid.ContainerRunDir + "/" + secretenv.ContainerSockName

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	sub := os.Args[1]
	rest := os.Args[2:]

	var err error
	switch sub {
	case "event":
		err = event.Run(rest)
	case "host-exec":
		err = runHostExec(rest)
	case "mcp-exec":
		err = runMCPExec(rest)
	case "secret-run":
		err = runSecretRun(rest)
	case "sockbridge":
		err = runSockBridge(rest)
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown subcommand: %s\n", appid.BridgeBin, sub)
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appid.BridgeBin, err)
		os.Exit(1)
	}
}

func runHostExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: host-exec <binary> [args...]")
	}

	conn, err := net.Dial("unix", hostExecSockPath)
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

func runMCPExec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: mcp-exec <alias>")
	}

	conn, err := net.Dial("unix", mcpSockPath)
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

// runSecretRun implements the "credproxy run" shim.
// Parses "run --env-file X -- cmd args...", sends the env-file path to the host broker,
// receives the resolved env map, merges it into the current environment, and exec's cmd.
func runSecretRun(args []string) error {
	// args[0] is expected to be "run" (the subcommand from "credproxy run ...").
	if len(args) == 0 || args[0] != "run" {
		return fmt.Errorf("usage: secret-run run --env-file <path> -- cmd [args...]")
	}

	fs := flag.NewFlagSet("credproxy run", flag.ContinueOnError)
	envFile := fs.String("env-file", "", "path to env-file with secret references")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("secret-run: %w", err)
	}
	if *envFile == "" {
		return fmt.Errorf("secret-run: --env-file is required")
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("secret-run: no command specified after --env-file")
	}
	// Consume optional "--" separator.
	if rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return fmt.Errorf("secret-run: no command specified after --")
	}

	conn, err := net.Dial("unix", secretEnvSockPath)
	if err != nil {
		return fmt.Errorf("secret-run: broker unavailable: %w", err)
	}
	defer conn.Close()

	absEnvFile, err := filepath.Abs(*envFile)
	if err != nil {
		return fmt.Errorf("secret-run: resolve env-file path: %w", err)
	}
	req := secretenv.Request{EnvFilePath: absEnvFile}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("secret-run: send request: %w", err)
	}

	var resp secretenv.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("secret-run: read response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("secret-run: %s", resp.Error)
	}
	// conn is not closed explicitly here: Go net sockets have FD_CLOEXEC set, so
	// syscall.Exec closes it at the OS level. The defer conn.Close() handles all
	// error-return paths above.

	env := mergeSecretEnv(os.Environ(), resp.Env)
	cmd, err := resolveExecPath(rest[0])
	if err != nil {
		return fmt.Errorf("secret-run: %w", err)
	}
	return syscall.Exec(cmd, rest, env)
}

// mergeSecretEnv merges resolved secret values into the base environment.
// Existing entries with the same key are replaced; new keys are appended.
func mergeSecretEnv(base []string, resolved map[string]string) []string {
	if len(resolved) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(resolved))
	for _, kv := range base {
		name := envKey(kv)
		if _, ok := resolved[name]; !ok {
			out = append(out, kv)
		}
	}
	for k, v := range resolved {
		out = append(out, k+"="+v)
	}
	return out
}

func envKey(kv string) string {
	for i, c := range kv {
		if c == '=' {
			return kv[:i]
		}
	}
	return kv
}

func resolveExecPath(name string) (string, error) {
	if len(name) > 0 && name[0] == '/' {
		if err := checkExecutable(name); err != nil {
			return "", err
		}
		return name, nil
	}
	path := os.Getenv("PATH")
	if path == "" {
		return "", fmt.Errorf("PATH not set")
	}
	for _, dir := range splitPath(path) {
		candidate := dir + "/" + name
		if err := checkExecutable(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%q: executable not found in PATH", name)
}

// checkExecutable returns nil if path is a regular executable file.
func checkExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", path)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("%q is not executable", path)
	}
	return nil
}

func splitPath(path string) []string {
	var dirs []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == ':' {
			if dir := path[start:i]; dir != "" {
				dirs = append(dirs, dir)
			}
			start = i + 1
		}
	}
	return dirs
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: reactor-bridge <subcommand> [args...]

Subcommands:
  event <type>               Send an event to the client daemon
  host-exec <bin>            Execute a host binary via the hostexec broker
  mcp-exec <alias>           Relay stdio to a host MCP server via the mcpproxy broker
  secret-run run --env-file  Resolve secret env-file and exec command (credproxy shim)
  sockbridge                 TCP↔unix socket bridge (fixed-socket; credproxy broker)
`)
}
