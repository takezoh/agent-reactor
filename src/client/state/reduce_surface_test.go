package state

import (
	"bytes"
	"testing"
)

func TestReduceSurfaceReadTextNotFound(t *testing.T) {
	s := New()
	ev := EvCmdSurfaceReadText{ConnID: 1, ReqID: "r1", SessionID: "no-such", Lines: 10}
	_, effs := Reduce(s, ev)
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendError)
	if !ok {
		t.Fatalf("effect = %T, want EffSendError", effs[0])
	}
	if e.Code != "not_found" {
		t.Errorf("code = %q, want not_found", e.Code)
	}
}

func TestReduceSurfaceReadTextFound(t *testing.T) {
	s := New()
	s.Sessions["sess-1"] = Session{ID: "sess-1"}
	ev := EvCmdSurfaceReadText{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Lines: 20}
	_, effs := Reduce(s, ev)
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendResponseSync)
	if !ok {
		t.Fatalf("effect = %T, want EffSendResponseSync", effs[0])
	}
	reply, ok := e.Body.(SurfaceReadTextReply)
	if !ok {
		t.Fatalf("body = %T, want SurfaceReadTextReply", e.Body)
	}
	if reply.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", reply.SessionID)
	}
	if reply.Lines != 20 {
		t.Errorf("Lines = %d, want 20", reply.Lines)
	}
}

func TestReduceSurfaceReadTextDefaultLines(t *testing.T) {
	s := New()
	s.Sessions["sess-1"] = Session{ID: "sess-1"}
	ev := EvCmdSurfaceReadText{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Lines: 0}
	_, effs := Reduce(s, ev)
	reply := effs[0].(EffSendResponseSync).Body.(SurfaceReadTextReply)
	if reply.Lines != 30 {
		t.Errorf("default Lines = %d, want 30", reply.Lines)
	}
}

func TestReduceSurfaceSendText(t *testing.T) {
	s := New()
	s.Sessions["sess-1"] = Session{ID: "sess-1"}
	ev := EvCmdSurfaceSendText{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Text: "hello"}
	_, effs := Reduce(s, ev)
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendPaneKeys)
	if !ok {
		t.Fatalf("effect = %T, want EffSendPaneKeys", effs[0])
	}
	if !e.WithEnter {
		t.Error("WithEnter should be true for send_text")
	}
	if e.Text != "hello" {
		t.Errorf("Text = %q, want hello", e.Text)
	}
}

func TestReduceSurfaceSendKey(t *testing.T) {
	s := New()
	s.Sessions["sess-1"] = Session{ID: "sess-1"}
	ev := EvCmdSurfaceSendKey{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Key: "Escape"}
	_, effs := Reduce(s, ev)
	e, ok := effs[0].(EffSendPaneKeys)
	if !ok {
		t.Fatalf("effect = %T, want EffSendPaneKeys", effs[0])
	}
	if e.WithEnter {
		t.Error("WithEnter should be false for send_key")
	}
	if e.Key != "Escape" {
		t.Errorf("Key = %q, want Escape", e.Key)
	}
}

func TestReduceDriverList(t *testing.T) {
	s := New()
	ev := EvCmdDriverList{ConnID: 1, ReqID: "r1"}
	_, effs := Reduce(s, ev)
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendResponseSync)
	if !ok {
		t.Fatalf("effect = %T, want EffSendResponseSync", effs[0])
	}
	if _, ok := e.Body.(DriverListReply); !ok {
		t.Fatalf("body = %T, want DriverListReply", e.Body)
	}
}

// newStateWithFramedSession returns a New() state that has "sess-1" with one frame,
// satisfying the activeFrame() check.
func newStateWithFramedSession() State {
	s := New()
	s.Sessions["sess-1"] = Session{
		ID:            "sess-1",
		Frames:        []SessionFrame{{ID: "f1"}},
		ActiveFrameID: "f1",
	}
	return s
}

func TestReduceSurfaceSubscribeOK(t *testing.T) {
	s := newStateWithFramedSession()
	next, effs := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})
	if len(effs) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effs))
	}
	start, ok := effs[0].(EffSurfaceSubscribeStart)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSurfaceSubscribeStart", effs[0])
	}
	if start.ConnID != 1 || start.SessionID != "sess-1" {
		t.Errorf("EffSurfaceSubscribeStart = %+v, want {ConnID:1, SessionID:sess-1}", start)
	}
	if _, ok := effs[1].(EffSendResponse); !ok {
		t.Fatalf("effs[1] = %T, want EffSendResponse", effs[1])
	}
	if _, ok := next.SurfaceSubs[1]["sess-1"]; !ok {
		t.Error("SurfaceSubs[1][sess-1] should be present after subscribe")
	}
}

func TestReduceSurfaceSubscribeIdempotent(t *testing.T) {
	s := newStateWithFramedSession()
	s1, _ := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})
	// Second subscribe — must not emit EffSurfaceSubscribeStart again.
	_, effs := Reduce(s1, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r2", SessionID: "sess-1"})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect on idempotent subscribe, got %d", len(effs))
	}
	if _, ok := effs[0].(EffSendResponse); !ok {
		t.Fatalf("effs[0] = %T, want EffSendResponse (okResp only)", effs[0])
	}
}

func TestReduceSurfaceSubscribeNoFrame(t *testing.T) {
	s := New()
	// Session without frames — activeFrame returns false.
	s.Sessions["sess-1"] = Session{ID: "sess-1"}
	orig := s

	next, effs := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendError)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSendError", effs[0])
	}
	if e.Code != ErrCodeFrameNotReady {
		t.Errorf("code = %q, want %q", e.Code, ErrCodeFrameNotReady)
	}
	// Purity: state unchanged.
	if len(next.SurfaceSubs) != len(orig.SurfaceSubs) {
		t.Errorf("SurfaceSubs mutated: got len=%d, want %d", len(next.SurfaceSubs), len(orig.SurfaceSubs))
	}
	if _, present := next.SurfaceSubs[1]; present {
		t.Error("SurfaceSubs[1] should not exist after error path")
	}
}

func TestReduceSurfaceSubscribeNotFound(t *testing.T) {
	s := New()
	_, effs := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "no-such"})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendError)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSendError", effs[0])
	}
	if e.Code != ErrCodeNotFound {
		t.Errorf("code = %q, want %q", e.Code, ErrCodeNotFound)
	}
}

func TestReduceSurfaceSubscribeResourceExhausted(t *testing.T) {
	s := New()
	// Build 8 subscriptions for ConnID 1.
	inner := map[SessionID]struct{}{}
	for i := 1; i <= 8; i++ {
		sid := SessionID("s" + string(rune('0'+i)))
		inner[sid] = struct{}{}
		s.Sessions[sid] = Session{
			ID:            sid,
			Frames:        []SessionFrame{{ID: "f1"}},
			ActiveFrameID: "f1",
		}
	}
	s.SurfaceSubs[1] = inner
	// 9th new session.
	s.Sessions["s9"] = Session{
		ID:            "s9",
		Frames:        []SessionFrame{{ID: "f1"}},
		ActiveFrameID: "f1",
	}

	_, effs := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "s9"})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendError)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSendError", effs[0])
	}
	if e.Code != ErrCodeResourceExhausted {
		t.Errorf("code = %q, want %q", e.Code, ErrCodeResourceExhausted)
	}

	// Re-subscribing one of the existing 8 must succeed (idempotent, cap exempt).
	_, effs2 := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r2", SessionID: "s1"})
	if len(effs2) != 1 {
		t.Fatalf("re-subscribe of existing: expected 1 effect, got %d", len(effs2))
	}
	if _, ok := effs2[0].(EffSendResponse); !ok {
		t.Fatalf("re-subscribe of existing: effs[0] = %T, want EffSendResponse", effs2[0])
	}
}

func TestReduceSurfaceUnsubscribeOK(t *testing.T) {
	s := newStateWithFramedSession()
	s1, _ := Reduce(s, EvCmdSurfaceSubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})

	next, effs := Reduce(s1, EvCmdSurfaceUnsubscribe{ConnID: 1, ReqID: "r2", SessionID: "sess-1"})
	if len(effs) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effs))
	}
	if _, ok := effs[0].(EffSurfaceSubscribeStop); !ok {
		t.Fatalf("effs[0] = %T, want EffSurfaceSubscribeStop", effs[0])
	}
	if _, ok := effs[1].(EffSendResponse); !ok {
		t.Fatalf("effs[1] = %T, want EffSendResponse", effs[1])
	}
	// When last entry removed, outer key should be deleted.
	if _, present := next.SurfaceSubs[1]; present {
		t.Error("SurfaceSubs[1] should be deleted after last unsubscribe")
	}
}

func TestReduceSurfaceUnsubscribeIdempotent(t *testing.T) {
	s := New()
	// No prior subscription.
	next, effs := Reduce(s, EvCmdSurfaceUnsubscribe{ConnID: 1, ReqID: "r1", SessionID: "sess-1"})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	if _, ok := effs[0].(EffSendResponse); !ok {
		t.Fatalf("effs[0] = %T, want EffSendResponse", effs[0])
	}
	if len(next.SurfaceSubs) != 0 {
		t.Error("SurfaceSubs should remain empty on idempotent unsubscribe")
	}
}

func TestReduceSurfaceResize(t *testing.T) {
	s := newStateWithFramedSession()
	_, effs := Reduce(s, EvCmdSurfaceResize{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Cols: 80, Rows: 24})
	if len(effs) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effs))
	}
	resize, ok := effs[0].(EffSurfaceResize)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSurfaceResize", effs[0])
	}
	if resize.SessionID != "sess-1" || resize.Cols != 80 || resize.Rows != 24 {
		t.Errorf("EffSurfaceResize = %+v, want {sess-1, 80, 24}", resize)
	}
	if _, ok := effs[1].(EffSendResponse); !ok {
		t.Fatalf("effs[1] = %T, want EffSendResponse", effs[1])
	}
}

func TestReduceSurfaceResizeNotFound(t *testing.T) {
	s := New()
	orig := s
	next, effs := Reduce(s, EvCmdSurfaceResize{ConnID: 1, ReqID: "r1", SessionID: "no-such", Cols: 80, Rows: 24})
	if len(effs) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effs))
	}
	e, ok := effs[0].(EffSendError)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSendError", effs[0])
	}
	if e.Code != ErrCodeNotFound {
		t.Errorf("code = %q, want %q", e.Code, ErrCodeNotFound)
	}
	// Purity: SurfaceSubs not mutated.
	if len(next.SurfaceSubs) != len(orig.SurfaceSubs) {
		t.Errorf("SurfaceSubs mutated: got len=%d, want %d", len(next.SurfaceSubs), len(orig.SurfaceSubs))
	}
}

func TestReduceSurfaceWriteRaw(t *testing.T) {
	s := newStateWithFramedSession()
	data := []byte{0x1b, 0x5b, 0x41}
	_, effs := Reduce(s, EvCmdSurfaceWriteRaw{ConnID: 1, ReqID: "r1", SessionID: "sess-1", Data: data})
	if len(effs) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effs))
	}
	wr, ok := effs[0].(EffSurfaceWriteRaw)
	if !ok {
		t.Fatalf("effs[0] = %T, want EffSurfaceWriteRaw", effs[0])
	}
	if wr.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", wr.SessionID)
	}
	if !bytes.Equal(wr.Data, data) {
		t.Errorf("Data = %v, want %v", wr.Data, data)
	}
	if _, ok := effs[1].(EffSendResponse); !ok {
		t.Fatalf("effs[1] = %T, want EffSendResponse", effs[1])
	}
}

func TestReduceConnClosedClearsSurfaceSubs(t *testing.T) {
	s := New()
	s.SurfaceSubs[1] = map[SessionID]struct{}{
		"s1": {},
		"s2": {},
	}

	next, effs := Reduce(s, EvConnClosed{ConnID: 1})
	if _, present := next.SurfaceSubs[1]; present {
		t.Error("SurfaceSubs[1] should be removed after EvConnClosed")
	}
	// Collect stop effects as a set (order non-deterministic).
	gotStops := map[SessionID]struct{}{}
	for _, eff := range effs {
		if stop, ok := eff.(EffSurfaceSubscribeStop); ok {
			gotStops[stop.SessionID] = struct{}{}
		}
	}
	for _, sid := range []SessionID{"s1", "s2"} {
		if _, ok := gotStops[sid]; !ok {
			t.Errorf("missing EffSurfaceSubscribeStop for SessionID %q", sid)
		}
	}
}

func TestReduceConnClosedNoSurface(t *testing.T) {
	s := New()
	// No Subscribers, no SurfaceSubs — existing behaviour: return nil effs.
	_, effs := Reduce(s, EvConnClosed{ConnID: 99})
	if len(effs) != 0 {
		t.Errorf("expected 0 effects, got %d", len(effs))
	}
}
