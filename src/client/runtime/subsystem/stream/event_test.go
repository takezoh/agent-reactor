package stream

import (
	"encoding/json"
	"testing"

	"github.com/takezoh/agent-roost/client/state"
)

func newTestBackend() (*Backend, *fakeRuntime) {
	fr := &fakeRuntime{}
	b := New(fr, "sid", "/p", "codex", nil, "", false, false, "/sock", "/csock", 0,
		func() state.FrameID { return "" }, 0)
	return b, fr
}

func TestHandleThreadStarted(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", startDir: "/work"}
	b.mu.Unlock()

	raw := json.RawMessage(`{"threadId":"t1","cwd":"/work"}`)
	b.handleThreadStarted(raw)

	b.mu.Lock()
	bound := b.frames["f1"]
	threadFrame := b.threads["t1"]
	b.mu.Unlock()
	if bound.threadID != "t1" || bound.resumePhase != resumePhaseAttached {
		t.Errorf("binding not updated: %+v", bound)
	}
	if threadFrame != "f1" {
		t.Errorf("thread map: %q", threadFrame)
	}
	if len(fr.events) == 0 {
		t.Errorf("expected emitted event")
	}
}

func TestHandleThreadStartedNoMatch(t *testing.T) {
	b, fr := newTestBackend()
	b.handleThreadStarted([]byte(`{"threadId":"t1","cwd":"/none"}`))
	if len(fr.events) != 0 {
		t.Errorf("expected no events, got %d", len(fr.events))
	}
}

func TestHandleTurnCompleted(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleTurnCompleted([]byte(`{"threadId":"t1","text":"hello"}`))
	if len(fr.events) == 0 {
		t.Errorf("expected event")
	}
	b.mu.Lock()
	last := b.frames["f1"].lastAssistant
	b.mu.Unlock()
	if last != "hello" {
		t.Errorf("lastAssistant = %q", last)
	}
}

func TestHandleTurnCompletedUnknownThread(t *testing.T) {
	b, fr := newTestBackend()
	b.handleTurnCompleted([]byte(`{"threadId":"unknown"}`))
	if len(fr.events) != 0 {
		t.Errorf("expected no events")
	}
}

func TestHandleAgentMessageDelta(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"abc"}`))
	b.handleAgentMessageDelta([]byte(`{"threadId":"t1","delta":"def"}`))
	b.mu.Lock()
	last := b.frames["f1"].lastAssistant
	b.mu.Unlock()
	if last != "abcdef" {
		t.Errorf("lastAssistant = %q", last)
	}
	if len(fr.events) != 2 {
		t.Errorf("expected 2 events, got %d", len(fr.events))
	}
}

func TestHandleAgentMessageDeltaIgnored(t *testing.T) {
	b, fr := newTestBackend()
	b.handleAgentMessageDelta([]byte(`bad`))           // bad json
	b.handleAgentMessageDelta([]byte(`{}`))            // no text
	b.handleAgentMessageDelta([]byte(`{"delta":"x"}`)) // no thread match
	if len(fr.events) != 0 {
		t.Errorf("expected no events, got %d", len(fr.events))
	}
}

func TestHandleNotificationUnknownMethodIsNoop(t *testing.T) {
	b, fr := newTestBackend()
	b.handleNotification("unknown/method", []byte(`{}`))
	if len(fr.events) != 0 {
		t.Errorf("unknown method should emit nothing, got %d events", len(fr.events))
	}
}

func TestHandleNotificationRoutesToHandlers(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()

	for _, method := range []string{"turn/started", "turn/plan/updated", "turn/diff/updated"} {
		b.handleNotification(method, []byte(`{"threadId":"t1"}`))
	}
	if len(fr.events) != 3 {
		t.Errorf("expected 3 events from known methods, got %d", len(fr.events))
	}
}

func TestResolveFrameForStartedThreadCandidates(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", startDir: "/work"}
	b.mu.Unlock()
	if got := b.resolveFrameForStartedThread("", "/work"); got != "" {
		t.Errorf("empty threadID should return empty: %q", got)
	}
	if got := b.resolveFrameForStartedThread("t1", "/work"); got != "f1" {
		t.Errorf("got %q, want f1", got)
	}
}

func TestResolveFrameAlreadyBound(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1", threadID: "t1"}
	b.threads["t1"] = "f1"
	b.mu.Unlock()
	if got := b.resolveFrameForStartedThread("t1", ""); got != "f1" {
		t.Errorf("already bound: got %q", got)
	}
}

func TestFailFrame(t *testing.T) {
	b, fr := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{frameID: "f1"}
	b.mu.Unlock()
	b.failFrame("f1", nil)
	if len(fr.events) != 1 {
		t.Errorf("expected 1 event, got %d", len(fr.events))
	}
	// duplicate suppressed
	b.failFrame("f1", nil)
	if len(fr.events) != 1 {
		t.Errorf("duplicate failFrame should be suppressed, got %d", len(fr.events))
	}
	// unknown frame is no-op
	b.failFrame("unknown", nil)
	if len(fr.events) != 1 {
		t.Errorf("unknown frame: got %d events", len(fr.events))
	}
}

func TestEmitToThreadUnknown(t *testing.T) {
	b, fr := newTestBackend()
	b.emitToThread("unknown", state.SubsystemTurnStarted, nil)
	if len(fr.events) != 0 {
		t.Errorf("unknown thread should emit nothing")
	}
}

func TestPayloadFromBinding(t *testing.T) {
	b, _ := newTestBackend()
	b.mu.Lock()
	b.frames["f1"] = &frameBinding{
		frameID:     "f1",
		threadID:    "t1",
		requestedID: "req",
		observedID:  "obs",
		resumePhase: "phase",
	}
	b.mu.Unlock()
	p := b.payload("f1")
	if p.SessionID != "t1" || p.RequestedTargetID != "req" || p.ResumePhase != "phase" {
		t.Errorf("payload = %+v", p)
	}
	// Unknown frame: empty payload
	pe := b.payload("missing")
	if pe.SessionID != "" {
		t.Errorf("missing frame should produce empty payload: %+v", pe)
	}
}
