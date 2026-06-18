package stream

// FuzzStreamRouting explores interleavings of frame binds, thread starts, active
// switches, and message deltas, then checks routing invariants.
//
// Two tiers of invariant:
//
//   - Structural (always on): a thread's marker is delivered to at most one
//     frame, and only ever to a real frame. This holds even on the current
//     (buggy) demux, so the seed corpus runs as a normal regression guard in CI
//     under `go test` and `-race`, catching duplication, garbage-frame routing,
//     panics, and deadlocks.
//   - Isolation (gated by REACTOR_ROUTING_PINS): the marker is delivered to its
//     declared owner frame. This is RED on the current demux (ambiguous-cwd
//     starts fall back to activeLookup); the fix flips it GREEN and the gate is
//     removed so the fuzzer guards cross-talk regressions permanently.

import (
	"os"
	"strconv"
	"testing"

	"github.com/takezoh/agent-reactor/client/state"
)

func FuzzStreamRouting(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00})
	f.Add([]byte{0x01, 0x01, 0x01})
	f.Add([]byte{0xff, 0x00, 0xaa})
	f.Add([]byte{0x02, 0x01, 0x02, 0x00, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		const n = 3
		if len(data) < n {
			return
		}
		h := newInProc(t)
		frames := []state.FrameID{"F0", "F1", "F2"}
		threads := []string{"t0", "t1", "t2"}
		markers := []string{"MK0", "MK1", "MK2"}
		cwds := make([]string, n)

		// Assign cwds from the input so collisions (the shared-container case)
		// arise; register each frame as an unbound cold start.
		for i := 0; i < n; i++ {
			cwds[i] = "/c" + strconv.Itoa(int(data[i])%2)
			h.bindCold(frames[i], cwds[i])
		}

		// Drive each frame's thread.started + message, with the active frame
		// chosen by the input — this is what the ambiguous fallback keys on.
		for i := 0; i < n; i++ {
			h.setActive(frames[int(data[i])%n])
			h.started(threads[i], cwds[i]) // ground-truth owner of threads[i] is frames[i]
			h.message(threads[i], markers[i])
		}

		frameset := map[state.FrameID]bool{"F0": true, "F1": true, "F2": true}
		pins := os.Getenv("REACTOR_ROUTING_PINS") != ""
		for i := 0; i < n; i++ {
			got := h.rt.framesWithMarker(markers[i])

			// Structural invariant — must hold regardless of the demux bug.
			if len(got) > 1 {
				t.Fatalf("marker %s duplicated across frames %v", markers[i], got)
			}
			for _, g := range got {
				if !frameset[g] {
					t.Fatalf("marker %s routed to non-frame %q", markers[i], g)
				}
			}

			// Isolation invariant — gated until the demux fix lands.
			if pins && (len(got) != 1 || got[0] != frames[i]) {
				t.Errorf("isolation: marker %s -> %v, want [%s]", markers[i], got, frames[i])
			}
		}
	})
}
