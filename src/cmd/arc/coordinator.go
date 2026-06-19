package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/connector"
	statedriver "github.com/takezoh/agent-reactor/client/driver"
	"github.com/takezoh/agent-reactor/client/runtime"
	"github.com/takezoh/agent-reactor/client/runtime/worker"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/appid"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/credproxy"
	"github.com/takezoh/agent-reactor/platform/features"
	libnotify "github.com/takezoh/agent-reactor/platform/lib/notify"
	"github.com/takezoh/agent-reactor/platform/logger"
	sandboxdc "github.com/takezoh/agent-reactor/platform/sandbox/devcontainer"
	"github.com/takezoh/agent-reactor/platform/shellalias"
)

func runCoordinator() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.Sandbox.Validate(); err != nil {
		return err
	}
	sessionName := cfg.Tmux.SessionName
	slog.Info("starting coordinator", "session", sessionName)

	dataDir := cfg.ResolveDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}

	// Take the single-daemon lock before touching the sessions directory. Two
	// coordinators against the same data dir each run their own event loop and
	// persistence pass, fighting over ~/.agent-reactor/sessions (one rewrites
	// session files the other has just deleted), which makes terminated
	// sessions resurrect on every cold start.
	lock, err := acquireDaemonLock(filepath.Join(dataDir, appid.PidFileName))
	if err != nil {
		return err
	}
	defer lock.release()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idleThreshold := time.Duration(cfg.Monitor.IdleThresholdSec) * time.Second
	registerDefaultDrivers(cfg, dataDir, idleThreshold)

	// Resolve the passwd login shell once: it drives both alias resolution and
	// the shell driver's display name.
	loginShell, err := shellalias.LoginShell(ctx, shellalias.RealRunner)
	if err != nil {
		slog.Warn("shellalias: login shell lookup failed; commands stay literal", "err", err)
	}

	rt, sockPath, _, err := buildRuntime(ctx, cfg, loginShell, dataDir)
	if err != nil {
		return err
	}
	// Bind subsystem goroutines / spawned process groups to the daemon context
	// so shutdown cascades into them. Must precede cold-start spawning.
	rt.SetBaseContext(ctx)
	// Reap host process groups (codex app-server, sockbridge) orphaned by a
	// prior daemon boot that died without a graceful Stop. Must run before any
	// new spawn so it only targets earlier boots' markers.
	rt.PruneProcessGroups()

	if err := startSession(ctx, rt, loginShell, idleThreshold); err != nil {
		return err
	}

	return runAndWait(ctx, cancel, rt, sockPath)
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
// the socket path it will listen on, the resolved client binary path, and any error.
//
// Backend is hard-wired to PtyBackend (ADR 0004 / B1b). The runtime drives a
// private termvt.Manager directly; tmux is no longer involved in coordinator
// startup. Config.Tap is nil — the legacy TmuxPipePaneTap is gone, and a
// termvt.Session.Subscribe-based pty_tap lands in plan A together with the web
// display surface that will consume it.
func buildRuntime(ctx context.Context, cfg *config.Config, loginShell string, dataDir string) (*runtime.Runtime, string, string, error) {
	ptyBackend := runtime.NewPtyBackend()
	pollInterval := time.Duration(cfg.Monitor.PollIntervalMs) * time.Millisecond
	fastPollInterval := time.Duration(cfg.Monitor.FastPollIntervalMs) * time.Millisecond
	sockPath := filepath.Join(dataDir, appid.SocketFileName)

	pool := worker.NewPool(ctx, 4)
	ln, err := libnotify.New(ctx, runtime.FindHelperFile("notify.ps1"))
	if err != nil {
		return nil, "", "", fmt.Errorf("notify: %w", err)
	}

	featureSet := features.FromConfig(cfg.Features.Enabled, features.All())
	sbResolver := platformconfig.NewSandboxResolver(cfg.Sandbox)
	agentLauncher, streamDispatcher, err := newAgentLauncher(ctx, cfg.Sandbox, sbResolver, cfg.Projects, dataDir, sockPath)
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
		Tmux:              ptyBackend,
		Persist:           runtime.NewFilePersist(dataDir),
		EventLog:          runtime.NewFileEventLog(dataDir),
		ToolLog:           runtime.NewFileToolLog(dataDir),
		Pool:              pool,
		Notifier:          runtime.NewNotifier(&cfg.Notifications, ln),
		Tap:               nil,
		Features:          featureSet,
		Launcher:          agentLauncher,
		StreamDispatcher:  streamDispatcher,
		StreamReadTimeout: time.Duration(cfg.Codex.ReadTimeoutMs) * time.Millisecond,
	})
	resolved, err := shellalias.Resolve(ctx, loginShell, cfg.Session.Commands, shellalias.RealRunner)
	if err != nil {
		slog.Warn("shellalias: resolve failed; commands stay literal", "err", err)
	}
	rt.SetAliases(resolved)
	rt.SetDefaultCommand(cfg.Session.DefaultCommand)
	rt.SetSandboxedProjectResolver(func(project string) bool {
		return sbResolver.Resolve(project).IsSandboxed()
	})
	return rt, sockPath, exePath, nil
}

// startSession registers the shell driver and runs the cold-start bootstrap.
// Warm restart is gone with the tmux backend: PtyBackend's termvt sessions die
// with the daemon, so cross-restart pane recovery is not in scope (ADR 0004,
// decision 2). LoadSessionPanes / LoadSnapshot still run and walk persisted
// state to recreate frames from scratch.
func startSession(ctx context.Context, rt *runtime.Runtime, loginShell string, idleThreshold time.Duration) error {
	shellDriver := statedriver.NewShellDriver(statedriver.ShellDriverName, shellDisplayName(loginShell), idleThreshold)
	if err := coldStart(ctx, rt, shellDriver); err != nil {
		return err
	}
	go rt.CleanupSubsystems(ctx)
	return nil
}

func coldStart(ctx context.Context, rt *runtime.Runtime, shellDriver statedriver.ShellDriver) error {
	slog.Info("creating new session")
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
	// Tell the launcher we are in cold-start mode so EnsureProject /
	// WrapLaunch issued during prewarm + recreate discard any container
	// that survived a non-graceful daemon exit and provision a fresh one.
	if cs, ok := launcherFor(rt).(runtime.ColdStartAware); ok {
		cs.BeginColdStart()
		defer cs.EndColdStart()
	}
	rt.PrewarmContainers(ctx)
	if err := rt.RecreateAll(); err != nil {
		slog.Error("recreate failed", "err", err)
	}
	return nil
}

// launcherFor exposes the runtime's AgentLauncher for the coordinator's
// cold-start switchover. Wrapped so callers don't reach into runtime
// internals directly.
func launcherFor(rt *runtime.Runtime) runtime.AgentLauncher {
	return rt.Launcher()
}

// superviseRun runs fn and converts panics into logged errors on errCh,
// then cancels the parent context so the rest of the daemon can shut down.
// Panics must not escape: any goroutine panic kills the process, dropping all
// IPC clients via socket EOF.
func superviseRun(cancel context.CancelFunc, errCh chan<- error, fn func() error) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("runtime: goroutine panicked",
				"err", fmt.Sprintf("%v", rec),
				"stack", string(debug.Stack()))
			errCh <- fmt.Errorf("runtime panic: %v", rec)
			cancel()
		}
	}()
	if err := fn(); err != nil && !errors.Is(err, context.Canceled) {
		errCh <- err
	}
}

// installSignalHandlers wires SIGINT/SIGTERM/SIGHUP into the coordinator
// context. SIGINT/SIGTERM cancel the context for graceful shutdown. SIGHUP is
// logged and ignored — a parent terminal can deliver spurious SIGHUP when its
// own window closes (WSL2 init quirks etc.), and the daemon should outlive
// that signal because session state lives in memory and persists across the
// IPC socket rather than in the controlling terminal.
//
// Returns a stop function the caller must defer to restore default handlers.
func installSignalHandlers(cancel context.CancelFunc) func() {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for sig := range sigCh {
			slog.Info("coordinator: signal received", "signal", sig.String())
			if sig == syscall.SIGHUP {
				continue
			}
			cancel()
			return
		}
	}()
	return func() {
		signal.Stop(sigCh)
		close(sigCh)
		<-done
	}
}

// runAndWait starts the event loop and IPC server, then blocks until the
// runtime exits (cancel from signal handler / runtime error). With the tmux
// backend gone there is no `tmux attach-session` step — the daemon is
// headless and clients drive it through the IPC socket. Plan A wires the web
// display surface onto runtime via pure-core reuse; until then the daemon is
// reachable but visually opaque.
func runAndWait(ctx context.Context, cancel context.CancelFunc, rt *runtime.Runtime, sockPath string) error {
	stopSignals := installSignalHandlers(cancel)
	defer stopSignals()
	runErrCh := make(chan error, 1)
	go superviseRun(cancel, runErrCh, func() error { return rt.Run(ctx) })
	// StartTapsForRestoredFrames is intentionally not called here: with
	// Config.Tap=nil the tap_manager early-returns and the bootstrap event
	// would be a no-op (B1b / ADR 0004). Plan A reinstates the call once a
	// termvt.Session.Subscribe-backed pty_tap lands.
	if err := rt.StartIPC(sockPath); err != nil {
		return fmt.Errorf("ipc: %w", err)
	}
	slog.Info("server started", "sock", sockPath)
	if relay, err := runtime.NewFileRelay(rt); err != nil {
		slog.Warn("filerelay: start failed", "err", err)
	} else {
		defer relay.Close()
		relay.WatchLog(logger.LogFilePath())
		rt.SetRelay(relay)
	}
	<-rt.Done()
	close(runErrCh)
	if err, ok := <-runErrCh; ok {
		return fmt.Errorf("runtime: %w", err)
	}
	slog.Info("coordinator: runtime stopped")
	return nil
}

// newAgentLauncher returns the AgentLauncher (TTY, for tmux panes) and
// StreamDispatcher (non-TTY, for codex app-server stdio) for the configured
// sandbox mode. Both dispatchers share the same devcontainer manager so
// container provisioning is consistent.
func newAgentLauncher(ctx context.Context, sb platformconfig.SandboxConfig, resolver *platformconfig.SandboxResolver, projects platformconfig.ProjectsConfig, dataDir, sockPath string) (runtime.AgentLauncher, agentlaunch.Dispatcher, error) {
	d := &agentlaunch.SandboxDispatcher{
		Resolver: resolver,
		Direct:   agentlaunch.DirectDispatcher{SockPath: sockPath},
	}
	sd := &agentlaunch.SandboxDispatcher{
		Resolver: resolver,
		Direct:   agentlaunch.DirectDispatcher{SockPath: sockPath},
	}
	if sb.Mode == "devcontainer" {
		if _, err := exec.LookPath("docker"); err != nil {
			return nil, nil, fmt.Errorf("sandbox: devcontainer mode requires docker in PATH: %w", err)
		}
		currentHost := os.Getenv("DOCKER_HOST")
		if host := platformconfig.ResolveDockerHost(
			currentHost,
			os.Getenv("XDG_RUNTIME_DIR"),
			func(p string) bool { _, err := os.Stat(p); return err == nil },
		); host != "" {
			_ = os.Setenv("DOCKER_HOST", host)
			slog.Info("sandbox: rootless docker detected", "DOCKER_HOST", host)
		} else if currentHost == "" {
			slog.Info("sandbox: using default docker socket (rootless not detected)")
		}
		runner, err := credproxy.Start(ctx, dataDir, resolver.Resolve, agentlaunch.BuildProviderHooks(resolver.Resolve, projects), credproxy.Paths{
			RunDir:  agentlaunch.ContainerRunDir,
			BinPath: agentlaunch.ContainerBinaryPath,
			MCPSock: agentlaunch.ContainerMCPSockPath,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("sandbox: start in-process credproxy: %w", err)
		}
		// credproxy providers run under a child of this ctx (the daemon context).
		// The coordinator's defer cancel() on graceful shutdown cascades into
		// them, reaping provider-managed processes such as the ssh-agent. detach
		// leaves ctx live so the agent survives for warm restart.
		overlayFn := agentlaunch.BuildContainerOverlay(func(project string) platformconfig.SandboxConfig {
			return resolver.Resolve(project)
		}, projects, runner, dataDir, statedriver.SetupSubcmds())
		mgr := sandboxdc.New(overlayFn)
		d.Devcontainer = agentlaunch.NewDevcontainerLauncher(mgr,
			func(project string) platformconfig.SandboxConfig { return resolver.Resolve(project) },
			func(project string) *platformconfig.SandboxConfig { return resolver.ResolveProjectScope(project) },
			runner,
			dataDir,
			true, // TUI runs agents in interactive tmux panes: allocate a TTY
		)
		sd.Devcontainer = agentlaunch.NewDevcontainerLauncher(mgr,
			func(project string) platformconfig.SandboxConfig { return resolver.Resolve(project) },
			func(project string) *platformconfig.SandboxConfig { return resolver.ResolveProjectScope(project) },
			runner,
			dataDir,
			false, // stream daemon uses stdio JSON-RPC: no TTY
		)
		slog.Info("sandbox: devcontainer backend enabled")
	}
	return runtime.NewDispatcherAdapter(d), sd, nil
}

// shellDisplayName picks the display name (basename) for the shell driver from
// the user's passwd login-shell path — the same shell the client launches for
// `shell` sessions. It does not consult tmux's default-shell option or $SHELL.
func shellDisplayName(shell string) string {
	if name := filepath.Base(shell); name != "" && name != "." && name != "/" {
		return name
	}
	return statedriver.ShellDriverName
}
