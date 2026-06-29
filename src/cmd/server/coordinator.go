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
	statedriver "github.com/takezoh/agent-reactor/client/driver"
	"github.com/takezoh/agent-reactor/client/lib/agenthook"
	"github.com/takezoh/agent-reactor/client/runtime"
	"github.com/takezoh/agent-reactor/client/runtime/worker"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	"github.com/takezoh/agent-reactor/platform/appid"
	platformconfig "github.com/takezoh/agent-reactor/platform/config"
	"github.com/takezoh/agent-reactor/platform/credproxy"
	"github.com/takezoh/agent-reactor/platform/features"
	"github.com/takezoh/agent-reactor/platform/logger"
	sandboxdc "github.com/takezoh/agent-reactor/platform/sandbox/devcontainer"
	"github.com/takezoh/agent-reactor/platform/shellalias"
)

// runDaemon boots the coordinator (event loop, IPC socket, persistence) and,
// once the socket is bound, the co-resident HTTP/WS gateway goroutine. Daemon
// and gateway share one cancellation context: SIGINT/SIGTERM cancels both.
func runDaemon(df *daemonFlagSet) error {
	if df == nil {
		df = defaultDaemonFlags()
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.Sandbox.Validate(); err != nil {
		return err
	}
	slog.Info("starting coordinator")

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
	registerHostAgentHooks(dataDir)

	// Resolve the passwd login shell once: it drives both alias resolution and
	// the shell driver's display name.
	loginShell, err := shellalias.LoginShell(ctx, shellalias.RealRunner)
	if err != nil {
		slog.Warn("shellalias: login shell lookup failed; commands stay literal", "err", err)
	}

	rt, sockPath, err := buildRuntime(ctx, cfg, loginShell, dataDir)
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

	return runAndWait(ctx, cancel, rt, sockPath, df)
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
}

// agentHookPostCreateSubcmds returns the reactor-bridge subcommand list the
// devcontainer postCreate runs to register reactor hooks inside the
// container, derived from agenthook.All so adding an agent in one place
// flows through to the container surface automatically.
func agentHookPostCreateSubcmds() []string {
	out := make([]string, 0, len(agenthook.All))
	for _, spec := range agenthook.All {
		out = append(out, spec.SubcmdName)
	}
	return out
}

// registerHostAgentHooks ensures the host's ~/.<agent>/settings.json routes
// every supported agent's lifecycle events at THIS server binary. Without
// it, host-direct (non-devcontainer) sessions never reach the daemon:
// SessionID stays empty, transcript path is never resolved, and the
// session card title sticks on "New Session" with status frozen. The
// in-container path is covered separately by the devcontainer postCreate;
// this fills the gap for SandboxModeDirect / SandboxOverrideHost sessions,
// which spawn the agent in the same filesystem namespace the daemon runs in.
//
// `-data-dir` is always appended. Yes, daemon-spawned agents inherit
// ROOST_SOCKET in their env and win via that env, but an agent process the
// user starts manually from a terminal (no ROOST_SOCKET) needs the flag to
// reach this daemon's socket. The redundant flag for the daemon-spawn case
// is cheaper than a missing flag for the manual-launch case — and the path
// is explicit and self-documenting in the registered settings.json.
//
// Failure for any one agent is best-effort and isolated: a settings.json
// the daemon can't write (read-only FS, permission denied, malformed
// pre-existing JSON) is logged at warn level and the other agents proceed.
// The user can still drive sessions for the agents whose registration
// succeeded; the failed agent just won't surface state in the card. A hard
// abort would punish every other code path for one file-permission edge case.
//
// Agent fan-out is driven by agenthook.All so adding a future agent CLI
// (Codex hooks, …) is a one-Spec change at the package level, not a
// parallel edit here AND in cmd/reactor-bridge.
func registerHostAgentHooks(dataDir string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		slog.Warn("host agent hooks: HOME unresolved, skipping registration", "err", err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("host agent hooks: self path unresolved, skipping registration", "err", err)
		return
	}
	if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = resolved
	}
	for _, spec := range agenthook.All {
		registerOneHostAgentHook(home, exe, dataDir, spec)
	}
}

func registerOneHostAgentHook(home, exe, dataDir string, spec agenthook.Spec) {
	hookCmd := agenthook.BuildHookCmd(exe, dataDir, spec)
	settings := filepath.Join(home, spec.SettingsRel)
	registered, err := agenthook.Install(settings, hookCmd, spec)
	if err != nil {
		slog.Warn("host agent hooks: registration failed",
			"agent", spec.Name, "settings", settings, "err", err)
		return
	}
	if len(registered) == 0 {
		slog.Debug("host agent hooks: already up to date",
			"agent", spec.Name, "settings", settings)
		return
	}
	slog.Info("host agent hooks: registered",
		"agent", spec.Name, "settings", settings,
		"events", len(registered), "hookCmd", hookCmd)
}

// buildRuntime constructs and configures the Runtime. Returns the Runtime,
// the socket path it will listen on, and any error.
//
// Backend is hard-wired to PtyBackend (ADR 0004 / B1b). The runtime drives a
// private termvt.Manager directly. Config.Tap is a PtyFrameTap (plan A 5a/5b) that wraps the same
// Manager's Session.Subscribe stream, so the existing tap_manager + vt.Terminal
// pipeline parses OSC 0/9/133 back into EvFrameOsc/EvFramePrompt — restoring
// driver run-state detection on top of the pty backend.
func buildRuntime(ctx context.Context, cfg *config.Config, loginShell string, dataDir string) (*runtime.Runtime, string, error) {
	ptyBackend := runtime.NewPtyBackend(cfg.Terminal.ScrollbackLines)
	ptyTap := runtime.NewPtyFrameTap(ptyBackend)
	pollInterval := time.Duration(cfg.Monitor.PollIntervalMs) * time.Millisecond
	sockPath := filepath.Join(dataDir, appid.SocketFileName)

	pool := worker.NewPool(ctx, 4)

	featureSet := features.FromConfig(cfg.Features.Enabled, features.All())
	sbResolver := platformconfig.NewSandboxResolver(cfg.Sandbox)
	agentLauncher, streamDispatcher, err := newAgentLauncher(ctx, cfg.Sandbox, sbResolver, cfg.Projects, dataDir, sockPath, cfg.ResolveDevcontainerPrefix())
	if err != nil {
		return nil, "", err
	}
	rt := runtime.New(runtime.Config{
		DataDir:           dataDir,
		TickInterval:      pollInterval,
		Backend:           ptyBackend,
		Persist:           runtime.NewFilePersist(dataDir),
		EventLog:          runtime.NewFileEventLog(dataDir),
		ToolLog:           runtime.NewFileToolLog(dataDir),
		Pool:              pool,
		Tap:               ptyTap,
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
	return rt, sockPath, nil
}

// startSession registers the shell driver and runs the boot bootstrap.
// PtyBackend's termvt sessions die with the daemon, so each boot creates fresh
// frames from the persisted snapshot. Devcontainer containers and codex remote
// threads can outlive a daemon restart, so the boot sequence first asks the
// sandbox to adopt surviving frames (RecoverSandboxFrames) and asks durable
// drivers to resume their state (RecoverWarmStartSessions) before falling back
// to fresh container provisioning for un-adoptable frames (RecreateAll).
//
// References: ADR 0004 (decision 2).
func startSession(ctx context.Context, rt *runtime.Runtime, loginShell string, idleThreshold time.Duration) error {
	shellDriver := statedriver.NewShellDriver(statedriver.ShellDriverName, shellDisplayName(loginShell), idleThreshold)
	if err := bootSession(ctx, rt, shellDriver); err != nil {
		return err
	}
	go rt.CleanupSubsystems(ctx)
	return nil
}

func bootSession(ctx context.Context, rt *runtime.Runtime, shellDriver statedriver.ShellDriver) error {
	slog.Info("booting session")
	state.Register(shellDriver)
	if err := rt.LoadSnapshot(true); err != nil {
		slog.Error("snapshot load failed", "err", err)
	}

	// Try to adopt sandbox containers / codex threads that outlived the previous
	// daemon. Adopted frames carry their cleanup callback + container mounts
	// forward so the subsequent fresh-frame spawn re-attaches to the same agent.
	// Failures are silent: the frame just goes through the normal cold-spawn
	// path in RecreateAll.
	adoptCtx, adoptCancel := context.WithTimeout(ctx, 2*time.Minute)
	adopted := rt.RecoverSandboxFrames(adoptCtx)
	adoptCancel()
	rt.RecoverWarmStartSessions()
	slog.Info("boot: sandbox adoption complete", "adopted_frames", len(adopted))

	rt.PrewarmContainers(ctx)
	if err := rt.RecreateAll(); err != nil {
		slog.Error("recreate failed", "err", err)
	}
	return nil
}

// superviseRun runs fn and converts panics into logged errors on errCh,
// then cancels the parent context so the rest of the daemon can shut down.
// Panics must not escape: any goroutine panic kills the process, dropping all
// IPC clients via socket EOF.
//
// errCh is closed when fn returns (the outer defer fires last in LIFO order, so
// the panic-recovery defer still sees an open channel for its send). This lets
// runAndWait read the channel with a comma-ok receive that distinguishes "no
// error reported" from "channel closed without a send" without racing main
// against this goroutine on close.
func superviseRun(cancel context.CancelFunc, errCh chan<- error, fn func() error) {
	defer close(errCh)
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

// shutdownDrainTimeout caps how long we wait for the EffReleaseFrameSandboxes
// drain. Sized to fit inside systemd's TimeoutStopSec= so a stuck per-frame
// cleanup never blocks the unit indefinitely.
const shutdownDrainTimeout = 8 * time.Second

// installSignalHandlers wires SIGINT/SIGTERM/SIGHUP into the coordinator
// context. SIGINT/SIGTERM trigger graceful shutdown: requestShutdown
// enqueues EventShutdown so the reducer can drain sandbox cleanups
// (containers, codex app-servers, …) before the runtime context is
// cancelled and the event loop exits. SIGHUP is logged and ignored — a
// parent terminal can deliver spurious SIGHUP when its own window closes
// (WSL2 init quirks etc.), and the daemon should outlive that signal
// because session state lives in memory and persists across the IPC
// socket rather than in the controlling terminal.
//
// requestShutdown is a callback rather than a *runtime.Runtime so tests
// (and any future caller without a live runtime) can install handlers
// against a no-op stub. Pass nil to skip the shutdown drain entirely.
//
// Returns a stop function the caller must defer to restore default handlers.
func installSignalHandlers(requestShutdown func(time.Duration), cancel context.CancelFunc) func() {
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
			if requestShutdown != nil {
				requestShutdown(shutdownDrainTimeout)
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

// runAndWait starts the event loop, IPC server, and the co-resident HTTP/WS
// gateway, then blocks until the runtime exits (cancel from signal handler /
// runtime error). The gateway runs in-process on the daemon ctx — a panic in
// the gateway's listener or per-conn goroutines is contained by recover() in
// startGateway so the daemon keeps serving its IPC socket.
func runAndWait(ctx context.Context, cancel context.CancelFunc, rt *runtime.Runtime, sockPath string, df *daemonFlagSet) error {
	stopSignals := installSignalHandlers(rt.RequestShutdown, cancel)
	defer stopSignals()
	runErrCh := make(chan error, 1)
	go superviseRun(cancel, runErrCh, func() error { return rt.Run(ctx) })
	// Cold-start path: LoadSnapshot → RecreateAll spawns backend sessions
	// directly without emitting EffRegisterFrame, so root frames restored from
	// sessions.json never reach tap_manager.start through the reducer. With
	// Config.Tap now wired to PtyFrameTap (plan A0), reinstate the bootstrap
	// call so restored frames get an OSC tap.
	rt.StartTapsForRestoredFrames()
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
	gw, err := startGateway(ctx, cancel, sockPath, df)
	if err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	defer gw.Close()
	<-rt.Done()
	// superviseRun closes runErrCh on return. comma-ok distinguishes
	// "supervised goroutine reported an error" from "clean exit".
	if err, ok := <-runErrCh; ok {
		return fmt.Errorf("runtime: %w", err)
	}
	slog.Info("coordinator: runtime stopped")
	return nil
}

// newAgentLauncher returns the AgentLauncher (TTY, for backend frames) and
// StreamDispatcher (non-TTY, for codex app-server stdio) for the configured
// sandbox mode. Both dispatchers share the same devcontainer manager so
// container provisioning is consistent. namePrefix is propagated to the
// devcontainer Manager and ultimately to ContainerName + label keys, so two
// peer daemons (primary vs. run-dev gateway) under distinct prefixes never
// compete for the same docker container name.
func newAgentLauncher(ctx context.Context, sb platformconfig.SandboxConfig, resolver *platformconfig.SandboxResolver, projects platformconfig.ProjectsConfig, dataDir, sockPath, namePrefix string) (runtime.AgentLauncher, agentlaunch.Dispatcher, error) {
	newDispatcher := func() *agentlaunch.SandboxDispatcher {
		return &agentlaunch.SandboxDispatcher{
			Resolver: resolver,
			Direct:   agentlaunch.DirectDispatcher{SockPath: sockPath},
		}
	}
	d := newDispatcher()
	sd := newDispatcher()
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
		// "{claude,gemini}-setup-hooks" run as devcontainer postCreate so the
		// moment a container comes up, ~/.<agent>/settings.json inside it
		// points every lifecycle event at /opt/agent-reactor/run/reactor-bridge.
		// Without these the daemon never learns SessionID for in-container
		// sessions and card titles stick on "New Session" forever. Phase F-C
		// had pushed this to scripts/setup-{claude,gemini}.sh; that delegation
		// made hook state an out-of-band setup step rather than a runtime
		// invariant, and the scripts have since been removed in favour of
		// reactor-bridge owning the registration end-to-end via
		// client/lib/agenthook.
		overlayFn := agentlaunch.BuildContainerOverlay(func(project string) platformconfig.SandboxConfig {
			return resolver.Resolve(project)
		}, projects, runner, dataDir, agentHookPostCreateSubcmds())
		mgr := sandboxdc.NewWithPrefix(overlayFn, namePrefix)
		slog.Info("sandbox: devcontainer prefix", "prefix", mgr.NamePrefix())
		newDC := func(tty bool) *agentlaunch.DevcontainerLauncher {
			return agentlaunch.NewDevcontainerLauncher(mgr,
				func(project string) platformconfig.SandboxConfig { return resolver.Resolve(project) },
				func(project string) *platformconfig.SandboxConfig { return resolver.ResolveProjectScope(project) },
				runner,
				dataDir,
				tty,
			)
		}
		// d is for interactive backend frames (TTY=true); sd is the stream
		// dispatcher used by codex app-server stdio (TTY=false).
		d.Devcontainer = newDC(true)
		sd.Devcontainer = newDC(false)
		slog.Info("sandbox: devcontainer backend enabled")
	}
	return runtime.NewDispatcherAdapter(d), sd, nil
}

// shellDisplayName picks the display name (basename) for the shell driver from
// the user's passwd login-shell path — the same shell the client launches for
// `shell` sessions. It does not consult a multiplexer default-shell option or $SHELL.
func shellDisplayName(shell string) string {
	if name := filepath.Base(shell); name != "" && name != "." && name != "/" {
		return name
	}
	return statedriver.ShellDriverName
}
