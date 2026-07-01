package systemdnotify

import (
	"fmt"

	"github.com/coreos/go-systemd/v22/daemon"
)

// Ready sends READY=1 to systemd when NOTIFY_SOCKET is present.
// Non-systemd launches leave NOTIFY_SOCKET unset, and SdNotify treats that as
// a silent no-op.
func Ready() error {
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		return fmt.Errorf("sd_notify READY=1: %w", err)
	}
	_ = sent
	return nil
}
