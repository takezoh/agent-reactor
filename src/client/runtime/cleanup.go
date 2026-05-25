package runtime

import (
	"log/slog"
	"sync"

	"github.com/takezoh/agent-roost/client/state"
)

// storeFrameCleanup registers a sandbox cleanup callback for a frame.
// Called from goroutines (spawnTmuxWindowAsync) so the map access is mutex-guarded.
func (r *Runtime) storeFrameCleanup(frameID state.FrameID, fn func() error) {
	r.sandboxCleanupsMu.Lock()
	r.sandboxCleanups[frameID] = fn
	r.sandboxCleanupsMu.Unlock()
}

// invokeFrameCleanup retrieves the registered sandbox cleanup for the frame,
// removes it from the map, and runs it in a goroutine so the event loop is not blocked.
func (r *Runtime) invokeFrameCleanup(frameID state.FrameID) {
	r.containerMounts.Delete(frameID)
	r.sandboxCleanupsMu.Lock()
	fn := r.sandboxCleanups[frameID]
	delete(r.sandboxCleanups, frameID)
	r.sandboxCleanupsMu.Unlock()
	if fn == nil {
		return
	}
	go func() {
		if err := fn(); err != nil {
			slog.Warn("runtime: sandbox cleanup failed", "frame", frameID, "err", err)
		}
	}()
}

// drainFrameCleanups invokes all pending sandbox cleanups concurrently and
// waits for them to finish. Called at daemon shutdown before the launcher
// itself is shut down.
func (r *Runtime) drainFrameCleanups() {
	r.sandboxCleanupsMu.Lock()
	fns := r.sandboxCleanups
	r.sandboxCleanups = map[state.FrameID]func() error{}
	r.sandboxCleanupsMu.Unlock()
	var wg sync.WaitGroup
	for frameID, fn := range fns {
		wg.Add(1)
		go func(frameID state.FrameID, fn func() error) {
			defer wg.Done()
			if err := fn(); err != nil {
				slog.Warn("runtime: sandbox cleanup (drain) failed", "frame", frameID, "err", err)
			}
		}(frameID, fn)
	}
	wg.Wait()
}
