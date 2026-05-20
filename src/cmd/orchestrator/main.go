package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/takezoh/agent-roost/platform/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("orchestrator", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workflow := fs.String("workflow", "./WORKFLOW.md", "path to WORKFLOW.md")
	port := fs.Int("port", 0, "HTTP server port (future)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := logger.Init("info"); err != nil {
		fmt.Fprintf(stderr, "orchestrator: logger init: %v\n", err)
		return 1
	}
	defer logger.Close()

	if _, err := os.Stat(*workflow); err != nil {
		fmt.Fprintf(stderr, "orchestrator: workflow file not found: %s\n", *workflow)
		slog.Error("workflow file not found", "path", *workflow)
		return 1
	}

	slog.Info("orchestrator starting", "workflow", *workflow, "port", *port)
	<-ctx.Done()
	slog.Info("orchestrator shutting down")
	return 0
}
