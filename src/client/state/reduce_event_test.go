package state

import (
	"encoding/json"
	"testing"
	"time"
)

// pushStubState is a driver state used in EffPushDriver tests.
type pushStubState struct {
	DriverStateBase
}

// pushDriverStub emits EffPushDriver from its hook handler.
type pushDriverStub struct{}

func (pushDriverStub) Name() string                       { return "pushstub" }
func (pushDriverStub) DisplayName() string                { return "pushstub" }
func (pushDriverStub) Status(s DriverState) Status        { return StatusIdle }
func (pushDriverStub) NewState(now time.Time) DriverState { return pushStubState{} }
func (pushDriverStub) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (pushDriverStub) Persist(s DriverState) map[string]string { return nil }
func (pushDriverStub) Restore(bag map[string]string, now time.Time) DriverState {
	return pushStubState{}
}
func (pushDriverStub) View(s DriverState) View {
	return View{Card: Card{BorderTitle: Tag{Text: "pushstub"}}}
}
func (pushDriverStub) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	// On any hook, emit EffPushDriver with empty SessionID (to be filled by postProcessEffect).
	if _, ok := ev.(DEvHook); ok {
		return prev, []Effect{EffPushDriver{Command: "stub"}}, View{}
	}
	return prev, nil, View{}
}

func init() {
	if _, exists := driverRegistry["pushstub"]; !exists {
		Register(pushDriverStub{})
	}
}

type subsystemStubState struct {
	DriverStateBase
	lastKind   SubsystemEventKind
	lastSource SubsystemKind
}

type subsystemDriverStub struct{}

func (subsystemDriverStub) Name() string                       { return "subsystemstub" }
func (subsystemDriverStub) DisplayName() string                { return "subsystemstub" }
func (subsystemDriverStub) Status(s DriverState) Status        { return StatusIdle }
func (subsystemDriverStub) NewState(now time.Time) DriverState { return subsystemStubState{} }
func (subsystemDriverStub) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (subsystemDriverStub) Persist(s DriverState) map[string]string { return nil }
func (subsystemDriverStub) Restore(bag map[string]string, now time.Time) DriverState {
	return subsystemStubState{}
}
func (subsystemDriverStub) View(s DriverState) View {
	return View{Card: Card{BorderTitle: Tag{Text: "subsystemstub"}}}
}
func (subsystemDriverStub) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	st, _ := prev.(subsystemStubState)
	if sev, ok := ev.(DEvSubsystem); ok {
		st.lastKind = sev.Kind
		st.lastSource = sev.Source
		return st, nil, View{}
	}
	return st, nil, View{}
}

func init() {
	if _, exists := driverRegistry["subsystemstub"]; !exists {
		Register(subsystemDriverStub{})
	}
}

// bogusSessionDriverStub emits EffPushDriver with a non-existent SessionID.
type bogusSessionDriverStub struct{}

func (bogusSessionDriverStub) Name() string                       { return "bogussessionstub" }
func (bogusSessionDriverStub) DisplayName() string                { return "bogussessionstub" }
func (bogusSessionDriverStub) Status(s DriverState) Status        { return StatusIdle }
func (bogusSessionDriverStub) NewState(now time.Time) DriverState { return pushStubState{} }
func (bogusSessionDriverStub) PrepareLaunch(s DriverState, mode LaunchMode, project, baseCommand string, options LaunchOptions, _ bool) (LaunchPlan, error) {
	return LaunchPlan{Command: baseCommand, StartDir: project}, nil
}
func (bogusSessionDriverStub) Persist(s DriverState) map[string]string { return nil }
func (bogusSessionDriverStub) Restore(bag map[string]string, now time.Time) DriverState {
	return pushStubState{}
}
func (bogusSessionDriverStub) View(s DriverState) View {
	return View{Card: Card{BorderTitle: Tag{Text: "bogussessionstub"}}}
}
func (bogusSessionDriverStub) Step(prev DriverState, ctx FrameContext, ev DriverEvent) (DriverState, []Effect, View) {
	if _, ok := ev.(DEvHook); ok {
		return prev, []Effect{EffPushDriver{SessionID: "does-not-exist", Command: "stub"}}, View{}
	}
	return prev, nil, View{}
}

func init() {
	if _, exists := driverRegistry["bogussessionstub"]; !exists {
		Register(bogusSessionDriverStub{})
	}
}

func TestDriverHookEffPushDriverBogusSessionIDDropped(t *testing.T) {
	s := New()
	s.Now = time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	sid := SessionID("sess-bogus")
	frameID := FrameID("frame-bogus")
	s.Sessions = map[SessionID]Session{
		sid: {
			ID:      sid,
			Project: "/project",
			Command: "bogussessionstub",
			Driver:  pushStubState{},
			Frames: []SessionFrame{{
				ID:      frameID,
				Project: "/project",
				Command: "bogussessionstub",
				Driver:  pushStubState{},
			}},
		},
	}

	payload, _ := json.Marshal(map[string]string{"hook_event_name": "test"})
	_, effs := Reduce(s, EvDriverEvent{
		ConnID:    1,
		ReqID:     "r",
		Event:     "test",
		Timestamp: time.Now(),
		SenderID:  frameID,
		Payload:   json.RawMessage(payload),
	})

	// EffPushDriver with bogus SessionID should be dropped — no EffSpawnPaneWindow.
	if _, ok := findEff[EffSpawnPaneWindow](effs); ok {
		t.Error("expected EffSpawnPaneWindow to be absent (bogus SessionID should be dropped)")
	}
}

func TestDriverHookEffPushDriverIsResolved(t *testing.T) {
	s := New()
	s.Now = time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	sid := SessionID("sess-push")
	frameID := FrameID("frame-push")
	s.Sessions = map[SessionID]Session{
		sid: {
			ID:      sid,
			Project: "/project",
			Command: "pushstub",
			Driver:  pushStubState{},
			Frames: []SessionFrame{{
				ID:      frameID,
				Project: "/project",
				Command: "pushstub",
				Driver:  pushStubState{},
			}},
		},
	}

	payload, _ := json.Marshal(map[string]string{"hook_event_name": "test"})
	next, effs := Reduce(s, EvDriverEvent{
		ConnID:    1,
		ReqID:     "r",
		Event:     "test",
		Timestamp: time.Now(),
		SenderID:  frameID,
		Payload:   json.RawMessage(payload),
	})
	_ = next

	// EffPushDriver should have been resolved into EffSpawnPaneWindow.
	spawn, ok := findEff[EffSpawnPaneWindow](effs)
	if !ok {
		t.Fatal("expected EffSpawnPaneWindow from resolved EffPushDriver")
	}
	// Project should fall back to the parent session's project,
	// since the driver's EffPushDriver carries no project.
	if spawn.Project != "/project" {
		t.Errorf("spawn.Project = %q, want /project", spawn.Project)
	}
	// Session should now have 2 frames.
	sess := next.Sessions[sid]
	if len(sess.Frames) != 2 {
		t.Errorf("frame count = %d, want 2", len(sess.Frames))
	}
	if sess.Frames[1].Project != "/project" {
		t.Errorf("new frame project = %q, want /project", sess.Frames[1].Project)
	}
}

func TestSubsystemEventRoutesToDriver(t *testing.T) {
	s := New()
	sid := SessionID("sess-sub")
	frameID := FrameID("frame-sub")
	s.Sessions = map[SessionID]Session{
		sid: {
			ID:      sid,
			Project: "/project",
			Command: "subsystemstub",
			Driver:  subsystemStubState{},
			Frames: []SessionFrame{{
				ID:      frameID,
				Project: "/project",
				Command: "subsystemstub",
				Driver:  subsystemStubState{},
			}},
		},
	}

	next, effs := Reduce(s, EvSubsystem{
		ConnID:    1,
		ReqID:     "r1",
		FrameID:   frameID,
		Source:    SubsystemStream,
		Kind:      SubsystemMessageUpdated,
		Timestamp: time.Now(),
		Payload:   SubsystemPayload{TargetID: "thread-1"},
	})

	st, ok := next.Sessions[sid].Frames[0].Driver.(subsystemStubState)
	if !ok {
		t.Fatalf("driver state = %T", next.Sessions[sid].Frames[0].Driver)
	}
	if st.lastKind != SubsystemMessageUpdated {
		t.Fatalf("lastKind = %q", st.lastKind)
	}
	if st.lastSource != SubsystemStream {
		t.Fatalf("lastSource = %q", st.lastSource)
	}
	if got := next.Sessions[sid].Frames[0].TargetID; got != "thread-1" {
		t.Fatalf("TargetID = %q", got)
	}
	if _, ok := findEff[EffPersistSnapshot](effs); !ok {
		t.Fatal("expected EffPersistSnapshot")
	}
	if _, ok := findEff[EffBroadcastSessionsChanged](effs); !ok {
		t.Fatal("expected EffBroadcastSessionsChanged")
	}
	if _, ok := findEff[EffSendResponse](effs); !ok {
		t.Fatal("expected EffSendResponse")
	}
}

func TestSubsystemEventUnknownFrameReturnsError(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvSubsystem{
		ConnID:  1,
		ReqID:   "r1",
		FrameID: "ghost",
		Source:  SubsystemStream,
		Kind:    SubsystemMessageUpdated,
	})

	errEff, ok := findEff[EffSendError](effs)
	if !ok {
		t.Fatal("expected EffSendError")
	}
	if errEff.Code != string(ErrCodeNotFound) {
		t.Fatalf("error code = %q", errEff.Code)
	}
}

func TestSubsystemEventMissingFrameReturnsError(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvSubsystem{
		ConnID: 1,
		ReqID:  "r1",
		Source: SubsystemStream,
		Kind:   SubsystemMessageUpdated,
	})

	errEff, ok := findEff[EffSendError](effs)
	if !ok {
		t.Fatal("expected EffSendError")
	}
	if errEff.Code != string(ErrCodeInvalidArgument) {
		t.Fatalf("error code = %q", errEff.Code)
	}
}
