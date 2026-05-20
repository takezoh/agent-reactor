package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/takezoh/agent-roost/orchestrator/agent"
	"github.com/takezoh/agent-roost/orchestrator/httpserver"
	"github.com/takezoh/agent-roost/orchestrator/scheduler"
	"github.com/takezoh/agent-roost/orchestrator/tracker"
	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
	"github.com/takezoh/agent-roost/orchestrator/workflowfile"
	"github.com/takezoh/agent-roost/orchestrator/workspace"
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
	port := fs.Int("port", 0, "HTTP server port (0 = disabled unless set in WORKFLOW.md)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Detect whether --port was explicitly supplied on the command line.
	portExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portExplicit = true
		}
	})

	if err := logger.Init("info"); err != nil {
		fmt.Fprintf(stderr, "orchestrator: logger init: %v\n", err)
		return 1
	}
	defer logger.Close()

	slog.Info("orchestrator starting", "workflow", *workflow)

	absPath, err := filepath.Abs(*workflow)
	if err != nil {
		fmt.Fprintf(stderr, "orchestrator: workflow path: %v\n", err)
		slog.Error("workflow path error", "path", *workflow, "err", err)
		return 1
	}

	wf, err := workflowfile.Load(absPath)
	if err != nil {
		fmt.Fprintf(stderr, "orchestrator: %v\n", err)
		slog.Error("workflow load failed", "path", absPath, "err", err)
		return 1
	}

	cfg, err := wfconfig.Resolve(wf.Config, filepath.Dir(absPath))
	if err != nil {
		fmt.Fprintf(stderr, "orchestrator: config error: %v\n", err)
		slog.Error("config resolve failed", "err", err)
		return 1
	}

	if err := scheduler.Preflight(cfg); err != nil {
		fmt.Fprintf(stderr, "orchestrator: %v\n", err)
		slog.Error("preflight failed", "err", err)
		return 1
	}

	sched, cleanup, err := buildScheduler(ctx, absPath, cfg, wf.PromptTemplate)
	if err != nil {
		fmt.Fprintf(stderr, "orchestrator: %v\n", err)
		slog.Error("scheduler setup failed", "err", err)
		return 1
	}
	defer cleanup()

	if err := maybeStartHTTPServer(ctx, sched, cfg, portExplicit, *port, stderr); err != nil {
		return 1
	}

	if err := sched.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "orchestrator: scheduler: %v\n", err)
		slog.Error("scheduler error", "err", err)
		return 1
	}

	slog.Info("orchestrator stopped")
	return 0
}

// maybeStartHTTPServer resolves the effective port and starts the HTTP server if enabled.
// portExplicit indicates whether --port was explicitly provided on the CLI.
func maybeStartHTTPServer(ctx context.Context, sched *scheduler.Scheduler, cfg wfconfig.Config, portExplicit bool, cliPort int, stderr io.Writer) error {
	effectivePort := cfg.Server.Port
	if portExplicit {
		effectivePort = cliPort
	}
	if effectivePort == 0 {
		return nil
	}
	srv, err := httpserver.New(
		httpserver.Config{Bind: cfg.Server.Bind, Port: effectivePort},
		httpserver.NewMux(sched, cfg.Workspace.Root),
	)
	if err != nil {
		fmt.Fprintf(stderr, "orchestrator: http server: %v\n", err)
		slog.Error("http server setup failed", "err", err)
		return err
	}
	go srv.Serve(ctx)
	return nil
}

// buildScheduler wires the tracker, agent dispatcher, and scheduler together.
// The returned cleanup stops background services started for the dispatcher.
func buildScheduler(ctx context.Context, absPath string, cfg wfconfig.Config, tmpl string) (*scheduler.Scheduler, func(), error) {
	tr, err := tracker.New(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("tracker: %w", err)
	}

	dispatcher, dispatcherCleanup, err := buildDispatcher(ctx, cfg.Workspace.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("dispatcher: %w", err)
	}

	if err := ensureProject(ctx, dispatcher, cfg.Workspace.Root); err != nil {
		dispatcherCleanup()
		return nil, nil, err
	}

	ws := workspace.New(cfg)
	runner := agent.New(ws, cfg, tmpl, dispatcher, tr)

	sched := scheduler.New(absPath, cfg, scheduler.Deps{
		Tracker:        tr,
		RefreshTracker: tr,
		Workspace:      ws,
		Spawn:          runner.Spawn,
	})
	runner.WorkerDone = sched.WorkerDone()
	runner.CodexActivity = sched.CodexActivity()
	return sched, dispatcherCleanup, nil
}
