package proto

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	defaultRequestTimeout = 5 * time.Second
	createSessionTimeout  = 5 * time.Minute
)

func (c *Client) sendDefault(cmd Command) (Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	return c.Send(ctx, cmd)
}

func sendJSONEventTimeout[R Response](c *Client, eventName string, req any, timeout time.Duration) (R, error) {
	var payload json.RawMessage
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			var zero R
			return zero, err
		}
		payload = json.RawMessage(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resp, err := c.Send(ctx, CmdEvent{Event: eventName, Payload: payload})
	if err != nil {
		var zero R
		return zero, err
	}
	r, ok := resp.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("proto: unexpected response type for %s", eventName)
	}
	return r, nil
}

func sendJSONEvent[R Response](c *Client, eventName string, req any) (R, error) {
	return sendJSONEventTimeout[R](c, eventName, req, defaultRequestTimeout)
}

// PeerSend sends a peer message to the target frame.
func (c *Client) PeerSend(fromFrameID, toFrameID, text, replyTo string) error {
	_, err := c.sendDefault(CmdPeerSend{
		FromFrameID: fromFrameID,
		ToFrameID:   toFrameID,
		Text:        text,
		ReplyTo:     replyTo,
	})
	return err
}

// PeerList lists peers visible to the caller frame.
func (c *Client) PeerList(fromFrameID, scope string) ([]PeerPeerInfo, error) {
	resp, err := c.sendDefault(CmdPeerList{
		FromFrameID: fromFrameID,
		Scope:       scope,
	})
	if err != nil {
		return nil, err
	}
	r, ok := resp.(RespPeerList)
	if !ok {
		return nil, fmt.Errorf("proto: unexpected response for peer.list")
	}
	return r.Peers, nil
}

// PeerSetSummary updates the caller's peer summary.
func (c *Client) PeerSetSummary(fromFrameID, summary string) error {
	_, err := c.sendDefault(CmdPeerSetSummary{
		FromFrameID: fromFrameID,
		Summary:     summary,
	})
	return err
}

// PeerDrainInbox reads and clears the peer inbox for the given frame.
func (c *Client) PeerDrainInbox(frameID string) ([]PeerMessage, error) {
	resp, err := c.sendDefault(CmdPeerDrainInbox{FromFrameID: frameID})
	if err != nil {
		return nil, err
	}
	r, ok := resp.(RespPeerDrainInbox)
	if !ok {
		return nil, fmt.Errorf("proto: unexpected response for peer.drain_inbox")
	}
	return r.Messages, nil
}

// SendHookEvent sends a hook-event command to the container endpoint.
func (c *Client) SendHookEvent(token, hook string, timestamp time.Time, payload json.RawMessage) error {
	return c.SendWithTimeout(CmdHookEvent{
		Token:     token,
		Hook:      hook,
		Timestamp: timestamp,
		Payload:   payload,
	}, defaultRequestTimeout)
}

// SendEvent ships a generic event to the daemon.
func (c *Client) SendEvent(eventName string, timestamp time.Time, senderID string, payload json.RawMessage) error {
	return c.SendWithTimeout(CmdEvent{
		Event:     eventName,
		Timestamp: timestamp,
		SenderID:  senderID,
		Payload:   payload,
	}, defaultRequestTimeout)
}

func (c *Client) SendSubsystemEvent(token, frameID, source, kind string, timestamp time.Time, payload json.RawMessage) error {
	return c.SendWithTimeout(CmdSubsystemEvent{
		Token:     token,
		FrameID:   frameID,
		Source:    source,
		Kind:      kind,
		Timestamp: timestamp,
		Payload:   payload,
	}, defaultRequestTimeout)
}
