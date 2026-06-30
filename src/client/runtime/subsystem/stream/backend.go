// Package stream implements the stream subsystem backend that fronts
// structured app-servers (currently codex app-server) via WebSocket-over-UDS.
// This is the only location in runtime/ permitted to import driver/<tool>.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
	"github.com/takezoh/agent-reactor/platform/agent/codexclient"
	"github.com/takezoh/agent-reactor/platform/agentlaunch"
	libcodex "github.com/takezoh/agent-reactor/platform/lib/codex"
	"github.com/takezoh/agent-reactor/platform/pathmap"
	"github.com/takezoh/agent-reactor/platform/procgroup"
)

const (
	serverDialTimeout   = 15 * time.Second
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
// client Session. It manages the per-session app-server process, the
// WebSocket-over-UDS connection, and per-frame thread bindings.
type Backend struct {
	runtime     RuntimeHook
	dispatcher  agentlaunch.Dispatcher
	subsystemID state.SubsystemID
	sessionID   state.SessionID
	project     string
	serverBin   string
	serverArgs  []string
	model       string
	sandboxed   bool
	autoApprove bool
	readTimeout time.Duration
	ctx         context.Context    // subsystem-scoped; child of the daemon ctx
	cancel      context.CancelFunc // cancels ctx → reaps read loop + process group
	done        chan struct{}      // closed when waitProcess returns (process reaped)
	tracker     *procgroup.Tracker // records pgids for crash-path reaping; may be nil
	spawnRes    agentlaunch.SpawnResult
	conn        *codexclient.Conn
	listenSock  string // UDS path the app-server binds (container-absolute under a devcontainer)
	dialSock    string // host-side UDS path the daemon dials; resolved from listenSock + bind mounts in spawnServer
	mounts      pathmap.Mounts
	mu          sync.Mutex
	frames      map[state.FrameID]*frameBinding
	threads     map[string]state.FrameID
}

type frameBinding struct {
	frameID         state.FrameID
	startDir        string
	worktreePath    string // non-empty when a managed worktree was adopted or created
	threadID        string
	sessionID       string
	rolloutPath     string
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
	listenSock string,
	readTimeout time.Duration,
) *Backend {
	return &Backend{
		runtime:     rt,
		dispatcher:  dispatcher,
		subsystemID: subsystemID,
		sessionID:   sessionID,
		project:     project,
		serverBin:   serverBin,
		serverArgs:  serverArgs,
		model:       model,
		sandboxed:   sandboxed,
		autoApprove: autoApprove,
		readTimeout: readTimeout,
		listenSock:  listenSock,
		frames:      map[state.FrameID]*frameBinding{},
		threads:     map[string]state.FrameID{},
	}
}

// Kind implements subsystem.Subsystem.
func (b *Backend) Kind() state.LaunchSubsystem { return state.LaunchSubsystemStream }

// Start launches the app-server process, dials the WebSocket, and begins
// the read loop. On failure the caller must not call Start again — create
// a new Backend instead.
func (b *Backend) Start(ctx context.Context) error {
	// Derive a subsystem-scoped context from the daemon context. Cancelling it
	// (via Stop, or daemon shutdown cascading from the parent) tears down the
	// read loop and SIGKILLs the app-server process group.
	b.ctx, b.cancel = context.WithCancel(ctx)
	b.done = make(chan struct{})

	res, serverErr, err := b.spawnServer(ctx)
	if err != nil {
		b.cancel()
		return err
	}

	t, err := codexclient.DialUDS(b.dialSock, serverDialTimeout)
	if err != nil {
		b.cancel()
		// Reap the process first: cmd.Wait blocks until the stderr copier
		// goroutine has flushed everything into serverErr, so the captured
		// output reflects what the app-server printed before it died.
		_ = res.Wait()
		slog.Error("stream backend: app-server dial failed",
			"subsystem", b.subsystemID, "sock", b.dialSock,
			"stderr", strings.TrimSpace(serverErr.String()))
		return err
	}
	b.spawnRes = res
	// The app-server speaks JSON-RPC over the UDS; its stdout pipe is never
	// read. Drain it to discard so a chatty app-server can't block on a full
	// stdout pipe. Ends when the process exits and Wait closes the pipe.
	go func() { _, _ = io.Copy(io.Discard, res.Stdout) }()
	b.conn = codexclient.NewConn(t, b.readTimeout)
	go func() {
		if err := b.conn.Run(b.ctx, b); err != nil {
			slog.Debug("stream backend: read loop closed", "subsystem", b.subsystemID, "err", err)
		}
	}()
	if err := codexclient.Initialize(b.conn); err != nil {
		_ = b.conn.Close()
		b.cancel()
		// Reap the app-server we just SIGKILLed via cancel, mirroring the
		// dial-failure path; otherwise the process is orphaned (waitProcess
		// is not started on this path and the pgid was never tracked).
		_ = res.Wait()
		return err
	}
	b.trackProcessGroups()
	go b.waitProcess()
	return nil
}

// resolveDialSock returns the host-side path the daemon dials for an app-server
// that binds listenSock. In container mode listenSock is container-absolute and
// the launch's bind mounts expose it at a host path; in host mode there are no
// mounts, so the dial path equals the listen path.
func resolveDialSock(listenSock string, wrapped agentlaunch.WrappedLaunch) string {
	if host, ok := wrapped.HostPath(listenSock); ok {
		return host
	}
	return listenSock
}

func toPathmapMounts(ms []agentlaunch.Mount) pathmap.Mounts {
	if len(ms) == 0 {
		return nil
	}
	out := make(pathmap.Mounts, len(ms))
	for i, m := range ms {
		out[i] = pathmap.Mount{Host: m.Host, Container: m.Container}
	}
	return out
}

type normalizedResumeTarget struct {
	rpc state.ResumeTarget
}

func normalizeResumeTarget(target state.ResumeTarget, mounts pathmap.Mounts) (normalizedResumeTarget, error) {
	target.ThreadID = strings.TrimSpace(target.ThreadID)
	target.RolloutPath = strings.TrimSpace(target.RolloutPath)
	if target.ThreadID == "" && target.RolloutPath == "" {
		return normalizedResumeTarget{}, nil
	}
	if target.RolloutPath == "" {
		return normalizedResumeTarget{rpc: state.ResumeTarget{ThreadID: target.ThreadID}}, nil
	}
	cliPath, _, err := translateRolloutPath(target.RolloutPath, mounts)
	if err != nil {
		return normalizedResumeTarget{}, err
	}
	return normalizedResumeTarget{
		rpc: state.ResumeTarget{ThreadID: target.ThreadID, RolloutPath: cliPath},
	}, nil
}

func translateRolloutPath(path string, mounts pathmap.Mounts) (cliPath, hostPath string, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", errors.New("stream backend: codex resume requires rollout path")
	}
	if len(mounts) == 0 {
		return path, path, nil
	}
	if container, ok := mounts.ToContainer(path); ok {
		return container, path, nil
	}
	if host, ok := mounts.ToHost(path); ok {
		return path, host, nil
	}
	return "", "", fmt.Errorf("stream backend: rollout path %q is not reachable from current sandbox mounts", path)
}

// spawnServer wraps the app-server using the dispatcher, resolves the host dial
// path from the launch's bind mounts, and spawns the process.
// The returned *strings.Builder accumulates the app-server's stderr; read it
// only after the process is reaped (res.Wait) so the copier goroutine is done.
func (b *Backend) spawnServer(ctx context.Context) (agentlaunch.SpawnResult, *strings.Builder, error) {
	argv := libcodex.AppServerListenArgs(b.serverBin, b.listenSock, b.serverArgs, b.sandboxed)

	plan := agentlaunch.LaunchPlan{
		Command:  strings.Join(argv, " "),
		Argv:     argv,
		Project:  b.project,
		StartDir: "",
	}

	errBuf := &strings.Builder{}
	var wrapped agentlaunch.WrappedLaunch
	var err error
	if b.dispatcher != nil {
		wrapped, err = b.dispatcher.Wrap(ctx, string(b.subsystemID), plan)
		if err != nil {
			return agentlaunch.SpawnResult{}, errBuf, fmt.Errorf("stream backend: dispatch wrap: %w", err)
		}
	} else {
		wrapped = agentlaunch.WrappedLaunch{Argv: argv}
	}

	b.dialSock = resolveDialSock(b.listenSock, wrapped)
	b.mounts = toPathmapMounts(wrapped.Mounts)
	// Clear any stale socket before the app-server binds it (e.g. a crashed
	// predecessor left the file behind). Removing the host path also clears the
	// bind-mounted container path.
	_ = os.Remove(b.dialSock)

	res, err := agentlaunch.Spawn(b.ctx, wrapped, agentlaunch.SpawnOptions{
		InheritEnv: true,
		Stderr:     newPrefixWriter(errBuf, 8192),
	})
	return res, errBuf, err
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
	_, sessionID, resumeTarget, err := b.bindThread(req.FrameID, startDir, worktreePath, req.Plan.Stream, req.Stdin)
	if err != nil {
		return subsystem.BindResult{}, err
	}
	stableSessionID := strings.TrimSpace(req.Plan.Stream.ColdStartSessionID)
	if stableSessionID == "" {
		stableSessionID = sessionID
	}
	result.Plan.Command = strings.Join(libcodex.RemoteAttachArgs(b.listenSock, startDir), " ")
	result.Plan.Stdin = nil
	result.Plan.Stream.ResumeTarget = resumeTarget
	result.Plan.Stream.ColdStartSessionID = stableSessionID
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

// bindThread associates a new frame with a thread in the app-server and returns
// the bound thread ID — resumed for a warm start, or created synchronously via
// thread/start for a cold start.
func (b *Backend) bindThread(frameID state.FrameID, startDir, worktreePath string, opts state.StreamLaunchOptions, stdin []byte) (string, string, state.ResumeTarget, error) {
	b.mu.Lock()
	b.frames[frameID] = &frameBinding{
		frameID:      frameID,
		startDir:     startDir,
		worktreePath: worktreePath,
	}
	b.mu.Unlock()
	resumeTarget, err := normalizeResumeTarget(opts.ResumeTarget, b.mounts)
	if err != nil {
		return "", "", state.ResumeTarget{}, err
	}
	if resumeTarget.rpc.ThreadID != "" || resumeTarget.rpc.RolloutPath != "" {
		return b.bindResume(frameID, startDir, opts, resumeTarget)
	}
	return b.bindColdStart(frameID, startDir, stdin)
}

func (b *Backend) bindResume(frameID state.FrameID, startDir string, opts state.StreamLaunchOptions, resumeTarget normalizedResumeTarget) (string, string, state.ResumeTarget, error) {
	session, err := codexclient.ResumeThread(b.conn, codexclient.ResumeOptions{
		ThreadID:    resumeTarget.rpc.ThreadID,
		RolloutPath: resumeTarget.rpc.RolloutPath,
		Cwd:         startDir,
	})
	if err != nil {
		return "", "", state.ResumeTarget{}, err
	}
	threadID := strings.TrimSpace(session.ThreadID)
	if threadID == "" {
		return "", "", state.ResumeTarget{}, errors.New("stream backend: thread/resume returned an empty thread id")
	}
	hostPath := b.resolveHostPath(session.RolloutPath, strings.TrimSpace(opts.ResumeTarget.RolloutPath), threadID, "thread/resume")
	stableSessionID := firstNonEmpty(strings.TrimSpace(session.SessionID), strings.TrimSpace(opts.ColdStartSessionID))
	b.mu.Lock()
	if binding := b.frames[frameID]; binding != nil {
		binding.threadID = threadID
		binding.sessionID = stableSessionID
		binding.rolloutPath = hostPath
		binding.requestedID = opts.ResumeTarget.ThreadID
		binding.observedID = threadID
		binding.resumePhase = resumePhasePending
		b.threads[threadID] = frameID
	}
	b.mu.Unlock()
	return threadID, stableSessionID, state.ResumeTarget{ThreadID: threadID, RolloutPath: hostPath}, nil
}

func (b *Backend) bindColdStart(frameID state.FrameID, startDir string, stdin []byte) (string, string, state.ResumeTarget, error) {
	// Create the thread synchronously (thread/start request) so its id is known
	// here and the frame binds deterministically — no async thread.started
	// guessing, no cwd/active-frame heuristic. The spawned frame then resumes
	// this id (RemoteAttachArgs with a non-empty threadID).
	session, err := codexclient.StartThread(b.conn, startDir, nil, codexclient.ThreadOptions{})
	if err != nil {
		return "", "", state.ResumeTarget{}, err
	}
	threadID := strings.TrimSpace(session.ThreadID)
	if threadID == "" {
		return "", "", state.ResumeTarget{}, fmt.Errorf("stream backend: app-server returned an empty thread id on cold start")
	}
	hostPath := b.resolveHostPath(session.RolloutPath, "", threadID, "thread/start")
	sessionID := strings.TrimSpace(session.SessionID)
	b.mu.Lock()
	if binding := b.frames[frameID]; binding != nil {
		binding.threadID = threadID
		binding.sessionID = sessionID
		binding.rolloutPath = hostPath
		binding.requestedID = threadID
		binding.observedID = threadID
		binding.resumePhase = resumePhaseAttached
		b.threads[threadID] = frameID
	}
	b.mu.Unlock()
	// The thread/start response is authoritative for the id (a thread.started
	// notification may not follow), so surface readiness now; a later
	// thread.started re-confirms idempotently.
	b.emit(frameID, state.SubsystemSessionReady, b.payload(frameID))
	// Inject an initial prompt only when one was supplied (orchestrator-style);
	// interactive frames drive their own turns.
	if len(stdin) > 0 {
		if err := codexclient.StartTurn(b.conn, threadID, startDir, stdin, codexclient.TurnOptions{}); err != nil {
			return "", "", state.ResumeTarget{}, err
		}
	}
	return threadID, sessionID, state.ResumeTarget{ThreadID: threadID, RolloutPath: hostPath}, nil
}

// resolveHostPath translates a rollout path returned by the app-server into a
// host-side path, falling back to the requested host path on translation
// failure. method is used purely for diagnostic logging.
func (b *Backend) resolveHostPath(rolloutPath, fallback, threadID, method string) string {
	rolloutPath = strings.TrimSpace(rolloutPath)
	if rolloutPath == "" {
		return fallback
	}
	_, hostPath, err := translateRolloutPath(rolloutPath, b.mounts)
	if err != nil {
		slog.Debug("stream backend: ignoring unusable rollout path",
			"subsystem", b.subsystemID,
			"method", method,
			"thread", threadID,
			"rollout_path", rolloutPath,
			"err", err)
		return fallback
	}
	return hostPath
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
