// Package stream implements the stream subsystem backend that fronts
// structured app-servers (currently codex app-server) via WebSocket-over-UDS.
// This is the only location in runtime/ permitted to import driver/<tool>.
package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
)

const (
	serverDialTimeout    = 15 * time.Second
	containerEnsureLimit = 120 * time.Second // matches devcontainer_launcher.containerEnsureTimeout; cannot share due to import cycle

	resumePhasePending  = "resume_pending"
	resumePhaseAttached = "attached"
)

// RuntimeHook is implemented by *runtime.Runtime and lets the stream backend
// enqueue events and build container server commands without a circular import.
type RuntimeHook interface {
	Enqueue(event state.Event)
	// ContainerExecConfig returns docker exec parameters for the project's devcontainer,
	// or nil if the project runs directly on the host.
	ContainerExecConfig(ctx context.Context, project string) (*ContainerExecConfig, error)
}

// ContainerExecConfig carries the docker exec parameters needed to run a
// command inside the project container.
type ContainerExecConfig struct {
	ContainerID string
	User        string // empty = default user
	WorkDir     string // empty = default cwd
	PreExec     string // shell command to run before the binary (mise/asdf init), may be empty
}

// Backend is the codex app-server stream subsystem. One instance exists per
// subsystem ID (project×sandbox). It manages the app-server process, the
// WebSocket-over-UDS connection, and per-frame thread bindings.
type Backend struct {
	runtime       RuntimeHook
	subsystemID   state.SubsystemID
	project       string
	serverBin     string
	serverArgs    []string
	model         string
	sandboxed     bool
	autoApprove   bool
	readTimeout   time.Duration
	cmd           *exec.Cmd
	conn          *codexclient.Conn
	sockPath      string // UDS path dialed by daemon (host-side)
	containerSock string // UDS path inside container
	hostBridgeCmd *exec.Cmd
	bridgePort    int
	mu            sync.Mutex
	frames        map[state.FrameID]*frameBinding
	threads       map[string]state.FrameID
	activeLookup  func() state.FrameID
}

type frameBinding struct {
	frameID         state.FrameID
	startDir        string
	worktreePath    string // non-empty when a managed worktree was adopted or created
	threadID        string
	requestedID     string
	observedID      string
	resumePhase     string
	threadStatus    string
	waitApproval    bool
	activeTurnID    string
	lastAssistant   string
	history         []state.SubsystemTurn
	failureReported bool
}

// New constructs a Backend. Call Start before calling BindFrame.
func New(
	rt RuntimeHook,
	subsystemID state.SubsystemID,
	project, serverBin string,
	serverArgs []string,
	model string,
	sandboxed, autoApprove bool,
	sockPath, containerSock string,
	bridgePort int,
	activeLookup func() state.FrameID,
	readTimeout time.Duration,
) *Backend {
	return &Backend{
		runtime:       rt,
		subsystemID:   subsystemID,
		project:       project,
		serverBin:     serverBin,
		serverArgs:    serverArgs,
		model:         model,
		sandboxed:     sandboxed,
		autoApprove:   autoApprove,
		readTimeout:   readTimeout,
		sockPath:      sockPath,
		containerSock: containerSock,
		bridgePort:    bridgePort,
		activeLookup:  activeLookup,
		frames:        map[state.FrameID]*frameBinding{},
		threads:       map[string]state.FrameID{},
	}
}

// Kind implements subsystem.Subsystem.
func (b *Backend) Kind() state.LaunchSubsystem { return state.LaunchSubsystemStream }

// BridgePort returns the loopback TCP port for the sockbridge TUI relay.
func (b *Backend) BridgePort() int { return b.bridgePort }

// Start launches the app-server process, dials the WebSocket, and begins
// the read loop. On failure the caller must not call Start again — create
// a new Backend instead.
func (b *Backend) Start(ctx context.Context) error {
	_ = os.Remove(b.sockPath)
	cmd, err := b.buildServerCommand(ctx)
	if err != nil {
		return err
	}
	var serverErrBuf strings.Builder
	cmd.Stderr = newPrefixWriter(&serverErrBuf, 8192)
	if err := cmd.Start(); err != nil {
		return err
	}
	t, err := codexclient.DialUDS(b.sockPath, serverDialTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		slog.Error("stream backend: app-server dial failed",
			"subsystem", b.subsystemID, "sock", b.sockPath,
			"stderr", strings.TrimSpace(serverErrBuf.String()))
		return err
	}
	b.cmd = cmd
	b.conn = codexclient.NewConn(t, b.readTimeout)
	go func() {
		if err := b.conn.Run(context.Background(), b); err != nil {
			slog.Debug("stream backend: read loop closed", "subsystem", b.subsystemID, "err", err)
		}
	}()
	if err := codexclient.Initialize(b.conn); err != nil {
		_ = b.conn.Close()
		_ = cmd.Process.Kill()
		return err
	}
	// Container mode: sockbridge is part of devcontainer postCreate
	// (registered via ContainerBridgeSpec). Host mode: spawn here.
	isContainer, err := b.isContainerProject(ctx)
	if err != nil {
		_ = b.conn.Close()
		_ = cmd.Process.Kill()
		return err
	}
	if !isContainer {
		if err := b.startHostBridge(); err != nil {
			_ = b.conn.Close()
			_ = cmd.Process.Kill()
			return fmt.Errorf("stream backend: host bridge: %w", err)
		}
	}
	go b.waitProcess()
	return nil
}

// BindFrame implements subsystem.Subsystem. It resolves worktree (if requested),
// binds or resumes an app-server thread, and rewrites Plan.Command to the
// remote-attach command.
func (b *Backend) BindFrame(ctx context.Context, req subsystem.BindRequest) (subsystem.BindResult, error) {
	result := subsystem.BindResult{Plan: req.Plan}
	startDir := req.Plan.StartDir

	// Worktree resolution: same logic as CLI backend.
	var worktreePath string
	switch {
	case subsystem.IsManagedWorktreePath(startDir):
		worktreePath = startDir
		result.WorktreeStartDir = startDir
	case req.Plan.Options.Worktree.Enabled:
		names := subsystem.GenerateWorktreeNames(subsystem.WorktreeNameAttempts)
		wt, err := subsystem.CreateWorktree(ctx, subsystem.WorktreeInput{
			RepoDir:        startDir,
			CandidateNames: names,
		})
		if err != nil {
			return subsystem.BindResult{}, err
		}
		startDir = wt.StartDir
		worktreePath = wt.StartDir
		result.Plan.StartDir = startDir
		result.WorktreeStartDir = wt.StartDir
		result.WorktreeName = wt.Name
	}

	// Thread binding.
	threadID, err := b.bindThread(req.FrameID, startDir, worktreePath, req.Plan.Stream, req.Stdin)
	if err != nil {
		return subsystem.BindResult{}, err
	}
	result.Plan.Command = BuildRemoteCommand(b.bridgePort, threadID, startDir)
	result.Plan.Stdin = nil
	result.Plan.Stream.ResumeThreadID = threadID
	return result, nil
}

// ReleaseFrame removes the frame from the registry and its thread mapping.
func (b *Backend) ReleaseFrame(frameID state.FrameID) {
	b.mu.Lock()
	binding := b.frames[frameID]
	delete(b.frames, frameID)
	if binding != nil && binding.threadID != "" {
		delete(b.threads, binding.threadID)
	}
	b.mu.Unlock()
}

// Stop kills the app-server process. The waitProcess goroutine handles cleanup.
func (b *Backend) Stop(_ context.Context) {
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}

// OnNotification implements codexclient.Handler.
func (b *Backend) OnNotification(method string, params json.RawMessage) {
	b.handleNotification(method, params)
}

// OnServerRequest implements codexclient.Handler.
func (b *Backend) OnServerRequest(id int64, method string, params json.RawMessage) {
	b.handleRequest(id, method, params)
}

func (b *Backend) isContainerProject(ctx context.Context) (bool, error) {
	cfg, err := b.runtime.ContainerExecConfig(ctx, b.project)
	return cfg != nil, err
}

func (b *Backend) startHostBridge() error {
	bin, err := findHelperBin("sockbridge")
	if err != nil {
		return err
	}
	cmd := exec.Command(bin,
		"-listen", fmt.Sprintf("127.0.0.1:%d", b.bridgePort),
		"-socket", b.sockPath,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	b.hostBridgeCmd = cmd
	return nil
}

func (b *Backend) buildServerCommand(ctx context.Context) (*exec.Cmd, error) {
	args := buildServerArgs(b.serverArgs, b.sandboxed, b.sockPath)
	containerCtx, cancel := context.WithTimeout(ctx, containerEnsureLimit)
	defer cancel()
	containerCfg, err := b.runtime.ContainerExecConfig(containerCtx, b.project)
	if err != nil {
		return nil, err
	}
	if containerCfg == nil {
		return exec.Command(b.serverBin, args...), nil
	}
	containerArgs := buildServerArgs(b.serverArgs, b.sandboxed, b.containerSock)
	execArgs := []string{"exec", "-i"}
	if containerCfg.User != "" {
		execArgs = append(execArgs, "-u", containerCfg.User)
	}
	if containerCfg.WorkDir != "" {
		execArgs = append(execArgs, "-w", containerCfg.WorkDir)
	}
	execArgs = append(execArgs, containerCfg.ContainerID)
	if containerCfg.PreExec != "" {
		serverCmdline := shellJoinArgv(append([]string{b.serverBin}, containerArgs...))
		execArgs = append(execArgs, "bash", "-lc", containerCfg.PreExec+"; exec "+serverCmdline)
	} else {
		execArgs = append(execArgs, b.serverBin)
		execArgs = append(execArgs, containerArgs...)
	}
	return exec.Command("docker", execArgs...), nil
}

// bindThread associates a new frame with a thread in the app-server and
// returns the thread ID bound (either resumed or empty if a new thread).
func (b *Backend) bindThread(frameID state.FrameID, startDir, worktreePath string, opts state.StreamLaunchOptions, stdin []byte) (string, error) {
	b.mu.Lock()
	b.frames[frameID] = &frameBinding{
		frameID:      frameID,
		startDir:     startDir,
		worktreePath: worktreePath,
	}
	b.mu.Unlock()
	if opts.ResumeThreadID != "" {
		raw, err := codexclient.ResumeThread(b.conn, opts.ResumeThreadID, startDir)
		if err != nil {
			return "", err
		}
		threadID := extractThreadID(raw)
		if threadID == "" {
			threadID = opts.ResumeThreadID
		}
		b.mu.Lock()
		if binding := b.frames[frameID]; binding != nil {
			binding.threadID = threadID
			binding.requestedID = opts.ResumeThreadID
			binding.observedID = threadID
			binding.resumePhase = resumePhasePending
		}
		b.threads[threadID] = frameID
		b.mu.Unlock()
		return threadID, nil
	}
	if err := codexclient.StartTurn(b.conn, "", startDir, stdin); err != nil {
		return "", err
	}
	return "", nil
}

func (b *Backend) waitProcess() {
	err := b.cmd.Wait()
	if err != nil {
		slog.Error("stream backend exited", "subsystem", b.subsystemID, "err", err)
	} else {
		slog.Warn("stream backend exited", "subsystem", b.subsystemID)
	}
	if b.hostBridgeCmd != nil && b.hostBridgeCmd.Process != nil {
		_ = b.hostBridgeCmd.Process.Kill()
		_ = b.hostBridgeCmd.Wait()
	}
	_ = b.conn.Close()
	b.mu.Lock()
	frameIDs := make([]state.FrameID, 0, len(b.frames))
	for frameID := range b.frames {
		frameIDs = append(frameIDs, frameID)
	}
	b.mu.Unlock()
	for _, frameID := range frameIDs {
		b.failFrame(frameID, fmt.Errorf("stream backend stopped: %w", err))
	}
}

func (b *Backend) frameForThread(threadID string) state.FrameID {
	if threadID == "" {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.threads[threadID]
}

func (b *Backend) emit(frameID state.FrameID, kind state.SubsystemEventKind, payload state.SubsystemPayload) {
	b.runtime.Enqueue(state.EvSubsystem{
		ConnID:    0,
		FrameID:   frameID,
		Source:    state.SubsystemStream,
		Kind:      kind,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

// findHelperBin locates a helper binary by checking the directory of the current
// executable first, then PATH.
func findHelperBin(name string) (string, error) {
	if selfPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(selfPath), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("stream backend: %s binary not found in PATH or alongside roost", name)
	}
	return path, nil
}
