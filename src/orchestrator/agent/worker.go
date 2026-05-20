package agent

import (
	"context"
	"log/slog"
	"sync"
)

// Worker implements scheduler.Worker (SPEC §7.2 / §16.5).
// It holds the cancellation function and a done channel for the codex subprocess.
type Worker struct {
	cancel  context.CancelFunc
	done    <-chan struct{}
	cleanup func(context.Context) error
	once    sync.Once
}

// Kill stops the underlying codex subprocess by cancelling the worker context.
// It blocks until the subprocess exits, then runs Cleanup once.
func (w *Worker) Kill(reason string) error {
	slog.Info("agent: killing worker", "reason", reason)
	w.cancel()
	<-w.done
	w.runCleanup()
	return nil
}

// runCleanup calls the WrappedLaunch.Cleanup function exactly once.
func (w *Worker) runCleanup() {
	if w.cleanup == nil {
		return
	}
	w.once.Do(func() {
		if err := w.cleanup(context.Background()); err != nil {
			slog.Warn("agent: cleanup error", "err", err)
		}
	})
}
