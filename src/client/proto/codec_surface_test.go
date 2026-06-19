package proto

import (
	"reflect"
	"testing"
)

func TestCodecSurfaceRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("commands", func(t *testing.T) {
		t.Parallel()
		cmds := []Command{
			CmdSurfaceSubscribe{SessionID: "s1"},
			CmdSurfaceUnsubscribe{SessionID: "s1"},
			CmdSurfaceResize{SessionID: "s1", Cols: 80, Rows: 24},
			CmdSurfaceWriteRaw{SessionID: "s1", Data: []byte{0x1b, 0x5b, 0x41}},
		}
		for _, c := range cmds {
			t.Run(c.CommandName(), func(t *testing.T) {
				t.Parallel()
				raw, err := EncodeCommand("req-1", c)
				if err != nil {
					t.Fatalf("EncodeCommand: %v", err)
				}
				env, err := DecodeEnvelope(raw)
				if err != nil {
					t.Fatalf("DecodeEnvelope: %v", err)
				}
				got, err := DecodeCommand(env)
				if err != nil {
					t.Fatalf("DecodeCommand: %v", err)
				}
				if !reflect.DeepEqual(got, c) {
					t.Errorf("round-trip mismatch: got %#v, want %#v", got, c)
				}
			})
		}
	})

	t.Run("events", func(t *testing.T) {
		t.Parallel()
		evts := []ServerEvent{
			EvtSurfaceOutput{
				SessionID: "s1",
				TimeSec:   1.5,
				DataB64:   "YWJj",
				Sequence:  42,
			},
			EvtPromptEvent{
				FrameID: "f1",
				Phase:   "start",
				NowRFC:  "2026-06-19T00:00:00Z",
			},
		}
		for _, e := range evts {
			t.Run(e.EventName(), func(t *testing.T) {
				t.Parallel()
				raw, err := EncodeEvent(e)
				if err != nil {
					t.Fatalf("EncodeEvent: %v", err)
				}
				env, err := DecodeEnvelope(raw)
				if err != nil {
					t.Fatalf("DecodeEnvelope: %v", err)
				}
				got, err := DecodeEvent(env)
				if err != nil {
					t.Fatalf("DecodeEvent: %v", err)
				}
				if !reflect.DeepEqual(got, e) {
					t.Errorf("round-trip mismatch: got %#v, want %#v", got, e)
				}
			})
		}
	})
}
