package peers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/takezoh/agent-roost/client/proto"
)

// runMCPServer starts the roost-peers MCP stdio server.
func runMCPServer() error {
	server := mcp.NewServer(&mcp.Implementation{Name: "roost-peers", Version: "1.0"}, nil)
	dial := defaultDialer()

	registerListPeers(server, dial)
	registerPeerSend(server, dial)
	registerSetSummary(server, dial)
	registerCheckMessages(server, dial)

	return server.Run(context.Background(), &mcp.StdioTransport{})
}

// listPeersArgs are the args for the list_peers tool.
type listPeersArgs struct {
	Scope string `json:"scope" jsonschema:"scope: workspace, project, or all,default=workspace"`
}

func registerListPeers(server *mcp.Server, dial dialer) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_peers",
		Description: "List peer agent frames visible from this frame",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args listPeersArgs) (*mcp.CallToolResult, any, error) {
		scope := args.Scope
		if scope == "" {
			scope = "workspace"
		}
		res, err := handleListPeers(dial, callerFrameID(), scope)
		return res, nil, err
	})
}

// peerSendArgs are the args for the peer_send tool.
type peerSendArgs struct {
	To      string `json:"to" jsonschema:"target frame_id"`
	Text    string `json:"text" jsonschema:"message text"`
	ReplyTo string `json:"reply_to,omitempty" jsonschema:"optional message id to reply to"`
}

func registerPeerSend(server *mcp.Server, dial dialer) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "peer_send",
		Description: "Send a message to a peer agent frame",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args peerSendArgs) (*mcp.CallToolResult, any, error) {
		res, err := handlePeerSend(dial, callerFrameID(), args.To, args.Text, args.ReplyTo)
		return res, nil, err
	})
}

// setSummaryArgs are the args for the set_summary tool.
type setSummaryArgs struct {
	Summary string `json:"summary" jsonschema:"brief description of what this agent is currently doing"`
}

func registerSetSummary(server *mcp.Server, dial dialer) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_summary",
		Description: "Update this frame's peer summary visible to other agents",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args setSummaryArgs) (*mcp.CallToolResult, any, error) {
		res, err := handleSetSummary(dial, callerFrameID(), args.Summary)
		return res, nil, err
	})
}

// checkMessagesArgs are the args for the check_messages tool (none).
type checkMessagesArgs struct{}

func registerCheckMessages(server *mcp.Server, dial dialer) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_messages",
		Description: "Drain and return inbox messages for the current frame (polling fallback)",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ checkMessagesArgs) (*mcp.CallToolResult, any, error) {
		res, err := handleCheckMessages(dial, callerFrameID())
		return res, nil, err
	})
}

func handleListPeers(dial dialer, frameID, scope string) (*mcp.CallToolResult, error) {
	client, err := dial()
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	defer client.Close()

	peers, err := client.PeerList(frameID, scope)
	if err != nil {
		return nil, fmt.Errorf("peer.list: %w", err)
	}
	return jsonResult(peers)
}

func handlePeerSend(dial dialer, fromID, toID, text, replyTo string) (*mcp.CallToolResult, error) {
	client, err := dial()
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	defer client.Close()

	if err := client.PeerSend(fromID, toID, text, replyTo); err != nil {
		return nil, fmt.Errorf("peer.send: %w", err)
	}
	return textResult("sent"), nil
}

func handleSetSummary(dial dialer, frameID, summary string) (*mcp.CallToolResult, error) {
	client, err := dial()
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	defer client.Close()

	if err := client.PeerSetSummary(frameID, summary); err != nil {
		return nil, fmt.Errorf("peer.set_summary: %w", err)
	}
	return textResult("ok"), nil
}

func handleCheckMessages(dial dialer, frameID string) (*mcp.CallToolResult, error) {
	client, err := dial()
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	defer client.Close()

	msgs, err := client.PeerDrainInbox(frameID)
	if err != nil {
		return nil, fmt.Errorf("peer.drain_inbox: %w", err)
	}
	return jsonResult(struct {
		Messages []proto.PeerMessage `json:"messages"`
		Count    int                 `json:"count"`
	}{Messages: msgs, Count: len(msgs)})
}

// textResult wraps a string as an MCP tool result.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// jsonResult marshals v as JSON and wraps it as an MCP tool result.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return textResult(string(b)), nil
}
