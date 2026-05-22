// linear_graphql is a small CLI shim that forwards a Linear GraphQL request to
// a claude-app-server tool-bridge socket and prints the JSON response.
//
// Usage:
//
//	linear_graphql '<json-arguments>'
//	echo '<json-arguments>' | linear_graphql
//
// Environment:
//
//	TOOL_BRIDGE_SOCKET  Unix socket path written by the claude-app-server shim.
//
// The JSON arguments must match the linear_graphql input schema:
//
//	{"query": "<graphql-query>", "variables": {...optional...}}
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "linear_graphql:", err)
		os.Exit(1)
	}
}

func run() error {
	socketPath := os.Getenv("TOOL_BRIDGE_SOCKET")
	if socketPath == "" {
		return fmt.Errorf("TOOL_BRIDGE_SOCKET env var not set")
	}

	argsJSON, err := readArgs()
	if err != nil {
		return err
	}

	var args json.RawMessage
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to tool bridge: %w", err)
	}

	req := struct {
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}{
		Tool:      "linear_graphql",
		Arguments: args,
	}
	if encErr := json.NewEncoder(conn).Encode(req); encErr != nil {
		conn.Close()
		return fmt.Errorf("send request: %w", encErr)
	}

	var resp struct {
		Success bool   `json:"success"`
		Output  string `json:"output"`
		Error   string `json:"error,omitempty"`
	}
	decErr := json.NewDecoder(conn).Decode(&resp)
	conn.Close()
	if decErr != nil {
		return fmt.Errorf("read response: %w", decErr)
	}

	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "tool error:", resp.Error)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, resp.Output)
	return nil
}

// readArgs reads JSON arguments from the first CLI argument or stdin.
func readArgs() (string, error) {
	if len(os.Args) > 1 {
		return os.Args[1], nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(b), nil
}
