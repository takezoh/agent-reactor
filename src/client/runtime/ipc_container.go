package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/takezoh/agent-roost/client/proto"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/pathmap"
)

const (
	payloadFieldCwd            = "cwd"
	payloadFieldTranscriptPath = "transcript_path"
	payloadFieldContainerCwd   = "container_cwd"
)

// containerEndpoint listens on the per-project Unix socket that is
// bind-mounted into the devcontainer at /opt/roost/run/roost.sock.
// It accepts hook-event and subsystem-event commands.
//
// Authentication is via a bearer token (ROOST_SOCKET_TOKEN) carried
// in each CmdHookEvent. A valid token resolves to the FrameID of the
// spawning frame, which becomes the event SenderID.
type containerEndpoint struct {
	listener  net.Listener
	tokens    *tokenStore
	enqueue   func(state.Event)
	getMounts func(state.FrameID) (pathmap.Mounts, bool)
}

func startContainerEndpoint(sockPath string, tokens *tokenStore, enqueue func(state.Event), getMounts func(state.FrameID) (pathmap.Mounts, bool)) (*containerEndpoint, error) {
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}
	// 0o666: any process in the container can connect; the bearer token is
	// the real authentication boundary.
	if err := os.Chmod(sockPath, 0o666); err != nil {
		_ = ln.Close()
		return nil, err
	}
	ep := &containerEndpoint{listener: ln, tokens: tokens, enqueue: enqueue, getMounts: getMounts}
	go ep.accept()
	slog.Info("runtime: container endpoint listening", "sock", sockPath)
	return ep, nil
}

func (ep *containerEndpoint) close() {
	_ = ep.listener.Close()
}

func (ep *containerEndpoint) accept() {
	for {
		conn, err := ep.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			slog.Error("runtime: container endpoint accept", "err", err)
			continue
		}
		go ep.serve(conn)
	}
}

const containerConnIdleTimeout = 30 * time.Second

func (ep *containerEndpoint) serve(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	w := bufio.NewWriter(conn)
	for {
		// Reset deadline before each round-trip so a slow or hung agent
		// cannot pin a goroutine. Both read and write share the same window.
		_ = conn.SetDeadline(time.Now().Add(containerConnIdleTimeout))
		var env proto.Envelope
		if err := dec.Decode(&env); err != nil {
			return
		}
		ep.handle(w, env)
	}
}

func (ep *containerEndpoint) handle(w *bufio.Writer, env proto.Envelope) {
	switch env.Cmd {
	case proto.CmdNameHookEvent:
		ep.handleHook(w, env)
	case proto.CmdNameSubsystem:
		ep.handleSubsystem(w, env)
	default:
		containerWriteError(w, env.ReqID, proto.ErrUnsupported, "unsupported command")
	}
}

func (ep *containerEndpoint) handleHook(w *bufio.Writer, env proto.Envelope) {
	var cmd proto.CmdHookEvent
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &cmd); err != nil {
			containerWriteError(w, env.ReqID, proto.ErrInvalidArgument, "bad payload")
			return
		}
	}

	frameID, ok := ep.tokens.Lookup(cmd.Token)
	if !ok {
		containerWriteError(w, env.ReqID, proto.ErrInvalidArgument, "invalid token")
		return
	}

	ts := cmd.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	payload := ep.translatePayloadPaths(frameID, cmd.Payload)

	// ConnID=0: reduceDriverHook skips IPC response routing (responses only
	// go to ConnID != 0). OK is sent here, before the event is processed —
	// success means "enqueued", not "state updated".
	ep.enqueue(state.EvDriverEvent{
		ConnID:    0,
		ReqID:     env.ReqID,
		Event:     cmd.Hook,
		Timestamp: ts,
		SenderID:  frameID,
		Payload:   payload,
	})
	containerWriteOK(w, env.ReqID)
}

func (ep *containerEndpoint) handleSubsystem(w *bufio.Writer, env proto.Envelope) {
	var cmd proto.CmdSubsystemEvent
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &cmd); err != nil {
			containerWriteError(w, env.ReqID, proto.ErrInvalidArgument, "bad payload")
			return
		}
	}

	frameID, ok := ep.tokens.Lookup(cmd.Token)
	if !ok {
		containerWriteError(w, env.ReqID, proto.ErrInvalidArgument, "invalid token")
		return
	}

	ts := cmd.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	decoded, err := ep.translateSubsystemPayload(frameID, cmd.Payload)
	if err != nil {
		containerWriteError(w, env.ReqID, proto.ErrInvalidArgument, "bad subsystem payload")
		return
	}

	ep.enqueue(state.EvSubsystem{
		ConnID:    0,
		ReqID:     env.ReqID,
		FrameID:   frameID,
		Source:    state.SubsystemKind(cmd.Source),
		Kind:      state.SubsystemEventKind(cmd.Kind),
		Timestamp: ts,
		Payload:   decoded,
	})
	containerWriteOK(w, env.ReqID)
}

// translatePayloadPaths rewrites known path fields in a hook payload from
// container-absolute to host-absolute. Fields that cannot be mapped are set
// to "" so the driver treats them as absent (graceful degrade).
// For the "cwd" field the original container value is also preserved as
// "container_cwd" so the driver can still use it for path-encoding that
// depends on the container-side working directory (e.g. transcript project dir).
// Fields without a registered mount (non-sandbox frames) are left unchanged.
func (ep *containerEndpoint) translatePayloadPaths(frameID state.FrameID, payload json.RawMessage) json.RawMessage {
	if ep.getMounts == nil || len(payload) == 0 {
		return payload
	}
	ms, ok := ep.getMounts(frameID)
	if !ok || len(ms) == 0 {
		return payload
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return payload
	}

	changed := translateCwdField(frameID, fields, ms)
	changed = translateTranscriptField(frameID, fields, ms) || changed

	if !changed {
		return payload
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return payload
	}
	return out
}

func (ep *containerEndpoint) translateSubsystemPayload(frameID state.FrameID, raw json.RawMessage) (state.SubsystemPayload, error) {
	var p state.SubsystemPayload
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, err
		}
	}
	if ep.getMounts != nil {
		if ms, ok := ep.getMounts(frameID); ok && len(ms) > 0 {
			translateSubsystemToolPaths(&p, ms)
			translateSubsystemDiffPaths(&p, ms)
		}
	}
	return p, nil
}

func translateSubsystemToolPaths(p *state.SubsystemPayload, ms pathmap.Mounts) {
	if p == nil {
		return
	}
	if p.Tool != nil {
		if host, ok := ms.ToHost(p.Tool.Path); ok {
			p.Tool.Path = host
		} else if p.Tool.Path != "" {
			p.Tool.Path = ""
		}
	}
	if host, ok := ms.ToHost(p.TranscriptPath); ok {
		p.TranscriptPath = host
	} else if p.TranscriptPath != "" {
		p.TranscriptPath = ""
	}
	if p.Approval != nil {
		if host, ok := ms.ToHost(p.Approval.Path); ok {
			p.Approval.Path = host
		} else if p.Approval.Path != "" {
			p.Approval.Path = ""
		}
	}
}

func translateSubsystemDiffPaths(p *state.SubsystemPayload, ms pathmap.Mounts) {
	if p == nil || p.Diff == nil || len(p.Diff.Paths) == 0 {
		return
	}
	paths := make([]string, 0, len(p.Diff.Paths))
	for _, path := range p.Diff.Paths {
		if host, ok := ms.ToHost(path); ok {
			paths = append(paths, host)
		} else {
			slog.Debug("ipc_container: diff path not covered by mount, dropping", "path", path)
		}
	}
	p.Diff.Paths = paths
}

func translateCwdField(frameID state.FrameID, fields map[string]json.RawMessage, ms pathmap.Mounts) bool {
	raw, exists := fields[payloadFieldCwd]
	if !exists {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return false
	}
	fields[payloadFieldContainerCwd] = raw
	if host, ok := ms.ToHost(s); ok {
		if enc, err := json.Marshal(host); err == nil {
			fields[payloadFieldCwd] = enc
		}
	} else {
		fields[payloadFieldCwd] = json.RawMessage(`""`)
		slog.Debug("ipc_container: cwd not covered by any mount; clearing",
			"frame", frameID, "container_path", s)
	}
	return true
}

func translateTranscriptField(frameID state.FrameID, fields map[string]json.RawMessage, ms pathmap.Mounts) bool {
	raw, exists := fields[payloadFieldTranscriptPath]
	if !exists {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return false
	}
	if host, ok := ms.ToHost(s); ok {
		if enc, err := json.Marshal(host); err == nil {
			fields[payloadFieldTranscriptPath] = enc
			return true
		}
	} else {
		fields[payloadFieldTranscriptPath] = json.RawMessage(`""`)
		slog.Debug("ipc_container: transcript_path not covered by any mount; clearing",
			"frame", frameID, "container_path", s)
		return true
	}
	return false
}

// startContainerEndpointIfNeeded starts the container endpoint for the
// given project at sockPath if one is not already running. At most one
// endpoint per project path. Thread-safe.
func (r *Runtime) startContainerEndpointIfNeeded(project, sockPath string) {
	// Claim the slot with a sentinel to prevent concurrent startups.
	sentinel := &containerEndpoint{}
	if _, loaded := r.containerEndpoints.LoadOrStore(project, sentinel); loaded {
		return
	}
	ep, err := startContainerEndpoint(sockPath, &r.containerTokens, r.Enqueue, r.MountsForFrame)
	if err != nil {
		slog.Error("runtime: container endpoint start failed", "project", project, "sock", sockPath, "err", err)
		r.containerEndpoints.Delete(project)
		return
	}
	r.containerEndpoints.Store(project, ep)
}

// shutdownContainerEndpoints closes all active container endpoint listeners.
// Called from shutdownIPC.
func (r *Runtime) shutdownContainerEndpoints() {
	r.containerEndpoints.Range(func(_, v any) bool {
		if ep, ok := v.(*containerEndpoint); ok && ep.listener != nil {
			ep.close()
		}
		return true
	})
}

func containerWriteOK(w *bufio.Writer, reqID string) {
	wire, err := proto.EncodeResponse(reqID, proto.RespOK{})
	if err != nil {
		return
	}
	_, _ = w.Write(wire)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}

func containerWriteError(w *bufio.Writer, reqID string, code proto.ErrCode, msg string) {
	wire, err := proto.EncodeError(reqID, code, msg, nil)
	if err != nil {
		return
	}
	_, _ = w.Write(wire)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}
