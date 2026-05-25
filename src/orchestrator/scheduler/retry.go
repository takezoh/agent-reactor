package scheduler

import (
	"errors"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

const continuationDelay = 1000 * time.Millisecond

// errNoSlots records that a retry fired but no dispatch slot was free; the issue is requeued.
var errNoSlots = errors.New("no available orchestrator slots")

// backoffDelay calculates the exponential backoff delay for a failure retry (SPEC §8.4).
// attempt is the upcoming attempt number (already incremented by workerExitAbnormal).
func backoffDelay(attempt int, cfg wfconfig.Config) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 62 {
		shift = 62
	}
	ms := 10_000 * (1 << shift)
	maxMS := cfg.Agent.MaxRetryBackoffMS
	if maxMS > 0 && ms > maxMS {
		ms = maxMS
	}
	return time.Duration(ms) * time.Millisecond
}
