package cli

import (
	"fmt"

	"github.com/takezoh/agent-reactor/client/proto"
	psess "github.com/takezoh/agent-reactor/client/proto/sessions"
)

func init() {
	Register("statusline-click", "Notify daemon of a status-bar click (internal backend binding)", runStatusLineClick)
}

// runStatusLineClick implements `client statusline-click [range_name]`.
// Called by the backend's MouseDown1Status binding:
//
//	client statusline-click #{mouse_status_range}
//
// range_name is the backend's mouse_status_range value; empty means no named region was hit.
func runStatusLineClick(args []string) error {
	rangeName := ""
	if len(args) > 0 {
		rangeName = args[0]
	}
	sockPath, err := resolveSocketPath()
	if err != nil {
		return fmt.Errorf("statusline-click: %w", err)
	}
	raw, err := proto.Dial(sockPath)
	if err != nil {
		return fmt.Errorf("statusline-click: dial: %w", err)
	}
	defer raw.Close()
	return psess.Wrap(raw).StatusLineClick(rangeName)
}
