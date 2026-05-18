package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/takezoh/agent-roost/driver"
	"github.com/takezoh/agent-roost/sandbox"
	"github.com/takezoh/agent-roost/state"
	"github.com/takezoh/credproxy/container"
)

type codexBackend struct {
	r             *Runtime
	subsystemID   state.SubsystemID
	project       string
	serverBin     string
	serverArgs    []string
	model         string
	sandboxed     bool
	autoApprove   bool
	cmd           *exec.Cmd
	wsConn        *websocket.Conn // WebSocket-over-UDS connection to app-server
	sockPath      string          // UDS path the daemon dials (host-side view)
	containerSock string          // UDS path the in-container app-server listens on
	hostBridgeCmd *exec.Cmd       // sockbridge subprocess for host mode (nil in container mode)
	bridgePort    int             // loopback TCP port for TUI's ws:// URL
	readDone      chan error
	writeMu       sync.Mutex
	mu            sync.Mutex
	nextID        int64
	pending       map[int64]chan rpcMessage
	frames        map[state.FrameID]*codexFrameBinding
	threads       map[string]state.FrameID
	activeLookup  func() state.FrameID
}

type codexFrameBinding struct {
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

func (r *Runtime) prepareStreamLaunch(frameID state.FrameID, subsystemID state.SubsystemID, plan state.LaunchPlan) (state.LaunchPlan, error) {
	if plan.Subsystem != state.LaunchSubsystemStream {
		return plan, nil
	}
	cfg, err := parseCodexCommand(plan.Command)
	if err != nil {
		return plan, err
	}
	backend, err := r.ensureCodexBackend(subsystemID, plan.Project, cfg, plan.Stream)
	if err != nil {
		return plan, err
	}
	threadID, err := backend.bindFrame(frameID, plan.StartDir, plan.Stream, plan.Stdin)
	if err != nil {
		return plan, err
	}
	plan.Command = buildCodexRemoteCommand(backend.bridgePort, threadID, plan.StartDir)
	plan.Stdin = nil
	plan.Stream.ResumeThreadID = threadID
	return plan, nil
}

func (r *Runtime) ensureCodexBackend(subsystemID state.SubsystemID, project string, cfg codexCommandConfig, opts state.StreamLaunchOptions) (*codexBackend, error) {
	if existing, ok := r.codexBackends.Load(subsystemID); ok {
		return existing.(*codexBackend), nil
	}

	dataDir := r.cfg.DataDir
	if dataDir == "" {
		dataDir = os.TempDir()
	}
	runDir, err := EnsureProjectRunDir(filepath.Join(dataDir, "run"), project)
	if err != nil {
		return nil, fmt.Errorf("stream backend: run dir: %w", err)
	}
	// One app-server per project — the run dir (host) and ContainerRunDir
	// (container) are both project-private (separate dirs / separate bind
	// mounts), so a fixed filename is sufficient and avoids encoding the
	// project a second time.
	sockPath := filepath.Join(runDir, driver.CodexAppServerSockName)
	containerSock := sockPath
	if launcher(r.cfg).IsContainer(project) {
		containerSock = ContainerRunDir + "/" + driver.CodexAppServerSockName
	}

	backend := &codexBackend{
		r:             r,
		subsystemID:   subsystemID,
		project:       project,
		serverBin:     cfg.serverBin,
		serverArgs:    cfg.serverArgs,
		model:         cfg.model,
		sandboxed:     opts.SandboxPolicy == state.StreamSandboxPolicyExternal,
		autoApprove:   opts.ApprovalPolicy == state.StreamApprovalPolicyAutoApprove,
		sockPath:      sockPath,
		containerSock: containerSock,
		bridgePort:    driver.CodexAppServerLoopbackPort,
		pending:       map[int64]chan rpcMessage{},
		frames:        map[state.FrameID]*codexFrameBinding{},
		threads:       map[string]state.FrameID{},
		activeLookup: func() state.FrameID {
			return r.activeFrameID
		},
	}
	actual, loaded := r.codexBackends.LoadOrStore(subsystemID, backend)
	if loaded {
		return actual.(*codexBackend), nil
	}
	if err := backend.start(); err != nil {
		r.codexBackends.Delete(subsystemID)
		return nil, err
	}
	return backend, nil
}

func (b *codexBackend) start() error {
	_ = os.Remove(b.sockPath)
	cmd, err := b.buildServerCommand()
	if err != nil {
		return err
	}
	// Capture app-server stderr to an internal buffer so a startup failure
	// (which would otherwise be invisible) can be surfaced via slog when the
	// dial times out. Not written to host stderr (TUI silence rule).
	var serverErrBuf strings.Builder
	cmd.Stderr = newPrefixWriter(&serverErrBuf, 8192)
	if err := cmd.Start(); err != nil {
		return err
	}
	wsConn, err := dialWebSocketUDS(b.sockPath, 15*time.Second)
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
	b.readDone = make(chan error, 1)
	go func() {
		b.readDone <- b.runReadLoop()
	}()
	if err := b.initialize(); err != nil {
		_ = wsConn.Close(websocket.StatusInternalError, "initialize failed")
		_ = cmd.Process.Kill()
		return err
	}
	// Container mode: sockbridge is part of the devcontainer postCreate
	// (registered via codexContainerBridgeSpec → buildPostCreate). Host
	// mode: spawn sockbridge here next to the app-server.
	if !launcher(b.r.cfg).IsContainer(b.project) {
		if err := b.startHostBridge(); err != nil {
			_ = wsConn.Close(websocket.StatusInternalError, "host bridge failed")
			_ = cmd.Process.Kill()
			return fmt.Errorf("stream backend: host bridge: %w", err)
		}
	}
	go b.waitProcess()
	return nil
}

// startHostBridge launches the sockbridge subprocess on the host so the
// codex TUI (which only accepts ws:// for --remote) can attach to the
// host-side UDS app-server via TCP loopback. In container mode this role
// is filled by a BridgeSpec started during devcontainer postCreate.
func (b *codexBackend) startHostBridge() error {
	bin, err := findHelperBinary("sockbridge")
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

func (b *codexBackend) buildServerCommand() (*exec.Cmd, error) {
	args := buildCodexServerArgs(b.serverArgs, b.sandboxed, b.sockPath)
	if !launcher(b.r.cfg).IsContainer(b.project) {
		return exec.Command(b.serverBin, args...), nil
	}
	dl := devcontainerLauncherFor(launcher(b.r.cfg))
	if dl == nil {
		return nil, fmt.Errorf("runtime: unsupported container launcher for stream backend")
	}
	ctx, cancel := context.WithTimeout(context.Background(), containerEnsureTimeout)
	defer cancel()
	inst, err := dl.mgr.EnsureInstance(ctx, b.project, "", sandbox.StartOptions{})
	if err != nil {
		return nil, err
	}
	cs := inst.Internal
	containerID := cs.ContainerID()
	user := cs.EffectiveUser()
	workDir := cs.WorkspaceTarget()
	preExec := cs.PreExec()

	// Inside the container the socket path is the container-internal view of
	// the bind-mounted run directory.
	containerArgs := buildCodexServerArgs(b.serverArgs, b.sandboxed, b.containerSock)
	execArgs := []string{"exec", "-i"}
	if user != "" {
		execArgs = append(execArgs, "-u", user)
	}
	if workDir != "" {
		execArgs = append(execArgs, "-w", workDir)
	}
	execArgs = append(execArgs, containerID)
	// Match the pane spawn envelope (BuildLaunchCommand): if preExecCommand
	// is set, wrap with `bash -lc 'preExec; exec <cmd>'` so the app-server
	// sees the same shell init (tool-version managers, env loaders) the TUI
	// pane sees. Without this, mise/asdf-style shims fail before codex even
	// starts.
	if preExec != "" {
		serverCmdline := shellJoinArgv(append([]string{b.serverBin}, containerArgs...))
		execArgs = append(execArgs, "bash", "-lc", preExec+"; exec "+serverCmdline)
	} else {
		execArgs = append(execArgs, b.serverBin)
		execArgs = append(execArgs, containerArgs...)
	}
	return exec.Command("docker", execArgs...), nil
}

// shellJoinArgv single-quote-escapes each argv element and joins them with
// spaces so the result can be safely re-parsed by `bash -c`.
func shellJoinArgv(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = shellQuote(a)
	}
	return strings.Join(out, " ")
}

func (b *codexBackend) bindFrame(frameID state.FrameID, startDir string, opts state.StreamLaunchOptions, stdin []byte) (string, error) {
	b.mu.Lock()
	if binding, ok := b.frames[frameID]; ok && binding.threadID != "" {
		threadID := binding.threadID
		b.mu.Unlock()
		return threadID, nil
	}
	b.frames[frameID] = &codexFrameBinding{frameID: frameID, startDir: startDir}
	b.mu.Unlock()

	threadID := strings.TrimSpace(opts.ResumeThreadID)
	if threadID == "" {
		// Cold start: don't pre-create the thread. The pane TUI will
		// initiate it (`codex --remote ws://...`); handleThreadStarted will
		// associate it with this frame via resolveFrameForStartedThread.
		// Pre-creating with thread/start produces a thread ID without a
		// rollout file on disk, which makes `codex resume <id>` fail.
		return "", nil
	}
	statusRaw, err := b.resumeThread(threadID, startDir)
	if err != nil {
		b.failFrame(frameID, err)
		return "", err
	}

	if strings.TrimSpace(string(stdin)) != "" {
		if err := b.startTurn(threadID, startDir, stdin); err != nil {
			b.failFrame(frameID, err)
			return "", err
		}
	}

	b.mu.Lock()
	binding := b.frames[frameID]
	binding.threadID = threadID
	binding.requestedID = threadID
	binding.observedID = threadID
	binding.resumePhase = "attached"
	b.threads[threadID] = frameID
	prevStatus := binding.threadStatus
	prevWaiting := binding.waitApproval
	b.mu.Unlock()

	b.emit(frameID, state.SubsystemSessionReady, b.payload(frameID))
	events, nextStatus, nextWaiting := threadStatusEvents(statusRaw, threadID, prevStatus, prevWaiting)
	b.mu.Lock()
	binding = b.frames[frameID]
	if binding != nil {
		binding.threadStatus = nextStatus
		binding.waitApproval = nextWaiting
	}
	b.mu.Unlock()
	for _, ev := range events {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
	return threadID, nil
}

func (b *codexBackend) resumeThread(threadID, startDir string) (json.RawMessage, error) {
	params := map[string]any{"threadId": threadID}
	if b.model != "" {
		params["model"] = b.model
	}
	if startDir != "" {
		params["cwd"] = startDir
	}
	msg, err := b.request("thread/resume", params)
	if err != nil {
		return nil, err
	}
	return msg.Result, nil
}

func (b *codexBackend) startTurn(threadID, startDir string, stdin []byte) error {
	text := strings.TrimSpace(string(stdin))
	if text == "" {
		return nil
	}
	params := map[string]any{
		"threadId": threadID,
		"input": []map[string]any{{
			"type": "text",
			"text": text,
		}},
	}
	if startDir != "" {
		params["cwd"] = startDir
	}
	if b.model != "" {
		params["model"] = b.model
	}
	if b.sandboxed {
		params["sandboxPolicy"] = map[string]any{"type": "dangerFullAccess"}
	}
	if b.autoApprove {
		params["approvalPolicy"] = "never"
	}
	_, err := b.request("turn/start", params)
	return err
}

func (b *codexBackend) initialize() error {
	if _, err := b.request("initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "roost", "version": "0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return err
	}
	return b.notify("initialized", map[string]any{})
}

func (b *codexBackend) waitProcess() {
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

func (b *codexBackend) runReadLoop() error {
	ctx := context.Background()
	for {
		_, data, err := b.wsConn.Read(ctx)
		if err != nil {
			return err
		}
		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.ID != nil && msg.Method == "" {
			b.resolvePending(*msg.ID, msg)
			continue
		}
		if msg.Method == "" {
			continue
		}
		if msg.ID != nil {
			b.handleRequest(msg)
			continue
		}
		b.handleNotification(msg)
	}
}

func (b *codexBackend) handleNotification(msg rpcMessage) {
	switch msg.Method {
	case "thread/started":
		b.handleThreadStarted(msg.Params)
	case "turn/started":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemTurnStarted, func(p *state.SubsystemPayload) {
			p.TurnID = extractTurnID(msg.Params)
		})
	case "turn/completed":
		b.handleTurnCompleted(msg.Params)
	case "turn/plan/updated":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemPlanUpdated, func(p *state.SubsystemPayload) {
			p.Plan = &state.SubsystemPlan{Summary: summarizePlan(msg.Params)}
		})
	case "turn/diff/updated":
		b.emitToThread(extractThreadID(msg.Params), state.SubsystemDiffUpdated, func(p *state.SubsystemPayload) {
			p.Diff = &state.SubsystemDiff{Summary: summarizeDiff(msg.Params), Paths: diffPaths(msg.Params)}
		})
	case "item/started":
		b.emitItemLifecycle("item/started", msg.Params)
	case "item/completed":
		b.emitItemLifecycle("item/completed", msg.Params)
	case "thread/status/changed":
		b.handleThreadStatusChanged(msg.Params)
	case "item/agentMessage/delta":
		b.handleAgentMessageDelta(msg.Params)
	case "error":
		slog.Error("stream backend: app-server error", "subsystem", b.subsystemID, "params", string(msg.Params))
	case "warning", "guardianWarning", "deprecationNotice":
		slog.Warn("stream backend: app-server notice", "method", msg.Method, "subsystem", b.subsystemID, "params", string(msg.Params))
	}
}

func (b *codexBackend) handleThreadStarted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.resolveFrameForStartedThread(threadID, extractThreadCWD(raw))
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.threadID = threadID
		binding.requestedID = threadID
		binding.observedID = threadID
		binding.resumePhase = "attached"
		b.threads[threadID] = frameID
	}
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemSessionReady, b.payload(frameID))
}

func (b *codexBackend) handleTurnCompleted(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	last := strings.TrimSpace(extractText(raw))
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.activeTurnID = ""
		if last != "" {
			binding.lastAssistant = last
			appendHistory(&binding.history, "assistant", last)
		}
	}
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemTurnCompleted, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *codexBackend) handleAgentMessageDelta(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	text := extractText(raw)
	if text == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding != nil {
		binding.lastAssistant += text
	}
	last := binding.lastAssistant
	history := append([]state.SubsystemTurn(nil), binding.history...)
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemMessageUpdated, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
		p.LastAssistantMessage = last
		p.Message = &state.SubsystemMessage{RecentTurns: history}
	}))
}

func (b *codexBackend) resolveFrameForStartedThread(threadID, cwd string) state.FrameID {
	if threadID == "" {
		return ""
	}
	if frameID := b.frameForThread(threadID); frameID != "" {
		return frameID
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var candidates []state.FrameID
	for frameID, binding := range b.frames {
		if binding.threadID != "" {
			continue
		}
		if binding.startDir == cwd {
			candidates = append(candidates, frameID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	active := b.activeLookup()
	if active != "" {
		if _, ok := b.frames[active]; ok {
			return active
		}
	}
	return ""
}

func (b *codexBackend) handleThreadStatusChanged(raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	prevStatus := binding.threadStatus
	prevWaiting := binding.waitApproval
	b.mu.Unlock()
	events, nextStatus, nextWaiting := threadStatusEvents(raw, threadID, prevStatus, prevWaiting)
	b.mu.Lock()
	binding = b.frames[frameID]
	if binding != nil {
		binding.threadStatus = nextStatus
		binding.waitApproval = nextWaiting
	}
	b.mu.Unlock()
	for _, ev := range events {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *codexBackend) emitItemLifecycle(method string, raw json.RawMessage) {
	threadID := extractThreadID(raw)
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	for _, ev := range itemLifecycleEvents(method, raw, threadID) {
		ev.payload = b.withTracking(frameID, ev.payload)
		b.emit(frameID, ev.kind, ev.payload)
	}
}

func (b *codexBackend) handleRequest(msg rpcMessage) {
	switch msg.Method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		threadID := extractThreadID(msg.Params)
		frameID := b.frameForThread(threadID)
		if frameID == "" {
			return
		}
		approval := approvalFromParams(msg.Method, msg.Params, b.autoApprove)
		b.emit(frameID, state.SubsystemApprovalRequested, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
		result := "accept"
		if b.autoApprove {
			result = "acceptForSession"
		}
		_ = b.reply(*msg.ID, result)
		approval.Resolved = true
		b.emit(frameID, state.SubsystemApprovalResolved, b.payloadWith(frameID, func(p *state.SubsystemPayload) {
			p.Approval = &approval
		}))
	default:
		slog.Warn("stream backend: rejecting unhandled server request",
			"method", msg.Method, "subsystem", b.subsystemID)
		if msg.ID != nil {
			_ = b.replyError(*msg.ID, "method not supported by roost")
		}
	}
}

func (b *codexBackend) emitToThread(threadID string, kind state.SubsystemEventKind, mutate func(*state.SubsystemPayload)) {
	frameID := b.frameForThread(threadID)
	if frameID == "" {
		return
	}
	b.emit(frameID, kind, b.payloadWith(frameID, mutate))
}

func (b *codexBackend) frameForThread(threadID string) state.FrameID {
	if threadID == "" {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.threads[threadID]
}

func (b *codexBackend) payload(frameID state.FrameID) state.SubsystemPayload {
	return b.payloadWith(frameID, nil)
}

func (b *codexBackend) payloadWith(frameID state.FrameID, mutate func(*state.SubsystemPayload)) state.SubsystemPayload {
	b.mu.Lock()
	binding := b.frames[frameID]
	payload := state.SubsystemPayload{}
	if binding != nil {
		payload = state.SubsystemPayload{
			SessionID:         binding.threadID,
			TargetID:          binding.threadID,
			RequestedTargetID: binding.requestedID,
			ObservedTargetID:  binding.observedID,
			ResumePhase:       binding.resumePhase,
		}
	}
	b.mu.Unlock()
	if mutate != nil {
		mutate(&payload)
	}
	return payload
}

func (b *codexBackend) withTracking(frameID state.FrameID, payload state.SubsystemPayload) state.SubsystemPayload {
	base := b.payload(frameID)
	if payload.SessionID == "" {
		payload.SessionID = base.SessionID
	}
	if payload.TargetID == "" {
		payload.TargetID = base.TargetID
	}
	payload.RequestedTargetID = base.RequestedTargetID
	payload.ObservedTargetID = base.ObservedTargetID
	payload.ResumePhase = base.ResumePhase
	return payload
}

func (b *codexBackend) failFrame(frameID state.FrameID, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	b.mu.Lock()
	binding := b.frames[frameID]
	if binding == nil || binding.failureReported {
		b.mu.Unlock()
		return
	}
	binding.failureReported = true
	b.mu.Unlock()
	b.emit(frameID, state.SubsystemFailed, state.SubsystemPayload{Error: msg})
}

func (b *codexBackend) emit(frameID state.FrameID, kind state.SubsystemEventKind, payload state.SubsystemPayload) {
	b.r.Enqueue(state.EvSubsystem{
		ConnID:    0,
		FrameID:   frameID,
		Source:    state.SubsystemStream,
		Kind:      kind,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}

func (b *codexBackend) request(method string, params map[string]any) (rpcMessage, error) {
	id := atomic.AddInt64(&b.nextID, 1)
	ch := make(chan rpcMessage, 1)
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	if err := b.writeRPC(rpcMessage{ID: &id, Method: method, Params: mustJSON(params)}); err != nil {
		return rpcMessage{}, err
	}
	select {
	case msg := <-ch:
		if len(msg.Error) > 0 && string(msg.Error) != "null" {
			return msg, fmt.Errorf("stream backend: %s error: %s", method, msg.Error)
		}
		return msg, nil
	case <-time.After(15 * time.Second):
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return rpcMessage{}, fmt.Errorf("stream backend: timeout waiting for %s", method)
	}
}

func (b *codexBackend) notify(method string, params map[string]any) error {
	return b.writeRPC(rpcMessage{Method: method, Params: mustJSON(params)})
}

func (b *codexBackend) reply(id int64, result any) error {
	return b.writeRPC(rpcMessage{ID: &id, Result: mustJSON(result)})
}

func (b *codexBackend) replyError(id int64, errMsg string) error {
	return b.writeRPC(rpcMessage{ID: &id, Error: mustJSON(map[string]any{"message": errMsg})})
}

func (b *codexBackend) writeRPC(msg rpcMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.wsConn.Write(context.Background(), websocket.MessageText, data)
}

func (b *codexBackend) resolvePending(id int64, msg rpcMessage) {
	b.mu.Lock()
	ch := b.pending[id]
	delete(b.pending, id)
	b.mu.Unlock()
	if ch != nil {
		ch <- msg
	}
}

// dialWebSocketUDS opens a WebSocket connection to codex app-server over a
// unix domain socket. The codex app-server's `--listen unix://PATH` transport
// is WebSocket-over-UDS (HTTP Upgrade handshake required) — raw JSON-RPC
// would hang forever waiting for an Upgrade. Retries the underlying unix
// dial every 50 ms until the socket file appears.
func dialWebSocketUDS(sockPath string, timeout time.Duration) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				for {
					c, err := net.Dial("unix", sockPath)
					if err == nil {
						return c, nil
					}
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(50 * time.Millisecond):
					}
				}
			},
		},
	}
	conn, _, err := websocket.Dial(ctx, "ws://localhost/", &websocket.DialOptions{
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("stream backend: websocket upgrade %s: %w", sockPath, err)
	}
	// codex frames can exceed default read limit (32KB). Disable cap.
	conn.SetReadLimit(-1)
	return conn, nil
}

type codexCommandConfig struct {
	serverBin  string
	serverArgs []string
	model      string
}

func parseCodexCommand(command string) (codexCommandConfig, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 || fields[0] != driver.CodexDriverName {
		return codexCommandConfig{}, fmt.Errorf("stream backend: unsupported command %q", command)
	}
	cfg := codexCommandConfig{serverBin: driver.CodexDriverName}
	for i := 1; i < len(fields); i++ {
		arg := fields[i]
		switch arg {
		case "resume":
			// Skip the thread ID arg; the resume target comes from plan.Stream.ResumeThreadID.
			i++
		case "-m", "--model":
			if i+1 < len(fields) {
				cfg.model = fields[i+1]
				i++
			}
		case "-c", "--config", "--enable", "--disable":
			if i+1 < len(fields) {
				cfg.serverArgs = append(cfg.serverArgs, arg, fields[i+1])
				i++
			}
		}
	}
	return cfg, nil
}

func buildCodexServerArgs(extra []string, sandboxExternal bool, sockPath string) []string {
	args := []string{"app-server", "--listen", "unix://" + sockPath}
	args = append(args, extra...)
	if sandboxExternal {
		args = append(args, "-c", `sandbox_mode="danger-full-access"`)
	}
	return args
}

// buildCodexRemoteCommand assembles the pane command that attaches the codex
// TUI to the shared app-server via the sockbridge listener. The bridge sits on
// a fixed loopback port (started by postCreate in container mode, by
// startHostBridge in host mode).
//
// Cold start (threadID == ""): `codex --remote ...` so the TUI creates the
// thread; pre-creating with thread/start produces a thread with no on-disk
// rollout file, causing `codex resume <id>` to fail. Warm start uses
// `codex resume <id> --remote ...`.
//
// --dangerously-bypass-approvals-and-sandbox: container is the sandbox.
// -C <startDir>: codex TUI process cwd does not become the agent working root;
// without -C the agent uses the shared app-server cwd (project root).
func buildCodexRemoteCommand(bridgePort int, threadID, startDir string) string {
	remote := fmt.Sprintf("ws://127.0.0.1:%d", bridgePort)
	args := []string{driver.CodexDriverName}
	if threadID != "" {
		args = append(args, "resume", threadID)
	}
	args = append(args, "--remote", remote, "--dangerously-bypass-approvals-and-sandbox")
	if startDir != "" {
		args = append(args, "-C", startDir)
	}
	return strings.Join(args, " ")
}

// prefixWriter is an io.Writer that captures up to max bytes into dst,
// discarding the rest. Used to capture child stderr without unbounded growth.
type prefixWriter struct {
	dst *strings.Builder
	max int
}

func newPrefixWriter(dst *strings.Builder, max int) *prefixWriter {
	return &prefixWriter{dst: dst, max: max}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	if p.dst.Len() < p.max {
		room := p.max - p.dst.Len()
		if room > len(b) {
			room = len(b)
		}
		p.dst.Write(b[:room])
	}
	return len(b), nil
}

// codexContainerBridgeSpec returns the BridgeSpec that runs sockbridge inside
// the project devcontainer next to the codex app-server UDS. It is appended
// to the credproxy provider bridges so postCreate starts it as part of the
// standard container bootstrap.
func codexContainerBridgeSpec() container.BridgeSpec {
	return container.BridgeSpec{
		ListenAddr:          fmt.Sprintf("127.0.0.1:%d", driver.CodexAppServerLoopbackPort),
		ContainerSocketPath: ContainerRunDir + "/" + driver.CodexAppServerSockName,
	}
}

func appendHistory(history *[]state.SubsystemTurn, role, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*history = append(*history, state.SubsystemTurn{Role: role, Text: text})
	if len(*history) > 6 {
		*history = (*history)[len(*history)-6:]
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

type rpcMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}
