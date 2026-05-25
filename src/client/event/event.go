package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/takezoh/agent-roost/client/config"
	"github.com/takezoh/agent-roost/client/proto"
	"golang.org/x/term"
)

// Run implements `roost event <eventType>`.
// Reads stdin (if piped), captures ROOST_FRAME_ID and a timestamp,
// then sends a CmdEvent (host) or CmdHookEvent (container) to the daemon.
func Run(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: roost event <event-type>")
		return errors.New("event: missing event type")
	}
	eventType := args[0]

	if os.Getenv("ROOST_SOCKET") == "" {
		slog.Debug("event: ROOST_SOCKET not set; dropping event", "type", eventType)
		return nil
	}
	senderID := os.Getenv("ROOST_FRAME_ID")
	ts := time.Now()

	var input []byte
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		input, _ = io.ReadAll(os.Stdin)
	}

	slog.Debug("event",
		"type", eventType,
		"sender", senderID,
		"input_len", len(input),
	)

	token := os.Getenv("ROOST_SOCKET_TOKEN")
	var sendErr error
	if token != "" {
		sendErr = sendHookEventToDaemon(token, eventType, ts, json.RawMessage(input))
	} else {
		sendErr = sendToDaemon(eventType, ts, senderID, json.RawMessage(input))
	}
	if sendErr != nil {
		slog.Warn("event: send failed", "err", sendErr)
	}
	return nil
}

// ResolveSocketPath returns the roost daemon UDS path, preferring the
// ROOST_SOCKET env var when set.
func ResolveSocketPath() (string, error) {
	if s := os.Getenv("ROOST_SOCKET"); s != "" {
		return s, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("config load: %w", err)
	}
	return filepath.Join(cfg.ResolveDataDir(), "roost.sock"), nil
}

func sendHookEventToDaemon(token, hook string, ts time.Time, payload json.RawMessage) error {
	sockPath, err := ResolveSocketPath()
	if err != nil {
		return err
	}
	client, err := proto.Dial(sockPath)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.SendHookEvent(token, hook, ts, payload); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

func sendToDaemon(eventType string, ts time.Time, senderID string, payload json.RawMessage) error {
	sockPath, err := ResolveSocketPath()
	if err != nil {
		return err
	}
	client, err := proto.Dial(sockPath)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	if err := client.SendEvent(eventType, ts, senderID, payload); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}
