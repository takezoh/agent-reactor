package runtime

import (
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/driver"
	"github.com/takezoh/agent-reactor/client/state"
)

func TestRecoverWarmStartSessions_ReinstallsTranscriptWatch(t *testing.T) {
	watcher := &recordingWatcher{}
	persist := &recordingPersist{}
	r := New(Config{
		TickInterval: 10 * time.Second,
		Backend:      newFakeBackend(),
		Watcher:      watcher,
		Persist:      persist,
	})
	now := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	d := driver.NewCodexDriver("/tmp/events")
	r.state.Sessions["s1"] = state.Session{
		ID:        "s1",
		Project:   "/repo",
		CreatedAt: now,
		Frames: []state.SessionFrame{{
			ID:        "f1",
			Project:   "/repo",
			Command:   "codex",
			CreatedAt: now,
			Driver: d.Restore(map[string]string{
				"transcript_path":  "/tmp/t.jsonl",
				"codex_session_id": "sess-1",
			}, now),
		}},
	}

	r.RecoverWarmStartSessions()

	watcher.mu.Lock()
	gotPath := watcher.watches["f1"]
	watcher.mu.Unlock()
	if gotPath != "/tmp/t.jsonl" {
		t.Fatalf("watch path = %q, want /tmp/t.jsonl", gotPath)
	}
	if len(r.state.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(r.state.Jobs))
	}
	got := r.state.Sessions["s1"].Frames[0].Driver.(driver.CodexState)
	if !got.TranscriptInFlight {
		t.Fatal("TranscriptInFlight should be true")
	}
	if persist.saves == 0 {
		t.Fatal("expected persist on rehydrate")
	}
}

func TestLoadSnapshot_ColdStartConvertsRunningToWaiting(t *testing.T) {
	snaps := []SessionSnapshot{
		{
			ID: "s1",
			Frames: []SessionFrameSnapshot{{
				ID:      "f1",
				Command: "generic",
				DriverState: map[string]string{
					"status": "running",
				},
			}},
		},
	}
	persist := &snapLoader{snaps: snaps}
	r := New(Config{
		Persist: persist,
	})

	// Cold start: should convert to waiting
	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	s1 := r.state.Sessions["s1"]
	drv := state.GetDriver("generic")
	if drv.Status(s1.Driver) != state.StatusWaiting {
		t.Errorf("Cold start status = %v, want waiting", drv.Status(s1.Driver))
	}

	// Reset and try warm start with a fresh snap map
	r.state.Sessions = make(map[state.SessionID]state.Session)
	persist.snaps = []SessionSnapshot{
		{
			ID: "s1",
			Frames: []SessionFrameSnapshot{{
				ID:      "f1",
				Command: "generic",
				DriverState: map[string]string{
					"status": "running",
				},
			}},
		},
	}
	if err := r.LoadSnapshot(false); err != nil {
		t.Fatalf("LoadSnapshot(false): %v", err)
	}
	s1 = r.state.Sessions["s1"]
	if drv.Status(s1.Driver) != state.StatusRunning {
		t.Errorf("Warm start status = %v, want running", drv.Status(s1.Driver))
	}
}

type snapLoader struct {
	noopPersist
	snaps   []SessionSnapshot
	deleted []string
}

func (s *snapLoader) Load() ([]SessionSnapshot, error) {
	return s.snaps, nil
}

func (s *snapLoader) Delete(id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

// codexThreadID is a representative resumable thread id (alphanumeric+hyphen),
// matching the format codex persists for a started/resumed thread.
const codexThreadID = "019e727e-fde4-7432-9036-ae6604ce1b27"

// TestLoadSnapshot_ColdStartKeepsRecoverableStoppedCodexFrame guards the cold
// start regression where a stopped codex session was dropped (and deleted from
// disk) even though its conversation lives in a host-mounted thread that can be
// resumed against a fresh app-server. Codex implements ColdStartRecoverer, so a
// stopped frame with a resumable thread must survive cold start.
func TestLoadSnapshot_ColdStartKeepsRecoverableStoppedCodexFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-sess",
		Frames: []SessionFrameSnapshot{{
			ID:      "f1",
			Command: "codex",
			DriverState: map[string]string{
				"status":    "stopped",
				"thread_id": codexThreadID,
			},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	sess, ok := r.state.Sessions["codex-sess"]
	if !ok {
		t.Fatal("recoverable stopped codex session dropped on cold start; want kept for thread resume")
	}
	if len(sess.Frames) != 1 {
		t.Fatalf("frames = %d, want 1", len(sess.Frames))
	}
	for _, id := range persist.deleted {
		if id == "codex-sess" {
			t.Error("recoverable snapshot must not be deleted from disk")
		}
	}
}

// TestLoadSnapshot_ColdStartDropsStoppedCodexFrameWithoutThread ensures the
// recovery is gated on an actual resumable thread: with no thread id there is
// nothing to resume, so the stopped frame is dropped like any other.
func TestLoadSnapshot_ColdStartDropsStoppedCodexFrameWithoutThread(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-nothread",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "codex",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, ok := r.state.Sessions["codex-nothread"]; ok {
		t.Error("stopped codex frame with no resumable thread should be dropped on cold start")
	}
}

// TestLoadSnapshot_ColdStartDropsStoppedGenericFrame ensures the default policy
// is unchanged for drivers without durable state: a stopped frame is dropped.
func TestLoadSnapshot_ColdStartDropsStoppedGenericFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "generic-sess",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "generic",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(true); err != nil {
		t.Fatalf("LoadSnapshot(true): %v", err)
	}
	if _, ok := r.state.Sessions["generic-sess"]; ok {
		t.Error("stopped generic frame (no durable state) must still be dropped on cold start")
	}
}

// TestLoadSnapshot_WarmStartKeepsStoppedCodexFrame ensures warm start is
// unaffected — it keeps every frame, recoverable or not, since the live backend
// frame is still attached for inspection.
func TestLoadSnapshot_WarmStartKeepsStoppedCodexFrame(t *testing.T) {
	persist := &snapLoader{snaps: []SessionSnapshot{{
		ID: "codex-warm",
		Frames: []SessionFrameSnapshot{{
			ID:          "f1",
			Command:     "codex",
			DriverState: map[string]string{"status": "stopped"},
		}},
	}}}
	r := New(Config{Persist: persist})

	if err := r.LoadSnapshot(false); err != nil {
		t.Fatalf("LoadSnapshot(false): %v", err)
	}
	if _, ok := r.state.Sessions["codex-warm"]; !ok {
		t.Error("warm start must keep stopped frames (dead frame still attached for inspection)")
	}
}
