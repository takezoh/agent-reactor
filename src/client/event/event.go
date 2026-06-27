package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takezoh/agent-reactor/client/config"
	"github.com/takezoh/agent-reactor/client/proto"
	"github.com/takezoh/agent-reactor/platform/appid"
	"golang.org/x/term"
)

// Run implements `server event <eventType> [-data-dir DIR]`.
// Reads stdin (if piped), captures ROOST_FRAME_ID and a timestamp,
// then sends a CmdEvent (host) or CmdHookEvent (container) to the daemon.
//
// Socket resolution order (host path; container path always uses ROOST_SOCKET):
//  1. ROOST_SOCKET env var (set by the daemon when spawning agents; also the
//     bind-mounted path inside sandbox containers)
//  2. -data-dir flag value → "<dir>/<appid.SocketFileName>". Lets a hook
//     installed in Claude/Gemini settings.json route to a non-default daemon
//     when multiple daemons run with different -data-dir on the same host.
//  3. ROOST_DATA_DIR env (consumed by config.Load → ResolveDataDir)
//  4. ~/.agent-reactor/<appid.SocketFileName>
//
// If neither ROOST_SOCKET nor -data-dir is set, the event is silently dropped
// so that hooks fired in completely unrelated contexts (e.g. a Claude Code
// invocation on a host with no daemon configured) do not spam dial errors.
func Run(args []string) error {
	eventType, dataDir, err := parseEventArgs(args)
	if err != nil {
		if errors.Is(err, errMissingEventType) {
			fmt.Fprintf(os.Stderr, "usage: %s event <event-type> [-data-dir DIR]\n", appid.ClientBin)
		}
		return err
	}

	if os.Getenv("ROOST_SOCKET") == "" && dataDir == "" {
		slog.Debug("event: no socket context; dropping event", "type", eventType)
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
		// Container path: ROOST_SOCKET is the bind-mounted UDS, the agent
		// process inherits ROOST_SOCKET_TOKEN, and -data-dir does not apply
		// (the daemon side is fixed by the mount). Pass "" to ignore it.
		sendErr = sendHookEventToDaemon(token, eventType, ts, json.RawMessage(input))
	} else {
		sendErr = sendToDaemon(dataDir, eventType, ts, senderID, json.RawMessage(input))
	}
	if sendErr != nil {
		slog.Warn("event: send failed", "err", sendErr)
	}
	return nil
}

// errMissingEventType is returned by parseEventArgs when no positional
// argument is left after flag stripping. Run uses errors.Is to decide
// whether to print the usage banner.
var errMissingEventType = errors.New("event: missing event type")

// parseEventArgs extracts -data-dir and the event type from args without
// requiring a positional/flag ordering. Go's flag package stops parsing at
// the first non-flag token, but the setup scripts emit the hook command in
// the form `<bin> event <type> -data-dir <dir>` — flag AFTER positional —
// because that order reads more naturally in settings.json. So we do a
// hand-rolled scan that accepts:
//
//	-data-dir <dir>
//	--data-dir <dir>
//	-data-dir=<dir>
//	--data-dir=<dir>
//
// anywhere in args, and treats the first remaining token as the event type.
// Unknown flags surface as an error so a typo in the hook command is loud.
func parseEventArgs(args []string) (eventType, dataDir string, err error) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-data-dir" || a == "--data-dir":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("event: -data-dir requires a value")
			}
			dataDir = args[i+1]
			i++
		case strings.HasPrefix(a, "-data-dir=") || strings.HasPrefix(a, "--data-dir="):
			_, dataDir, _ = strings.Cut(a, "=")
		case strings.HasPrefix(a, "-") && a != "-" && a != "--":
			return "", "", fmt.Errorf("event: unknown flag: %s", a)
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return "", "", errMissingEventType
	}
	return rest[0], dataDir, nil
}

// ResolveSocketPath returns the server daemon UDS path. Resolution order:
//  1. ROOST_SOCKET env var (always wins; set by the daemon and the sandbox mount)
//  2. dataDirOverride argument when non-empty → "<dir>/<appid.SocketFileName>"
//  3. config.Load().ResolveDataDir() → "<resolved>/<appid.SocketFileName>"
//     (which honours ROOST_DATA_DIR env and config DataDir, falling back to
//     ~/.agent-reactor/)
//
// Pass "" for dataDirOverride when the caller has no explicit override (e.g.
// the host-exec/mcp-exec sockbridge paths route through their own constants
// and only need ROOST_SOCKET / config fallback).
func ResolveSocketPath(dataDirOverride string) (string, error) {
	if s := os.Getenv("ROOST_SOCKET"); s != "" {
		return s, nil
	}
	if dataDirOverride != "" {
		return filepath.Join(dataDirOverride, appid.SocketFileName), nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("config load: %w", err)
	}
	return filepath.Join(cfg.ResolveDataDir(), appid.SocketFileName), nil
}

func sendHookEventToDaemon(token, hook string, ts time.Time, payload json.RawMessage) error {
	// Container path: -data-dir is meaningless here (the daemon is reached
	// through the bind-mounted socket pinned by ROOST_SOCKET), so pass "".
	sockPath, err := ResolveSocketPath("")
	if err != nil {
		return err
	}
	return DeliverHookEvent(sockPath, token, hook, ts, payload)
}

// hookDeliverBudget bounds how long DeliverHookEvent retries while the daemon
// brings up the per-frame container registration. Steady-state sends succeed on
// the first attempt and never sleep; the budget only applies during the brief
// window right after a frame is spawned.
const (
	hookDeliverBudget   = 2 * time.Second
	hookDeliverInterval = 40 * time.Millisecond
)

// DeliverHookEvent dials the daemon's container endpoint and sends one
// hook-event, retrying for a bounded window while the daemon finishes per-frame
// registration. A containerized agent can launch and emit its first hooks (e.g.
// SessionStart, which seeds transcript watching) before the endpoint is
// listening or this frame's token is registered — registration happens on the
// event loop after the agent process is spawned. The daemon registers the token
// and mounts (atomically) before it starts the listener, so a successful
// dial+send always implies the token and its mounts are present; retrying until
// success is therefore safe and never delivers against a half-registered frame.
func DeliverHookEvent(sockPath, token, hook string, ts time.Time, payload json.RawMessage) error {
	deadline := time.Now().Add(hookDeliverBudget)
	for attempt := 0; ; attempt++ {
		err := deliverHookOnce(sockPath, token, hook, ts, payload)
		if err == nil {
			if attempt > 0 {
				slog.Debug("event: hook delivered after retry", "hook", hook, "attempts", attempt+1)
			}
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(hookDeliverInterval)
	}
}

func deliverHookOnce(sockPath, token, hook string, ts time.Time, payload json.RawMessage) error {
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

func sendToDaemon(dataDirOverride, eventType string, ts time.Time, senderID string, payload json.RawMessage) error {
	sockPath, err := ResolveSocketPath(dataDirOverride)
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
