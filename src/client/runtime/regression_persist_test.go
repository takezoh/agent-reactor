package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/state"
)

// regression_persist_test.go pins persistence properties that the current
// FilePersist + Runtime.Run design violates.
//
// The investigation in "sessions.json が終了時の snapshot になっていない"
// identified two structural failures:
//
//   H2: FilePersist.Save([]) on a fresh process wipes pre-existing session
//       files via pruneObsolete. A transient in-memory empty state
//       (e.g. a cascade of EvSpawnFailed before LoadSnapshot has
//       repopulated state) becomes a permanent disk loss.
//
//   H3: Runtime.Run has no defer flush, so SIGINT/SIGTERM-driven shutdown
//       does not persist any state mutations that happened since the last
//       EffPersistSnapshot.
//
// Both tests are expected to FAIL on HEAD.

// T2: A new FilePersist instance pointed at a directory with pre-existing
// session files MUST NOT delete those files on its first Save when the
// caller passed an empty list. The empty-list signal is ambiguous (could
// be "no sessions live" or "in-memory state not yet populated") and the
// destructive interpretation is unsafe across process boundaries.
//
// Today the first Save([]) on a fresh FilePersist treats lastKnownIDs==nil
// as "needs full prune" and wipes the directory. After redesign D2,
// deletion must be explicit (per-session Delete), not implied by absence
// from a bulk list.
func TestRegressionFreshPersistDoesNotWipeOnEmptySave(t *testing.T) {
	dir := t.TempDir()
	sessDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate pre-existing state from a prior daemon run.
	for _, id := range []string{"alpha", "bravo"} {
		data, _ := json.Marshal(SessionSnapshot{ID: id, Project: "/p/" + id})
		if err := os.WriteFile(filepath.Join(sessDir, id+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Fresh process: new FilePersist with lastKnownIDs == nil.
	p := NewFilePersist(dir)

	// The runtime fires EffPersistSnapshot while in-memory state is
	// transiently empty — for example after a reduceSpawnFailed
	// cascade. This must not be a destructive operation.
	if err := p.Save(nil); err != nil {
		t.Fatalf("Save(nil): %v", err)
	}

	for _, id := range []string{"alpha", "bravo"} {
		path := filepath.Join(sessDir, id+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("pre-existing %s.json was wiped by empty Save on a fresh FilePersist — "+
				"this is the catastrophic-loss path (H2). err=%v", id, err)
		}
	}
}

// T4: On cold start, a session whose frames are all status=stopped
// MUST be dropped entirely — both from in-memory state and from
// disk. The dead panes that gave those frames inspection value lived
// in the previous backend session, which no longer exists; keeping the
// metadata around just creates a zombie session the user cannot
// display or terminate.
func TestRegressionColdStartDropsStoppedFrames(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersist(dir)

	live := SessionSnapshot{
		ID:        "live-session",
		Project:   "/p/live",
		CreatedAt: "2026-05-19T00:00:00Z",
		Frames: []SessionFrameSnapshot{{
			ID:          "live-frame",
			Project:     "/p/live",
			Command:     "shell",
			CreatedAt:   "2026-05-19T00:00:00Z",
			Driver:      "shell",
			DriverState: map[string]string{"status": "idle"},
		}},
	}
	dead := SessionSnapshot{
		ID:        "dead-session",
		Project:   "/p/dead",
		CreatedAt: "2026-05-19T00:00:00Z",
		Frames: []SessionFrameSnapshot{{
			ID:          "dead-frame",
			Project:     "/p/dead",
			Command:     "shell",
			CreatedAt:   "2026-05-19T00:00:00Z",
			Driver:      "shell",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}
	if err := p.Save([]SessionSnapshot{live, dead}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	backend := newFakeBackend()
	l := &trackingLauncher{calls: map[string]int{}}
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      p,
		Launcher:     l,
	})
	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}

	if _, ok := r.state.Sessions["live-session"]; !ok {
		t.Error("live session must remain after LoadSnapshot")
	}
	if _, ok := r.state.Sessions["dead-session"]; ok {
		t.Error("session with all frames status=stopped must be dropped on cold start (would otherwise be a zombie)")
	}

	deadFile := filepath.Join(dir, "sessions", "dead-session.json")
	if _, err := os.Stat(deadFile); !os.IsNotExist(err) {
		t.Errorf("dead-session.json must be removed from disk on cold start, got err=%v", err)
	}
	liveFile := filepath.Join(dir, "sessions", "live-session.json")
	if _, err := os.Stat(liveFile); err != nil {
		t.Errorf("live-session.json must remain on disk: %v", err)
	}

	if err := r.RecreateAll(); err != nil {
		t.Fatalf("RecreateAll: %v", err)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.spawnCalls != 1 {
		t.Errorf("spawnCalls = %d, want 1 (only the live frame should spawn)", backend.spawnCalls)
	}
}

// T1a: A successful create-session must result in the new session being
// persisted, even if the daemon is shut down before the spawn pipeline
// fires its own EffPersistSnapshot. With the dispatch-level reconcile
// in place, the very act of mutating r.state.Sessions triggers a Save
// at the runtime layer.
func TestRegressionCreateSessionReachesDisk(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      persist,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	r.Enqueue(state.EvEvent{
		ConnID: 1, ReqID: "req-1", Event: "create-session",
		Payload: json.RawMessage(`{"project":"/tmp/test","command":"shell"}`),
	})

	waitForPersist(t, persist, 1, 2*time.Second)

	cancel()
	<-r.Done()

	persist.mu.Lock()
	defer persist.mu.Unlock()
	if len(persist.last) != 1 {
		t.Fatalf("final snapshot len = %d, want 1 (create-session never reached disk)", len(persist.last))
	}
	if persist.last[0].Project != "/tmp/test" {
		t.Errorf("persisted project = %q, want /tmp/test", persist.last[0].Project)
	}
}

// T1c: A spawn failure deletes a session from state. The runtime must
// translate that into Persist.Delete for the session's file so a
// subsequent cold start does not resurrect the dead session.
func TestRegressionSpawnFailureReachesDiskAsDelete(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second,
		Backend:      backend,
		Persist:      persist,
	})

	drv := state.GetDriver("shell")
	now := time.Now()
	r.state.Sessions["doomed"] = state.Session{
		ID:      "doomed",
		Project: "/p/doomed",
		Command: "shell",
		Driver:  drv.NewState(now),
		Frames: []state.SessionFrame{{
			ID:      "doomed",
			Project: "/p/doomed",
			Command: "shell",
			Driver:  drv.NewState(now),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	r.Enqueue(state.EvSpawnFailed{
		SessionID: "doomed",
		FrameID:   "doomed",
		Err:       "injected",
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		persist.mu.Lock()
		gotDelete := false
		for _, d := range persist.deletes {
			if d == "doomed" {
				gotDelete = true
				break
			}
		}
		persist.mu.Unlock()
		if gotDelete {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-r.Done()

	persist.mu.Lock()
	defer persist.mu.Unlock()
	found := false
	for _, d := range persist.deletes {
		if d == "doomed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Persist.Delete(\"doomed\"); got deletes=%v", persist.deletes)
	}
}

// waitForPersist blocks until at least minSaves Save calls have been
// observed, or the deadline expires.
func waitForPersist(t *testing.T, p *recordingPersist, minSaves int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		n := p.saves
		p.mu.Unlock()
		if n >= minSaves {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d Save call(s); got %d", minSaves, p.saves)
}

// T3: When the runtime's context is cancelled (the SIGINT/SIGTERM path),
// any in-memory state mutations that did not happen to emit
// EffPersistSnapshot must still reach disk before Run returns. Today
// Run() has no defer flush, so a reducer that mutates state without
// emitting persist (e.g. reduceSpawnFailed — see T1c) leaves the
// disk lagging in-memory until termination, at which point the mutation
// is silently lost.
//
// This test pre-populates two sessions, enqueues EvSpawnFailed for
// one of them (which deletes the session from in-memory state without
// emitting EffPersistSnapshot), then cancels the context. The final
// Save call observed by the persist backend MUST reflect the eviction.
func TestRegressionRunFlushesPendingMutationsOnCancel(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:  "reactor-test",
		TickInterval: 10 * time.Second, // suppress periodic ticks that would mask the bug
		Backend:      backend,
		Persist:      persist,
	})

	drv := state.GetDriver("shell")
	now := time.Now()
	for _, id := range []state.SessionID{"keep", "doomed"} {
		r.state.Sessions[id] = state.Session{
			ID:      id,
			Project: "/p/" + string(id),
			Command: "shell",
			Driver:  drv.NewState(now),
			Frames: []state.SessionFrame{{
				ID:      state.FrameID(id),
				Project: "/p/" + string(id),
				Command: "shell",
				Driver:  drv.NewState(now),
			}},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	// Wait until the event loop is actually consuming events. We can't
	// rely on persist.saves here (T1c shows EvSpawnFailed never
	// triggers a save today), so we send a known-flushing event first.
	r.Enqueue(state.EvEvent{ConnID: 0, ReqID: "warmup", Event: "list-sessions"})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		persist.mu.Lock()
		ready := persist.saves >= 0 // any sign of life is fine
		persist.mu.Unlock()
		_ = ready
		// also wait long enough for goroutine to start
		time.Sleep(20 * time.Millisecond)
		break
	}

	// The mutation under test: spawn failure evicts "doomed" from state.
	r.Enqueue(state.EvSpawnFailed{
		SessionID: "doomed",
		FrameID:   "doomed",
		Err:       "injected",
	})

	// Give the event a chance to be dispatched before we cancel. We
	// intentionally do NOT wait for a persist tick; the whole point is
	// that the eviction does not by itself trigger a save.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-r.Done()

	persist.mu.Lock()
	defer persist.mu.Unlock()
	if persist.saves == 0 {
		t.Fatalf("Run() returned without any Persist.Save call — final flush missing (H3)")
	}
	ids := make(map[string]bool, len(persist.last))
	for _, snap := range persist.last {
		ids[snap.ID] = true
	}
	if !ids["keep"] {
		t.Errorf("final snapshot missing 'keep' session: %v", ids)
	}
	if ids["doomed"] {
		t.Errorf("final snapshot still contains evicted 'doomed' session — "+
			"the eviction did not reach disk before Run returned (H3). final=%v", ids)
	}
}

// TestRegressionEvictedSessionStaysEvictedAfterPaneEvents reproduces the
// "session restored every cold start" bug.
//
// Observed: a daemon evicted session "doomed" via EvPaneDied (log shows
// "reducePaneDied evictFrame ok"), Persist.Delete fired, file removed.
// Then while the daemon kept running, sessions/doomed.json was written
// again — Persist.Save received "doomed" in its session list — meaning
// state.Sessions had re-acquired the session despite the eviction. The
// only Save path is r.snapshotSessions(), so re-acquisition is the
// only consistent explanation. No log line attributes this re-add to
// any specific event (the new diagnostic added in this commit will catch
// the trigger on the next reproduction).
//
// The bug must be in some event path that runs AFTER eviction and re-
// inserts the session. The eviction reducer removes the session from a
// cloned map; legitimate adders are reduceCreateSession / forkSession
// (new random ID, cannot collide) and the bootstrap path (only on
// startup). All findFrame-gated paths return early when the session is
// missing. Subsystem and panetap events for the dead frame are the
// remaining suspects — particularly because executeUnregisterPane
// early-returns when sessionPanes was already cleared by executeKill
// SessionWindow, so the panetap for an evicted frame leaks and keeps
// emitting EvPaneOsc / EvPanePrompt events with the dead FrameID.
//
// This test pre-loads "doomed" + a peer "keeper", evicts "doomed" via
// EvPaneDied (same path the broken daemon took), then fires the events
// the leaked tap can emit (EvPaneOsc, EvPanePrompt) and verifies the
// session remains absent from state, no Save call includes its ID, and
// Persist.Delete was called exactly once.
func TestRegressionEvictedSessionStaysEvictedAfterPaneEvents(t *testing.T) {
	backend := newFakeBackend()
	persist := &recordingPersist{}
	r := New(Config{
		SessionName:      "reactor-test",
		TickInterval:     50 * time.Millisecond,
		FastTickInterval: 25 * time.Millisecond,
		Backend:          backend,
		Persist:          persist,
	})

	drv := state.GetDriver("shell")
	now := time.Now()
	for _, id := range []state.SessionID{"keeper", "doomed"} {
		r.state.Sessions[id] = state.Session{
			ID:      id,
			Project: "/p/" + string(id),
			Command: "shell",
			Driver:  drv.NewState(now),
			Frames: []state.SessionFrame{{
				ID:      state.FrameID(id),
				Project: "/p/" + string(id),
				Command: "shell",
				Driver:  drv.NewState(now),
			}},
		}
	}
	r.sessionPanes["doomed"] = "%doomed"
	r.sessionPanes["keeper"] = "%keeper"
	r.state.ActiveSession = "doomed"
	r.state.ActiveOccupant = state.OccupantFrame

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Run(ctx) }()

	// Wait for event loop to be live before injecting.
	r.Enqueue(state.EvEvent{ConnID: 0, ReqID: "warmup", Event: "list-sessions"})
	time.Sleep(80 * time.Millisecond)

	// Evict via the same path the broken daemon took.
	r.Enqueue(state.EvPaneDied{
		Pane:         "{sessionName}:0.1",
		OwnerFrameID: "doomed",
	})

	// Wait for the eviction's Persist.Delete to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		persist.mu.Lock()
		gotDelete := false
		for _, d := range persist.deletes {
			if d == "doomed" {
				gotDelete = true
				break
			}
		}
		persist.mu.Unlock()
		if gotDelete {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now fire the events a leaked panetap would still emit for the
	// dead frame. If any of these re-inserts the session, subsequent
	// Save calls will include "doomed".
	r.Enqueue(state.EvPaneOsc{
		FrameID: "doomed",
		Cmd:     0,
		Title:   "ghost title",
		Now:     time.Now(),
	})
	r.Enqueue(state.EvPaneOsc{
		FrameID: "doomed",
		Cmd:     9,
		Title:   "ghost notification",
		Body:    "body",
		Now:     time.Now(),
	})
	exit := 0
	r.Enqueue(state.EvPanePrompt{
		FrameID:  "doomed",
		Phase:    state.PromptPhaseComplete,
		ExitCode: &exit,
		Now:      time.Now(),
	})
	r.Enqueue(state.EvFileChanged{
		FrameID: "doomed",
		Path:    "/tmp/ghost",
	})

	// Let several ticks fire so any periodic Save catches state mutations.
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-r.Done()

	persist.mu.Lock()
	defer persist.mu.Unlock()
	// "doomed" must have been deleted.
	deleted := false
	for _, d := range persist.deletes {
		if d == "doomed" {
			deleted = true
			break
		}
	}
	if !deleted {
		t.Errorf("Persist.Delete(\"doomed\") never called: deletes=%v", persist.deletes)
	}

	// "doomed" must NOT appear in the final Save's session list.
	for _, snap := range persist.last {
		if snap.ID == "doomed" {
			t.Errorf("final Save still contained evicted session 'doomed' (last=%v)", snapshotIDs(persist.last))
			break
		}
	}

	// And it must NOT appear in any Save the runtime produced after eviction
	// — sample by checking the in-memory state at shutdown.
	if _, alive := r.state.Sessions["doomed"]; alive {
		t.Errorf("state.Sessions still has 'doomed' after eviction + ghost events")
	}
}

func snapshotIDs(snaps []SessionSnapshot) []string {
	out := make([]string, len(snaps))
	for i, s := range snaps {
		out[i] = s.ID
	}
	return out
}
