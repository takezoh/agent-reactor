// Package sessions extends proto.Client with session/frame/occupant management
// methods that reference state-package types. Bridge code should use
// proto.Client directly; TUI and daemon-side code uses sessions.Client.
package sessions

import (
	"path/filepath"

	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/client/state"
)

// Client wraps proto.Client with session management methods.
// All proto.Client methods (Send, Close, Events, SendEvent, etc.)
// are available through embedding.
type Client struct {
	*proto.Client
}

// Wrap promotes a *proto.Client to a sessions.Client.
func Wrap(c *proto.Client) *Client { return &Client{c} }

// Subscribe registers this client to receive broadcast events.
func (c *Client) Subscribe() error {
	ctx, cancel := newDefaultCtx()
	defer cancel()
	_, err := c.Send(ctx, proto.CmdSubscribe{})
	return err
}

// CreateSession asks the daemon to spawn a new session.
// Uses a long timeout because devcontainer cold starts can take several minutes.
func (c *Client) CreateSession(project, command string, sandbox state.SandboxOverride, options state.LaunchOptions) (sessionID string, err error) {
	r, err := sendJSONEventTimeout[proto.RespCreateSession](c.Client, state.EventCreateSession, state.CreateSessionParams{
		Project: canonicalProjectPath(project),
		Command: command,
		Sandbox: sandbox,
		Options: options,
	}, createSessionTimeout)
	if err != nil {
		return "", err
	}
	return r.SessionID, nil
}

// StopSession kills a session by id.
func (c *Client) StopSession(id string) error {
	_, err := sendJSONEvent[proto.RespOK](c.Client, state.EventStopSession, map[string]string{"session_id": id})
	return err
}

// ListSessions returns the current session table, active session id, and the
// list of enabled runtime feature flags.
func (c *Client) ListSessions() ([]proto.SessionInfo, string, []string, error) {
	r, err := sendJSONEvent[proto.RespSessions](c.Client, state.EventListSessions, nil)
	if err != nil {
		return nil, "", nil, err
	}
	return r.Sessions, r.ActiveSessionID, r.Features, nil
}

// PreviewSession swaps a session into pane 0.0 without focusing it.
func (c *Client) PreviewSession(sessionID string) (string, error) {
	r, err := sendJSONEvent[proto.RespActiveSession](c.Client, state.EventPreviewSession, map[string]string{"session_id": sessionID})
	if err != nil {
		return "", err
	}
	return r.ActiveSessionID, nil
}

// SwitchSession swaps a session into pane 0.0 and focuses it.
func (c *Client) SwitchSession(sessionID string) (string, error) {
	r, err := sendJSONEvent[proto.RespActiveSession](c.Client, state.EventSwitchSession, map[string]string{"session_id": sessionID})
	if err != nil {
		return "", err
	}
	return r.ActiveSessionID, nil
}

// PreviewProject deactivates the current session and broadcasts project-selected.
func (c *Client) PreviewProject(project string) error {
	_, err := sendJSONEvent[proto.RespOK](c.Client, state.EventPreviewProject, map[string]string{"project": project})
	return err
}

// Shutdown tells the daemon to terminate.
func (c *Client) Shutdown() error {
	_, err := sendJSONEvent[proto.RespOK](c.Client, state.EventShutdown, nil)
	return err
}

// ActivateFrame switches the active frame for a session.
func (c *Client) ActivateFrame(sessionID, frameID string) error {
	_, err := sendJSONEvent[proto.RespOK](c.Client, state.EventActivateFrame, state.ActivateFrameParams{
		SessionID: sessionID,
		FrameID:   frameID,
	})
	return err
}

// PushDriver asks the daemon to push a new driver frame onto the given session.
func (c *Client) PushDriver(sessionID, command string, input []byte) error {
	_, err := sendJSONEvent[proto.RespCreateSession](c.Client, state.EventPushDriver, state.PushDriverParams{
		SessionID: sessionID,
		Command:   command,
		Input:     input,
	})
	return err
}

// ForkSession asks the daemon to fork the given session's conversation into
// a new independent session. The fork command is resolved daemon-side by the
// root frame's driver; returns the new session ID on success.
func (c *Client) ForkSession(sessionID string) (string, error) {
	r, err := sendJSONEvent[proto.RespCreateSession](c.Client, state.EventForkSession, state.ForkSessionParams{
		SessionID: sessionID,
	})
	if err != nil {
		return "", err
	}
	return r.SessionID, nil
}

// canonicalProjectPath returns the canonical absolute path for a project directory.
func canonicalProjectPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}
	return resolved
}
