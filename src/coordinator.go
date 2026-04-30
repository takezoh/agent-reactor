package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/config"
	"github.com/takezoh/agent-roost/connector"
	statedriver "github.com/takezoh/agent-roost/driver"
	"github.com/takezoh/agent-roost/features"
	libnotify "github.com/takezoh/agent-roost/lib/notify"
	"github.com/takezoh/agent-roost/lib/tmux"
	"github.com/takezoh/agent-roost/logger"
	"github.com/takezoh/agent-roost/runtime"
	"github.com/takezoh/agent-roost/runtime/worker"
	sandboxdc "github.com/takezoh/agent-roost/sandbox/devcontainer"
	"github.com/takezoh/agent-roost/state"
)

func runCoordinator() error { //nolint:funlen
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.Sandbox.Validate(); err != nil {
		return err
	}
	sessionName := cfg.Tmux.SessionName
	slog.Info("starting coordinator", "session", sessionName)
	client := tmux.NewClient(sessionName)

	dataDir := cfg.ResolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	home, _ := os.UserHomeDir()

	idleThreshold := time.Duration(cfg.Monitor.IdleThresholdSec) * time.Second
	eventLogDir := filepath.Join(dataDir, "events")
	statedriver.RegisterDefaults(statedriver.RegisterOptions{
		Home:             home,
		EventLogDir:      eventLogDir,
		IdleThreshold:    idleThreshold,
		DriverConfigs:    cfg.Drivers,
		SummarizeCommand: cfg.Driver.SummarizeCommand,
		Pager:            cfg.Driver.Pager,
	})

	tmuxBackend := runtime.NewRealTmuxBackend(client)
	pollInterval := time.Duration(cfg.Monitor.PollIntervalMs) * time.Millisecond
	fastPollInterval := time.Duration(cfg.Monitor.FastPollIntervalMs) * time.Millisecond
	sockPath := filepath.Join(dataDir, "roost.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	terminalEvict := statedriver.RegisterRunners(tmuxBackend.CapturePaneEscaped, cfg.Driver.SummarizeCommand)
	connector.RegisterDefaults()
	connector.RegisterRunners()
	pool := worker.NewPool(ctx, 4)

	ln, err := libnotify.New(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("notify: %w", err)
	}

	tapDir := filepath.Join(dataDir, "tap")
	if err := os.MkdirAll(tapDir, 0o755); err != nil {
		return fmt.Errorf("mkdir tap dir: %w", err)
	}
	paneTap := runtime.NewTmuxPipePaneTap(tmuxBackend.PipePane, tapDir)

	featureSet := features.FromConfig(cfg.Features.Enabled, features.All())
	sbResolver := config.NewSandboxResolver(cfg.Sandbox)
	agentLauncher, err := newAgentLauncher(ctx, cfg.Sandbox, sbResolver, dataDir)
	if err != nil {
		return err
	}
	exePath := resolveExe()
	rt := runtime.New(runtime.Config{
		SessionName:       sessionName,
		RoostExe:          exePath,
		DataDir:           dataDir,
		TickInterval:      pollInterval,
		FastTickInterval:  fastPollInterval,
		MainPaneHeightPct: cfg.Tmux.PaneRatioVertical,
		Tmux:              tmuxBackend,
		Persist:           runtime.NewFilePersist(dataDir),
		EventLog:          runtime.NewFileEventLog(dataDir),
		ToolLog:           runtime.NewFileToolLog(dataDir),
		Pool:              pool,
		Notifier:          runtime.NewNotifier(&cfg.Notifications, ln),
		TerminalEvict:     terminalEvict,
		Tap:               paneTap,
		Features:          featureSet,
		Launcher:          agentLauncher,
	})

	rt.SetAliases(cfg.Session.Aliases)
	rt.SetDefaultCommand(cfg.Session.DefaultCommand)
	rt.SetSandboxedProjectResolver(func(project string) bool {
		return sbResolver.Resolve(project).IsSandboxed()
	})

	warmRestart := client.SessionExists()
	if warmRestart {
		slog.Info("session exists, restoring")
		ensureHiddenWindow(client, sessionName, exePath)
		state.Register(statedriver.NewShellDriver("shell", resolveShellDisplay(client), idleThreshold))
		if err := rt.LoadSnapshot(false); err != nil {
			slog.Error("snapshot load failed", "err", err)
		}
		if err := rt.LoadSessionPanes(); err != nil {
			slog.Warn("window map load failed", "err", err)
		}
		rt.RecoverActivePaneAtMain()
		restoreSession(client, cfg, sessionName, exePath)
		rt.ReconcileOrphans()
		rt.RecoverSandboxFrames()
		rt.RecoverWarmStartSessions()
	} else {
		slog.Info("creating new session")
		if err := setupNewSession(client, cfg, sessionName, exePath); err != nil {
			return err
		}
		state.Register(statedriver.NewShellDriver("shell", resolveShellDisplay(client), idleThreshold))
		if err := rt.LoadSessionPanes(); err != nil {
			slog.Warn("window map load failed", "err", err)
		}
		// Cold start: wipe warm-only state (container tokens etc.) before
		// loading the snapshot so there is no stale data from a prior warm run.
		if err := rt.ResetWarmState(); err != nil {
			slog.Warn("cold start: warm wipe failed", "err", err)
		}
		if err := rt.LoadSnapshot(true); err != nil {
			slog.Error("snapshot load failed", "err", err)
		}
		if err := rt.RecreateAll(); err != nil {
			slog.Error("recreate failed", "err", err)
		}
	}

	runErrCh := make(chan error, 1)
	go func() {
		if err := rt.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			runErrCh <- err
		}
	}()

	rt.StartTapsForRestoredFrames()

	if err := rt.StartIPC(sockPath); err != nil {
		return fmt.Errorf("ipc: %w", err)
	}
	slog.Info("server started", "sock", sockPath)

	if relay, err := runtime.NewFileRelay(rt); err != nil {
		slog.Warn("filerelay: start failed, TUI will show backfill only", "err", err)
	} else {
		defer relay.Close()
		relay.WatchLog(logger.LogFilePath())
		rt.SetRelay(relay)
	}

	// Spawn all TUI panes after StartIPC so proto.Dial succeeds on first attempt.
	rt.RespawnMainPane()
	respawnHeaderPane(client, sessionName, exePath)
	respawnSessionsPane(client, sessionName, exePath)
	respawnHiddenPane(client, sessionName, exePath)

	slog.Info("attaching to tmux session")
	if err := client.Attach(); err != nil {
		slog.Warn("attach exited", "err", err)
	}

	cancel()
	<-rt.Done()
	close(runErrCh)
	if err, ok := <-runErrCh; ok {
		return fmt.Errorf("runtime: %w", err)
	}

	if client.SessionExists() {
		slog.Info("detached, session kept alive")
	} else {
		slog.Info("tmux server exited")
	}
	return nil
}

// newAgentLauncher returns the AgentLauncher for the configured sandbox mode.
// Returns a SandboxDispatcher that routes each launch to direct or devcontainer
// based on the effective config for that project (user scope + optional project scope).
func newAgentLauncher(ctx context.Context, sb config.SandboxConfig, resolver *config.SandboxResolver, dataDir string) (runtime.AgentLauncher, error) {
	d := &runtime.SandboxDispatcher{
		Resolver: resolver,
		Direct:   runtime.DirectLauncher{},
	}
	if sb.Mode == "devcontainer" {
		if _, err := exec.LookPath("docker"); err != nil {
			return nil, fmt.Errorf("sandbox: devcontainer mode requires docker in PATH: %w", err)
		}
		var err error
		var runner *runtime.CredProxyRunner
		if sb.Proxy.Enabled {
			runner, err = runtime.StartCredProxy(ctx, dataDir)
			if err != nil {
				return nil, fmt.Errorf("sandbox: start in-process credproxy: %w", err)
			}
		}
		overlayFn := runtime.BuildOverlayFunc(func(project string) config.SandboxConfig {
			return resolver.Resolve(project)
		}, runner, dataDir, statedriver.ClaudeDriverName+" setup")
		mgr := sandboxdc.New(overlayFn)
		d.Devcontainer = runtime.NewDevcontainerLauncher(mgr, func(project string) config.SandboxConfig {
			return resolver.Resolve(project)
		}, runner, dataDir)
		slog.Info("sandbox: devcontainer backend enabled", "proxy", sb.Proxy.Enabled)
	}
	return d, nil
}

// resolveShellDisplayFromValues picks the display name (basename) for the
// shell driver. Pure function — used directly by tests.
func resolveShellDisplayFromValues(tmuxDefault, envSHELL string) string {
	for _, raw := range []string{tmuxDefault, envSHELL} {
		if name := filepath.Base(raw); name != "" && name != "." && name != "/" {
			return name
		}
	}
	return "shell"
}

// resolveShellDisplay queries tmux's default-shell option (the shell tmux
// will actually spawn for login-shell panes) and falls back to $SHELL.
func resolveShellDisplay(client *tmux.Client) string {
	tmuxDefault, _ := client.ShowOption("default-shell")
	return resolveShellDisplayFromValues(tmuxDefault, os.Getenv("SHELL"))
}
