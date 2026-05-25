// Package stream implements the stream subsystem backend that fronts
// structured app-servers (currently codex app-server) via WebSocket-over-UDS.
// This is the only location in runtime/ permitted to import driver/<tool>.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agent/codexclient"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-roost/platform/lib/codex"
	"github.com/takezoh/agent-roost/platform/procgroup"
)

const (
	serverDialTimeout    = 15 * time.Second
resumePhasePending  = "resume_pending"
	resumePhaseAttached = "attached"

	// stopGrace bounds how long Stop waits for the read loop + process Wait to
	// finish after cancelling. A little above procgroup's WaitDelay so the
	// SIGKILL'd group has time to be reaped before Stop returns.
	stopGrace = procgroup.DefaultWaitDelay + time.Second
)

// RuntimeHook is implemented by *runtime.Runtime and lets the stream backend
// enqueue events without a circular import.
type RuntimeHook interface {
	Enqueue(event state.Event)
}

// Backend is the codex app-server stream subsystem. One instance exists per
// roost Session. It manages the per-session app-server process, the
// WebSocket-over-UDS connection, and per-frame thread bindings.
type Backend struct {
	runtime       RuntimeHook
	dispatcher    agentlaunch.Dispatcher
	subsystemID   state.SubsystemID
	sessionID     state.SessionID
	project       string
	serverBin     string
	serverArgs    []string
	model         string
	sandboxed     bool
	autoApprove   bool
	readTimeout   time.Duration
	ctx           context.Context    // subsystem-scoped; child of the daemon ctx
	cancel        context.CancelFunc // cancels ctx → reaps read loop + process group
	done          chan struct{}      // closed when waitProcess returns (process reaped)
	tracker       *procgroup.Tracker // records pgids for crash-path reaping; may be nil
	spawnRes      agentlaunch.SpawnResult
	conn          *codexclient.Conn
	sockPath      string // UDS path dialed by daemon (host-side)
	containerSock string // UDS path inside container
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
	dispatcher agentlaunch.Dispatcher,
	subsystemID state.SubsystemID,
	sessionID state.SessionID,
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
		dispatcher:    dispatcher,
		subsystemID:   subsystemID,
		sessionID:     sessionID,
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
	// Derive a subsystem-scoped context from the daemon context. Cancelling it
	// (via Stop, or daemon shutdown cascading from the parent) tears down the
	// read loop and SIGKILLs the app-server process group.
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.done = make(chan struct{})

	res, serverErrBuf, err := b.spawnServer(ctx)
	if err != nil {
		b.cancel()
		return err
	}

	t, err := codexclient.DialUDS(b.sockPath, serverDialTimeout)
	if err != nil {
		b.cancel()
		_ = res.Wait()
		slog.Error("stream backend: app-server dial failed",
			"subsystem", b.subsystemID, "sock", b.sockPath,
			"stderr", strings.TrimSpace(serverErrBuf))
		return err
	}
	b.spawnRes = res
	b.conn = codexclient.NewConn(t, b.readTimeout)
	go func() {
		if err := b.conn.Run(b.ctx, b); err != nil {
			slog.Debug("stream backend: read loop closed", "subsystem", b.subsystemID, "err", err)
		}
	}()
	if err := codexclient.Initialize(b.conn); err != nil {
		_ = b.conn.Close()
		b.cancel()
		return err
	}
	b.trackProcessGroups()
	go b.waitProcess()
	return nil
}

// chooseSockPath returns the UDS socket path to pass to the app-server argv.
// Container mode uses the in-container path; host mode uses the host path.
func (b *Backend) chooseSockPath() string {
	if b.dispatcher != nil && b.dispatcher.IsContainer(b.project) {
		return b.containerSock
	}
	return b.sockPath
}

// spawnServer wraps the app-server using the dispatcher and spawns the process.
func (b *Backend) spawnServer(ctx context.Context) (agentlaunch.SpawnResult, string, error) {
	sockPath := b.chooseSockPath()
	argv := libcodex.AppServerListenArgs(b.serverBin, sockPath, b.serverArgs, b.sandboxed)

	plan := agentlaunch.LaunchPlan{
		Command:  strings.Join(argv, " "),
		Argv:     argv,
		Project:  b.project,
		StartDir: "",
	}

	var wrapped agentlaunch.WrappedLaunch
	var err error
	if b.dispatcher != nil {
		wrapped, err = b.dispatcher.Wrap(ctx, string(b.subsystemID), plan)
		if err != nil {
			return agentlaunch.SpawnResult{}, "", fmt.Errorf("stream backend: dispatch wrap: %w", err)
		}
	} else {
		wrapped = agentlaunch.WrappedLaunch{Argv: argv}
	}

	var errBuf strings.Builder
	res, err := agentlaunch.Spawn(b.ctx, wrapped, agentlaunch.SpawnOptions{
		InheritEnv: true,
		Stderr:     newPrefixWriter(&errBuf, 8192),
	})
	return res, errBuf.String(), err
}

// trackProcessGroups records the app-server pgid so a future boot's
// PruneOrphans reaps it if this daemon dies without a graceful Stop.
// No-op when tracker is nil.
func (b *Backend) trackProcessGroups() {
	if b.spawnRes.PID != 0 {
		b.tracker.Track(b.spawnRes.PID)
	}
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
	result.Plan.Command = strings.Join(libcodex.RemoteAttachArgs(b.bridgePort, string(b.sessionID), threadID, startDir), " ")
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

// Stop cancels the subsystem context (SIGKILLing the app-server process group
// via procgroup) and blocks until waitProcess has reaped it, so the call
// returns only once the spawned process is gone. A grace bound prevents a
// stuck Wait from blocking shutdown forever.
func (b *Backend) Stop(_ context.Context) {
	if b.cancel != nil {
		b.cancel()
	}
	if b.done == nil {
		return
	}
	select {
	case <-b.done:
	case <-time.After(stopGrace):
		slog.Warn("stream backend: Stop timed out waiting for reap", "subsystem", b.subsystemID)
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
	if err := codexclient.StartTurn(b.conn, "", startDir, stdin, codexclient.TurnOptions{}); err != nil {
		return "", err
	}
	return "", nil
}

func (b *Backend) waitProcess() {
	defer close(b.done)
	err := b.spawnRes.Wait()
	if b.spawnRes.PID != 0 {
		b.tracker.Untrack(b.spawnRes.PID)
	}
	if err != nil {
		slog.Error("stream backend exited", "subsystem", b.subsystemID, "err", err)
	} else {
		slog.Warn("stream backend exited", "subsystem", b.subsystemID)
	}
	_ = b.conn.Close()
	b.mu.Lock()
	frameIDs := make([]state.FrameID, 0, len(b.frames))
	for frameID := range b.frames {
		frameIDs = append(frameIDs, frameID)
	}
	b.mu.Unlock()
	var stopErr error
	if err != nil {
		stopErr = fmt.Errorf("stream backend stopped: %w", err)
	} else {
		stopErr = errors.New("stream backend stopped")
	}
	for _, frameID := range frameIDs {
		b.failFrame(frameID, stopErr)
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
