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

func runCoordinator() error {
	if v := os.Getenv("TMUX"); v != "" {
		return fmt.Errorf("refusing to start coordinator inside an existing tmux session ($TMUX is set); run `roost` outside tmux or detach first")
	}
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idleThreshold := time.Duration(cfg.Monitor.IdleThresholdSec) * time.Second
	registerDefaultDrivers(cfg, dataDir, idleThreshold)

	rt, sockPath, exePath, err := buildRuntime(ctx, cfg, client, dataDir)
	if err != nil {
		return err
	}

	if err := startSession(ctx, rt, client, cfg, sessionName, exePath, idleThreshold); err != nil {
		return err
	}

	return runAndWait(ctx, cancel, rt, client, sockPath, sessionName, exePath)
}

// registerDefaultDrivers wires all built-in drivers and worker runners.
func registerDefaultDrivers(cfg *config.Config, dataDir string, idleThreshold time.Duration) {
	home, _ := os.UserHomeDir()
	statedriver.RegisterDefaults(statedriver.RegisterOptions{
		Home:             home,
		EventLogDir:      filepath.Join(dataDir, "events"),
		IdleThreshold:    idleThreshold,
		DriverConfigs:    cfg.Drivers,
		SummarizeCommand: cfg.Driver.SummarizeCommand,
		Pager:            cfg.Driver.Pager,
	})
	statedriver.RegisterRunners(cfg.Driver.SummarizeCommand)
	connector.RegisterDefaults()
	connector.RegisterRunners()
}

// buildRuntime constructs and configures the Runtime. Returns the Runtime,
// the socket path it will listen on, the resolved roost binary path, and any error.
func buildRuntime(ctx context.Context, cfg *config.Config, client *tmux.Client, dataDir string) (*runtime.Runtime, string, string, error) {
	tmuxBackend := runtime.NewRealTmuxBackend(client)
	pollInterval := time.Duration(cfg.Monitor.PollIntervalMs) * time.Millisecond
	fastPollInterval := time.Duration(cfg.Monitor.FastPollIntervalMs) * time.Millisecond
	sockPath := filepath.Join(dataDir, "roost.sock")

	pool := worker.NewPool(ctx, 4)
	ln, err := libnotify.New(ctx, runtime.FindHelperFile("notify.ps1"))
	if err != nil {
		return nil, "", "", fmt.Errorf("notify: %w", err)
	}

	tapDir := filepath.Join(dataDir, "tap")
	if err := os.MkdirAll(tapDir, 0o755); err != nil {
		return nil, "", "", fmt.Errorf("mkdir tap dir: %w", err)
	}
	paneTap := runtime.NewTmuxPipePaneTap(tmuxBackend.PipePane, tapDir)

	featureSet := features.FromConfig(cfg.Features.Enabled, features.All())
	sbResolver := config.NewSandboxResolver(cfg.Sandbox)
	agentLauncher, err := newAgentLauncher(ctx, cfg.Sandbox, sbResolver, dataDir, sockPath)
	if err != nil {
		return nil, "", "", err
	}
	exePath := resolveExe()
	rt := runtime.New(runtime.Config{
		SessionName:       cfg.Tmux.SessionName,
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
		Tap:               paneTap,
		Features:          featureSet,
		Launcher:          agentLauncher,
	})
	rt.SetAliases(cfg.Session.Aliases)
	rt.SetDefaultCommand(cfg.Session.DefaultCommand)
	rt.SetSandboxedProjectResolver(func(project string) bool {
		return sbResolver.Resolve(project).IsSandboxed()
	})
	return rt, sockPath, exePath, nil
}

// startSession performs warm or cold startup, registering the shell driver and
// restoring (or creating) the tmux session and persisted frame stack.
func startSession(ctx context.Context, rt *runtime.Runtime, client *tmux.Client, cfg *config.Config, sessionName, exePath string, idleThreshold time.Duration) error {
	shellDriver := statedriver.NewShellDriver(statedriver.ShellDriverName, resolveShellDisplay(client), idleThreshold)
	if client.SessionExists() {
		return warmStart(rt, client, cfg, sessionName, exePath, shellDriver)
	}
	if err := coldStart(ctx, rt, client, cfg, sessionName, exePath, shellDriver); err != nil {
		return err
	}
	go rt.CleanupSubsystems(ctx)
	return nil
}

func warmStart(rt *runtime.Runtime, client *tmux.Client, cfg *config.Config, sessionName, exePath string, shellDriver statedriver.ShellDriver) error {
	slog.Info("session exists, restoring")
	ensureHiddenWindow(client, sessionName, exePath)
	state.Register(shellDriver)
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
	return nil
}

func coldStart(ctx context.Context, rt *runtime.Runtime, client *tmux.Client, cfg *config.Config, sessionName, exePath string, shellDriver statedriver.ShellDriver) error {
	slog.Info("creating new session")
	if err := setupNewSession(client, cfg, sessionName, exePath); err != nil {
		return err
	}
	state.Register(shellDriver)
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
	rt.PrewarmContainers(ctx)
	if err := rt.RecreateAll(); err != nil {
		slog.Error("recreate failed", "err", err)
	}
	return nil
}

// runAndWait starts the event loop, IPC server, TUI panes, and blocks until
// the session ends or the runtime errors.
func runAndWait(ctx context.Context, cancel context.CancelFunc, rt *runtime.Runtime, client *tmux.Client, sockPath, sessionName, exePath string) error {
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
		if shouldKeepRuntimeAliveAfterAttach(err, client.SessionExists()) {
			slog.Info("attach failed; keeping runtime alive", "session", sessionName)
			<-rt.Done()
			if err, ok := <-runErrCh; ok {
				close(runErrCh)
				return fmt.Errorf("runtime: %w", err)
			}
			close(runErrCh)
			if client.SessionExists() {
				slog.Info("detached, session kept alive")
			} else {
				slog.Info("tmux server exited")
			}
			return nil
		}
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

func shouldKeepRuntimeAliveAfterAttach(err error, sessionExists bool) bool {
	return err != nil && sessionExists
}

// newAgentLauncher returns the AgentLauncher for the configured sandbox mode.
// Returns a SandboxDispatcher that routes each launch to direct or devcontainer
// based on the effective config for that project (user scope + optional project scope).
func newAgentLauncher(ctx context.Context, sb config.SandboxConfig, resolver *config.SandboxResolver, dataDir, sockPath string) (runtime.AgentLauncher, error) {
	d := &runtime.SandboxDispatcher{
		Resolver: resolver,
		Direct:   runtime.DirectLauncher{SockPath: sockPath},
	}
	if sb.Mode == "devcontainer" {
		if _, err := exec.LookPath("docker"); err != nil {
			return nil, fmt.Errorf("sandbox: devcontainer mode requires docker in PATH: %w", err)
		}
		currentHost := os.Getenv("DOCKER_HOST")
		if host := runtime.ResolveDockerHost(
			currentHost,
			os.Getenv("XDG_RUNTIME_DIR"),
			func(p string) bool { _, err := os.Stat(p); return err == nil },
		); host != "" {
			_ = os.Setenv("DOCKER_HOST", host)
			slog.Info("sandbox: rootless docker detected", "DOCKER_HOST", host)
		} else if currentHost == "" {
			slog.Info("sandbox: using default docker socket (rootless not detected)")
		}
		runner, err := runtime.StartCredProxy(ctx, dataDir, func(project string) config.SandboxConfig {
			return resolver.Resolve(project)
		})
		if err != nil {
			return nil, fmt.Errorf("sandbox: start in-process credproxy: %w", err)
		}
		overlayFn := runtime.BuildOverlayFunc(func(project string) config.SandboxConfig {
			return resolver.Resolve(project)
		}, runner, dataDir, statedriver.SetupSubcmds())
		mgr := sandboxdc.New(overlayFn)
		d.Devcontainer = runtime.NewDevcontainerLauncher(mgr, func(project string) config.SandboxConfig {
			return resolver.Resolve(project)
		}, runner, dataDir)
		slog.Info("sandbox: devcontainer backend enabled")
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
	return statedriver.ShellDriverName
}

// resolveShellDisplay queries tmux's default-shell option (the shell tmux
// will actually spawn for login-shell panes) and falls back to $SHELL.
func resolveShellDisplay(client *tmux.Client) string {
	tmuxDefault, _ := client.ShowOption("default-shell")
	return resolveShellDisplayFromValues(tmuxDefault, os.Getenv("SHELL"))
}
