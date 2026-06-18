package stream

// FuzzStreamRouting explores interleavings of message delivery and frame release
// across several same-cwd frames bound to distinct thread ids, and checks that
// routing stays by exact thread id: a marker reaches at most one frame, only its
// owner, or nobody (after release). It guards the by-id MESSAGE-ROUTING invariant
// (a reintroduced active-frame steal in frameForThread fails these assertions)
// and catches duplication, garbage-frame routing, panics, and deadlocks. The
// binding path itself — handleThreadStarted dropping unknown threads — is pinned
// by the TestStreamRoutingContract/thread_started_* cases. Always-on (no gate),
// run under the CI fuzz job.

import (
	"testing"

	"github.com/takezoh/agent-reactor/client/state"
)

func FuzzStreamRouting(f *testing.F) {
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{2, 2, 1, 0})
	f.Add([]byte{0xff, 0x00, 0xaa})
	f.Add([]byte{3, 4, 5, 1, 2, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		const n = 3
		if len(data) < n {
			return
		}
		h := newInProc(t)
		frames := []state.FrameID{"F0", "F1", "F2"}
		threads := []string{"t0", "t1", "t2"}
		markers := []string{"MK0", "MK1", "MK2"}
		// All three frames share a cwd but are bound to distinct thread ids — the
		// shared-container case the fix makes safe by construction.
		for i := 0; i < n; i++ {
			h.bind(frames[i], threads[i], "/work")
		}

		// Drive messages and releases in an input-derived order.
		for _, bcmd := range data {
			i := int(bcmd) % n
			if (int(bcmd)/n)%2 == 0 {
				h.message(threads[i], markers[i])
			} else {
				h.release(frames[i])
			}
		}

		frameset := map[state.FrameID]bool{"F0": true, "F1": true, "F2": true}
		for i := 0; i < n; i++ {
			got := h.rt.framesWithMarker(markers[i])
			if len(got) > 1 {
				t.Fatalf("marker %s duplicated across frames %v", markers[i], got)
			}
			for _, g := range got {
				if !frameset[g] {
					t.Fatalf("marker %s routed to non-frame %q", markers[i], g)
				}
				if g != frames[i] {
					t.Errorf("isolation: marker %s -> %q, want %s", markers[i], g, frames[i])
				}
			}
		}
	})
}
