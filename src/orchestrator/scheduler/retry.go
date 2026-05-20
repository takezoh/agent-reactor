package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/wfconfig"
)

const continuationDelay = 1000 * time.Millisecond

// backoffDelay calculates the exponential backoff delay for a failure retry (SPEC §8.4).
// attempt is the upcoming attempt number (already incremented by WorkerExitAbnormal).
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

// retryFireReq is sent by a timer callback to the Run loop retry channel.
type retryFireReq struct {
	IssueID string
	Attempt int
}

// scheduleRetry sets DueAtMS and Timer on entry, enqueues it in state, and arranges
// for a retryFireReq to be delivered to fireCh when the delay elapses.
func scheduleRetry(st *State, clk Clock, fireCh chan<- retryFireReq, ctx context.Context, entry RetryEntry, delay time.Duration) {
	entry.DueAtMS = clk.Now().Add(delay).UnixMilli()
	issueID := entry.IssueID
	attempt := entry.Attempt
	entry.Timer = RetryTimer{t: clk.NewTimer(delay, func() {
		select {
		case fireCh <- retryFireReq{IssueID: issueID, Attempt: attempt}:
		case <-ctx.Done():
		}
	})}
	st.EnqueueRetry(entry)
	slog.Info("retry scheduled", "issue_id", issueID, "attempt", attempt, "delay_ms", delay.Milliseconds())
}
