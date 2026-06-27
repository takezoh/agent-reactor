package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/takezoh/agent-reactor/client/proto"
	psess "github.com/takezoh/agent-reactor/client/proto/sessions"
	"github.com/takezoh/agent-reactor/platform/appid"
)

func init() {
	Register("activate-occupant", "Switch the main pane occupant (main|log)", runActivateOccupant)
}

// runActivateOccupant implements `arc activate-occupant <kind>`.
// kind must be "main" or "log". Called by the prefix+l keybinding.
func runActivateOccupant(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s activate-occupant <main|log>\n", appid.ClientBin)
		return errors.New("activate-occupant: missing kind")
	}
	kind := args[0]
	if kind != "main" && kind != "log" {
		return fmt.Errorf("activate-occupant: unknown kind %q (want main or log)", kind)
	}
	sockPath, err := resolveSocketPath()
	if err != nil {
		return fmt.Errorf("activate-occupant: %w", err)
	}
	raw, err := proto.Dial(sockPath)
	if err != nil {
		return fmt.Errorf("activate-occupant: dial: %w", err)
	}
	defer raw.Close()
	return psess.Wrap(raw).ActivateOccupant(kind, "", "")
}
