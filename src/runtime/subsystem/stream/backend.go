// Package stream implements the stream subsystem backend that fronts
// structured app-servers (currently codex app-server) via WebSocket-over-UDS.
// This is the only location in runtime/ permitted to import driver/<tool>.
package stream

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-roost/state"
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
	// HelperBinaryPath resolves a helper binary (e.g. "sockbridge") using the
	// canonical exe-adjacent + libexec search implemented in runtime/rundir.go.
	HelperBinaryPath(name string) (string, error)
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
	cmd           *exec.Cmd
	wsConn        *websocket.Conn
	sockPath      string // UDS path dialed by daemon (host-side)
	containerSock string // UDS path inside container
	hostBridgeCmd *exec.Cmd
	bridgePort    int
	writeMu       sync.Mutex
	mu            sync.Mutex
	nextID        int64
	pending       map[int64]chan rpcMessage
	frames        map[state.FrameID]*frameBinding
	threads       map[string]state.FrameID
	activeLookup  func() state.FrameID
}

type frameBinding struct {
	frameID         state.FrameID
	startDir        string
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
		sockPath:      sockPath,
		containerSock: containerSock,
		bridgePort:    bridgePort,
		activeLookup:  activeLookup,
		pending:       map[int64]chan rpcMessage{},
		frames:        map[state.FrameID]*frameBinding{},
		threads:       map[string]state.FrameID{},
	}
}

// BridgePort returns the loopback TCP port for the sockbridge TUI relay.
func (b *Backend) BridgePort() int { return b.bridgePort }

// Start launches the app-server process, dials the WebSocket, and begins
// the read loop. On failure the caller must not call Start again — create
// a new Backend instead.
func (b *Backend) Start() error {
	_ = os.Remove(b.sockPath)
	cmd, err := b.buildServerCommand()
	if err != nil {
		return err
	}
	var serverErrBuf strings.Builder
	cmd.Stderr = newPrefixWriter(&serverErrBuf, 8192)
	if err := cmd.Start(); err != nil {
		return err
	}
	wsConn, err := dialWebSocketUDS(b.sockPath, serverDialTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		slog.Error("stream backend: app-server dial failed",
			"subsystem", b.subsystemID, "sock", b.sockPath,
			"stderr", strings.TrimSpace(serverErrBuf.String()))
		return err
	}
	b.cmd = cmd
	b.wsConn = wsConn
	go func() {
		if err := b.runReadLoop(); err != nil {
			slog.Debug("stream backend: read loop closed", "subsystem", b.subsystemID, "err", err)
		}
	}()
	if err := b.initialize(); err != nil {
		_ = wsConn.Close(websocket.StatusInternalError, "initialize failed")
		_ = cmd.Process.Kill()
		return err
	}
	// Container mode: sockbridge is part of devcontainer postCreate
	// (registered via ContainerBridgeSpec). Host mode: spawn here.
	isContainer, err := b.isContainerProject()
	if err != nil {
		_ = wsConn.Close(websocket.StatusInternalError, "container check failed")
		_ = cmd.Process.Kill()
		return err
	}
	if !isContainer {
		if err := b.startHostBridge(); err != nil {
			_ = wsConn.Close(websocket.StatusInternalError, "host bridge failed")
			_ = cmd.Process.Kill()
			return fmt.Errorf("stream backend: host bridge: %w", err)
		}
	}
	go b.waitProcess()
	return nil
}

func (b *Backend) isContainerProject() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg, err := b.runtime.ContainerExecConfig(ctx, b.project)
	return cfg != nil, err
}

func (b *Backend) startHostBridge() error {
	bin, err := b.runtime.HelperBinaryPath("sockbridge")
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

func (b *Backend) buildServerCommand() (*exec.Cmd, error) {
	args := buildServerArgs(b.serverArgs, b.sandboxed, b.sockPath)
	ctx, cancel := context.WithTimeout(context.Background(), containerEnsureLimit)
	defer cancel()
	containerCfg, err := b.runtime.ContainerExecConfig(ctx, b.project)
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

// BindFrame associates a new frame with a thread in the app-server and
// returns the thread ID bound (either resumed or empty if a new thread).
func (b *Backend) BindFrame(frameID state.FrameID, startDir string, opts state.StreamLaunchOptions, stdin []byte) (string, error) {
	b.mu.Lock()
	b.frames[frameID] = &frameBinding{
		frameID:  frameID,
		startDir: startDir,
	}
	b.mu.Unlock()
	if opts.ResumeThreadID != "" {
		raw, err := b.resumeThread(opts.ResumeThreadID, startDir)
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
	if err := b.startTurn("", startDir, stdin); err != nil {
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
	_ = b.wsConn.Close(websocket.StatusNormalClosure, "")
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


