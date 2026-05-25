package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/takezoh/agent-roost/client/proto"
)

const (
	defaultRequestTimeout = 5 * time.Second
	createSessionTimeout  = 5 * time.Minute
)

func newDefaultCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultRequestTimeout)
}

func sendJSONEventTimeout[R proto.Response](c *proto.Client, eventName string, req any, timeout time.Duration) (R, error) {
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
	resp, err := c.Send(ctx, proto.CmdEvent{Event: eventName, Payload: payload})
	if err != nil {
		var zero R
		return zero, err
	}
	r, ok := resp.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("proto/sessions: unexpected response type for %s", eventName)
	}
	return r, nil
}

func sendJSONEvent[R proto.Response](c *proto.Client, eventName string, req any) (R, error) {
	return sendJSONEventTimeout[R](c, eventName, req, defaultRequestTimeout)
}
