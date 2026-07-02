package stream

// Frame init serialization invariants — the property that makes adopt
// deterministic without any cwd/heuristic guessing.
//
// The design is a mutex-guarded single-slot reservation (`initState`, see
// initsem.go) with four atomic ops: acquire, takeAny, takeIfOwned,
// takeIfExpired. A fresh BindFrame acquires; handleThreadStarted's adopt
// path takes it. Between those the invariant "at most one pending frame
// per Backend" holds — so any incoming unknown thread has an unambiguous
// owner. These tests pin each edge case: acquisition ordering, timeout,
// release, and the reaper safety net.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/runtime/subsystem"
	"github.com/takezoh/agent-reactor/client/state"
)

// rawStarted mimics the app-server thread/started payload shape (nested
// under "thread") that handleThreadStarted parses.
func rawStarted(threadID, cwd string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"thread": map[string]any{"id": threadID, "cwd": cwd},
	})
	return b
}

// pendingCount inspects initState non-destructively for tests. Not exported
// on Backend because production code should treat the slot as opaque.
func pendingCount(b *Backend) int {
	b.initState.mu.Lock()
	defer b.initState.mu.Unlock()
	if b.initState.slot == nil {
		return 0
	}
	return 1
}

// TestBindFrame_SerializesConcurrentInit — the invariant: two fresh
// BindFrame calls in flight both succeed, but the second one blocks until
// the first one is adopted. Ordering is preserved (FIFO channel semantics).
func TestBindFrame_SerializesConcurrentInit(t *testing.T) {
	b, _ := newTestBackend()

	// Launch two concurrent BindFrame goroutines. F1 goes first; F2 races
	// but should block until F1's adopt is complete.
	f1Done := make(chan error, 1)
	f2Done := make(chan error, 1)

	go func() {
		_, err := b.BindFrame(context.Background(), makeBindReq("F1", "/work-1"))
		f1Done <- err
	}()

	// Give F1 a moment to acquire the slot before F2 races in.
	time.Sleep(10 * time.Millisecond)

	go func() {
		_, err := b.BindFrame(context.Background(), makeBindReq("F2", "/work-2"))
		f2Done <- err
	}()

	// F1 must have completed BindFrame (it doesn't wait for adopt).
	select {
	case err := <-f1Done:
		if err != nil {
			t.Fatalf("F1 BindFrame: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("F1 BindFrame did not return within 1s")
	}

	// F2 is blocked on initState.acquire because F1 still holds the slot.
	select {
	case err := <-f2Done:
		t.Fatalf("F2 BindFrame returned early (err=%v); should be blocked on initState.acquire", err)
	case <-time.After(50 * time.Millisecond):
		// good: F2 still waiting
	}

	// Adopt F1's CLI-created thread. This drains the slot and lets F2 in.
	b.handleThreadStarted(rawStarted("thread-for-F1", "/work-1"))

	select {
	case err := <-f2Done:
		if err != nil {
			t.Fatalf("F2 BindFrame after F1 adopt: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("F2 BindFrame did not unblock after F1 adopt")
	}

	// F1's binding should be threadID = "thread-for-F1" (adopted).
	b.mu.Lock()
	f1Bind := b.frames["F1"]
	f2Bind := b.frames["F2"]
	b.mu.Unlock()
	if f1Bind == nil || f1Bind.threadID != "thread-for-F1" {
		t.Errorf("F1 binding = %+v, want threadID=thread-for-F1", f1Bind)
	}
	// F2 is now the pending frame (threadID still empty until its own adopt).
	if f2Bind == nil || f2Bind.threadID != "" {
		t.Errorf("F2 binding = %+v, want empty threadID (pending)", f2Bind)
	}
	if pendingCount(b) != 1 {
		t.Errorf("initState should hold F2, got count=%d", pendingCount(b))
	}
}

// TestBindFrame_AcquireTimeoutOnStuckSlot — if the previous frame's adopt
// never comes, a subsequent fresh BindFrame times out and errors rather
// than hanging forever. This uses a shortened acquire timeout via context.
func TestBindFrame_AcquireTimeoutOnStuckSlot(t *testing.T) {
	b, _ := newTestBackend()

	if _, err := b.BindFrame(context.Background(), makeBindReq("F1", "/work")); err != nil {
		t.Fatalf("F1 BindFrame: %v", err)
	}

	// Simulate F1's CLI never issuing thread/start — F2 tries to acquire
	// under a short context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := b.BindFrame(ctx, makeBindReq("F2", "/work"))
	if err == nil {
		t.Fatal("F2 BindFrame should have failed on stuck slot")
	}
	// context timeout was inside acquirePendingSlot; err surfaces from there.
}

// TestReleaseFrame_DrainsPendingSlot — killing a pending frame frees the
// slot immediately (no reaper delay). Next BindFrame acquires without wait.
func TestReleaseFrame_DrainsPendingSlot(t *testing.T) {
	b, _ := newTestBackend()

	if _, err := b.BindFrame(context.Background(), makeBindReq("F1", "/work")); err != nil {
		t.Fatalf("F1 BindFrame: %v", err)
	}
	if pendingCount(b) != 1 {
		t.Fatalf("initState should hold F1 before ReleaseFrame; count=%d", pendingCount(b))
	}
	b.ReleaseFrame("F1")
	if pendingCount(b) != 0 {
		t.Errorf("initState should be empty after ReleaseFrame; count=%d", pendingCount(b))
	}

	// A follow-up BindFrame acquires immediately.
	done := make(chan error, 1)
	go func() {
		_, err := b.BindFrame(context.Background(), makeBindReq("F2", "/work"))
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("F2 BindFrame after release: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("F2 BindFrame did not acquire the released slot promptly")
	}
}

// TestReleaseFrame_DoesNotDrainOtherFramesSlot — releasing a frame that is
// NOT the pending one must leave the slot alone. Otherwise a stray kill
// could steal another frame's adopt reservation.
func TestReleaseFrame_DoesNotDrainOtherFramesSlot(t *testing.T) {
	b, _ := newTestBackend()

	if _, err := b.BindFrame(context.Background(), makeBindReq("F1", "/work-1")); err != nil {
		t.Fatalf("F1 BindFrame: %v", err)
	}
	// Register F2 in b.frames only (no slot), then ReleaseFrame it.
	b.registerPendingFrame("F2", "/work-2", "")
	// Sanity: initState still holds F1.
	if pendingCount(b) != 1 {
		t.Fatalf("initState count = %d, want 1 (F1's slot)", pendingCount(b))
	}
	b.ReleaseFrame("F2")
	if pendingCount(b) != 1 {
		t.Errorf("initState drained by unrelated ReleaseFrame; count=%d", pendingCount(b))
	}
}

// TestReapExpiredSlot_CleansOrphanBinding — if the CLI crashes before its
// first thread/start and Runtime doesn't call ReleaseFrame (per Explore
// agent Q3), the reaper loop reclaims the slot after the adopt deadline.
// This test drives one reap iteration synchronously to avoid depending on
// the ticker cadence.
func TestReapExpiredSlot_CleansOrphanBinding(t *testing.T) {
	b, _ := newTestBackend()

	// Manually inject a slot with a past deadline to simulate an expired
	// pending frame.
	b.registerPendingFrame("F-stuck", "/work", "")
	b.initState.mu.Lock()
	b.initState.slot = &pendingSlot{frameID: "F-stuck", deadline: time.Now().Add(-time.Second)}
	b.initState.mu.Unlock()

	// Emulate one reaper iteration: takeIfExpired atomically clears the slot
	// if its deadline has passed. Production loop calls this on every tick.
	expired := b.initState.takeIfExpired(time.Now())
	if expired == nil {
		t.Fatal("expected takeIfExpired to return the past-deadline slot")
	}
	b.mu.Lock()
	delete(b.frames, expired.frameID)
	b.mu.Unlock()
	if pendingCount(b) != 0 {
		t.Errorf("initState not cleared; count=%d", pendingCount(b))
	}
	b.mu.Lock()
	_, exists := b.frames["F-stuck"]
	b.mu.Unlock()
	if exists {
		t.Error("stuck frame binding not cleaned up")
	}
}

// TestHandleThreadStarted_UnknownWithNoPending_SilentDrop — arriving
// unknown thread with no pending frame is a legitimate no-op (silent
// drop). This is the recovery-race case (a thread notification arrives
// after ReleaseFrame or before any BindFrame).
func TestHandleThreadStarted_UnknownWithNoPending_SilentDrop(t *testing.T) {
	b, rt := newTestBackend()

	b.handleThreadStarted(rawStarted("orphan-thread", "/nowhere"))

	// No events emitted, no binding created.
	if len(rt.events) != 0 {
		t.Errorf("expected no EvSubsystem, got %d", len(rt.events))
	}
	b.mu.Lock()
	if len(b.frames) != 0 {
		t.Errorf("no frame should be created by orphan thread; got %d", len(b.frames))
	}
	if len(b.threads) != 0 {
		t.Errorf("no thread should be registered; got %d", len(b.threads))
	}
	b.mu.Unlock()
}

// TestBindFrame_RecoveryPathSkipsInitSem — cold-start recovery pre-registers
// the persisted thread id directly (`registerBoundFrame`), leaving the
// initState untouched so fresh interactive frames can proceed in parallel.
func TestBindFrame_RecoveryPathSkipsInitSem(t *testing.T) {
	b, _ := newTestBackend()

	req := makeBindReq("F-recovered", "/work")
	req.Plan.Stream.ResumeTarget = state.ResumeTarget{ThreadID: "persisted-T", RolloutPath: ""}
	if _, err := b.BindFrame(context.Background(), req); err != nil {
		t.Fatalf("recovery BindFrame: %v", err)
	}

	if pendingCount(b) != 0 {
		t.Errorf("initState occupied on recovery path; count=%d", pendingCount(b))
	}
	b.mu.Lock()
	binding := b.frames["F-recovered"]
	fromReverse := b.threads["persisted-T"]
	b.mu.Unlock()
	if binding == nil || binding.threadID != "persisted-T" {
		t.Errorf("binding = %+v, want threadID=persisted-T", binding)
	}
	if fromReverse != "F-recovered" {
		t.Errorf("b.threads[persisted-T] = %q, want F-recovered", fromReverse)
	}
}

// TestBindFrame_RecoveryCollision_RejectsSecondBind — the ADR-0001
// routing-isolation invariant: two frames must not share a threadID in
// b.threads. registerBoundFrame's collision guard (added in Loop 2)
// rejects the second recovery bind so events for the persisted thread
// never route to the wrong frame.
func TestBindFrame_RecoveryCollision_RejectsSecondBind(t *testing.T) {
	b, _ := newTestBackend()

	req1 := makeBindReq("F-a", "/work-a")
	req1.Plan.Stream.ResumeTarget = state.ResumeTarget{ThreadID: "shared-T"}
	if _, err := b.BindFrame(context.Background(), req1); err != nil {
		t.Fatalf("first recovery BindFrame: %v", err)
	}

	req2 := makeBindReq("F-b", "/work-b")
	req2.Plan.Stream.ResumeTarget = state.ResumeTarget{ThreadID: "shared-T"}
	_, err := b.BindFrame(context.Background(), req2)
	if err == nil {
		t.Fatal("second recovery BindFrame with same threadID should have errored")
	}

	// F-a's routing must be preserved.
	b.mu.Lock()
	owner := b.threads["shared-T"]
	_, fbExists := b.frames["F-b"]
	b.mu.Unlock()
	if owner != "F-a" {
		t.Errorf("b.threads[shared-T] = %q, want F-a (collision must not steal ownership)", owner)
	}
	if fbExists {
		t.Error("F-b's binding leaked on collision reject")
	}
}

// TestBindFrame_WorktreeCleanupOnError — the Loop 3 defer must call
// removeWorktree when a later step in BindFrame returns an error after
// createWorktree already succeeded. Overrides both indirections so the
// test doesn't need a real git repo, then triggers a registerBoundFrame
// collision to force the error path. A revert of the defer to a no-op
// (or of the collision reject to silent overwrite) fails this test.
func TestBindFrame_WorktreeCleanupOnError(t *testing.T) {
	b, _ := newTestBackend()

	prevCreate := createWorktree
	createWorktree = func(_ context.Context, in subsystem.WorktreeInput) (subsystem.WorktreeResult, error) {
		return subsystem.WorktreeResult{
			StartDir: "/repo/.agent-reactor/worktrees/wt-fake",
			Name:     "wt-fake",
		}, nil
	}
	t.Cleanup(func() { createWorktree = prevCreate })

	removed := make(chan string, 1)
	prevRemove := removeWorktree
	removeWorktree = func(path string) { removed <- path }
	t.Cleanup(func() { removeWorktree = prevRemove })

	// Seed the collision that will fire in the second bind.
	first := makeBindReq("F-1", "/work-1")
	first.Plan.Stream.ResumeTarget = state.ResumeTarget{ThreadID: "conflict-T"}
	if _, err := b.BindFrame(context.Background(), first); err != nil {
		t.Fatalf("first BindFrame: %v", err)
	}

	// Second bind with Worktree.Enabled=true — createWorktree returns the
	// stub result → createdWorktree is populated → registerBoundFrame
	// rejects the collision → defer must call removeWorktree.
	second := makeBindReq("F-2", "/work-2")
	second.Plan.Options.Worktree.Enabled = true
	second.Plan.Stream.ResumeTarget = state.ResumeTarget{ThreadID: "conflict-T"}
	if _, err := b.BindFrame(context.Background(), second); err == nil {
		t.Fatal("collision BindFrame should error")
	}
	b.mu.Lock()
	_, exists := b.frames["F-2"]
	b.mu.Unlock()
	if exists {
		t.Error("F-2 partial binding leaked; defer cleanup did not fire")
	}
	select {
	case path := <-removed:
		if path != "/repo/.agent-reactor/worktrees/wt-fake" {
			t.Errorf("removeWorktree called with %q, want the stub worktree path", path)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("BindFrame's defer did not call removeWorktree on the error path")
	}
}

// TestReapExpiredSlot_GoesThroughReleaseFrame — the Loop 5 fix wires
// reapOnce to call Backend.ReleaseFrame so worktree cleanup runs for a
// reaped frame. Overrides removeWorktree to observe the actual call and
// drives reapOnce directly. A revert of the reaper to inline
// `delete(b.frames, ...)` skips ReleaseFrame → removeWorktree is never
// called → this test fails.
func TestReapExpiredSlot_GoesThroughReleaseFrame(t *testing.T) {
	b, _ := newTestBackend()

	got := make(chan string, 1)
	prev := removeWorktree
	removeWorktree = func(path string) { got <- path }
	t.Cleanup(func() { removeWorktree = prev })

	b.registerPendingFrame("F-stuck", "/work", "/repo/.agent-reactor/worktrees/wt-42")
	b.initState.mu.Lock()
	b.initState.slot = &pendingSlot{
		frameID:  "F-stuck",
		deadline: time.Now().Add(-time.Second),
	}
	b.initState.mu.Unlock()

	// Drive the reaper loop body directly (extracted for testability).
	b.reapOnce(time.Now())

	b.mu.Lock()
	_, exists := b.frames["F-stuck"]
	b.mu.Unlock()
	if exists {
		t.Error("reapOnce did not drop the expired binding")
	}
	if pendingCount(b) != 0 {
		t.Errorf("initState not empty after reapOnce; count=%d", pendingCount(b))
	}
	select {
	case path := <-got:
		if path != "/repo/.agent-reactor/worktrees/wt-42" {
			t.Errorf("removeWorktree called with %q, want the reaped frame's worktreePath", path)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("removeWorktree was not called; reaper skipped ReleaseFrame's worktree cleanup path")
	}
}

// makeBindReq is a small ctor used by every test above to avoid repeating
// the LaunchPlan boilerplate.
func makeBindReq(frameID state.FrameID, startDir string) subsystem.BindRequest {
	return subsystem.BindRequest{
		FrameID: frameID,
		Plan:    state.LaunchPlan{StartDir: startDir},
	}
}
