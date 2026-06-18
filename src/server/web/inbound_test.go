package web

import (
	"testing"

	"github.com/takezoh/agent-reactor/platform/termvt"
)

// fakeAttacher records the input/resize calls applyInbound makes, so a test can
// assert what an inbound frame did without a real pty session.
type fakeAttacher struct {
	input   []byte
	resizes [][2]int
}

func (f *fakeAttacher) Subscribe() (int, <-chan termvt.Event) { return 0, nil }
func (f *fakeAttacher) Unsubscribe(int)                       {}
func (f *fakeAttacher) WriteInput(b []byte)                   { f.input = append(f.input, b...) }
func (f *fakeAttacher) Resize(cols, rows int) error {
	f.resizes = append(f.resizes, [2]int{cols, rows})
	return nil
}

func TestApplyInbound(t *testing.T) {
	cases := []struct {
		name        string
		frame       string
		wantInput   string
		wantResizes [][2]int
		wantAction  bool
	}{
		{"input", `{"k":"i","d":"abc"}`, "abc", nil, true},
		{"resize", `{"k":"r","cols":120,"rows":40}`, "", [][2]int{{120, 40}}, true},
		{"resize zero ignored", `{"k":"r","cols":0,"rows":40}`, "", nil, false},
		{"resize negative ignored", `{"k":"r","cols":-5,"rows":-1}`, "", nil, false},
		{"unknown kind", `{"k":"x","d":"abc"}`, "", nil, false},
		{"malformed json", `{not json`, "", nil, false},
		{"empty", ``, "", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAttacher{}
			got := applyInbound(f, []byte(tc.frame))
			if got != tc.wantAction {
				t.Errorf("applyInbound action = %v, want %v", got, tc.wantAction)
			}
			if string(f.input) != tc.wantInput {
				t.Errorf("input = %q, want %q", f.input, tc.wantInput)
			}
			if len(f.resizes) != len(tc.wantResizes) {
				t.Fatalf("resizes = %v, want %v", f.resizes, tc.wantResizes)
			}
			for i, r := range tc.wantResizes {
				if f.resizes[i] != r {
					t.Errorf("resize[%d] = %v, want %v", i, f.resizes[i], r)
				}
			}
		})
	}
}

// FuzzInbound is the untrusted-input backstop: arbitrary client bytes must never
// panic the reader, and applyInbound must never forward a non-positive terminal
// size regardless of input. (The absolute UPPER bound that keeps the pty/VT grid
// safe from uint16 overflow and OOM is enforced and tested downstream — see
// termvt.normalizeSize / TestNormalizeSizeClamp.) The /ws data plane carries
// client-controlled frames, so this is the server-side analogue of the client's
// defensive JSON.parse guard.
func FuzzInbound(f *testing.F) {
	for _, seed := range []string{
		`{"k":"i","d":"hello"}`,
		`{"k":"r","cols":80,"rows":24}`,
		`{"k":"r","cols":-1,"rows":0}`,
		`{"k":"r","cols":2147483647,"rows":2147483647}`,
		`{"k":"x"}`,
		`{"k":"i","d":"\u0000\uffff"}`,
		`not json`,
		``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		fake := &fakeAttacher{}
		applyInbound(fake, data) // must not panic on any bytes
		for _, r := range fake.resizes {
			if r[0] <= 0 || r[1] <= 0 {
				t.Fatalf("Resize requested with non-positive dims %dx%d from %q", r[0], r[1], data)
			}
		}
	})
}
