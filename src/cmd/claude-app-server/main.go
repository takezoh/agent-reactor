package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	codexschemav1 "github.com/takezoh/agent-roost/platform/agent/codexschema/v1"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, codexclient.DefaultStdioTransport())
	stop()
	os.Exit(code)
}

// initializeResponse builds a schema-valid Codex InitializeResponse. The shim
// reports its own platform metadata; codexHome falls back to the conventional
// ~/.codex when $CODEX_HOME is unset.
func initializeResponse() codexschemav1.InitializeResponse {
	platformOS := runtime.GOOS
	if platformOS == "darwin" {
		platformOS = "macos"
	}
	family := "unix"
	if runtime.GOOS == "windows" {
		family = "windows"
	}
	return codexschemav1.InitializeResponse{
		CodexHome:      codexHome(),
		PlatformFamily: family,
		PlatformOS:     platformOS,
		UserAgent:      "claude-app-server/0",
	}
}

func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return filepath.Join(home, ".codex")
}
