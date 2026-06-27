package proto

import (
	"testing"
)

// seedSurfaceCommands returns seed bytes for FuzzDecodeCommand.
// 4 Surface 系 + 3 既存コマンドの計 7 シード。
func seedSurfaceCommands() [][]byte {
	return [][]byte{
		// Surface 系 4 個
		[]byte(`{"type":"cmd","req_id":"r1","cmd":"surface.subscribe","data":{"session_id":"s1"}}`),
		[]byte(`{"type":"cmd","req_id":"r2","cmd":"surface.unsubscribe","data":{"session_id":"s1"}}`),
		[]byte(`{"type":"cmd","req_id":"r3","cmd":"surface.resize","data":{"session_id":"s1","cols":80,"rows":24}}`),
		[]byte(`{"type":"cmd","req_id":"r4","cmd":"surface.write_raw","data":{"session_id":"s1","data":"G1tB"}}`),
		// 既存コマンド 2 個
		[]byte(`{"type":"cmd","req_id":"r5","cmd":"subscribe","data":{"filters":["sessions-changed"]}}`),
		[]byte(`{"type":"cmd","req_id":"r7","cmd":"hook-event","data":{"token":"deadbeef","hook":"SessionStart","payload":{}}}`),
	}
}

// seedSurfaceEvents returns seed bytes for FuzzDecodeEvent.
// 2 Surface 系 + 1 既存イベントの計 3 シード。
func seedSurfaceEvents() [][]byte {
	return [][]byte{
		// Surface 系 2 個
		[]byte(`{"type":"evt","name":"surface-output","data":{"session_id":"s1","time_sec":1.5,"data_b64":"YWJj","sequence":1}}`),
		[]byte(`{"type":"evt","name":"prompt-event","data":{"frame_id":"f1","phase":"start","now_rfc":"2026-06-19T00:00:00Z"}}`),
		// 既存イベント 1 個
		[]byte(`{"type":"evt","name":"sessions-changed","data":{"sessions":[],"active_session_id":""}}`),
	}
}

// FuzzDecodeCommand fuzzes the Command decode path: envelope → DecodeCommand → EncodeCommand.
// Unknown or malformed inputs are expected to return errors, not panic.
func FuzzDecodeCommand(f *testing.F) {
	for _, s := range seedSurfaceCommands() {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		env, err := DecodeEnvelope(data)
		if err != nil {
			return
		}
		if env.Type != TypeCommand {
			return
		}
		c, err := DecodeCommand(env)
		if err != nil {
			return
		}
		if c == nil {
			t.Fatal("nil Command but no error")
		}
		// Round-trip: EncodeCommand must not panic.
		_, _ = EncodeCommand("r", c)
	})
}

// FuzzDecodeEvent fuzzes the Event decode path: envelope → DecodeEvent → EncodeEvent.
// Unknown or malformed inputs are expected to return errors, not panic.
func FuzzDecodeEvent(f *testing.F) {
	for _, s := range seedSurfaceEvents() {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		env, err := DecodeEnvelope(data)
		if err != nil {
			return
		}
		if env.Type != TypeEvent {
			return
		}
		e, err := DecodeEvent(env)
		if err != nil {
			return
		}
		if e == nil {
			t.Fatal("nil ServerEvent but no error")
		}
		// Round-trip: EncodeEvent must not panic.
		_, _ = EncodeEvent(e)
	})
}
